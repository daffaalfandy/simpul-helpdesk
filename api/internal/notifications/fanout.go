// Package notifications stores and delivers notifications: the fan-out from ticket changes,
// the inbox and settings API, and the email jobs. Who hears about what is decided by the pure
// notification package.
package notifications

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
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
	args := job.Args
	return w.store.Tx(ctx, func(tx store.Tx) error {
		rows, err := tx.Q.ListAuditByIDs(ctx, db.ListAuditByIDsParams{
			TargetType: &targetTicket, TargetID: &args.TicketID, Ids: args.AuditIDs,
		})
		if err != nil || len(rows) == 0 {
			return err
		}
		t, err := tx.Q.GetTicketForFanout(ctx, args.TicketID)
		if err != nil {
			return err
		}
		watchers, err := tx.Q.ListTicketWatcherIDs(ctx, args.TicketID)
		if err != nil {
			return err
		}
		in := notification.Input{
			Actor: rows[0].ActorUserID, Assignee: t.AssigneeID, Requester: t.RequesterID,
			Watchers: watchers, Priority: sla.Priority(t.Priority), Changes: tickets.NotificationChanges(rows),
		}
		if err := loadRecipients(ctx, tx.Q, &in); err != nil {
			return err
		}
		deliveries, dropped := notification.Route(in)
		if dropped > 0 {
			w.log.ErrorContext(ctx, "notification rule reached client staff with an internal note; dropped",
				"ticket_id", args.TicketID, "dropped", dropped)
		}
		source := rows[0].ID
		for _, d := range deliveries {
			data, err := json.Marshal(notification.DataFor(d))
			if err != nil {
				return err
			}
			if err := tx.Q.InsertNotification(ctx, db.InsertNotificationParams{
				UserID: d.UserID, TicketID: args.TicketID, EventType: string(d.Event), ActorUserID: in.Actor,
				SourceID: source, MessageID: d.Change.MessageID, Data: data,
				Web: d.Channels.Web, Email: d.Channels.Email, Urgent: d.Urgent,
			}); err != nil {
				return err
			}
		}
		return nil
	})
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
