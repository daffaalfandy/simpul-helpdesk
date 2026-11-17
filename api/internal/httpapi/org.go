package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
)

// requireOrg is auth.Require for operations on the active organization's data: someone with no
// organization to work in gets 409 organization_required instead of an empty answer.
func requireOrg(ctx context.Context, c auth.Capability) (*auth.Principal, error) {
	p, err := auth.Require(ctx, c)
	if err != nil {
		return nil, err
	}
	return withOrg(p)
}

// signedInOrg is requireOrg for any signed-in user.
func signedInOrg(ctx context.Context) (*auth.Principal, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, apperr.Unauthenticated()
	}
	return withOrg(p)
}

func withOrg(p *auth.Principal) (*auth.Principal, error) {
	if p.OrganizationID == uuid.Nil {
		return nil, apperr.OrganizationRequired()
	}
	return p, nil
}
