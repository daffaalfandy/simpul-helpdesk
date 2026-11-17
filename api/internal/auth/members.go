package auth

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// Field limits, matching the users table's CHECK constraints.
const (
	MaxNameLen     = 200
	MaxJobTitleLen = 100
)

// ConstraintUserEmail is the unique email constraint (users_email_key).
const ConstraintUserEmail = "users_email_key"

// StaffRoles are the roles of the vendor's team, the ones Members & roles invites and changes:
// install-wide administrators, and each organization's supervisors and agents. Client staff
// (requesters) are invited on their client (owner, 17 Nov).
var StaffRoles = []Role{RoleAdmin, RoleSupervisor, RoleAgent}

// Member is one person on an organization's team, with their role there.
type Member struct {
	ID           uuid.UUID
	Email        string
	Name         string
	Role         Role
	JobTitle     *string
	Status       UserStatus
	LastLoginAt  *time.Time
	InviteSentAt *time.Time // the latest invite; nil when none was sent
}

// Members is Settings › Members & roles for one organization: how many people hold each role
// (suspended accounts left out) and the team.
type Members struct {
	Counts map[Role]int
	Team   []Member
}

// NewMember is an invitation to an organization's team (or, for an administrator, to the
// install).
type NewMember struct {
	Email    string
	Name     string
	Role     Role
	JobTitle string
}

func isStaff(r Role) bool { return slices.Contains(StaffRoles, r) }

// Members lists an organization's team and its role counts.
func (s *Service) Members(ctx context.Context, orgID uuid.UUID) (Members, error) {
	q := s.store.Q()
	rows, err := q.ListTeamMembers(ctx, db.ListTeamMembersParams{OrganizationID: orgID, AdminRole: string(RoleAdmin)})
	if err != nil {
		return Members{}, err
	}
	counts, err := q.CountUsersByRole(ctx, db.CountUsersByRoleParams{
		OrganizationID: orgID, Suspended: string(UserStatusSuspended), AdminRole: string(RoleAdmin),
	})
	if err != nil {
		return Members{}, err
	}
	invites, err := q.ListLatestInvites(ctx)
	if err != nil {
		return Members{}, err
	}
	sent := make(map[uuid.UUID]time.Time, len(invites))
	for _, i := range invites {
		sent[i.UserID] = i.SentAt
	}
	out := Members{Counts: map[Role]int{}, Team: make([]Member, len(rows))}
	for _, c := range counts {
		out.Counts[Role(c.Role)] = int(c.N)
	}
	for i, r := range rows {
		out.Team[i] = Member{
			ID: r.ID, Email: r.Email, Name: r.Name, Role: Role(r.Role), JobTitle: r.JobTitle, Status: UserStatus(r.Status),
			LastLoginAt: r.LastLoginAt,
		}
		if at, ok := sent[r.ID]; ok {
			out.Team[i].InviteSentAt = &at
		}
	}
	return out, nil
}

// ValidateNewUser checks an invitation's name and email; email is already normalised.
func ValidateNewUser(email, name, jobTitle string) error {
	switch {
	case !strings.Contains(email, emailAt) || utf8.RuneCountInString(email) > MaxNameLen:
		return apperr.Invalid(apperr.CodeInvalidEmail, "Enter a valid email address", "")
	case name == "" || utf8.RuneCountInString(name) > MaxNameLen:
		return apperr.Invalid(apperr.CodeInvalidName, "Enter a name", "")
	case utf8.RuneCountInString(jobTitle) > MaxJobTitleLen:
		return apperr.InvalidRequest("job title too long")
	}
	return nil
}

// InviteMember adds someone to the organization's team. A new email gets an account without a
// password and a link to set one; a supervisor or agent who already works for another of the
// install's organizations just joins this one.
func (s *Service) InviteMember(ctx context.Context, actor *Principal, orgID uuid.UUID, in NewMember) (uuid.UUID, error) {
	in.Email = NormalizeEmail(in.Email)
	in.Name = strings.TrimSpace(in.Name)
	in.JobTitle = strings.TrimSpace(in.JobTitle)
	if err := ValidateNewUser(in.Email, in.Name, in.JobTitle); err != nil {
		return uuid.Nil, err
	}
	if !isStaff(in.Role) {
		return uuid.Nil, apperr.Invalid(apperr.CodeInvalidRole, "Choose administrator, supervisor or agent", string(in.Role))
	}
	var id uuid.UUID
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		existing, err := tx.Q.GetUserIDByEmail(ctx, in.Email)
		switch {
		case err == nil:
			id = existing.ID
			return s.addExistingMember(ctx, tx, actor, orgID, existing, in.Role)
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		account := RoleStaff
		if in.Role == RoleAdmin {
			account = RoleAdmin
		}
		if id, err = tx.Q.CreateInvitedUser(ctx, db.CreateInvitedUserParams{
			Email: in.Email, Name: in.Name, Role: string(account), JobTitle: strPtr(in.JobTitle), Status: string(UserStatusInvited),
		}); err != nil {
			return err
		}
		if account == RoleStaff {
			if err := tx.Q.UpsertMembership(ctx, db.UpsertMembershipParams{UserID: id, OrganizationID: orgID, Role: string(in.Role)}); err != nil {
				return err
			}
		}
		if err := audit.Write(ctx, tx.Q, actor.AuditActor().Entry(ActionUserInvited, TargetUser, &id, map[string]any{auditDataRole: in.Role})); err != nil {
			return err
		}
		return s.QueueInvite(ctx, tx, orgID, id, &actor.UserID)
	})
	return id, err
}

// addExistingMember lets a supervisor or agent of another organization join this one.
// Administrators already work everywhere, and requesters belong to their client.
func (s *Service) addExistingMember(ctx context.Context, tx store.Tx, actor *Principal, orgID uuid.UUID,
	u db.GetUserIDByEmailRow, role Role) error {
	if Role(u.Role) != RoleStaff || role == RoleAdmin {
		return errEmailTaken()
	}
	if _, err := tx.Q.GetMembershipForUpdate(ctx, db.GetMembershipForUpdateParams{UserID: u.ID, OrganizationID: orgID}); err == nil {
		return errEmailTaken()
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err := tx.Q.UpsertMembership(ctx, db.UpsertMembershipParams{UserID: u.ID, OrganizationID: orgID, Role: string(role)}); err != nil {
		return err
	}
	return audit.Write(ctx, tx.Q, actor.AuditActor().Entry(ActionMemberAdded, TargetUser, &u.ID, map[string]any{auditDataRole: role}))
}

func errEmailTaken() error {
	return apperr.Conflict(apperr.CodeEmailTaken, "Someone already uses this email", "")
}

// ChangeRole moves a team member to another role: supervisor or agent in this organization, or
// administrator of the install. An agent who stops being one is unlinked from this
// organization's clients; tickets assigned to them stay theirs (owner, 17 Nov).
func (s *Service) ChangeRole(ctx context.Context, actor *Principal, orgID, id uuid.UUID, role Role) error {
	if !isStaff(role) {
		return apperr.Invalid(apperr.CodeInvalidRole, "Choose administrator, supervisor or agent", string(role))
	}
	return s.changeMember(ctx, actor, orgID, id, func(tx store.Tx, u db.GetUserForUpdateRow) (string, map[string]any, error) {
		from, err := roleIn(ctx, tx, u, orgID)
		if err != nil {
			return "", nil, err
		}
		if from == role {
			return "", nil, nil
		}
		switch {
		case from == RoleAdmin:
			if UserStatus(u.Status) == UserStatusActive {
				if err := lastAdminGuard(ctx, tx); err != nil {
					return "", nil, err
				}
			}
			if err := tx.Q.UpdateUserRole(ctx, db.UpdateUserRoleParams{ID: id, Role: string(RoleStaff)}); err != nil {
				return "", nil, err
			}
			if err := tx.Q.UpsertMembership(ctx, db.UpsertMembershipParams{UserID: id, OrganizationID: orgID, Role: string(role)}); err != nil {
				return "", nil, err
			}
		case role == RoleAdmin:
			// Administrators work in every organization, so their memberships (and client
			// links) go.
			if err := tx.Q.UpdateUserRole(ctx, db.UpdateUserRoleParams{ID: id, Role: string(RoleAdmin)}); err != nil {
				return "", nil, err
			}
			if err := tx.Q.DeleteMemberships(ctx, id); err != nil {
				return "", nil, err
			}
		default:
			if err := tx.Q.UpsertMembership(ctx, db.UpsertMembershipParams{UserID: id, OrganizationID: orgID, Role: string(role)}); err != nil {
				return "", nil, err
			}
			if from == RoleAgent {
				if err := tx.Q.DeleteClientAgentsForMember(ctx, db.DeleteClientAgentsForMemberParams{UserID: id, OrganizationID: orgID}); err != nil {
					return "", nil, err
				}
			}
		}
		return ActionRoleChanged, map[string]any{auditDataRole: audit.Change{From: from, To: role}}, nil
	})
}

// RemoveMember takes a supervisor or agent off the organization: their account stays, with any
// other organization they work in, and their client links here go. Open tickets assigned to
// them here must be handed on first.
func (s *Service) RemoveMember(ctx context.Context, actor *Principal, orgID, id uuid.UUID, openStatuses []string) error {
	return s.changeMember(ctx, actor, orgID, id, func(tx store.Tx, u db.GetUserForUpdateRow) (string, map[string]any, error) {
		from, err := roleIn(ctx, tx, u, orgID)
		if err != nil {
			return "", nil, err
		}
		if from == RoleAdmin {
			return "", nil, apperr.Invalid(apperr.CodeInvalidRole, "Administrators work in every organization", "")
		}
		n, err := tx.Q.CountOpenTicketsAssignedIn(ctx, db.CountOpenTicketsAssignedInParams{
			OrganizationID: orgID, UserID: &id, OpenStatuses: openStatuses,
		})
		if err != nil {
			return "", nil, err
		}
		if n > 0 {
			return "", nil, apperr.Conflict(apperr.CodeMemberHasOpenTickets, "Reassign their open tickets first", "")
		}
		if err := tx.Q.DeleteMembership(ctx, db.DeleteMembershipParams{UserID: id, OrganizationID: orgID}); err != nil {
			return "", nil, err
		}
		if err := tx.Q.DeleteMemberTokens(ctx, db.DeleteMemberTokensParams{UserID: id, OrganizationID: orgID}); err != nil {
			return "", nil, err
		}
		return ActionMemberRemoved, map[string]any{auditDataRole: from}, nil
	})
}

// roleIn is the user's role in the organization: administrator, their membership's role, or
// 404 for anyone else (requesters keep theirs).
func roleIn(ctx context.Context, tx store.Tx, u db.GetUserForUpdateRow, orgID uuid.UUID) (Role, error) {
	switch Role(u.Role) {
	case RoleAdmin:
		return RoleAdmin, nil
	case RoleRequester:
		return RoleNone, apperr.Invalid(apperr.CodeInvalidRole, "Client staff keep their role", "")
	}
	m, err := tx.Q.GetMembershipForUpdate(ctx, db.GetMembershipForUpdateParams{UserID: u.ID, OrganizationID: orgID})
	if errors.Is(err, pgx.ErrNoRows) {
		return RoleNone, apperr.NotFound()
	}
	return Role(m), err
}

// Suspend stops someone signing in, at once and in every organization; their open sessions
// end. A suspended agent is unlinked from their clients and keeps the tickets assigned to them.
func (s *Service) Suspend(ctx context.Context, actor *Principal, orgID, id uuid.UUID) error {
	return s.changeMember(ctx, actor, orgID, id, func(tx store.Tx, u db.GetUserForUpdateRow) (string, map[string]any, error) {
		if UserStatus(u.Status) == UserStatusSuspended {
			return "", nil, nil
		}
		if Role(u.Role) == RoleAdmin && UserStatus(u.Status) == UserStatusActive {
			if err := lastAdminGuard(ctx, tx); err != nil {
				return "", nil, err
			}
		}
		if err := tx.Q.SetUserStatus(ctx, db.SetUserStatusParams{ID: id, Status: string(UserStatusSuspended)}); err != nil {
			return "", nil, err
		}
		if err := tx.Q.DeleteUserSessions(ctx, id); err != nil {
			return "", nil, err
		}
		if err := tx.Q.DeleteUserTokens(ctx, id); err != nil {
			return "", nil, err
		}
		if err := tx.Q.InvalidateUserInvites(ctx, id); err != nil {
			return "", nil, err
		}
		if err := tx.Q.DeleteClientAgentsForUser(ctx, id); err != nil {
			return "", nil, err
		}
		return ActionUserSuspended, nil, nil
	})
}

// Reactivate lets a suspended person back in: active when they have a password, else invited
// (resend their invite next).
func (s *Service) Reactivate(ctx context.Context, actor *Principal, orgID, id uuid.UUID) error {
	return s.changeMember(ctx, actor, orgID, id, func(tx store.Tx, u db.GetUserForUpdateRow) (string, map[string]any, error) {
		if UserStatus(u.Status) != UserStatusSuspended {
			return "", nil, nil
		}
		status := UserStatusActive
		if !u.HasPassword {
			status = UserStatusInvited
		}
		if err := tx.Q.SetUserStatus(ctx, db.SetUserStatusParams{ID: id, Status: string(status)}); err != nil {
			return "", nil, err
		}
		return ActionUserReactivated, nil, nil
	})
}

// ResendInvite emails a new link to someone who hasn't set their password yet; earlier links
// stop working when it goes out.
func (s *Service) ResendInvite(ctx context.Context, actor *Principal, orgID, id uuid.UUID) error {
	return s.changeMember(ctx, actor, orgID, id, func(tx store.Tx, u db.GetUserForUpdateRow) (string, map[string]any, error) {
		if UserStatus(u.Status) != UserStatusInvited {
			return "", nil, apperr.Conflict(apperr.CodeNotInvited, "This person has already set their password", "")
		}
		return ActionInviteSent, nil, s.QueueInvite(ctx, tx, orgID, id, &actor.UserID)
	})
}

// changeMember locks the install (for the administrator count) and the person, refuses changes
// to one's own account and to people outside the organization, applies change, and audits it
// when it changed something.
func (s *Service) changeMember(ctx context.Context, actor *Principal, orgID, id uuid.UUID,
	change func(store.Tx, db.GetUserForUpdateRow) (string, map[string]any, error)) error {
	if id == actor.UserID {
		return apperr.Conflict(apperr.CodeCannotChangeSelf, "You can't change your own account here", "")
	}
	return s.store.Tx(ctx, func(tx store.Tx) error {
		if err := tx.Q.LockInstall(ctx); err != nil {
			return err
		}
		in, err := tx.Q.UserInOrganization(ctx, db.UserInOrganizationParams{UserID: id, OrganizationID: orgID, AdminRole: string(RoleAdmin)})
		if err != nil {
			return err
		}
		if !in {
			return apperr.NotFound()
		}
		u, err := tx.Q.GetUserForUpdate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		action, data, err := change(tx, u)
		if err != nil || action == "" {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.AuditActor().Entry(action, TargetUser, &id, data))
	})
}

// lastAdminGuard refuses to leave the install without an active administrator.
func lastAdminGuard(ctx context.Context, tx store.Tx) error {
	n, err := tx.Q.CountActiveAdmins(ctx, db.CountActiveAdminsParams{AdminRole: string(RoleAdmin), Active: string(UserStatusActive)})
	if err != nil {
		return err
	}
	if n <= 1 {
		return apperr.Conflict(apperr.CodeLastAdmin, "Keep at least one active administrator", "")
	}
	return nil
}
