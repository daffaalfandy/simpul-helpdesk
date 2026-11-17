package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

// AcceptInvite sets the first password from an invite link and signs the person in.
func (s *server) AcceptInvite(ctx context.Context, req gen.AcceptInviteRequestObject) (gen.AcceptInviteResponseObject, error) {
	p, sess, err := s.auth.AcceptInvite(ctx, req.Body.Token, req.Body.Password)
	if err != nil {
		return nil, err
	}
	me, err := s.me(ctx, p)
	if err != nil {
		return nil, err
	}
	cookie := s.sessionCookie(sess.Token, sess.CookieExpires).String()
	return gen.AcceptInvite200JSONResponse{Body: me, Headers: gen.AcceptInvite200ResponseHeaders{SetCookie: &cookie}}, nil
}

func (s *server) ListMembers(ctx context.Context, _ gen.ListMembersRequestObject) (gen.ListMembersResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapUsersManage)
	if err != nil {
		return nil, err
	}
	m, err := s.auth.Members(ctx, p.OrganizationID)
	if err != nil {
		return nil, err
	}
	out := gen.ListMembers200JSONResponse{
		Counts: gen.MemberCounts{
			Admin: m.Counts[auth.RoleAdmin], Supervisor: m.Counts[auth.RoleSupervisor],
			Agent: m.Counts[auth.RoleAgent], Requester: m.Counts[auth.RoleRequester],
		},
		Team: make([]gen.Member, len(m.Team)),
	}
	for i, u := range m.Team {
		out.Team[i] = gen.Member{
			Id: u.ID, Email: u.Email, Name: u.Name, Role: gen.Role(u.Role), JobTitle: u.JobTitle,
			Status: gen.UserStatus(u.Status), LastLoginAt: u.LastLoginAt, InviteSentAt: u.InviteSentAt,
		}
	}
	return out, nil
}

func (s *server) InviteMember(ctx context.Context, req gen.InviteMemberRequestObject) (gen.InviteMemberResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapUsersManage)
	if err != nil {
		return nil, err
	}
	in := auth.NewMember{Email: string(req.Body.Email), Name: req.Body.Name, Role: auth.Role(req.Body.Role)}
	if req.Body.JobTitle != nil {
		in.JobTitle = *req.Body.JobTitle
	}
	if _, err := s.auth.InviteMember(ctx, p, p.OrganizationID, in); err != nil {
		return nil, err
	}
	return gen.InviteMember201Response{}, nil
}

func (s *server) ChangeMemberRole(ctx context.Context, req gen.ChangeMemberRoleRequestObject) (gen.ChangeMemberRoleResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapUsersManage)
	if err != nil {
		return nil, err
	}
	if err := s.auth.ChangeRole(ctx, p, p.OrganizationID, req.UserId, auth.Role(req.Body.Role)); err != nil {
		return nil, err
	}
	return gen.ChangeMemberRole204Response{}, nil
}

func (s *server) SuspendMember(ctx context.Context, req gen.SuspendMemberRequestObject) (gen.SuspendMemberResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapUsersManage)
	if err != nil {
		return nil, err
	}
	if err := s.auth.Suspend(ctx, p, p.OrganizationID, req.UserId); err != nil {
		return nil, err
	}
	return gen.SuspendMember204Response{}, nil
}

func (s *server) ReactivateMember(ctx context.Context, req gen.ReactivateMemberRequestObject) (gen.ReactivateMemberResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapUsersManage)
	if err != nil {
		return nil, err
	}
	if err := s.auth.Reactivate(ctx, p, p.OrganizationID, req.UserId); err != nil {
		return nil, err
	}
	return gen.ReactivateMember204Response{}, nil
}

func (s *server) ResendInvite(ctx context.Context, req gen.ResendInviteRequestObject) (gen.ResendInviteResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapUsersManage)
	if err != nil {
		return nil, err
	}
	if err := s.auth.ResendInvite(ctx, p, p.OrganizationID, req.UserId); err != nil {
		return nil, err
	}
	return gen.ResendInvite204Response{}, nil
}

// CompleteSetup is first-run setup: it names the organisation and signs its first admin in.
func (s *server) CompleteSetup(ctx context.Context, req gen.CompleteSetupRequestObject) (gen.CompleteSetupResponseObject, error) {
	p, sess, err := s.auth.Setup(ctx, auth.SetupInput{
		OrganizationName: req.Body.OrganizationName, Name: req.Body.Name, Email: req.Body.Email, Password: req.Body.Password,
	})
	if err != nil {
		return nil, err
	}
	me, err := s.me(ctx, p)
	if err != nil {
		return nil, err
	}
	cookie := s.sessionCookie(sess.Token, sess.CookieExpires).String()
	return gen.CompleteSetup200JSONResponse{Body: me, Headers: gen.CompleteSetup200ResponseHeaders{SetCookie: &cookie}}, nil
}
