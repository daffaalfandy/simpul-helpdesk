package tickets

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/richtext"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// MaxUATRequests caps the portal's "Waiting for your testing" list.
const MaxUATRequests = 50

// TicketUAT is what the client sees while a ticket waits In UAT for their test.
type TicketUAT struct {
	AutoApproveAt *time.Time // nil when the test waits for an answer however long it takes
	CanAnswer     bool       // the viewer may approve or reject (any requester of the client)
	Instructions  *string    // the team's latest public reply
}

// TicketReopen is the reopen window of a resolved ticket.
type TicketReopen struct {
	Until     time.Time // the ticket can be reopened until then, and closes after it
	CanReopen bool      // the viewer may reopen it now
}

// UATRequest is a test waiting on a requester, for the portal's home page.
type UATRequest struct {
	Key           string
	Subject       string
	Project       Ref
	Requester     Ref
	AutoApproveAt *time.Time
	Instructions  *string
}

// AnswerUAT records the client's answer to a test. Any requester of the ticket's client may
// answer (owner, 16 Nov). Approval meets the client items of the Definition of Done and
// resolves the ticket; a rejection posts what went wrong as a public reply and hands the
// ticket back to the team.
func (s *Service) AnswerUAT(ctx context.Context, p *auth.Principal, key string, answer ticket.UATAnswer, message string) (Ticket, error) {
	message = strings.TrimSpace(message)
	switch {
	case !answer.Valid():
		return Ticket{}, apperr.InvalidRequest("answer")
	case answer == ticket.UATReject && (message == "" || len([]rune(message)) > MaxMessageLen):
		return Ticket{}, apperr.Invalid(apperr.CodeInvalidMessage, "Tell the team what went wrong", "")
	case richtext.TooComplex(message):
		return Ticket{}, errTooComplex()
	}
	err := s.clientAction(ctx, p, key, func(tx store.Tx, row db.GetTicketForUpdateRow, cs *changeSet) error {
		from := ticket.Status(row.Status)
		if err := ticket.CheckUATAnswer(from); err != nil {
			return clientMoveError(err)
		}
		actor := p.AuditActor()
		move := statusMove{from: from, to: ticket.StatusAfterUATAnswer(answer, row.AssigneeID != nil)}
		if answer == ticket.UATApprove {
			sd, err := parseDoD(row.DodConfig, row.DodOverride)
			if err != nil {
				return err
			}
			eff, _ := sd.effective()
			if err := tickClientItems(ctx, tx.Q, cs, actor, eff, &p.UserID); err != nil {
				return err
			}
			move.cause = ticket.CauseUATApproved
			return s.writeStatus(ctx, tx, cs, actor, move)
		}
		created, err := tx.Q.CreateTicketMessage(ctx, db.CreateTicketMessageParams{
			TicketID: row.ID, AuthorID: p.UserID, Visibility: string(ticket.VisibilityPublic), Body: message,
		})
		if err != nil {
			return err
		}
		if err := s.recordMessage(ctx, tx, cs, actor, false, from, created.ID, ticket.VisibilityPublic, nil); err != nil {
			return err
		}
		move.cause, move.messageID = ticket.CauseUATRejected, &created.ID
		return s.writeStatus(ctx, tx, cs, actor, move)
	})
	if err != nil {
		return Ticket{}, err
	}
	return s.Get(ctx, p, key)
}

// Reopen hands a resolved ticket back to the team, with the same priority, while its reopen
// window lasts. Any requester of the ticket's client may reopen it.
func (s *Service) Reopen(ctx context.Context, p *auth.Principal, key string) (Ticket, error) {
	err := s.clientAction(ctx, p, key, func(tx store.Tx, row db.GetTicketForUpdateRow, cs *changeSet) error {
		from := ticket.Status(row.Status)
		var until time.Time
		if row.ResolvedAt != nil {
			schedules, err := sla.LoadSchedules(ctx, tx.Q, row.OrganizationID)
			if err != nil {
				return err
			}
			until = ticket.ReopenUntil(*row.ResolvedAt, schedules)
		}
		if err := ticket.CheckReopen(from, until, s.now()); err != nil {
			return clientMoveError(err)
		}
		return s.writeStatus(ctx, tx, cs, p.AuditActor(), statusMove{
			from: from, to: ticket.StatusAfterReopen(row.AssigneeID != nil), cause: ticket.CauseReopened,
		})
	})
	if err != nil {
		return Ticket{}, err
	}
	return s.Get(ctx, p, key)
}

// UATRequests lists the tests waiting on a requester: tickets of their client In UAT that they
// reported or follow, oldest first. Colleagues' other tests stay answerable from their pages.
func (s *Service) UATRequests(ctx context.Context, p *auth.Principal) ([]UATRequest, error) {
	a, err := clientAccess(p)
	if err != nil {
		return nil, err
	}
	q := s.store.Q()
	rows, err := q.ListUATRequests(ctx, db.ListUATRequestsParams{
		ClientID: *a.scopeClientID, Status: string(ticket.StatusInUAT), UserID: p.UserID, RowLimit: MaxUATRequests,
	})
	if err != nil || len(rows) == 0 {
		return []UATRequest{}, err
	}
	schedules, err := s.sla.Schedules(ctx, a.org)
	if err != nil {
		return nil, err
	}
	out := make([]UATRequest, 0, len(rows))
	for _, r := range rows {
		sd, err := parseDoD(r.DodConfig, r.DodOverride)
		if err != nil {
			return nil, err
		}
		req := UATRequest{
			Key: r.Key, Subject: r.Subject, Project: Ref{r.ProjectID, r.ProjectName},
			Requester: Ref{r.RequesterID, r.RequesterName}, AutoApproveAt: autoApproveAt(sd, r.StatusChangedAt, schedules),
		}
		// At most MaxUATRequests lookups, and usually one or two.
		reply, err := LatestStaffReply(ctx, q, r.ID)
		if err != nil {
			return nil, err
		}
		if reply != nil {
			req.Instructions = &reply.Body
		}
		out = append(out, req)
	}
	return out, nil
}

// autoApproveAt is when a ticket that entered In UAT at enteredAt is approved automatically.
func autoApproveAt(sd storedDoD, enteredAt time.Time, s sla.Schedules) *time.Time {
	eff, _ := sd.effective()
	at, ok := dod.AutoApproveAt(eff, enteredAt, s)
	if !ok {
		return nil
	}
	return &at
}

// clientState fills a ticket's UAT and reopen state for its status: the test while In UAT,
// the reopen window while resolved.
func (s *Service) clientState(ctx context.Context, q *db.Queries, t *Ticket, r db.GetTicketRow, requester bool, schedules sla.Schedules) error {
	switch t.Status {
	case ticket.StatusInUAT:
		sd, err := parseDoD(r.DodConfig, r.DodOverride)
		if err != nil {
			return err
		}
		u := TicketUAT{AutoApproveAt: autoApproveAt(sd, r.StatusChangedAt, schedules), CanAnswer: requester}
		reply, err := LatestStaffReply(ctx, q, r.ID)
		if err != nil {
			return err
		}
		if reply != nil {
			u.Instructions = &reply.Body
		}
		t.UAT = &u
	case ticket.StatusResolved:
		if t.ResolvedAt == nil {
			return nil
		}
		until := ticket.ReopenUntil(*t.ResolvedAt, schedules)
		t.Reopen = &TicketReopen{Until: until, CanReopen: requester && !s.now().After(until)}
	}
	return nil
}

// LatestStaffReply is the team's latest public reply on a ticket, or nil: the one rule for what a
// question or a test request quotes and what the portal shows as test instructions. It is a
// public message, so any requester of the client may read it.
func LatestStaffReply(ctx context.Context, q *db.Queries, ticketID uuid.UUID) (*db.GetLatestPublicStaffMessageRow, error) {
	r, err := q.GetLatestPublicStaffMessage(ctx, db.GetLatestPublicStaffMessageParams{
		TicketID: ticketID, Visibility: string(ticket.VisibilityPublic), RequesterRole: string(auth.RoleRequester),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// tickClientItems records the client's answer (or the system's, with doneBy nil) on the items of
// the client check the ticket's Definition of Done asks for.
func tickClientItems(ctx context.Context, q *db.Queries, cs *changeSet, actor audit.Actor, eff dod.Config, doneBy *uuid.UUID) error {
	var items []string
	for _, it := range dod.ClientRequired(eff) {
		items = append(items, string(it))
	}
	if len(items) == 0 {
		return nil
	}
	added, err := q.InsertTicketDoDItems(ctx, db.InsertTicketDoDItemsParams{TicketID: cs.ticketID, Items: items, DoneBy: doneBy})
	if err != nil || len(added) == 0 {
		return err
	}
	return cs.write(ctx, q, actor.Entry(ActionDoDItemsDone, TargetTicket, &cs.ticketID, map[string]any{dataItems: added}))
}

// clientMoveError maps the client's answer and reopen rules to API errors.
func clientMoveError(err error) error {
	switch {
	case errors.Is(err, ticket.ErrTicketClosed):
		return errTicketClosed()
	case errors.Is(err, ticket.ErrNotInUAT):
		return apperr.Conflict(apperr.CodeUATNotWaiting, "This ticket isn't waiting for your test any more",
			"Someone may have answered already.")
	}
	return apperr.Conflict(apperr.CodeReopenNotAllowed, "This ticket can no longer be reopened",
		"Report a new issue instead.")
}
