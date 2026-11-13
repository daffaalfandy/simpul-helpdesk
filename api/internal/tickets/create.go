package tickets

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// CreateInput opens a ticket. Requesters report for themselves through the portal; the
// staff-only fields are ignored for them.
type CreateInput struct {
	Type          ticket.Type
	Priority      sla.Priority
	Subject       string
	Description   string
	ProjectID     *uuid.UUID // incidents; service requests go to the client's default project
	CatalogItemID *uuid.UUID // service requests
	AttachmentIDs []uuid.UUID

	// Staff only: who the ticket is for and how it arrived.
	ClientID    *uuid.UUID
	RequesterID *uuid.UUID
	Channel     ticket.Channel
	AssignToMe  bool
}

// Create opens a ticket and links its uploaded files, snapshotting its project's SLA.
func (s *Service) Create(ctx context.Context, p *auth.Principal, in CreateInput) (Ticket, error) {
	a, err := accessFor(p)
	if err != nil {
		return Ticket{}, err
	}
	in, err = normalizeCreate(in)
	if err != nil {
		return Ticket{}, err
	}
	var clientID, requesterID uuid.UUID
	if a.staff {
		switch {
		case in.ClientID == nil:
			return Ticket{}, apperr.Invalid(apperr.CodeClientRequired, "Choose a client", "")
		case in.RequesterID == nil:
			return Ticket{}, apperr.Invalid(apperr.CodeInvalidRequester, "Choose who reported this", "")
		case !in.Channel.AgentLoggable():
			return Ticket{}, apperr.Invalid(apperr.CodeInvalidChannel, "Choose how the request arrived", "")
		}
		clientID, requesterID = *in.ClientID, *in.RequesterID
	} else {
		clientID, requesterID = *a.scopeClientID, p.UserID
		in.Channel, in.AssignToMe = ticket.ChannelPortal, false
	}

	var key string
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		if a.staff {
			if err := s.checkStaffTarget(ctx, tx.Q, a, clientID, requesterID); err != nil {
				return err
			}
		}
		projectID, err := ticketProject(ctx, tx.Q, clientID, in)
		if err != nil {
			return err
		}
		if in.CatalogItemID != nil {
			if _, err := tx.Q.GetActiveCatalogItem(ctx, *in.CatalogItemID); err != nil {
				return lookupError(err, apperr.CodeInvalidCatalogItem, "Choose an item from the catalogue")
			}
		}
		snap, err := SnapshotFor(ctx, tx.Q, projectID)
		if err != nil {
			return err
		}
		snapJSON, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		status, assignee := ticket.StatusNew, (*clients.Ref)(nil)
		if in.AssignToMe {
			me, err := clients.Assignable(ctx, tx.Q, clientID, p.UserID)
			if errors.Is(err, clients.ErrNotAssignable) {
				return errNotAssignable()
			}
			if err != nil {
				return err
			}
			status, assignee = ticket.StatusAfterAssignment(status, true), &me
		}
		org, err := tx.Q.GetOrganization(ctx)
		if err != nil {
			return err
		}
		number, err := tx.Q.NextTicketNumber(ctx)
		if err != nil {
			return err
		}
		key = ticket.FormatKey(org.TicketPrefix, number)
		params := db.CreateTicketParams{
			Number: number, Key: key, Type: string(in.Type), Status: string(status),
			Priority: string(in.Priority), Channel: string(in.Channel), ClientID: clientID,
			RequesterID: requesterID, CreatedBy: p.UserID, ProjectID: projectID,
			CatalogItemID: in.CatalogItemID, Subject: in.Subject, Description: in.Description,
			SlaSnapshot: snapJSON,
		}
		if assignee != nil {
			params.AssigneeID = &assignee.ID
		}
		created, err := tx.Q.CreateTicket(ctx, params)
		if err != nil {
			return err
		}
		if _, err := s.linkAttachments(ctx, tx.Q, p.UserID, created.ID, nil, in.AttachmentIDs); err != nil {
			return err
		}
		data := map[string]any{
			dataKey: key, dataType: in.Type, dataPriority: in.Priority, dataStatus: status,
			dataChannel: in.Channel, dataClient: clientID, dataRequester: requesterID, dataProject: projectID,
			dataAttachmentIDs: nonNilIDs(in.AttachmentIDs),
		}
		if assignee != nil {
			data[dataAssignee] = refData(assignee.ID, assignee.Name)
		}
		if in.CatalogItemID != nil {
			data[dataCatalogItem] = *in.CatalogItemID
		}
		return audit.Write(ctx, tx.Q, p.AuditActor().Entry(ActionCreated, TargetTicket, &created.ID, data))
	})
	if err != nil {
		return Ticket{}, err
	}
	return s.Get(ctx, p, key)
}

// ticketProject picks the ticket's project: the incident's chosen system, or the client's
// default project for a service request. A client without a project can't take tickets.
func ticketProject(ctx context.Context, q *db.Queries, clientID uuid.UUID, in CreateInput) (uuid.UUID, error) {
	def, err := q.GetClientDefaultProject(ctx, clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, apperr.Invalid(apperr.CodeClientHasNoProject, "This client has no project yet",
			"A supervisor adds one on the Projects page.")
	}
	if err != nil {
		return uuid.Nil, err
	}
	if in.Type == ticket.TypeRequest {
		return def.ID, nil
	}
	p, err := q.GetActiveClientProject(ctx, db.GetActiveClientProjectParams{ID: *in.ProjectID, ClientID: clientID})
	if err != nil {
		return uuid.Nil, lookupError(err, apperr.CodeInvalidProject, "Choose one of the client's systems")
	}
	return p.ID, nil
}

// checkStaffTarget checks an agent may log tickets for the client and that the requester
// belongs to it and isn't suspended.
func (s *Service) checkStaffTarget(ctx context.Context, q *db.Queries, a access, clientID, requesterID uuid.UUID) error {
	if _, err := q.GetClient(ctx, db.GetClientParams{ID: clientID, AgentID: a.scopeAgentID}); err != nil {
		return lookupError(err, apperr.CodeClientNotFound, "Choose one of your clients")
	}
	r, err := q.GetClientRequester(ctx, db.GetClientRequesterParams{
		ID: requesterID, ClientID: &clientID, Role: string(auth.RoleRequester),
	})
	if err != nil {
		return lookupError(err, apperr.CodeInvalidRequester, "Choose a requester of this client")
	}
	if auth.UserStatus(r.Status) == auth.UserStatusSuspended {
		return apperr.Invalid(apperr.CodeInvalidRequester, "This requester's account is suspended", "")
	}
	return nil
}

func normalizeCreate(in CreateInput) (CreateInput, error) {
	in.Subject = strings.TrimSpace(in.Subject)
	in.Description = strings.TrimSpace(in.Description)
	switch {
	case !in.Type.Valid():
		return in, apperr.InvalidRequest("type")
	case !in.Priority.Valid():
		return in, apperr.InvalidRequest("priority")
	case in.Subject == "" || len([]rune(in.Subject)) > MaxSubjectLen:
		return in, apperr.Invalid(apperr.CodeInvalidSubject, "Enter a short subject", "")
	case in.Description == "" || len([]rune(in.Description)) > MaxDescriptionLen:
		return in, apperr.Invalid(apperr.CodeInvalidDescription, "Describe the issue", "")
	case in.Type == ticket.TypeIncident && in.ProjectID == nil:
		return in, apperr.Invalid(apperr.CodeInvalidProject, "Choose which system has the problem", "")
	case in.ProjectID != nil && in.Type != ticket.TypeIncident:
		return in, apperr.Invalid(apperr.CodeInvalidProject, "Service requests go to the client's default project", "")
	case in.CatalogItemID != nil && in.Type != ticket.TypeRequest:
		return in, apperr.Invalid(apperr.CodeInvalidCatalogItem, "Only service requests name a catalogue item", "")
	}
	ids, err := dedupeAttachments(in.AttachmentIDs)
	in.AttachmentIDs = ids
	return in, err
}

// lookupError turns a missing row into a 400 with the given code.
func lookupError(err error, code, title string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.Invalid(code, title, "")
	}
	return err
}

func errNotAssignable() error {
	return apperr.Invalid(apperr.CodeInvalidAssignee, "This person can't take this client's tickets", "")
}

// refData is the audit form of a person or record.
func refData(id uuid.UUID, name string) map[string]any {
	return map[string]any{dataRefID: id, dataRefName: name}
}

func nonNilIDs(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}
