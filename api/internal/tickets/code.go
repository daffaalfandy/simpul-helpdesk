package tickets

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/codeflow"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/commitmsg"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// ActionCodeLinked records what one code event did to a ticket: its data names the event, never
// its message (UU PDP: audit entries can't be erased).
const ActionCodeLinked = "ticket.code_linked"

const (
	dataCodeEvent = "code_event_id"
	dataProvider  = "provider"
	dataCodeKind  = "kind"
	dataCodeRef   = "ref"
	dataBranch    = "branch"
	dataTarget    = "target"
	dataOutcome   = "outcome"
)

// ignoredOutcomes are link rows that record a key the event named without reaching the ticket.
var ignoredOutcomes = []codeflow.Outcome{codeflow.OutcomeTicketClosed, codeflow.OutcomeUnknownTicket, codeflow.OutcomeOtherRepository}

// CodeEvent is a stored code event as it reaches one ticket.
type CodeEvent struct {
	ID       uuid.UUID
	Provider gitevent.Provider
	Kind     gitevent.Kind
	// Action is what the event's text asks of this ticket; empty for CI runs and deployments,
	// which name no ticket.
	Action commitmsg.Action
	Ref    string // short commit sha, request or run number, or the environment deployed to
	Branch string
	Target string
	At     time.Time
	// AuthorID is the staff member who wrote it, matched by email; AuthorName is their name, or
	// else the provider's login or name.
	AuthorID   *uuid.UUID
	AuthorName string
}

// ApplyCode applies code events to one ticket in time order (HANDOFF §8): it links them, may
// hand a new ticket to its author, ticks Definition of Done items and moves the ticket on, each
// with its audit entry, SLA replay and notifications. It plans from the ticket's state under its
// lock (codeflow), so the same events applied again change nothing. A ticket whose project no
// longer reads code from repoID is left alone.
func (s *Service) ApplyCode(ctx context.Context, ticketID, repoID uuid.UUID, events []CodeEvent) error {
	slices.SortStableFunc(events, func(a, b CodeEvent) int { return a.At.Compare(b.At) })
	return s.store.Tx(ctx, func(tx store.Tx) error {
		row, err := tx.Q.GetTicketForCode(ctx, ticketID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if row.RepositoryID == nil || *row.RepositoryID != repoID {
			return nil
		}
		in, err := codeInput(ctx, tx.Q, row, events)
		if err != nil {
			return err
		}
		plan := codeflow.Decide(in)
		cs := newChangeSet(row.ID)
		status := ticket.Status(row.Status)
		var assignee *clients.Ref
		if row.AssigneeID != nil && row.AssigneeName != nil {
			assignee = &clients.Ref{ID: *row.AssigneeID, Name: *row.AssigneeName}
		}
		for i, e := range events {
			actor := audit.GitActor(e.AuthorID, e.AuthorName)
			for _, r := range plan.Results {
				if r.Event == i {
					if err := linkCode(ctx, tx.Q, cs, actor, row.Key, e, r); err != nil {
						return err
					}
				}
			}
			for _, st := range plan.Steps {
				if st.Event != i {
					continue
				}
				switch st.Kind {
				case codeflow.StepAssign:
					to, err := clients.Assignable(ctx, tx.Q, row.ClientID, *e.AuthorID)
					if err != nil {
						return err
					}
					if err := writeAssignee(ctx, tx.Q, cs, actor, status, assignee, &to); err != nil {
						return err
					}
					assignee, status = &to, ticket.StatusAfterAssignment(status, true)
				case codeflow.StepTick:
					if err := tickCodeItem(ctx, tx.Q, cs, actor, st.Item, e); err != nil {
						return err
					}
				case codeflow.StepMove:
					if err := s.writeStatus(ctx, tx, cs, actor, statusMove{from: st.From, to: st.To, cause: st.Cause}); err != nil {
						return err
					}
					status = st.To
				}
			}
		}
		return s.commit(ctx, tx, cs)
	})
}

// codeInput reads what codeflow needs: the ticket's Definition of Done, the items it met and when,
// which events reached it before, and whether each author may take it.
func codeInput(ctx context.Context, q *db.Queries, row db.GetTicketForCodeRow, events []CodeEvent) (codeflow.Input, error) {
	eff, _, err := dod.ParseEffective(row.DodConfig, row.DodOverride)
	if err != nil {
		return codeflow.Input{}, err
	}
	in := codeflow.Input{Status: ticket.Status(row.Status), HasAssignee: row.AssigneeID != nil, DoD: eff, Done: map[dod.Item]time.Time{}}
	items, err := q.ListTicketDoDItemTimes(ctx, row.ID)
	if err != nil {
		return codeflow.Input{}, err
	}
	for _, it := range items {
		in.Done[dod.Item(it.Item)] = it.At
	}
	links, err := q.ListTicketCodeLinks(ctx, &row.ID)
	if err != nil {
		return codeflow.Input{}, err
	}
	seen := map[uuid.UUID]bool{}
	for _, l := range links {
		seen[l.CodeEventID] = true
		if !slices.Contains(ignoredOutcomes, codeflow.Outcome(l.Outcome)) && (in.LinkedAt.IsZero() || l.OccurredAt.Before(in.LinkedAt)) {
			in.LinkedAt = l.OccurredAt
		}
	}
	for _, e := range events {
		canTake := false
		if e.AuthorID != nil && e.Action != "" {
			_, err := clients.Assignable(ctx, q, row.ClientID, *e.AuthorID)
			switch {
			case err == nil:
				canTake = true
			case !errors.Is(err, clients.ErrNotAssignable):
				return codeflow.Input{}, err
			}
		}
		in.Events = append(in.Events, codeflow.Event{
			Kind: e.Kind, Action: e.Action, Branch: e.Branch, Target: e.Target, At: e.At,
			New: !seen[e.ID], AuthorCanTake: canTake,
		})
	}
	return in, nil
}

// linkCode records what an event did to the ticket: a link row for the code activity and the
// recent-events log, and, unless the ticket was closed, an audit entry for its activity log.
func linkCode(ctx context.Context, q *db.Queries, cs *changeSet, actor audit.Actor, key string, e CodeEvent, r codeflow.Result) error {
	items := make([]string, len(r.Items))
	for i, it := range r.Items {
		items[i] = string(it)
	}
	var from, to *string
	if r.To != "" {
		f, t := string(r.From), string(r.To)
		from, to = &f, &t
	}
	if err := q.InsertCodeLink(ctx, db.InsertCodeLinkParams{
		CodeEventID: e.ID, TicketID: &cs.ticketID, TicketKey: key, Outcome: string(r.Outcome), Items: items,
		StatusFrom: from, StatusTo: to,
	}); err != nil {
		return err
	}
	if r.Outcome == codeflow.OutcomeTicketClosed {
		return nil
	}
	return cs.write(ctx, q, actor.Entry(ActionCodeLinked, TargetTicket, &cs.ticketID, map[string]any{
		dataCodeEvent: e.ID, dataProvider: e.Provider, dataCodeKind: e.Kind, dataCodeRef: e.Ref,
		dataBranch: e.Branch, dataTarget: e.Target, dataOutcome: r.Outcome, dataItems: items,
	}))
}

// tickCodeItem records a Definition of Done item met by a code event, by its author when known.
func tickCodeItem(ctx context.Context, q *db.Queries, cs *changeSet, actor audit.Actor, it dod.Item, e CodeEvent) error {
	added, err := q.InsertTicketDoDItems(ctx, db.InsertTicketDoDItemsParams{
		TicketID: cs.ticketID, Items: []string{string(it)}, DoneBy: e.AuthorID, CodeEventID: &e.ID,
	})
	if err != nil || len(added) == 0 {
		return err
	}
	return cs.write(ctx, q, actor.Entry(ActionDoDItemsDone, TargetTicket, &cs.ticketID, map[string]any{
		dataItems: added, dataCodeEvent: e.ID,
	}))
}
