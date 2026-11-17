package connector

import (
	"context"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// apply hands the delivery's commits to the tickets they name, one transaction per ticket, when
// the ticket's project reads this repository. Other keys are logged and ignored.
func (s *Service) apply(ctx context.Context, provider gitevent.Provider, repoID uuid.UUID, events []storedEvent) error {
	q := s.store.Q()
	var keys []string
	for _, e := range events {
		keys = append(keys, e.Keys...)
	}
	found, err := q.ListTicketsByKeys(ctx, keys)
	if err != nil {
		return err
	}
	byKey := map[string]db.ListTicketsByKeysRow{}
	for _, t := range found {
		byKey[t.Key] = t
	}
	perTicket := map[uuid.UUID][]tickets.CodeEvent{}
	var order []uuid.UUID
	for _, e := range events {
		for _, key := range e.Keys {
			t, ok := byKey[key]
			switch {
			case !ok:
				err = ignore(ctx, q, e.ID, nil, key, tickets.CodeUnknownTicket)
			case t.RepositoryID == nil || *t.RepositoryID != repoID:
				err = ignore(ctx, q, e.ID, &t.ID, key, tickets.CodeOtherRepository)
			default:
				if _, ok := perTicket[t.ID]; !ok {
					order = append(order, t.ID)
				}
				perTicket[t.ID] = append(perTicket[t.ID], codeEvent(provider, e))
			}
			if err != nil {
				return err
			}
		}
	}
	for _, id := range order {
		if err := s.tickets.ApplyCode(ctx, id, repoID, perTicket[id]); err != nil {
			return err
		}
	}
	return nil
}

func codeEvent(provider gitevent.Provider, e storedEvent) tickets.CodeEvent {
	return tickets.CodeEvent{
		ID: e.ID, Provider: provider, Kind: e.Event.Kind, Ref: shortSHA(e.Event.SHA), Branch: e.Event.Branch,
		At: e.Event.OccurredAt, AuthorID: e.AuthorID, AuthorName: e.AuthorName,
	}
}

// shortSHA is how the activity log names a commit, as GitHub and GitLab do.
func shortSHA(sha string) string {
	if len(sha) > tickets.ShortSHALen {
		return sha[:tickets.ShortSHALen]
	}
	return sha
}

// ignore logs a key a commit named without reaching its ticket.
func ignore(ctx context.Context, q *db.Queries, eventID uuid.UUID, ticketID *uuid.UUID, key string, o tickets.CodeOutcome) error {
	return q.InsertCodeLink(ctx, db.InsertCodeLinkParams{
		CodeEventID: eventID, TicketID: ticketID, TicketKey: key, Outcome: string(o),
	})
}
