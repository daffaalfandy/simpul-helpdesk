package seed

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notifications"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// demoWatcher follows a demo ticket: Dian, added by Hendra, so "People notified" isn't empty.
var demoWatcher = struct {
	number           int64
	watcher, addedBy string
}{number: 24871, watcher: emailDian, addedBy: emailHendra}

// notificationsReadAfter is how old a demo notification is before it counts as read, so the
// bell shows only the latest changes as unread.
const notificationsReadAfter = 24 * time.Hour

var targetTicket = tickets.TargetTicket

// seedNotifications adds the demo watcher, then runs the notification router over the demo
// tickets' history, one transaction (one audit time) at a time, as the live fan-out would have.
// Nothing is emailed for this history. Repeating it adds nothing.
func seedNotifications(ctx context.Context, q *db.Queries, now time.Time, r refs) error {
	numbers := make([]int64, len(demoTickets))
	for i, d := range demoTickets {
		numbers[i] = d.number
	}
	ids, err := q.ListTicketIDsByNumber(ctx, numbers)
	if err != nil {
		return err
	}
	watched, err := q.ListTicketIDsByNumber(ctx, []int64{demoWatcher.number})
	if err != nil || len(watched) == 0 {
		return err
	}
	if _, err := q.AddTicketWatcher(ctx, db.AddTicketWatcherParams{
		TicketID: watched[0], UserID: r.users[demoWatcher.watcher], AddedBy: r.users[demoWatcher.addedBy],
	}); err != nil {
		return fmt.Errorf("demo watcher: %w", err)
	}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, id := range ids {
		rows, err := q.ListAuditByTarget(ctx, db.ListAuditByTargetParams{TargetType: &targetTicket, TargetID: &id})
		if err != nil {
			return err
		}
		for _, group := range byTransaction(rows) {
			if err := notifications.Fanout(ctx, q, quiet, id, group); err != nil {
				return fmt.Errorf("notifications for ticket %s: %w", id, err)
			}
		}
	}
	return q.SettleSeedNotifications(ctx, db.SettleSeedNotificationsParams{
		ReadBefore: now.Add(-notificationsReadAfter), TicketIds: ids,
	})
}

// byTransaction groups a ticket's audit entries by time: entries written together share it.
func byTransaction(rows []db.ListAuditByTargetRow) [][]uuid.UUID {
	var out [][]uuid.UUID
	for i, r := range rows {
		if i == 0 || !r.At.Equal(rows[i-1].At) {
			out = append(out, nil)
		}
		out[len(out)-1] = append(out[len(out)-1], r.ID)
	}
	return out
}
