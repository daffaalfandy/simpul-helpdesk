package clients

import (
	"context"
	"errors"
	"slices"
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
	// MaxRequesterProjects caps one requester's projects: the API's projectIds maxItems.
	MaxRequesterProjects = 200
)

const (
	dataRole     = "role"
	dataClient   = "client_id"
	dataProjects = "project_ids"
)

const constraintUserEmail = "users_email_key"

// listedRequesterStatuses are the requesters a ticket can name: suspended people are left out.
var listedRequesterStatuses = []string{string(auth.UserStatusActive), string(auth.UserStatusInvited)}

// allRequesterStatuses adds the suspended, for the administrator managing a client's requesters.
var allRequesterStatuses = append(slices.Clone(listedRequesterStatuses), string(auth.UserStatusSuspended))

// Requester is a member of a client's staff who reports tickets. They report on, and see the
// tickets of, the client's projects they are linked to (owner, 18 Nov), plus their own.
type Requester struct {
	ID         uuid.UUID
	Name       string
	Email      string
	JobTitle   *string
	Status     auth.UserStatus
	ProjectIDs []uuid.UUID
}

type RequesterInput struct {
	Name     string
	Email    string
	JobTitle *string
	// ProjectIDs are projects of the client; at least one while the client has projects.
	ProjectIDs []uuid.UUID
}

// Assignee is someone who can take a client's tickets.
type Assignee struct {
	ID          uuid.UUID
	Name        string
	Role        auth.Role
	JobTitle    *string
	OpenTickets int
}

// Requesters lists a client's active and invited requesters, by name; with includeSuspended
// (administrators only) the suspended ones too.
func (s *Service) Requesters(ctx context.Context, p *auth.Principal, clientID uuid.UUID, includeSuspended bool) ([]Requester, error) {
	statuses := listedRequesterStatuses
	if includeSuspended {
		if !auth.Can(p.Role, auth.CapUsersManage) {
			return nil, apperr.Forbidden()
		}
		statuses = allRequesterStatuses
	}
	q := s.store.Q()
	if _, err := get(ctx, q, p.OrganizationID, clientID, AgentScope(p)); err != nil {
		return nil, err
	}
	rows, err := q.ListClientRequesters(ctx, db.ListClientRequestersParams{
		ClientID: &clientID, Role: string(auth.RoleRequester), Statuses: statuses,
	})
	if err != nil {
		return nil, err
	}
	links, err := q.ListClientRequesterProjects(ctx, db.ListClientRequesterProjectsParams{
		ClientID: clientID, OrganizationID: p.OrganizationID,
	})
	if err != nil {
		return nil, err
	}
	byUser := map[uuid.UUID][]uuid.UUID{}
	for _, l := range links {
		byUser[l.UserID] = append(byUser[l.UserID], l.ProjectID)
	}
	out := make([]Requester, 0, len(rows))
	for _, r := range rows {
		out = append(out, Requester{
			ID: r.ID, Name: r.Name, Email: r.Email, JobTitle: r.JobTitle, Status: auth.UserStatus(r.Status),
			ProjectIDs: nonNil(byUser[r.ID]),
		})
	}
	return out, nil
}

// InviteRequester adds a requester to a client, linked to the given projects of it, and emails
// them the link to set their password. Only administrators invite requesters (owner, 17 Nov);
// the handler checks that.
func (s *Service) InviteRequester(ctx context.Context, p *auth.Principal, clientID uuid.UUID, in RequesterInput) (Requester, error) {
	in, err := normalizeRequester(in)
	if err != nil {
		return Requester{}, err
	}
	out := Requester{Name: in.Name, Email: in.Email, JobTitle: in.JobTitle, Status: auth.UserStatusInvited}
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		if _, err := get(ctx, tx.Q, p.OrganizationID, clientID, AgentScope(p)); err != nil {
			return err
		}
		ids, err := clientProjects(ctx, tx.Q, p.OrganizationID, clientID, in.ProjectIDs)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			n, err := tx.Q.CountClientProjects(ctx, clientID)
			if err != nil {
				return err
			}
			if n > 0 {
				return apperr.Invalid(apperr.CodeInvalidProject, "Choose at least one project", "")
			}
		}
		id, err := tx.Q.CreateInvitedUser(ctx, db.CreateInvitedUserParams{
			Email: in.Email, Name: in.Name, Role: string(auth.RoleRequester), ClientID: &clientID,
			JobTitle: in.JobTitle, Status: string(auth.UserStatusInvited),
		})
		if err != nil {
			return err
		}
		out.ID, out.ProjectIDs = id, ids
		if _, err := tx.Q.AddRequesterProjects(ctx, db.AddRequesterProjectsParams{
			UserID: id, ProjectIds: ids, ClientID: clientID, OrganizationID: p.OrganizationID,
		}); err != nil {
			return err
		}
		if err := audit.Write(ctx, tx.Q, p.AuditActor().Entry(auth.ActionUserInvited, auth.TargetUser, &id, map[string]any{
			dataRole: auth.RoleRequester, dataClient: clientID, dataProjects: ids,
		})); err != nil {
			return err
		}
		return s.inviter.QueueInvite(ctx, tx, p.OrganizationID, id, &p.UserID)
	})
	if store.IsViolation(err, store.SQLStateUniqueViolation, constraintUserEmail) {
		return Requester{}, apperr.Conflict(apperr.CodeEmailTaken, "Someone already uses this email address", "")
	}
	if err != nil {
		return Requester{}, err
	}
	return out, nil
}

// SetRequesterProjects replaces which of the client's projects a requester reports on, and
// records what was added and removed. An empty list leaves them only their own tickets.
// Administrators only; the handler checks that.
func (s *Service) SetRequesterProjects(ctx context.Context, p *auth.Principal, clientID, userID uuid.UUID, projectIDs []uuid.UUID) (Requester, error) {
	var out Requester
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		if _, err := get(ctx, tx.Q, p.OrganizationID, clientID, AgentScope(p)); err != nil {
			return err
		}
		r, err := tx.Q.LockClientRequester(ctx, db.LockClientRequesterParams{
			ID: userID, ClientID: &clientID, Role: string(auth.RoleRequester),
		})
		if err != nil {
			return notFound(err)
		}
		want, err := clientProjects(ctx, tx.Q, p.OrganizationID, clientID, projectIDs)
		if err != nil {
			return err
		}
		have, err := tx.Q.ListRequesterProjectIDs(ctx, db.ListRequesterProjectIDsParams{UserID: userID, OrganizationID: p.OrganizationID})
		if err != nil {
			return err
		}
		out = Requester{
			ID: r.ID, Name: r.Name, Email: r.Email, JobTitle: r.JobTitle, Status: auth.UserStatus(r.Status), ProjectIDs: want,
		}
		added, removed := without(want, have), without(have, want)
		if len(added) == 0 && len(removed) == 0 {
			return nil
		}
		if _, err := tx.Q.AddRequesterProjects(ctx, db.AddRequesterProjectsParams{
			UserID: userID, ProjectIds: added, ClientID: clientID, OrganizationID: p.OrganizationID,
		}); err != nil {
			return err
		}
		if _, err := tx.Q.RemoveRequesterProjects(ctx, db.RemoveRequesterProjectsParams{
			UserID: userID, OrganizationID: p.OrganizationID, ProjectIds: removed,
		}); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, p.AuditActor().Entry(auth.ActionProjectsChanged, auth.TargetUser, &userID, map[string]any{
			dataClient: clientID, dataAdded: added, dataRemoved: removed,
		}))
	})
	if err != nil {
		return Requester{}, err
	}
	return out, nil
}

// clientProjects checks that every id is a project of the client, and returns them sorted
// without repeats.
func clientProjects(ctx context.Context, q *db.Queries, orgID, clientID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	out := slices.Clone(ids)
	slices.SortFunc(out, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	out = nonNil(slices.Compact(out))
	if len(out) > MaxRequesterProjects {
		return nil, errInvalidProject()
	}
	if len(out) == 0 {
		return out, nil
	}
	n, err := q.CountClientProjectsIn(ctx, db.CountClientProjectsInParams{Ids: out, ClientID: clientID, OrganizationID: orgID})
	if err != nil {
		return nil, err
	}
	if int(n) != len(out) {
		return nil, errInvalidProject()
	}
	return out, nil
}

func errInvalidProject() error {
	return apperr.Invalid(apperr.CodeInvalidProject, "Choose projects of this client", "")
}

// without returns the ids in a that aren't in b, never nil.
func without(a, b []uuid.UUID) []uuid.UUID {
	out := []uuid.UUID{}
	for _, id := range a {
		if !slices.Contains(b, id) {
			out = append(out, id)
		}
	}
	return out
}

func nonNil(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}

// Assignees lists who can take the client's tickets, with their open workload. An agent sees it
// for their clients, and for any client where a ticket is assigned to them, so they can still
// hand it on after being unlinked.
func (s *Service) Assignees(ctx context.Context, p *auth.Principal, clientID uuid.UUID) ([]Assignee, error) {
	q := s.store.Q()
	scope := AgentScope(p)
	if scope != nil {
		assigned, err := q.HasTicketAssignedAt(ctx, db.HasTicketAssignedAtParams{ClientID: clientID, UserID: scope})
		if err != nil {
			return nil, err
		}
		if assigned {
			scope = nil
		}
	}
	if _, err := get(ctx, q, p.OrganizationID, clientID, scope); err != nil {
		return nil, err
	}
	rows, err := q.ListAssignableUsers(ctx, db.ListAssignableUsersParams{
		OrganizationID: p.OrganizationID, OpenStatuses: statusStrings(ticket.OpenStatuses), Status: string(auth.UserStatusActive),
		AdminRole: string(auth.RoleAdmin), SupervisorRole: string(auth.RoleSupervisor), AgentRole: string(auth.RoleAgent),
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

// Assignable returns the user if they may take the client's tickets: an active administrator,
// a supervisor of the client's organization, or an agent of it linked to the client. It runs on
// q to check inside a transaction.
func Assignable(ctx context.Context, q *db.Queries, orgID, clientID, userID uuid.UUID) (Ref, error) {
	r, err := q.GetAssignableUser(ctx, db.GetAssignableUserParams{
		OrganizationID: orgID, ID: userID, Status: string(auth.UserStatusActive),
		AdminRole: string(auth.RoleAdmin), SupervisorRole: string(auth.RoleSupervisor), AgentRole: string(auth.RoleAgent),
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

func statusStrings(statuses []ticket.Status) []string {
	out := make([]string, len(statuses))
	for i, s := range statuses {
		out[i] = string(s)
	}
	return out
}
