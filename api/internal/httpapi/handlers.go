package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

// GetOrganization returns public branding.
func (s *server) GetOrganization(ctx context.Context, _ gen.GetOrganizationRequestObject) (gen.GetOrganizationResponseObject, error) {
	org, err := s.store.Q().GetOrganization(ctx)
	if err != nil {
		return nil, err
	}
	return gen.GetOrganization200JSONResponse{
		Name: org.Name, LogoUrl: org.LogoUrl, FooterNote: org.FooterNote, Timezone: org.Timezone,
	}, nil
}

// Login signs in and sets the session cookie.
func (s *server) Login(ctx context.Context, req gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	p, sess, err := s.auth.Login(ctx, req.Body.Email, req.Body.Password)
	if err != nil {
		return nil, err
	}
	cookie := s.sessionCookie(sess.Token, sess.CookieExpires).String()
	return gen.Login200JSONResponse{Body: meFrom(p), Headers: gen.Login200ResponseHeaders{SetCookie: &cookie}}, nil
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
	return gen.GetMe200JSONResponse(meFrom(p)), nil
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

// ListUsers lists everyone (admin only).
func (s *server) ListUsers(ctx context.Context, _ gen.ListUsersRequestObject) (gen.ListUsersResponseObject, error) {
	if _, err := auth.Require(ctx, auth.CapUsersManage); err != nil {
		return nil, err
	}
	rows, err := s.store.Q().ListUsers(ctx)
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

func meFrom(p *auth.Principal) gen.Me {
	me := gen.Me{Id: p.UserID, Email: p.Email, Name: p.Name, Role: gen.Role(p.Role), JobTitle: p.JobTitle}
	if p.ClientID != nil && p.ClientName != nil {
		me.Client = &gen.ClientRef{Id: *p.ClientID, Name: *p.ClientName}
	}
	return me
}
