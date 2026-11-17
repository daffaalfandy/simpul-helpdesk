package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
)

// account is what users.role stores: an install-wide administrator, staff (whose role comes
// from their membership in each organization), or a requester of one client.
type account struct {
	role          Role
	clientOrg     *uuid.UUID // requesters: their client's organization
	lastOrg       *uuid.UUID // where the user last worked
	requestedOrgs *uuid.UUID // the organization the request asks for (X-Organization-Id)
}

// resolveOrganization sets the organization p works in and p's role there:
//   - a requester works in their client's organization;
//   - an administrator in the requested organization, else their last one, else the first;
//   - staff in the requested organization if they belong to it, else their last one, else the
//     one they joined first, with that membership's role. Staff with no membership get no
//     organization and no role: every organization-scoped request answers organization_required.
func resolveOrganization(ctx context.Context, q *db.Queries, p *Principal, a account) error {
	switch a.role {
	case RoleRequester:
		p.Role = RoleRequester
		if a.clientOrg != nil {
			p.OrganizationID = *a.clientOrg
		}
		return nil
	case RoleAdmin:
		p.Role = RoleAdmin
		for _, want := range []*uuid.UUID{a.requestedOrgs, a.lastOrg} {
			if want == nil {
				continue
			}
			_, err := q.GetOrganization(ctx, *want)
			if err == nil {
				p.OrganizationID = *want
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		first, err := q.GetFirstOrganization(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		p.OrganizationID = first.ID
		return err
	}
	memberships, err := q.ListUserMemberships(ctx, p.UserID)
	if err != nil {
		return err
	}
	p.Role, p.OrganizationID = RoleNone, uuid.Nil
	for _, want := range []*uuid.UUID{a.requestedOrgs, a.lastOrg} {
		for _, m := range memberships {
			if want != nil && m.OrganizationID == *want {
				p.Role, p.OrganizationID = Role(m.Role), m.OrganizationID
				return nil
			}
		}
	}
	var first *db.ListUserMembershipsRow
	for i := range memberships {
		if first == nil || memberships[i].CreatedAt.Before(first.CreatedAt) {
			first = &memberships[i]
		}
	}
	if first != nil {
		p.Role, p.OrganizationID = Role(first.Role), first.OrganizationID
	}
	return nil
}
