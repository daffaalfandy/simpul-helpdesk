package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/organization"
)

// Login signs in and sets the session cookie.
func (s *server) Login(ctx context.Context, req gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	p, sess, err := s.auth.Login(ctx, req.Body.Email, req.Body.Password)
	if err != nil {
		return nil, err
	}
	me, err := s.me(ctx, p)
	if err != nil {
		return nil, err
	}
	cookie := s.sessionCookie(sess.Token, sess.CookieExpires).String()
	return gen.Login200JSONResponse{Body: me, Headers: gen.Login200ResponseHeaders{SetCookie: &cookie}}, nil
}

// Logout ends the session and clears the cookie.
func (s *server) Logout(ctx context.Context, _ gen.LogoutRequestObject) (gen.LogoutResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, apperr.Unauthenticated()
	}
	if err := s.auth.Logout(ctx, p); err != nil {
		return nil, err
	}
	cookie := s.clearCookie().String()
	return gen.Logout204Response{Headers: gen.Logout204ResponseHeaders{SetCookie: &cookie}}, nil
}

func (s *server) GetMe(ctx context.Context, _ gen.GetMeRequestObject) (gen.GetMeResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, apperr.Unauthenticated()
	}
	me, err := s.me(ctx, p)
	if err != nil {
		return nil, err
	}
	return gen.GetMe200JSONResponse(me), nil
}

// SetMyOrganization remembers where the user works, for their next sign-in.
func (s *server) SetMyOrganization(ctx context.Context, req gen.SetMyOrganizationRequestObject) (gen.SetMyOrganizationResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, apperr.Unauthenticated()
	}
	next, err := s.auth.SetLastOrganization(ctx, p, req.Body.OrganizationId)
	if err != nil {
		return nil, err
	}
	me, err := s.me(ctx, next)
	if err != nil {
		return nil, err
	}
	return gen.SetMyOrganization200JSONResponse(me), nil
}

// ForgotPassword always answers 202 so it can't be used to find accounts.
func (s *server) ForgotPassword(ctx context.Context, req gen.ForgotPasswordRequestObject) (gen.ForgotPasswordResponseObject, error) {
	if err := s.auth.RequestPasswordReset(ctx, req.Body.Email); err != nil {
		return nil, err
	}
	return gen.ForgotPassword202Response{}, nil
}

func (s *server) ResetPassword(ctx context.Context, req gen.ResetPasswordRequestObject) (gen.ResetPasswordResponseObject, error) {
	if err := s.auth.ResetPassword(ctx, req.Body.Token, req.Body.Password); err != nil {
		return nil, err
	}
	return gen.ResetPassword204Response{}, nil
}

// ListUsers lists the active organization's people, with their role there (admin only).
func (s *server) ListUsers(ctx context.Context, _ gen.ListUsersRequestObject) (gen.ListUsersResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapUsersManage)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.Q().ListUsers(ctx, db.ListUsersParams{OrganizationID: p.OrganizationID, AdminRole: string(auth.RoleAdmin)})
	if err != nil {
		return nil, err
	}
	out := make(gen.ListUsers200JSONResponse, len(rows))
	for i, u := range rows {
		out[i] = gen.User{
			Id: u.ID, Email: u.Email, Name: u.Name, Role: gen.Role(u.Role), Status: gen.UserStatus(u.Status),
			JobTitle: u.JobTitle, LastLoginAt: u.LastLoginAt,
		}
		if u.ClientID != nil && u.ClientName != nil {
			out[i].Client = &gen.ClientRef{Id: *u.ClientID, Name: *u.ClientName}
		}
	}
	return out, nil
}

// me is the signed-in user, with the organization they work in and the ones they can pick.
func (s *server) me(ctx context.Context, p *auth.Principal) (gen.Me, error) {
	choices, err := s.auth.Choices(ctx, p)
	if err != nil {
		return gen.Me{}, err
	}
	me := gen.Me{
		Id: p.UserID, Email: p.Email, Name: p.Name, JobTitle: p.JobTitle, IsAdmin: p.Role == auth.RoleAdmin,
		Organizations: make([]gen.OrganizationChoice, len(choices)),
	}
	if p.Role != auth.RoleNone {
		role := gen.Role(p.Role)
		me.Role = &role
	}
	for i, c := range choices {
		me.Organizations[i] = gen.OrganizationChoice{Id: c.OrganizationID, Name: c.Name, Role: gen.Role(c.Role)}
		if logo := organization.LogoOf(c.LogoKey, c.LogoContentType, c.LogoUpdatedAt); logo != nil {
			url := apiPrefix + logo.Path(c.OrganizationID)
			me.Organizations[i].LogoUrl = &url
		}
		if c.OrganizationID == p.OrganizationID {
			me.Organization = &me.Organizations[i]
		}
	}
	if p.ClientID != nil && p.ClientName != nil {
		me.Client = &gen.ClientRef{Id: *p.ClientID, Name: *p.ClientName}
	}
	return me, nil
}
