package tickets

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// SetStatus moves a ticket by hand. Moving to the current status changes nothing. With
// expected set, the move only happens while the ticket still has that status, so a stale board
// card can't undo someone else's move. Resolving a ticket whose Definition of Done isn't met
// needs a reason, which goes to the audit log and the supervisors (HANDOFF §3).
func (s *Service) SetStatus(ctx context.Context, p *auth.Principal, key string, to ticket.Status, expected *ticket.Status, reason string) (Ticket, error) {
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > MaxReasonLen {
		return Ticket{}, apperr.Invalid(apperr.CodeReasonRequired, "Give a shorter reason", "")
	}
	err := s.agentAction(ctx, p, key, func(tx store.Tx, row db.GetTicketForUpdateRow, cs *changeSet) error {
		from := ticket.Status(row.Status)
		if expected != nil && *expected != from {
			return errStatusChanged()
		}
		if from == to {
			return nil
		}
		if err := ticket.CheckManualMove(from, to, row.AssigneeID != nil); err != nil {
			return moveError(err)
		}
		move := statusMove{from: from, to: to}
		if to == ticket.StatusResolved {
			var err error
			if move.cause, move.reason, err = checkResolve(ctx, tx.Q, row, reason); err != nil {
				return err
			}
		}
		return s.writeStatus(ctx, tx, cs, p.AuditActor(), move)
	})
	if err != nil {
		return Ticket{}, err
	}
	return s.Get(ctx, p, key)
}

// SetAssignee gives the ticket to someone, or takes it back with nil. Assigning a new ticket
// makes it assigned; unassigning an assigned ticket makes it new again. Agents may only take an
// unassigned ticket or release their own (assignee_not_allowed); supervisors and admins assign
// anyone who works the client.
func (s *Service) SetAssignee(ctx context.Context, p *auth.Principal, key string, assigneeID *uuid.UUID) (Ticket, error) {
	err := s.agentAction(ctx, p, key, func(tx store.Tx, row db.GetTicketForUpdateRow, cs *changeSet) error {
		if sameID(row.AssigneeID, assigneeID) {
			return nil
		}
		if !auth.Can(p.Role, auth.CapTicketsAssignAny) && !ticket.SelfAssignment(row.AssigneeID, assigneeID, p.UserID) {
			return errAssigneeNotAllowed()
		}
		var to *clients.Ref
		if assigneeID != nil {
			ref, err := clients.Assignable(ctx, tx.Q, row.OrganizationID, row.ClientID, *assigneeID)
			if errors.Is(err, clients.ErrNotAssignable) {
				return errNotAssignable()
			}
			if err != nil {
				return err
			}
			to = &ref
		}
		var from *clients.Ref
		if row.AssigneeID != nil && row.AssigneeName != nil {
			from = &clients.Ref{ID: *row.AssigneeID, Name: *row.AssigneeName}
		}
		return writeAssignee(ctx, tx.Q, cs, p.AuditActor(), ticket.Status(row.Status), from, to)
	})
	if err != nil {
		return Ticket{}, err
	}
	t, err := s.Get(ctx, p, key)
	if apperr.IsCode(err, apperr.CodeNotFound) {
		// An agent unlinked from the client just handed on the ticket that kept it in their view:
		// answer once with the ticket as it now stands.
		return s.getAs(ctx, p, access{org: p.OrganizationID, staff: true}, key)
	}
	return t, err
}

// writeAssignee stores a new assignee (nil to unassign) and its audit entries. Assigning a new
// ticket makes it assigned and unassigning an assigned ticket makes it new again.
func writeAssignee(ctx context.Context, q *db.Queries, cs *changeSet, actor audit.Actor, cur ticket.Status, from, to *clients.Ref) error {
	var toID *uuid.UUID
	var old, next any
	if from != nil {
		old = refData(from.ID, from.Name)
	}
	if to != nil {
		toID, next = &to.ID, refData(to.ID, to.Name)
	}
	status := ticket.StatusAfterAssignment(cur, to != nil)
	if err := q.UpdateTicketAssignee(ctx, db.UpdateTicketAssigneeParams{ID: cs.ticketID, AssigneeID: toID, Status: string(status)}); err != nil {
		return err
	}
	if err := cs.write(ctx, q, actor.Entry(ActionAssigneeChanged, TargetTicket, &cs.ticketID, map[string]any{
		dataAssignee: audit.Change{From: old, To: next},
	})); err != nil {
		return err
	}
	if status == cur {
		return nil
	}
	return cs.write(ctx, q, actor.Entry(ActionStatusChanged, TargetTicket, &cs.ticketID, map[string]any{
		dataStatus: audit.Change{From: cur, To: status}, dataCause: ticket.CauseAssignment,
	}))
}

// SetPriority changes the priority. Agents must say why (HANDOFF §3); the reason goes to the
// audit log. The new target comes from the ticket's SLA snapshot, not the current plan.
func (s *Service) SetPriority(ctx context.Context, p *auth.Principal, key string, priority sla.Priority, reason string) (Ticket, error) {
	reason = strings.TrimSpace(reason)
	switch {
	case !priority.Valid():
		return Ticket{}, apperr.InvalidRequest("priority")
	case reason == "" || len([]rune(reason)) > MaxReasonLen:
		return Ticket{}, apperr.Invalid(apperr.CodeReasonRequired, "Give a reason", "")
	}
	err := s.agentAction(ctx, p, key, func(tx store.Tx, row db.GetTicketForUpdateRow, cs *changeSet) error {
		from := sla.Priority(row.Priority)
		if from == priority {
			return nil
		}
		if err := tx.Q.UpdateTicketPriority(ctx, db.UpdateTicketPriorityParams{ID: row.ID, Priority: string(priority)}); err != nil {
			return err
		}
		e := p.AuditActor().Entry(ActionPriorityChanged, TargetTicket, &row.ID, map[string]any{
			dataPriority: audit.Change{From: from, To: priority},
		})
		e.Reason = reason
		return cs.write(ctx, tx.Q, e)
	})
	if err != nil {
		return Ticket{}, err
	}
	return s.Get(ctx, p, key)
}

// agentAction locks a ticket the agent may see and runs fn in the same transaction, then
// queues notifications for what fn changed. Closed tickets take no changes.
func (s *Service) agentAction(ctx context.Context, p *auth.Principal, key string,
	fn func(store.Tx, db.GetTicketForUpdateRow, *changeSet) error) error {
	a, err := staffAccess(p)
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

func lockTicket(ctx context.Context, q *db.Queries, a access, key string) (db.GetTicketForUpdateRow, error) {
	key = ticket.NormalizeKey(key)
	if !ticket.ValidKey(key) {
		return db.GetTicketForUpdateRow{}, apperr.NotFound()
	}
	row, err := q.GetTicketForUpdate(ctx, db.GetTicketForUpdateParams{
		Key: key, OrganizationID: a.org, ScopeClientID: a.scopeClientID, ScopeAgentID: a.scopeAgentID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, elsewhere(ctx, q, a, key)
	}
	return row, err
}

// statusMove is one status change: cause is empty for a plain move by hand; reason is a staff
// reason for the audit log; messageID names the message that came with it (a rejected test).
type statusMove struct {
	from, to  ticket.Status
	cause     ticket.Cause
	reason    string
	messageID *uuid.UUID
}

// writeStatus stores a status change and its audit entry, and forgets the Definition of Done
// items the move clears (dod.Cleared): the client's answer when a done ticket opens again, the
// code items when the ticket goes back to work.
func (s *Service) writeStatus(ctx context.Context, tx store.Tx, cs *changeSet, actor audit.Actor, m statusMove) error {
	if err := tx.Q.UpdateTicketStatus(ctx, db.UpdateTicketStatusParams{
		ID: cs.ticketID, Status: string(m.to), Resolved: ticket.MarksResolved(m.to), Closed: m.to == ticket.StatusClosed,
	}); err != nil {
		return err
	}
	data := map[string]any{dataStatus: audit.Change{From: m.from, To: m.to}}
	if m.cause != "" {
		data[dataCause] = m.cause
	}
	if m.messageID != nil {
		data[dataMessageID] = *m.messageID
	}
	e := actor.Entry(ActionStatusChanged, TargetTicket, &cs.ticketID, data)
	e.Reason = m.reason
	if err := cs.write(ctx, tx.Q, e); err != nil {
		return err
	}
	return clearItems(ctx, tx.Q, cs, actor, dod.Cleared(m.from, m.to))
}

func errAssigneeNotAllowed() error {
	return &apperr.Error{Status: http.StatusForbidden, Code: apperr.CodeAssigneeNotAllowed,
		Title:  "Agents can only take an unassigned ticket or release their own",
		Detail: "Ask a supervisor to give it to someone else."}
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
