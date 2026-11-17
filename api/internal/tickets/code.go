package tickets

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// ActionCodeLinked records a commit reaching a ticket: its data names the commit, never its
// message (UU PDP: audit entries can't be erased).
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

// CodeOutcome is what a commit's key did. Values match the code_event_links.outcome CHECK.
// Entries written before 17 Nov may hold older outcomes (merged, fix_committed, …): history.
type CodeOutcome string

const (
	CodeLinked          CodeOutcome = "linked"           // the commit reached the ticket
	CodeTicketClosed    CodeOutcome = "ticket_closed"    // the ticket was closed: logged, not linked
	CodeUnknownTicket   CodeOutcome = "unknown_ticket"   // no ticket has that key
	CodeOtherRepository CodeOutcome = "other_repository" // the ticket's project reads another repository
)

// CodeEvent is a stored commit as it reaches one ticket.
type CodeEvent struct {
	ID       uuid.UUID
	Provider gitevent.Provider
	Kind     gitevent.Kind
	Ref      string // short commit sha
	Branch   string // the branch it was first pushed to
	At       time.Time
	// AuthorID is the staff member who wrote it, matched by email; AuthorName is their name, or
	// else the provider's login or name.
	AuthorID   *uuid.UUID
	AuthorName string
}

// ApplyCode links commits to one ticket (HANDOFF §8), each once, with an audit entry that feeds
// the activity log and the assignee's notification. Commits never change the ticket (owner,
// 17 Nov). A closed ticket gets a log row and nothing else; a ticket whose project no longer
// reads repoID is left alone. Applying the same commits again changes nothing.
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
		seen, err := tx.Q.ListTicketCodeLinks(ctx, &row.ID)
		if err != nil {
			return err
		}
		cs := newChangeSet(row.ID)
		for _, e := range events {
			if slices.Contains(seen, e.ID) {
				continue
			}
			seen = append(seen, e.ID)
			if err := linkCode(ctx, tx.Q, cs, row.Key, ticket.Status(row.Status).Locked(), e); err != nil {
				return err
			}
		}
		return s.commit(ctx, tx, cs)
	})
}

// linkCode records a commit reaching the ticket: a link row for the code activity and the
// recent-events log, and, unless the ticket is closed, an audit entry for its activity log.
func linkCode(ctx context.Context, q *db.Queries, cs *changeSet, key string, closed bool, e CodeEvent) error {
	outcome := CodeLinked
	if closed {
		outcome = CodeTicketClosed
	}
	if err := q.InsertCodeLink(ctx, db.InsertCodeLinkParams{
		CodeEventID: e.ID, TicketID: &cs.ticketID, TicketKey: key, Outcome: string(outcome),
	}); err != nil || closed {
		return err
	}
	actor := audit.GitActor(e.AuthorID, e.AuthorName)
	return cs.write(ctx, q, actor.Entry(ActionCodeLinked, TargetTicket, &cs.ticketID, map[string]any{
		dataCodeEvent: e.ID, dataProvider: e.Provider, dataCodeKind: e.Kind, dataCodeRef: e.Ref,
		dataBranch: e.Branch, dataOutcome: outcome,
	}))
}
