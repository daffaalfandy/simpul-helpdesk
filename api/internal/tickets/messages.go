package tickets

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

type MessageInput struct {
	Visibility    ticket.Visibility
	Body          string
	AttachmentIDs []uuid.UUID
}

// AddMessage posts a reply, or a staff internal note. The first public staff reply is the first
// response; a requester reply resumes a ticket waiting on them; notes leave updatedAt alone.
func (s *Service) AddMessage(ctx context.Context, p *auth.Principal, key string, in MessageInput) (Message, error) {
	a, err := accessFor(p)
	if err != nil {
		return Message{}, err
	}
	in.Body = strings.TrimSpace(in.Body)
	switch {
	case !in.Visibility.Valid():
		return Message{}, apperr.InvalidRequest("visibility")
	case in.Visibility == ticket.VisibilityInternal && !a.staff:
		return Message{}, apperr.Forbidden()
	case in.Body == "" || len([]rune(in.Body)) > MaxMessageLen:
		return Message{}, apperr.Invalid(apperr.CodeInvalidMessage, "Write a message", "")
	}
	ids, err := dedupeAttachments(in.AttachmentIDs)
	if err != nil {
		return Message{}, err
	}
	msg := Message{
		Author: Ref{p.UserID, p.Name}, AuthorJobTitle: p.JobTitle, FromSupport: a.staff,
		Visibility: in.Visibility, Body: in.Body,
	}
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		row, err := lockTicket(ctx, tx.Q, a, key)
		if err != nil {
			return err
		}
		from := ticket.Status(row.Status)
		if from.Locked() {
			return errTicketClosed()
		}
		created, err := tx.Q.CreateTicketMessage(ctx, db.CreateTicketMessageParams{
			TicketID: row.ID, AuthorID: p.UserID, Visibility: string(in.Visibility), Body: in.Body,
		})
		if err != nil {
			return err
		}
		msg.ID, msg.CreatedAt = created.ID, created.CreatedAt
		if msg.Attachments, err = s.linkAttachments(ctx, tx.Q, p.UserID, row.ID, &created.ID, ids); err != nil {
			return err
		}
		cs := newChangeSet(row.ID)
		if err := s.recordMessage(ctx, tx, cs, p.AuditActor(), a.staff, from, created.ID, in.Visibility, ids); err != nil {
			return err
		}
		return s.commit(ctx, tx, cs)
	})
	if err != nil {
		return Message{}, err
	}
	return msg, nil
}

// recordMessage writes what a new message changed: its audit entry, and for public messages the
// ticket's update time, the first response or the resumed status.
func (s *Service) recordMessage(ctx context.Context, tx store.Tx, cs *changeSet, actor audit.Actor, staff bool,
	from ticket.Status, messageID uuid.UUID, visibility ticket.Visibility, attachmentIDs []uuid.UUID) error {
	if err := cs.write(ctx, tx.Q, actor.Entry(ActionMessageAdded, TargetTicket, &cs.ticketID, map[string]any{
		dataMessageID: messageID, dataVisibility: visibility, dataAttachmentIDs: nonNilIDs(attachmentIDs),
	})); err != nil {
		return err
	}
	if visibility != ticket.VisibilityPublic {
		return nil
	}
	if err := tx.Q.TouchTicket(ctx, cs.ticketID); err != nil {
		return err
	}
	if staff {
		n, err := tx.Q.MarkTicketFirstResponse(ctx, cs.ticketID)
		if err != nil || n == 0 {
			return err
		}
		return cs.write(ctx, tx.Q, actor.Entry(ActionFirstResponded, TargetTicket, &cs.ticketID, map[string]any{
			dataMessageID: messageID,
		}))
	}
	if to, changed := ticket.StatusAfterRequesterReply(from); changed {
		return s.writeStatus(ctx, tx, cs, actor, statusMove{from: from, to: to, cause: ticket.CauseRequesterReply})
	}
	return nil
}
