package auth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
)

// Choice is an organization the user can work in, with their role there.
type Choice struct {
	OrganizationID  uuid.UUID
	Name            string
	Role            Role
	LogoKey         *string
	LogoContentType *string
	LogoUpdatedAt   *time.Time
}

// Choices lists where p can work, by name: every organization for an administrator, their
// memberships for staff, their client's organization for a requester.
func (s *Service) Choices(ctx context.Context, p *Principal) ([]Choice, error) {
	q := s.store.Q()
	switch {
	case p.Role == RoleAdmin:
		rows, err := q.ListOrganizations(ctx)
		out := make([]Choice, len(rows))
		for i, r := range rows {
			out[i] = Choice{OrganizationID: r.ID, Name: r.Name, Role: RoleAdmin, LogoKey: r.LogoKey,
				LogoContentType: r.LogoContentType, LogoUpdatedAt: r.LogoUpdatedAt}
		}
		return out, err
	case p.Role == RoleRequester:
		if p.OrganizationID == uuid.Nil {
			return []Choice{}, nil
		}
		o, err := q.GetOrganization(ctx, p.OrganizationID)
		if err != nil {
			return nil, err
		}
		return []Choice{{OrganizationID: o.ID, Name: o.Name, Role: RoleRequester, LogoKey: o.LogoKey,
			LogoContentType: o.LogoContentType, LogoUpdatedAt: o.LogoUpdatedAt}}, nil
	}
	rows, err := q.ListUserMemberships(ctx, p.UserID)
	out := make([]Choice, len(rows))
	for i, r := range rows {
		out[i] = Choice{OrganizationID: r.OrganizationID, Name: r.Name, Role: Role(r.Role), LogoKey: r.LogoKey,
			LogoContentType: r.LogoContentType, LogoUpdatedAt: r.LogoUpdatedAt}
	}
	return out, err
}

// SetLastOrganization makes orgID where p's next sign-in starts and returns p working there.
// Requesters always work in their client's organization (403); an organization p can't work in
// is a 404.
func (s *Service) SetLastOrganization(ctx context.Context, p *Principal, orgID uuid.UUID) (*Principal, error) {
	if p.Role == RoleRequester {
		return nil, apperr.Forbidden()
	}
	q := s.store.Q()
	user, err := q.GetUserByID(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	next := &Principal{SessionID: p.SessionID, UserID: p.UserID, Email: p.Email, Name: p.Name, JobTitle: p.JobTitle}
	if err := resolveOrganization(ctx, q, next, account{role: Role(user.Role), requestedOrgs: &orgID}); err != nil {
		return nil, err
	}
	if next.OrganizationID != orgID {
		return nil, apperr.NotFound()
	}
	if err := q.SetLastOrganization(ctx, db.SetLastOrganizationParams{ID: p.UserID, OrganizationID: &orgID}); err != nil {
		return nil, err
	}
	return next, nil
}
