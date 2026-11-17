package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

const invitePassword = "a-fresh-password-1"

// inviteToken sends the queued invites and returns the token from the last email to addr.
func (e *env) inviteToken(t *testing.T, addr string) string {
	t.Helper()
	runJobs(t, e, e.invite)
	var token string
	for _, m := range e.mailer.Messages() {
		if m.To != addr {
			continue
		}
		_, after, found := strings.Cut(m.Text, auth.ResetTokenFragment)
		if !found || !strings.Contains(m.Text, auth.AcceptInvitePath) {
			t.Fatalf("invite email lacks the link:\n%s", m.Text)
		}
		token = strings.Fields(after)[0]
	}
	if token == "" {
		t.Fatalf("no invite email to %s", addr)
	}
	return token
}

func (e *env) acceptInvite(token, password string) *httpResponse {
	rec := e.do(request{method: http.MethodPost, path: apiBase + "/auth/invite/accept",
		body: gen.AcceptInviteRequest{Token: token, Password: password}})
	return &httpResponse{code: rec.Code, body: rec.Body.String(), cookie: rec.Header().Get("Set-Cookie")}
}

type httpResponse struct {
	code         int
	body, cookie string
}

func member(t *testing.T, m gen.Members, email string) gen.Member {
	t.Helper()
	for _, u := range m.Team {
		if u.Email == email {
			return u
		}
	}
	t.Fatalf("no team member %s", email)
	return gen.Member{}
}

func TestInviteAndAccept(t *testing.T) {
	e := newEnv(t)
	admin := e.as(auth.RoleAdmin)
	expectProblem(t, e.as(auth.RoleSupervisor).do(http.MethodGet, "/members", nil), http.StatusForbidden, apperr.CodeForbidden)

	list := decode[gen.Members](t, admin.do(http.MethodGet, "/members", nil), http.StatusOK)
	if list.Counts.Admin != 1 || list.Counts.Supervisor != 1 || list.Counts.Agent != 1 || list.Counts.Requester != 1 {
		t.Fatalf("counts = %+v", list.Counts)
	}
	for _, u := range list.Team {
		if u.Role == gen.RoleRequester {
			t.Fatalf("client staff listed on the team: %s", u.Email)
		}
	}

	const addr = "yoga@example.test"
	in := gen.MemberInvite{Email: addr, Name: "Yoga Hidayat", Role: gen.StaffRoleAgent}
	if rec := admin.do(http.MethodPost, "/members", in); rec.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body)
	}
	expectProblem(t, admin.do(http.MethodPost, "/members", in), http.StatusConflict, apperr.CodeEmailTaken)
	first := e.inviteToken(t, addr)
	invited := member(t, decode[gen.Members](t, admin.do(http.MethodGet, "/members", nil), http.StatusOK), addr)
	if invited.Status != gen.UserStatusInvited || invited.InviteSentAt == nil {
		t.Fatalf("invited = %+v", invited)
	}
	// Invited people can't sign in yet.
	if rec := e.login(addr, invitePassword); rec.Code != http.StatusUnauthorized {
		t.Fatalf("login before accepting: %d", rec.Code)
	}

	// A resend makes the first link stop working.
	if rec := admin.do(http.MethodPost, "/members/"+invited.Id.String()+"/invite", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("resend: %d %s", rec.Code, rec.Body)
	}
	second := e.inviteToken(t, addr)
	if r := e.acceptInvite(first, invitePassword); r.code != http.StatusBadRequest || !strings.Contains(r.body, apperr.CodeInvalidToken) {
		t.Fatalf("old link: %d %s", r.code, r.body)
	}
	if r := e.acceptInvite(second, "short"); r.code != http.StatusBadRequest {
		t.Fatalf("weak password: %d %s", r.code, r.body)
	}

	r := e.acceptInvite(second, invitePassword)
	if r.code != http.StatusOK || !strings.Contains(r.cookie, "sd_session") || !strings.Contains(r.body, `"role":"agent"`) {
		t.Fatalf("accept: %d %s %s", r.code, r.body, r.cookie)
	}
	if again := e.acceptInvite(second, invitePassword); again.code != http.StatusBadRequest {
		t.Fatalf("link used twice: %d", again.code)
	}
	if rec := e.login(addr, invitePassword); rec.Code != http.StatusOK {
		t.Fatalf("login after accepting: %d", rec.Code)
	}
	if got := member(t, decode[gen.Members](t, admin.do(http.MethodGet, "/members", nil), http.StatusOK), addr); got.Status != gen.UserStatusActive {
		t.Fatalf("status after accepting = %s", got.Status)
	}
	expectProblem(t, admin.do(http.MethodPost, "/members/"+invited.Id.String()+"/invite", nil), http.StatusConflict, apperr.CodeNotInvited)
}

func TestMemberRolesAndSuspension(t *testing.T) {
	te := newTicketEnv(t)
	admin := te.admin
	agentPath := "/members/" + te.agentID.String()
	tk := te.requester.report(t, incident(te.projectA, "Printer offline"))
	decode[gen.Ticket](t, admin.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)

	adminID := userID(t, te.env, auth.RoleAdmin)
	expectProblem(t, admin.do(http.MethodPut, "/members/"+adminID.String()+"/role", gen.MemberRoleInput{Role: gen.StaffRoleAgent}),
		http.StatusConflict, apperr.CodeCannotChangeSelf)
	expectProblem(t, admin.do(http.MethodPost, "/members/"+adminID.String()+"/suspend", nil), http.StatusConflict, apperr.CodeCannotChangeSelf)
	expectProblem(t, admin.do(http.MethodPost, "/members/"+uuid.NewString()+"/suspend", nil), http.StatusNotFound, apperr.CodeNotFound)

	// Suspending the agent ends their session at once and unlinks their clients; the ticket
	// assigned to them stays theirs.
	if rec := admin.do(http.MethodPost, agentPath+"/suspend", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("suspend: %d %s", rec.Code, rec.Body)
	}
	if rec := te.agent.do(http.MethodGet, "/auth/me", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("suspended session still works: %d", rec.Code)
	}
	if n := te.count(t, `SELECT count(*) FROM client_agents WHERE user_id = $1`, te.agentID); n != 0 {
		t.Fatalf("client links after suspending = %d", n)
	}
	if got := te.admin.ticket(t, tk.Key); got.Assignee == nil || got.Assignee.Id != te.agentID {
		t.Fatalf("assignee after suspending = %+v", got.Assignee)
	}
	if rec := admin.do(http.MethodPost, agentPath+"/reactivate", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("reactivate: %d %s", rec.Code, rec.Body)
	}
	if rec := te.login(te.users[auth.RoleAgent], password); rec.Code != http.StatusOK {
		t.Fatalf("login after reactivating: %d", rec.Code)
	}

	// An agent moved to supervisor sees every client; their links go.
	decode[gen.Client](t, admin.do(http.MethodPut, "/clients/"+te.clientA.String()+"/agents",
		gen.ClientAgentsInput{UserIds: []uuid.UUID{te.agentID}}), http.StatusOK)
	if rec := admin.do(http.MethodPut, agentPath+"/role", gen.MemberRoleInput{Role: gen.StaffRoleSupervisor}); rec.Code != http.StatusNoContent {
		t.Fatalf("role: %d %s", rec.Code, rec.Body)
	}
	if n := te.count(t, `SELECT count(*) FROM client_agents WHERE user_id = $1`, te.agentID); n != 0 {
		t.Fatalf("client links after the role change = %d", n)
	}
	for _, action := range []string{auth.ActionUserSuspended, auth.ActionUserReactivated, auth.ActionRoleChanged} {
		if n := te.count(t, `SELECT count(*) FROM audit_log WHERE action = $1 AND target_id = $2`, action, te.agentID); n != 1 {
			t.Errorf("%s audit entries = %d", action, n)
		}
	}
}
