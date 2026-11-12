package auth

import (
	"context"

	"github.com/google/uuid"
)

// Role is a user's role in the organisation.
type Role string

// Roles, from HANDOFF §3. Values match the users.role CHECK constraint.
const (
	RoleAdmin      Role = "admin"
	RoleSupervisor Role = "supervisor"
	RoleAgent      Role = "agent"
	RoleRequester  Role = "requester"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleSupervisor, RoleAgent, RoleRequester:
		return true
	}
	return false
}

// UserStatus is a user's account state. Values match the users.status CHECK constraint.
type UserStatus string

// User statuses.
const (
	UserStatusActive    UserStatus = "active"
	UserStatusInvited   UserStatus = "invited"
	UserStatusSuspended UserStatus = "suspended"
)

// IsActive reports whether a stored status string means the user may sign in.
func IsActive(status string) bool { return UserStatus(status) == UserStatusActive }

// Principal is the signed-in user for the current request.
type Principal struct {
	SessionID  uuid.UUID
	UserID     uuid.UUID
	Email      string
	Name       string
	Role       Role
	JobTitle   *string
	ClientID   *uuid.UUID // requesters only
	ClientName *string
}

type principalKey struct{}

// WithPrincipal returns a context carrying p.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the signed-in user, if any.
func PrincipalFrom(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(*Principal)
	return p, ok && p != nil
}
