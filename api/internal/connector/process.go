package connector

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/commitmsg"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
)

// ProcessWorker reads a stored delivery: it records the commits that name a ticket and links
// them. Every step can run again: commits are unique per repository and link once per ticket.
type ProcessWorker struct {
	river.WorkerDefaults[ProcessArgs]
	svc *Service
	log *slog.Logger
}

func NewProcessWorker(svc *Service, log *slog.Logger) *ProcessWorker {
	return &ProcessWorker{svc: svc, log: log}
}

func (w *ProcessWorker) Work(ctx context.Context, job *river.Job[ProcessArgs]) error {
	return w.svc.process(ctx, w.log, job.Args.DeliveryID)
}

// storedEvent is a commit as recorded, with the ticket keys its message names.
type storedEvent struct {
	ID         uuid.UUID
	Event      gitevent.Event
	Keys       []string
	AuthorID   *uuid.UUID // the staff member who wrote it, matched by email
	AuthorName string     // their name, else the provider's login or name
}

func (s *Service) process(ctx context.Context, log *slog.Logger, deliveryID uuid.UUID) error {
	q := s.store.Q()
	d, err := q.GetDelivery(ctx, deliveryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // purged before the job ran
	}
	if err != nil || d.ProcessedAt != nil {
		return err
	}
	parsed, err := gitevent.Parse(gitevent.Provider(d.Provider), d.EventName, d.Payload)
	if err != nil {
		// Receive parsed it before storing it, so this can't be fixed by trying again.
		log.Error("webhook delivery unreadable", "delivery", deliveryID, "err", err)
		return q.MarkDeliveryProcessed(ctx, deliveryID)
	}
	prefixes, err := q.ListTicketPrefixes(ctx, d.OrganizationID)
	if err != nil {
		return err
	}
	parser := commitmsg.New(prefixes)
	var events []storedEvent
	for _, e := range parsed.Events {
		keys := parser.Keys(e.Text)
		if len(keys) == 0 {
			continue // most commits name no ticket; they aren't kept
		}
		author, err := s.staffByEmail(ctx, q, e.Author.Email)
		if err != nil {
			return err
		}
		var authorID *uuid.UUID
		name := authorName(e.Author.Login, e.Author.Name, nil)
		if author != nil {
			authorID, name = &author.ID, author.Name
		}
		id, err := q.UpsertCodeEvent(ctx, codeEventParams(d.RepositoryID, deliveryID, e, authorID))
		if err != nil {
			return err
		}
		events = append(events, storedEvent{ID: id, Event: e, Keys: keys, AuthorID: authorID, AuthorName: name})
	}
	if err := s.apply(ctx, gitevent.Provider(d.Provider), d.OrganizationID, d.RepositoryID, events); err != nil {
		return err
	}
	return q.MarkDeliveryProcessed(ctx, deliveryID)
}

func (s *Service) staffByEmail(ctx context.Context, q *db.Queries, email string) (*db.FindStaffByEmailRow, error) {
	if email == "" {
		return nil, nil
	}
	u, err := q.FindStaffByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// authorName is how the activity log names an event's author: the staff member when known,
// else the provider's login, else the name in the commit.
func authorName(login, name string, member *string) string {
	switch {
	case member != nil:
		return *member
	case login != "":
		return login
	}
	return name
}

func codeEventParams(repoID, deliveryID uuid.UUID, e gitevent.Event, author *uuid.UUID) db.UpsertCodeEventParams {
	return db.UpsertCodeEventParams{
		RepositoryID: repoID, Kind: string(e.Kind), ExternalID: e.ExternalID, Sha: e.SHA,
		Title: e.Title, Branch: e.Branch, Url: e.URL, AuthorName: e.Author.Name,
		AuthorLogin: e.Author.Login, AuthorUserID: author, DeliveryID: &deliveryID, OccurredAt: e.OccurredAt,
	}
}
