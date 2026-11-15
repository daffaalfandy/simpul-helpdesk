package tickets

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// JobInserter enqueues background jobs inside a transaction (a River client).
type JobInserter interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// fanoutMaxAttempts bounds retries of a notification fan-out; a change that still fails is
// logged by River rather than retried forever.
const fanoutMaxAttempts = 10

// changeSet collects the audit entries one ticket transaction writes, so one job can tell
// everyone about all of them together.
type changeSet struct {
	ticketID uuid.UUID
	auditIDs []uuid.UUID
}

func newChangeSet(ticketID uuid.UUID) *changeSet { return &changeSet{ticketID: ticketID} }

// write records an audit entry about the ticket in the same transaction as the change.
func (c *changeSet) write(ctx context.Context, q *db.Queries, e audit.Entry) error {
	id, err := audit.Insert(ctx, q, e)
	if err != nil {
		return err
	}
	c.auditIDs = append(c.auditIDs, id)
	return nil
}

// commit queues the notification fan-out for everything the transaction changed. Nothing is
// queued for a transaction that changed nothing.
func (s *Service) commit(ctx context.Context, tx store.Tx, c *changeSet) error {
	if len(c.auditIDs) == 0 {
		return nil
	}
	_, err := s.jobs.InsertTx(ctx, tx.PG, notification.FanoutArgs{TicketID: c.ticketID, AuditIDs: c.auditIDs},
		&river.InsertOpts{MaxAttempts: fanoutMaxAttempts})
	return err
}

// AuditRow is one audit entry read back for the notification fan-out.
type AuditRow = db.ListAuditByIDsRow

// NotificationChanges turns one transaction's audit entries into the changes the notification
// router understands. Entries it has no rule for (first response, attachments) are left out.
func NotificationChanges(rows []AuditRow) []notification.Change {
	out := make([]notification.Change, 0, len(rows))
	for _, r := range rows {
		data := map[string]json.RawMessage{}
		_ = json.Unmarshal(r.Data, &data)
		c := notification.Change{AuditID: r.ID}
		if r.Reason != nil {
			c.Reason = *r.Reason
		}
		switch r.Action {
		case ActionCreated:
			c.Kind = notification.ChangeCreated
			if ch := decode[ticket.Channel](data, dataChannel); ch != nil {
				c.StaffLogged = ch.AgentLoggable()
			}
		case ActionMessageAdded:
			c.Kind = notification.ChangeMessage
			c.MessageID = decode[uuid.UUID](data, dataMessageID)
			if v := decode[ticket.Visibility](data, dataVisibility); v == nil || *v != ticket.VisibilityPublic {
				c.Internal = true // fail closed: anything not marked public stays internal
			}
		case ActionStatusChanged:
			c.Kind = notification.ChangeStatus
			if st := decode[change[ticket.Status]](data, dataStatus); st != nil {
				c.StatusFrom, c.StatusTo = st.From, st.To
			}
		case ActionAssigneeChanged:
			c.Kind = notification.ChangeAssignee
			if a := decode[change[*refJSON]](data, dataAssignee); a != nil {
				c.AssigneeOut, c.AssigneeTo = person(a.From), person(a.To)
			}
		case ActionPriorityChanged:
			c.Kind = notification.ChangePriority
			if pr := decode[change[sla.Priority]](data, dataPriority); pr != nil {
				c.Priority = &notification.PriorityChange{From: pr.From, To: pr.To}
			}
		case ActionWatcherAdded:
			c.Kind = notification.ChangeWatcherAdded
			c.WatcherID = decode[uuid.UUID](data, dataWatcher)
		case ActionWatcherRemoved:
			c.Kind = notification.ChangeWatcherRemoved
			c.WatcherID = decode[uuid.UUID](data, dataWatcher)
		default:
			continue
		}
		out = append(out, c)
	}
	return out
}

func person(r *refJSON) *notification.Person {
	if r == nil {
		return nil
	}
	return &notification.Person{ID: r.ID, Name: r.Name}
}
