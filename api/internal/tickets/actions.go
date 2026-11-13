package tickets

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// SetStatus moves a ticket by hand. Moving to the current status changes nothing.
func (s *Service) SetStatus(ctx context.Context, p *auth.Principal, key string, to ticket.Status) (Ticket, error) {
	err := s.agentAction(ctx, p, key, func(tx store.Tx, row db.GetTicketForUpdateRow) error {
		from := ticket.Status(row.Status)
		if from == to {
			return nil
		}
		if err := ticket.CheckManualMove(from, to, row.AssigneeID != nil); err != nil {
			return moveError(err)
		}
		return s.writeStatus(ctx, tx, p.AuditActor(), row.ID, from, to, "")
	})
	if err != nil {
		return Ticket{}, err
	}
	return s.Get(ctx, p, key)
}

// SetAssignee gives the ticket to someone, or takes it back with nil. Assigning a new ticket
// makes it assigned; unassigning an assigned ticket makes it new again.
func (s *Service) SetAssignee(ctx context.Context, p *auth.Principal, key string, assigneeID *uuid.UUID) (Ticket, error) {
	err := s.agentAction(ctx, p, key, func(tx store.Tx, row db.GetTicketForUpdateRow) error {
		if sameID(row.AssigneeID, assigneeID) {
			return nil
		}
		var to *clients.Ref
		if assigneeID != nil {
			ref, err := clients.Assignable(ctx, tx.Q, row.ClientID, *assigneeID)
			if errors.Is(err, clients.ErrNotAssignable) {
				return errNotAssignable()
			}
			if err != nil {
				return err
			}
			to = &ref
		}
		from := ticket.Status(row.Status)
		status := ticket.StatusAfterAssignment(from, assigneeID != nil)
		if err := tx.Q.UpdateTicketAssignee(ctx, db.UpdateTicketAssigneeParams{
			ID: row.ID, AssigneeID: assigneeID, Status: string(status),
		}); err != nil {
			return err
		}
		var old, next any
		if row.AssigneeID != nil && row.AssigneeName != nil {
			old = refData(*row.AssigneeID, *row.AssigneeName)
		}
		if to != nil {
			next = refData(to.ID, to.Name)
		}
		actor := p.AuditActor()
		if err := audit.Write(ctx, tx.Q, actor.Entry(ActionAssigneeChanged, TargetTicket, &row.ID, map[string]any{
			dataAssignee: audit.Change{From: old, To: next},
		})); err != nil {
			return err
		}
		if status == from {
			return nil
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionStatusChanged, TargetTicket, &row.ID, map[string]any{
			dataStatus: audit.Change{From: from, To: status}, dataCause: ticket.CauseAssignment,
		}))
	})
	if err != nil {
		return Ticket{}, err
	}
	return s.Get(ctx, p, key)
}

// SetPriority changes the priority. Agents must say why (HANDOFF §3); the reason goes to the
// audit log. The new target comes from the ticket's SLA snapshot, not the current plan.
func (s *Service) SetPriority(ctx context.Context, p *auth.Principal, key string, priority sla.Priority, reason string) (Ticket, error) {
	reason = strings.TrimSpace(reason)
	switch {
	case !priority.Valid():
		return Ticket{}, apperr.InvalidRequest("priority")
	case reason == "" || len([]rune(reason)) > MaxReasonLen:
		return Ticket{}, apperr.Invalid(apperr.CodeReasonRequired, "Say why the priority changes", "")
	}
	err := s.agentAction(ctx, p, key, func(tx store.Tx, row db.GetTicketForUpdateRow) error {
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
		return audit.Write(ctx, tx.Q, e)
	})
	if err != nil {
		return Ticket{}, err
	}
	return s.Get(ctx, p, key)
}

// agentAction locks a ticket the agent may see and runs fn in the same transaction. Closed
// tickets take no changes.
func (s *Service) agentAction(ctx context.Context, p *auth.Principal, key string,
	fn func(store.Tx, db.GetTicketForUpdateRow) error) error {
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
		return fn(tx, row)
	})
}

func lockTicket(ctx context.Context, q *db.Queries, a access, key string) (db.GetTicketForUpdateRow, error) {
	key = ticket.NormalizeKey(key)
	if !ticket.ValidKey(key) {
		return db.GetTicketForUpdateRow{}, apperr.NotFound()
	}
	row, err := q.GetTicketForUpdate(ctx, db.GetTicketForUpdateParams{
		Key: key, ScopeClientID: a.scopeClientID, ScopeAgentID: a.scopeAgentID,
	})
	return row, notFound(err)
}

// writeStatus stores a status change and its audit entry. cause is empty for manual moves.
func (s *Service) writeStatus(ctx context.Context, tx store.Tx, actor audit.Actor, id uuid.UUID, from, to ticket.Status, cause ticket.Cause) error {
	if err := tx.Q.UpdateTicketStatus(ctx, db.UpdateTicketStatusParams{
		ID: id, Status: string(to), Resolved: ticket.MarksResolved(to),
	}); err != nil {
		return err
	}
	data := map[string]any{dataStatus: audit.Change{From: from, To: to}}
	if cause != "" {
		data[dataCause] = cause
	}
	return audit.Write(ctx, tx.Q, actor.Entry(ActionStatusChanged, TargetTicket, &id, data))
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
