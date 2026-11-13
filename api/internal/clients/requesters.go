package clients

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// Requester and contact field limits.
const (
	MaxEmailLen    = 254 // RFC 5321 path limit
	MaxJobTitleLen = 200
)

const (
	dataRole   = "role"
	dataClient = "client_id"
	dataStatus = "status"
)

const constraintUserEmail = "users_email_key"

// listedRequesterStatuses are the requesters a ticket can name: suspended people are left out.
var listedRequesterStatuses = []string{string(auth.UserStatusActive), string(auth.UserStatusInvited)}

// Requester is a member of a client's staff who reports tickets.
type Requester struct {
	ID       uuid.UUID
	Name     string
	Email    string
	JobTitle *string
	Status   auth.UserStatus
}

type RequesterInput struct {
	Name     string
	Email    string
	JobTitle *string
}

// Assignee is someone who can take a client's tickets.
type Assignee struct {
	ID          uuid.UUID
	Name        string
	Role        auth.Role
	JobTitle    *string
	OpenTickets int
}

// Requesters lists a client's active and invited requesters, by name.
func (s *Service) Requesters(ctx context.Context, p *auth.Principal, clientID uuid.UUID) ([]Requester, error) {
	q := s.store.Q()
	if _, err := get(ctx, q, clientID, scope(p)); err != nil {
		return nil, err
	}
	rows, err := q.ListClientRequesters(ctx, db.ListClientRequestersParams{
		ClientID: &clientID, Role: string(auth.RoleRequester), Statuses: listedRequesterStatuses,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Requester, 0, len(rows))
	for _, r := range rows {
		out = append(out, Requester{
			ID: r.ID, Name: r.Name, Email: r.Email, JobTitle: r.JobTitle, Status: auth.UserStatus(r.Status),
		})
	}
	return out, nil
}

// CreateRequester adds a requester to a client, for agents logging a call from someone new.
// The user is invited without a password, so they can't sign in until an invite sets one.
func (s *Service) CreateRequester(ctx context.Context, p *auth.Principal, clientID uuid.UUID, in RequesterInput) (Requester, error) {
	in, err := normalizeRequester(in)
	if err != nil {
		return Requester{}, err
	}
	out := Requester{Name: in.Name, Email: in.Email, JobTitle: in.JobTitle, Status: auth.UserStatusInvited}
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		if _, err := get(ctx, tx.Q, clientID, scope(p)); err != nil {
			return err
		}
		id, err := tx.Q.CreateInvitedUser(ctx, db.CreateInvitedUserParams{
			Email: in.Email, Name: in.Name, Role: string(auth.RoleRequester), ClientID: &clientID,
			JobTitle: in.JobTitle, Status: string(auth.UserStatusInvited),
		})
		if err != nil {
			return err
		}
		out.ID = id
		return audit.Write(ctx, tx.Q, p.AuditActor().Entry(auth.ActionUserCreated, auth.TargetUser, &id, map[string]any{
			dataRole: auth.RoleRequester, dataClient: clientID, dataStatus: auth.UserStatusInvited,
		}))
	})
	if store.IsViolation(err, store.SQLStateUniqueViolation, constraintUserEmail) {
		return Requester{}, apperr.Conflict(apperr.CodeEmailTaken, "Someone already uses this email address", "")
	}
	if err != nil {
		return Requester{}, err
	}
	return out, nil
}

// Assignees lists who can take the client's tickets, with their open workload.
func (s *Service) Assignees(ctx context.Context, p *auth.Principal, clientID uuid.UUID) ([]Assignee, error) {
	q := s.store.Q()
	if _, err := get(ctx, q, clientID, scope(p)); err != nil {
		return nil, err
	}
	rows, err := q.ListAssignableUsers(ctx, db.ListAssignableUsersParams{
		OpenStatuses: statusStrings(ticket.OpenStatuses), Status: string(auth.UserStatusActive),
		AllClientRoles: roleStrings(auth.RolesWith(auth.CapClientsViewAll)), AgentRole: string(auth.RoleAgent),
		ClientID: clientID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Assignee, 0, len(rows))
	for _, r := range rows {
		out = append(out, Assignee{
			ID: r.ID, Name: r.Name, Role: auth.Role(r.Role), JobTitle: r.JobTitle, OpenTickets: int(r.OpenTicketCount),
		})
	}
	return out, nil
}

// ErrNotAssignable means the user can't take the client's tickets.
var ErrNotAssignable = errors.New("user can't take this client's tickets")

// Assignable returns the user if they may take the client's tickets: an active user who sees
// every client, or an active agent assigned to it. It runs on q to check inside a transaction.
func Assignable(ctx context.Context, q *db.Queries, clientID, userID uuid.UUID) (Ref, error) {
	r, err := q.GetAssignableUser(ctx, db.GetAssignableUserParams{
		ID: userID, Status: string(auth.UserStatusActive),
		AllClientRoles: roleStrings(auth.RolesWith(auth.CapClientsViewAll)), AgentRole: string(auth.RoleAgent),
		ClientID: clientID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Ref{}, ErrNotAssignable
	}
	if err != nil {
		return Ref{}, err
	}
	return Ref{ID: r.ID, Name: r.Name}, nil
}

func normalizeRequester(in RequesterInput) (RequesterInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = auth.NormalizeEmail(in.Email)
	in.JobTitle = trimmed(in.JobTitle)
	switch {
	case in.Name == "" || len([]rune(in.Name)) > MaxNameLen:
		return in, apperr.Invalid(apperr.CodeInvalidName, "Enter the requester's name", "")
	case strings.Index(in.Email, emailAt) < 1 || len(in.Email) > MaxEmailLen:
		return in, apperr.Invalid(apperr.CodeInvalidEmail, "Enter a valid email address", "")
	case in.JobTitle != nil && len([]rune(*in.JobTitle)) > MaxJobTitleLen:
		return in, apperr.Invalid(apperr.CodeInvalidRequest, "The job title is too long", "")
	}
	return in, nil
}

func roleStrings(roles []auth.Role) []string {
	out := make([]string, len(roles))
	for i, r := range roles {
		out[i] = string(r)
	}
	return out
}

func statusStrings(statuses []ticket.Status) []string {
	out := make([]string, len(statuses))
	for i, s := range statuses {
		out[i] = string(s)
	}
	return out
}
