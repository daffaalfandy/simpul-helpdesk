package tickets

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/projects"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// Audit actions for a ticket's Definition of Done. Ticking and clearing items are recorded but
// stay out of the activity log and notifications: the status change they come with says it, and
// a commit reaching the branch already shows as code activity.
const (
	ActionDoDChanged      = "ticket.dod_changed"
	ActionDoDItemsDone    = "ticket.dod_items_done"
	ActionDoDItemsCleared = "ticket.dod_items_cleared"
)

// ActorNameCodeBranch names the system in audit entries when a commit reaching the branch ticks
// the Definition of Done.
const ActorNameCodeBranch = "code_branch"

const (
	dataDoD       = "dod"
	dataDoDSource = "dod_source"
	dataItems     = "items"
)

// TicketDoD is a ticket's Definition of Done as the agent app shows it.
type TicketDoD struct {
	Source         dod.Source
	Config         dod.Config // the one the ticket follows
	ProjectConfig  dod.Config // the project's, for "Project default"
	OverrideReason *string
	Items          []DoDItem // the checklist, in order
	Met            bool
	Pending        []dod.Item // required items not met yet
	// Mandatory is the project's switch: resolving with items pending needs a reason. Off, the
	// Definition of Done is guidance only.
	Mandatory bool
	// HasRepository: the project reads commits from a repository. Without one, code on a branch
	// doesn't apply (dod.Config.ForRepository) and is left off Items and Pending.
	HasRepository bool
	CanOverride   bool // the viewer may set or reset the ticket's own Definition of Done
}

// DoDItem is one checklist line: done once DoneAt is set. DoneBy is nil when the system or Git
// met it; Commit is the commit that met code on the branch.
type DoDItem struct {
	Item   dod.Item
	DoneAt *time.Time
	DoneBy *Ref
	Commit *DoDCommit
}

// DoDCommit is the commit that met a code item.
type DoDCommit struct {
	SHA string // short
	URL string // empty when the provider sent none
}

// storedDoD is a ticket's Definition of Done as its row holds it.
type storedDoD struct {
	project  dod.Config
	override *dod.Config
}

func parseDoD(projectJSON, overrideJSON []byte) (storedDoD, error) {
	project, err := dod.Parse(projectJSON)
	if err != nil {
		return storedDoD{}, err
	}
	sd := storedDoD{project: project}
	if overrideJSON != nil {
		o, err := dod.Parse(overrideJSON)
		if err != nil {
			return storedDoD{}, err
		}
		sd.override = &o
	}
	return sd, nil
}

func (sd storedDoD) effective() (dod.Config, dod.Source) {
	return dod.Effective(sd.project, sd.override)
}

// evaluated is the Definition of Done the ticket is held to: the effective one, without code on
// a branch while the project has no repository.
func (sd storedDoD) evaluated(hasRepository bool) dod.Config {
	eff, _ := sd.effective()
	return eff.ForRepository(hasRepository)
}

// doneSet turns stored item names into the set dod.Evaluate reads.
func doneSet(items []string) map[dod.Item]bool {
	out := make(map[dod.Item]bool, len(items))
	for _, it := range items {
		out[dod.Item(it)] = true
	}
	return out
}

// pendingFor lists what an open ticket still needs before it counts as done.
func pendingFor(sd storedDoD, hasRepository bool, done []string) []dod.Item {
	_, pending := dod.Evaluate(sd.evaluated(hasRepository), doneSet(done))
	if pending == nil {
		pending = []dod.Item{}
	}
	return pending
}

// dodView is what the agent app's view of a ticket's Definition of Done reads besides the
// stored configuration.
type dodView struct {
	reason        *string // why the ticket has its own
	mandatory     bool
	hasRepository bool
	canOverride   bool
}

// ticketDoD builds the agent app's view of the ticket's Definition of Done.
func ticketDoD(sd storedDoD, v dodView, rows []db.ListTicketDoDItemsRow) TicketDoD {
	eff, src := sd.effective()
	held := sd.evaluated(v.hasRepository)
	byItem := map[dod.Item]db.ListTicketDoDItemsRow{}
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		byItem[dod.Item(r.Item)] = r
		names = append(names, r.Item)
	}
	met, pending := dod.Evaluate(held, doneSet(names))
	if pending == nil {
		pending = []dod.Item{}
	}
	t := TicketDoD{
		Source: src, Config: eff, ProjectConfig: sd.project, Items: []DoDItem{}, Met: met, Pending: pending,
		Mandatory: v.mandatory, HasRepository: v.hasRepository, CanOverride: v.canOverride,
	}
	if src == dod.SourceTicket {
		t.OverrideReason = v.reason
	}
	for _, it := range dod.Required(held) {
		item := DoDItem{Item: it}
		if r, ok := byItem[it]; ok {
			at := r.DoneAt
			item.DoneAt = &at
			item.DoneBy = optionalRef(r.DoneBy, r.DoneByName)
			if r.CommitSha != nil {
				item.Commit = &DoDCommit{SHA: shortSHA(*r.CommitSha), URL: deref(r.CommitUrl)}
			}
		}
		t.Items = append(t.Items, item)
	}
	return t
}

// canOverrideDoD: agents may set a ticket's own Definition of Done when its project allows it;
// supervisors and admins always may (HANDOFF §3).
func canOverrideDoD(p *auth.Principal, projectAllows bool) bool {
	return projectAllows || auth.Can(p.Role, auth.CapProjectsManage)
}

// SetDoD gives the ticket a Definition of Done of its own, with a reason for the audit log, or
// returns it to the project's with nil. Whether it is mandatory stays the project's. When the
// change asks for code on a branch it didn't ask for before, the commits already linked to the
// ticket are checked against it at once.
func (s *Service) SetDoD(ctx context.Context, p *auth.Principal, key string, cfg *dod.Config, reason string) (Ticket, error) {
	reason = strings.TrimSpace(reason)
	if cfg != nil {
		if reason == "" || len([]rune(reason)) > MaxReasonLen {
			return Ticket{}, apperr.Invalid(apperr.CodeReasonRequired, "Give a reason", "")
		}
		normalized := dod.Normalize(*cfg)
		cfg = &normalized
	}
	err := s.agentAction(ctx, p, key, func(tx store.Tx, row db.GetTicketForUpdateRow, cs *changeSet) error {
		if !canOverrideDoD(p, row.AllowTicketOverride) {
			return errDoDOverrideNotAllowed()
		}
		if cfg != nil {
			if err := dod.Validate(*cfg, row.RepositoryID != nil); err != nil {
				return projects.DoDError(err)
			}
		}
		sd, err := parseDoD(row.DodConfig, row.DodOverride)
		if err != nil {
			return err
		}
		switch {
		case cfg == nil && sd.override == nil:
			return nil // already the project's
		case cfg != nil && sd.override != nil && *sd.override == *cfg &&
			row.DodOverrideReason != nil && *row.DodOverrideReason == reason:
			return nil // saved again unchanged
		}
		next := storedDoD{project: sd.project, override: cfg}
		if cfg == nil {
			if _, err := tx.Q.DeleteTicketDoDOverride(ctx, row.ID); err != nil {
				return err
			}
		} else if err := tx.Q.UpsertTicketDoDOverride(ctx, db.UpsertTicketDoDOverrideParams{
			TicketID: row.ID, Config: cfg.JSON(), Reason: reason, SetBy: p.UserID,
		}); err != nil {
			return err
		}
		prevCfg, prevSrc := sd.effective()
		nextCfg, nextSrc := next.effective()
		e := p.AuditActor().Entry(ActionDoDChanged, TargetTicket, &row.ID, map[string]any{
			dataDoD:       audit.Change{From: prevCfg, To: nextCfg},
			dataDoDSource: audit.Change{From: prevSrc, To: nextSrc},
		})
		e.Reason = reason
		if err := cs.write(ctx, tx.Q, e); err != nil {
			return err
		}
		if row.RepositoryID == nil || !dod.BranchChanged(prevCfg, nextCfg) {
			return nil
		}
		return tickLinkedCommit(ctx, tx.Q, cs, row.OrganizationID, *row.RepositoryID, nextCfg)
	})
	if err != nil {
		return Ticket{}, err
	}
	return s.Get(ctx, p, key)
}

// clearItems forgets Definition of Done items a status change clears (dod.Cleared).
func clearItems(ctx context.Context, q *db.Queries, cs *changeSet, actor audit.Actor, clear []dod.Item) error {
	if len(clear) == 0 {
		return nil
	}
	items := make([]string, len(clear))
	for i, it := range clear {
		items[i] = string(it)
	}
	removed, err := q.DeleteTicketDoDItems(ctx, db.DeleteTicketDoDItemsParams{TicketID: cs.ticketID, Items: items})
	if err != nil || len(removed) == 0 {
		return err
	}
	return cs.write(ctx, q, actor.Entry(ActionDoDItemsCleared, TargetTicket, &cs.ticketID, map[string]any{dataItems: removed}))
}

// checkResolve decides how a hand move to resolved is recorded: a plain move when the
// Definition of Done is met or only guidance (not mandatory, owner 18 Nov: no reason, no
// dod_skipped, nobody alerted), a skip with the reason when a mandatory one isn't met, and a
// refusal without one.
func checkResolve(ctx context.Context, q *db.Queries, row db.GetTicketForUpdateRow, reason string) (ticket.Cause, string, error) {
	sd, err := parseDoD(row.DodConfig, row.DodOverride)
	if err != nil {
		return "", "", err
	}
	rows, err := q.ListTicketDoDItems(ctx, row.ID)
	if err != nil {
		return "", "", err
	}
	done := make([]string, len(rows))
	for i, r := range rows {
		done[i] = r.Item
	}
	if !dod.ResolveNeedsReason(row.DodMandatory, pendingFor(sd, row.RepositoryID != nil, done)) {
		return "", "", nil
	}
	if reason == "" {
		return "", "", errDoDUnmet()
	}
	return ticket.CauseDoDSkipped, reason, nil
}

// tickLinkedCommit meets code on the branch eff names with a commit already linked to the
// ticket from repoID that has reached that branch, if there is one.
func tickLinkedCommit(ctx context.Context, q *db.Queries, cs *changeSet, orgID, repoID uuid.UUID, eff dod.Config) error {
	if !eff.CodeOnBranch.Enabled {
		return nil
	}
	eventID, err := q.FindLinkedCommitOnBranch(ctx, db.FindLinkedCommitOnBranchParams{
		TicketID: &cs.ticketID, LinkedOutcome: string(CodeLinked), RepositoryID: repoID, Branch: eff.CodeOnBranch.Branch,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return tickCodeItem(ctx, q, cs, orgID, eventID)
}

// tickCodeItem meets code on the branch with commit eventID. Git did it, so the audit entry is
// the system's and the item has no person; the ticket's status never changes. An item already
// met keeps its first commit and writes nothing.
func tickCodeItem(ctx context.Context, q *db.Queries, cs *changeSet, orgID, eventID uuid.UUID) error {
	added, err := q.InsertTicketCodeItem(ctx, db.InsertTicketCodeItemParams{
		TicketID: cs.ticketID, Item: string(dod.ItemCodeOnBranch), CodeEventID: &eventID,
	})
	if err != nil || added == 0 {
		return err
	}
	actor := audit.SystemActor(ActorNameCodeBranch).In(orgID)
	return cs.write(ctx, q, actor.Entry(ActionDoDItemsDone, TargetTicket, &cs.ticketID, map[string]any{
		dataItems: []dod.Item{dod.ItemCodeOnBranch}, dataCodeEvent: eventID,
	}))
}

func errDoDUnmet() error {
	return apperr.Conflict(apperr.CodeDoDUnmet, "This ticket hasn't met its Definition of Done",
		"Give a reason to resolve it anyway.")
}

func errDoDOverrideNotAllowed() error {
	return &apperr.Error{Status: http.StatusForbidden, Code: apperr.CodeDoDOverrideNotAllowed,
		Title: "This project doesn't let agents change a ticket's Definition of Done"}
}
