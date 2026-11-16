package tickets

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// Audit actions for a ticket's Definition of Done. Ticking and clearing items are recorded but
// stay out of the activity log and notifications: the status change they come with says it.
const (
	ActionDoDChanged      = "ticket.dod_changed"
	ActionDoDItemsDone    = "ticket.dod_items_done"
	ActionDoDItemsCleared = "ticket.dod_items_cleared"
)

const (
	dataDoD       = "dod"
	dataDoDSource = "dod_source"
	dataItems     = "items"
)

// projectHasRepository is false until the GitHub/GitLab connector links repositories (M7), so
// a ticket's own Definition of Done can't ask for code conditions either.
const projectHasRepository = false

// TicketDoD is a ticket's Definition of Done as the agent app shows it.
type TicketDoD struct {
	Source         dod.Source
	Config         dod.Config // the one the ticket follows
	ProjectConfig  dod.Config // the project's, for "Project default"
	OverrideReason *string
	Items          []DoDItem // the checklist, in order
	Met            bool
	Pending        []dod.Item // required items not met yet
	CanOverride    bool       // the viewer may set or reset the ticket's own Definition of Done
}

// DoDItem is one checklist line: done once DoneAt is set. DoneBy is nil when the system met it.
type DoDItem struct {
	Item   dod.Item
	DoneAt *time.Time
	DoneBy *Ref
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

// doneSet turns stored item names into the set dod.Evaluate reads.
func doneSet(items []string) map[dod.Item]bool {
	out := make(map[dod.Item]bool, len(items))
	for _, it := range items {
		out[dod.Item(it)] = true
	}
	return out
}

// pendingFor lists what an open ticket still needs before it counts as done.
func pendingFor(sd storedDoD, done []string) []dod.Item {
	eff, _ := sd.effective()
	_, pending := dod.Evaluate(eff, doneSet(done))
	if pending == nil {
		pending = []dod.Item{}
	}
	return pending
}

// ticketDoD builds the agent app's view of the ticket's Definition of Done.
func ticketDoD(sd storedDoD, reason *string, rows []db.ListTicketDoDItemsRow, canOverride bool) TicketDoD {
	eff, src := sd.effective()
	byItem := map[dod.Item]db.ListTicketDoDItemsRow{}
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		byItem[dod.Item(r.Item)] = r
		names = append(names, r.Item)
	}
	met, pending := dod.Evaluate(eff, doneSet(names))
	if pending == nil {
		pending = []dod.Item{}
	}
	t := TicketDoD{
		Source: src, Config: eff, ProjectConfig: sd.project, Items: []DoDItem{}, Met: met, Pending: pending,
		CanOverride: canOverride,
	}
	if src == dod.SourceTicket {
		t.OverrideReason = reason
	}
	for _, it := range dod.Checklist(eff) {
		item := DoDItem{Item: it}
		if r, ok := byItem[it]; ok {
			at := r.DoneAt
			item.DoneAt = &at
			item.DoneBy = optionalRef(r.DoneBy, r.DoneByName)
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
// returns it to the project's with nil. A ticket's own one can't ask for code conditions until
// its project has a repository.
func (s *Service) SetDoD(ctx context.Context, p *auth.Principal, key string, cfg *dod.Config, reason string) (Ticket, error) {
	reason = strings.TrimSpace(reason)
	if cfg != nil {
		if reason == "" || len([]rune(reason)) > MaxReasonLen {
			return Ticket{}, apperr.Invalid(apperr.CodeReasonRequired, "Give a reason", "")
		}
		if err := dod.Validate(*cfg, projectHasRepository); err != nil {
			return Ticket{}, apperr.Invalid(apperr.CodeInvalidDoD, "Check the Definition of Done", err.Error())
		}
	}
	err := s.agentAction(ctx, p, key, func(tx store.Tx, row db.GetTicketForUpdateRow, cs *changeSet) error {
		if !canOverrideDoD(p, row.AllowTicketOverride) {
			return errDoDOverrideNotAllowed()
		}
		sd, err := parseDoD(row.DodConfig, row.DodOverride)
		if err != nil {
			return err
		}
		if cfg == nil && sd.override == nil {
			return nil
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
		return cs.write(ctx, tx.Q, e)
	})
	if err != nil {
		return Ticket{}, err
	}
	return s.Get(ctx, p, key)
}

// clearClientItems forgets the client's answer when a done ticket opens again, so the next
// resolution asks again. Code items stay: the merge still happened.
func clearClientItems(ctx context.Context, q *db.Queries, cs *changeSet, actor audit.Actor) error {
	items := make([]string, len(dod.ClientItems))
	for i, it := range dod.ClientItems {
		items[i] = string(it)
	}
	removed, err := q.DeleteTicketDoDItems(ctx, db.DeleteTicketDoDItemsParams{TicketID: cs.ticketID, Items: items})
	if err != nil || len(removed) == 0 {
		return err
	}
	return cs.write(ctx, q, actor.Entry(ActionDoDItemsCleared, TargetTicket, &cs.ticketID, map[string]any{dataItems: removed}))
}

// checkResolve decides how a hand move to resolved is recorded: a plain move when the
// Definition of Done is met, a skip with the reason when it isn't, and a refusal without one.
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
	if len(pendingFor(sd, done)) == 0 {
		return "", "", nil
	}
	if reason == "" {
		return "", "", errDoDUnmet()
	}
	return ticket.CauseDoDSkipped, reason, nil
}

func errDoDUnmet() error {
	return apperr.Conflict(apperr.CodeDoDUnmet, "This ticket hasn't met its Definition of Done",
		"Give a reason to resolve it anyway.")
}

func errDoDOverrideNotAllowed() error {
	return &apperr.Error{Status: http.StatusForbidden, Code: apperr.CodeDoDOverrideNotAllowed,
		Title: "This project doesn't let agents change a ticket's Definition of Done"}
}
