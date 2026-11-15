// Package notifications stores and delivers notifications: the fan-out from ticket changes,
// the inbox and settings API, and the email jobs. Who hears about what is decided by the pure
// notification package.
package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

var targetTicket = tickets.TargetTicket

// FanoutWorker turns one ticket transaction into notifications.
type FanoutWorker struct {
	river.WorkerDefaults[notification.FanoutArgs]
	store *store.Store
	log   *slog.Logger
}

func NewFanoutWorker(st *store.Store, log *slog.Logger) *FanoutWorker {
	return &FanoutWorker{store: st, log: log}
}

// Work routes the transaction's changes and stores one notification per recipient. A retry
// adds nothing: the first audit entry plus the user is unique.
func (w *FanoutWorker) Work(ctx context.Context, job *river.Job[notification.FanoutArgs]) error {
	return w.store.Tx(ctx, func(tx store.Tx) error {
		return Fanout(ctx, tx.Q, w.log, job.Args.TicketID, job.Args.AuditIDs)
	})
}

// Fanout turns one ticket transaction's audit entries into notifications, in the caller's
// transaction. The seed runs it over the demo history.
func Fanout(ctx context.Context, q *db.Queries, log *slog.Logger, ticketID uuid.UUID, auditIDs []uuid.UUID) error {
	rows, err := q.ListAuditByIDs(ctx, db.ListAuditByIDsParams{
		TargetType: &targetTicket, TargetID: &ticketID, Ids: auditIDs,
	})
	if err != nil || len(rows) == 0 {
		return err
	}
	t, err := q.GetTicketForFanout(ctx, ticketID)
	if err != nil {
		return err
	}
	watchers, err := q.ListTicketWatcherIDs(ctx, ticketID)
	if err != nil {
		return err
	}
	in := notification.Input{
		Actor: rows[0].ActorUserID, Assignee: t.AssigneeID, Requester: t.RequesterID,
		Watchers: watchers, Priority: sla.Priority(t.Priority), Changes: tickets.NotificationChanges(rows),
	}
	if in.Supervisors, err = breachSupervisors(ctx, q, in.Changes, t.SlaSnapshot); err != nil {
		return err
	}
	if err := loadRecipients(ctx, q, &in); err != nil {
		return err
	}
	deliveries, dropped := notification.Route(in)
	if dropped > 0 {
		log.ErrorContext(ctx, "notification rule reached client staff with an internal note; dropped",
			"ticket_id", ticketID, "dropped", dropped)
	}
	if err := quoteQuestions(ctx, q, ticketID, deliveries); err != nil {
		return err
	}
	for _, d := range deliveries {
		data, err := json.Marshal(notification.DataFor(d))
		if err != nil {
			return err
		}
		if err := q.InsertNotification(ctx, db.InsertNotificationParams{
			UserID: d.UserID, TicketID: ticketID, EventType: string(d.Event), ActorUserID: in.Actor,
			SourceID: rows[0].ID, MessageID: d.Change.MessageID, Data: data,
			Web: d.Channels.Web, Email: d.Channels.Email, Urgent: d.Urgent, CreatedAt: rows[0].At,
		}); err != nil {
			return err
		}
	}
	return nil
}

// loadRecipients fills in who everyone named in the input is and what they chose to hear.
func loadRecipients(ctx context.Context, q *db.Queries, in *notification.Input) error {
	ids := []uuid.UUID{in.Requester}
	add := func(id *uuid.UUID) {
		if id != nil {
			ids = append(ids, *id)
		}
	}
	add(in.Actor)
	add(in.Assignee)
	ids = append(ids, in.Watchers...)
	ids = append(ids, in.Supervisors...)
	for _, c := range in.Changes {
		add(c.WatcherID)
		if c.AssigneeTo != nil {
			add(&c.AssigneeTo.ID)
		}
		if c.AssigneeOut != nil {
			add(&c.AssigneeOut.ID)
		}
	}
	users, err := q.ListNotifyRecipients(ctx, ids)
	if err != nil {
		return err
	}
	in.Users = make(map[uuid.UUID]notification.Recipient, len(users))
	for _, u := range users {
		in.Users[u.ID] = notification.Recipient{
			Staff:     auth.Role(u.Role) != auth.RoleRequester,
			Suspended: auth.UserStatus(u.Status) == auth.UserStatusSuspended,
		}
	}
	if in.Actor != nil {
		in.ActorStaff = in.Users[*in.Actor].Staff
	}
	prefs, err := q.ListNotificationPreferences(ctx, ids)
	if err != nil {
		return err
	}
	in.Prefs = map[uuid.UUID]map[notification.Event]notification.Pref{}
	for _, p := range prefs {
		if in.Prefs[p.UserID] == nil {
			in.Prefs[p.UserID] = map[notification.Event]notification.Pref{}
		}
		in.Prefs[p.UserID][notification.Event(p.EventType)] = notification.Pref{Web: p.Web, Email: p.Email}
	}
	return nil
}

// quoteQuestions points "question for you" notifications at the team's latest public reply,
// the question they are asked to answer.
func quoteQuestions(ctx context.Context, q *db.Queries, ticketID uuid.UUID, ds []notification.Delivery) error {
	var latest *uuid.UUID
	looked := false
	for i := range ds {
		if ds[i].Event != notification.EventQuestion || ds[i].Change.MessageID != nil {
			continue
		}
		if !looked {
			id, err := q.GetLatestPublicStaffMessageID(ctx, db.GetLatestPublicStaffMessageIDParams{
				TicketID: ticketID, Visibility: publicVisibility, RequesterRole: string(auth.RoleRequester),
			})
			switch {
			case err == nil:
				latest = &id
			case !errors.Is(err, pgx.ErrNoRows):
				return err
			}
			looked = true
		}
		ds[i].Change.MessageID = latest
	}
	return nil
}

// breachSupervisors are the active supervisors to alert about a breach, when the ticket's project
// asks for it (its snapshot's switch); nobody otherwise.
func breachSupervisors(ctx context.Context, q *db.Queries, changes []notification.Change, snapshot []byte) ([]uuid.UUID, error) {
	breach := false
	for _, c := range changes {
		breach = breach || c.Kind == notification.ChangeSLABreached
	}
	if !breach {
		return nil, nil
	}
	var snap ticket.SLASnapshot
	if err := json.Unmarshal(snapshot, &snap); err != nil || !snap.NotifySupervisorOnBreach {
		return nil, err
	}
	return q.ListActiveUserIDsByRole(ctx, db.ListActiveUserIDsByRoleParams{
		Role: string(auth.RoleSupervisor), Status: string(auth.UserStatusActive),
	})
}
