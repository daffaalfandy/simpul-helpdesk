package tickets

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// MaxWatcherCandidates caps the colleague picker's list; typing narrows it.
const MaxWatcherCandidates = 20

// watcherStatuses are the accounts a requester can add: active, or invited and still
// getting emails.
var watcherStatuses = []string{string(auth.UserStatusActive), string(auth.UserStatusInvited)}

// Colleague is a requester of the ticket's client: someone following it, or who could.
type Colleague struct {
	ID       uuid.UUID
	Name     string
	JobTitle *string
}

// Watchers lists the colleagues following a ticket ("People notified", besides the requester).
func (s *Service) Watchers(ctx context.Context, p *auth.Principal, key string) ([]Colleague, error) {
	a, err := accessFor(p)
	if err != nil {
		return nil, err
	}
	q := s.store.Q()
	t, err := s.getRow(ctx, q, a, key)
	if err != nil {
		return nil, err
	}
	return listWatchers(ctx, q, t.ID)
}

// WatcherCandidates lists the client's other requesters who could follow the ticket, by name.
// Only client staff manage who follows their tickets.
func (s *Service) WatcherCandidates(ctx context.Context, p *auth.Principal, key, query string) ([]Colleague, error) {
	a, err := clientAccess(p)
	if err != nil {
		return nil, err
	}
	q := s.store.Q()
	t, err := s.getRow(ctx, q, a, key)
	if err != nil {
		return nil, err
	}
	var pattern *string
	if query = strings.TrimSpace(query); query != "" {
		if len([]rune(query)) > MaxSearchLen {
			query = string([]rune(query)[:MaxSearchLen])
		}
		p := likeAny + likeEscaper.Replace(query) + likeAny
		pattern = &p
	}
	rows, err := q.ListWatcherCandidates(ctx, db.ListWatcherCandidatesParams{
		ClientID: &t.ClientID, Role: string(auth.RoleRequester), Statuses: watcherStatuses,
		RequesterID: t.RequesterID, TicketID: t.ID, Pattern: pattern, RowLimit: MaxWatcherCandidates,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Colleague, len(rows))
	for i, r := range rows {
		out[i] = Colleague{ID: r.ID, Name: r.Name, JobTitle: r.JobTitle}
	}
	return out, nil
}

// AddWatcher adds a colleague of the requester to the ticket's "People notified". Adding
// someone already there changes nothing.
func (s *Service) AddWatcher(ctx context.Context, p *auth.Principal, key string, userID uuid.UUID) ([]Colleague, error) {
	var ticketID uuid.UUID
	err := s.watcherAction(ctx, p, key, func(tx store.Tx, row db.GetTicketForUpdateRow, cs *changeSet) error {
		ticketID = row.ID
		if userID == row.RequesterID {
			return errInvalidWatcher()
		}
		r, err := tx.Q.GetClientRequester(ctx, db.GetClientRequesterParams{
			ID: userID, ClientID: &row.ClientID, Role: string(auth.RoleRequester),
		})
		if err != nil {
			return lookupError(err, apperr.CodeInvalidWatcher, errInvalidWatcherTitle)
		}
		if auth.UserStatus(r.Status) == auth.UserStatusSuspended {
			return errInvalidWatcher()
		}
		n, err := tx.Q.AddTicketWatcher(ctx, db.AddTicketWatcherParams{TicketID: row.ID, UserID: userID, AddedBy: p.UserID})
		if err != nil || n == 0 {
			return err
		}
		return cs.write(ctx, tx.Q, p.AuditActor().Entry(ActionWatcherAdded, TargetTicket, &row.ID, map[string]any{
			dataWatcher: userID,
		}))
	})
	if err != nil {
		return nil, err
	}
	return listWatchers(ctx, s.store.Q(), ticketID)
}

// RemoveWatcher takes a colleague off the ticket's "People notified".
func (s *Service) RemoveWatcher(ctx context.Context, p *auth.Principal, key string, userID uuid.UUID) error {
	return s.watcherAction(ctx, p, key, func(tx store.Tx, row db.GetTicketForUpdateRow, cs *changeSet) error {
		n, err := tx.Q.RemoveTicketWatcher(ctx, db.RemoveTicketWatcherParams{TicketID: row.ID, UserID: userID})
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.NotFound()
		}
		return cs.write(ctx, tx.Q, p.AuditActor().Entry(ActionWatcherRemoved, TargetTicket, &row.ID, map[string]any{
			dataWatcher: userID,
		}))
	})
}

// watcherAction is agentAction for client staff: it locks a ticket of their client that is
// still open to changes, runs fn, and queues notifications for what fn changed.
func (s *Service) watcherAction(ctx context.Context, p *auth.Principal, key string,
	fn func(store.Tx, db.GetTicketForUpdateRow, *changeSet) error) error {
	a, err := clientAccess(p)
	if err != nil {
		return err
	}
	return s.store.Tx(ctx, func(tx store.Tx) error {
		row, err := lockTicket(ctx, tx.Q, a, key)
		if err != nil {
			return err
		}
		if ticket.Status(row.Status).Locked() {
			return errTicketClosed()
		}
		cs := newChangeSet(row.ID)
		if err := fn(tx, row, cs); err != nil {
			return err
		}
		return s.commit(ctx, tx, cs)
	})
}

func listWatchers(ctx context.Context, q *db.Queries, ticketID uuid.UUID) ([]Colleague, error) {
	rows, err := q.ListTicketWatchers(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	out := make([]Colleague, len(rows))
	for i, r := range rows {
		out[i] = Colleague{ID: r.ID, Name: r.Name, JobTitle: r.JobTitle}
	}
	return out, nil
}

const errInvalidWatcherTitle = "Choose a colleague from your organisation"

func errInvalidWatcher() error {
	return apperr.Invalid(apperr.CodeInvalidWatcher, errInvalidWatcherTitle, "")
}
