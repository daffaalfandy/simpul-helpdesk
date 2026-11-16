package tickets

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
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
	ticketID   uuid.UUID
	auditIDs   []uuid.UUID
	movesClock bool // a change the SLA clocks must replay
}

func newChangeSet(ticketID uuid.UUID) *changeSet { return &changeSet{ticketID: ticketID} }

// write records an audit entry about the ticket in the same transaction as the change.
func (c *changeSet) write(ctx context.Context, q *db.Queries, e audit.Entry) error {
	id, err := audit.Insert(ctx, q, e)
	if err != nil {
		return err
	}
	c.auditIDs = append(c.auditIDs, id)
	c.movesClock = c.movesClock || slices.Contains(clockActions, e.Action)
	return nil
}

// commit replays the ticket's SLA clocks if the transaction moved them, and queues the
// notification fan-out for everything it changed. A transaction that changed nothing does neither.
func (s *Service) commit(ctx context.Context, tx store.Tx, c *changeSet) error {
	if len(c.auditIDs) == 0 {
		return nil
	}
	if c.movesClock {
		if err := s.replaySLA(ctx, tx.Q, c.ticketID, s.now()); err != nil {
			return err
		}
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
		c := notification.Change{AuditID: r.ID, At: r.At}
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
			if cause := decode[ticket.Cause](data, dataCause); cause != nil {
				c.Cause = *cause
			}
			c.MessageID = decode[uuid.UUID](data, dataMessageID) // a rejected test's message
		case ActionDoDChanged:
			// Supervisors hear when a ticket gets a Definition of Done of its own or its conditions
			// change; not a reset to the project's, nor a new reason for the same conditions.
			src := decode[change[dod.Source]](data, dataDoDSource)
			cfg := decode[change[dod.Config]](data, dataDoD)
			if src == nil || cfg == nil || src.To != dod.SourceTicket || (src.From == src.To && cfg.From == cfg.To) {
				continue
			}
			c.Kind = notification.ChangeDoD
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
		case ActionCodeLinked:
			c.Kind = notification.ChangeCode
			c.Code = &notification.Code{
				Provider: deref(decode[string](data, dataProvider)), Kind: deref(decode[string](data, dataCodeKind)),
				Ref: deref(decode[string](data, dataCodeRef)), Branch: deref(decode[string](data, dataBranch)),
				Target: deref(decode[string](data, dataTarget)), Outcome: deref(decode[string](data, dataOutcome)),
			}
		case ActionSLAAtRisk, ActionSLABreached:
			c.Kind = notification.ChangeSLAAtRisk
			if r.Action == ActionSLABreached {
				c.Kind = notification.ChangeSLABreached
			}
			if clock := decode[string](data, dataClock); clock != nil {
				c.SLAClock = *clock
			}
			c.SLADueAt = decode[time.Time](data, dataDueAt)
			if ms := decode[int64](data, dataLeftMs); ms != nil {
				left := time.Duration(*ms) * time.Millisecond
				c.SLALeft = &left
			}
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
