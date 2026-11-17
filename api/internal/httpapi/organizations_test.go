package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

// secondOrganization adds another vendor organization, with its defaults, and returns its id.
func (e *env) secondOrganization(t *testing.T, name, prefix string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	q := e.st.Q()
	id, err := q.CreateOrganization(ctx, db.CreateOrganizationParams{Name: name, TicketPrefix: prefix, Timezone: "Asia/Jayapura"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.InsertDefaultPriorities(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := q.InsertDefaultBusinessHours(ctx, id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *env) member(t *testing.T, role auth.Role, org uuid.UUID, as auth.Role) {
	t.Helper()
	if err := e.st.Q().UpsertMembership(context.Background(), db.UpsertMembershipParams{
		UserID: userID(t, e, role), OrganizationID: org, Role: string(as),
	}); err != nil {
		t.Fatal(err)
	}
}

func (s session) me(t *testing.T) gen.Me {
	t.Helper()
	return decode[gen.Me](t, s.do(http.MethodGet, "/auth/me", nil), http.StatusOK)
}

func roleOf(m gen.Me) gen.Role {
	if m.Role == nil {
		return ""
	}
	return *m.Role
}

func orgOf(m gen.Me) uuid.UUID {
	if m.Organization == nil {
		return uuid.Nil
	}
	return m.Organization.Id
}

// Rina is an agent at one vendor and a supervisor at another; each request works in the
// organization it names.
func TestRolesPerOrganization(t *testing.T) {
	te := newTicketEnv(t)
	b := te.secondOrganization(t, "PT Mitra Data", "MD")
	te.member(t, auth.RoleAgent, b, auth.RoleSupervisor)

	home := te.agent.me(t)
	if orgOf(home) != te.org || roleOf(home) != gen.RoleAgent || len(home.Organizations) != 2 || home.IsAdmin {
		t.Fatalf("agent at home = %+v", home)
	}
	there := te.agent.in(b).me(t)
	if orgOf(there) != b || roleOf(there) != gen.RoleSupervisor {
		t.Fatalf("agent in the other organization = %+v", there)
	}
	// Supervisors see every client of their organization; this one has none yet.
	if clients := decode[[]gen.Client](t, te.agent.in(b).do(http.MethodGet, "/clients", nil), http.StatusOK); len(clients) != 0 {
		t.Fatalf("clients of the other organization = %+v", clients)
	}
	// An organization the user doesn't belong to is ignored; they stay where they were.
	if got := te.agent.in(uuid.New()).me(t); orgOf(got) != te.org {
		t.Fatalf("unknown organization = %+v", got)
	}
	// Requesters always work in their client's organization.
	if got := te.requester.in(b).me(t); orgOf(got) != te.org || roleOf(got) != gen.RoleRequester {
		t.Fatalf("requester asking for another organization = %+v", got)
	}
	// Administrators work anywhere, and see every organization.
	admin := te.admin.in(b).me(t)
	if orgOf(admin) != b || roleOf(admin) != gen.RoleAdmin || !admin.IsAdmin || len(admin.Organizations) != 2 {
		t.Fatalf("admin = %+v", admin)
	}
}

// One organization's tickets, clients and settings never show in another.
func TestOrganizationsDontSeeEachOther(t *testing.T) {
	te := newTicketEnv(t)
	b := te.secondOrganization(t, "PT Mitra Data", "MD")
	tk := te.requester.report(t, incident(te.projectA, "Printer offline"))
	admin := te.admin.in(b)

	// The ticket itself answers with its organization (TestTicketOfAnotherOrganization).
	expectProblem(t, admin.do(http.MethodGet, "/tickets/"+tk.Key, nil), http.StatusConflict, apperr.CodeOtherOrganization)
	expectProblem(t, admin.do(http.MethodGet, "/clients/"+te.clientA.String(), nil), http.StatusNotFound, apperr.CodeNotFound)
	expectProblem(t, admin.do(http.MethodGet, "/projects/"+te.projectA.String(), nil), http.StatusNotFound, apperr.CodeNotFound)
	if list := decode[[]gen.TicketSummary](t, admin.do(http.MethodGet, "/tickets", nil), http.StatusOK); len(list) != 0 {
		t.Fatalf("tickets of the other organization = %+v", list)
	}
	if b := admin.board(t, ""); b.Stats.OpenTickets != 0 {
		t.Fatalf("board of the other organization counts %d open tickets", b.Stats.OpenTickets)
	}
	org := decode[gen.Organization](t, admin.do(http.MethodGet, "/organization", nil), http.StatusOK)
	if org.Name != "PT Mitra Data" || org.TicketPrefix != "MD" || org.Timezone != "Asia/Jayapura" {
		t.Fatalf("settings of the other organization = %+v", org)
	}
}

// Staff who belong to no organization can only wait to be added to one.
func TestStaffWithoutOrganization(t *testing.T) {
	e := newEnv(t)
	if err := e.st.Q().DeleteMemberships(context.Background(), userID(t, e, auth.RoleAgent)); err != nil {
		t.Fatal(err)
	}
	agent := e.as(auth.RoleAgent)
	me := agent.me(t)
	if me.Role != nil || me.Organization != nil || len(me.Organizations) != 0 {
		t.Fatalf("me = %+v", me)
	}
	expectProblem(t, agent.do(http.MethodGet, "/board", nil), http.StatusConflict, apperr.CodeOrganizationRequired)
}

// The organization picked last is where the next sign-in starts.
func TestRememberedOrganization(t *testing.T) {
	e := newEnv(t)
	b := e.secondOrganization(t, "PT Mitra Data", "MD")
	e.member(t, auth.RoleAgent, b, auth.RoleAgent)
	agent := e.as(auth.RoleAgent)

	expectProblem(t, agent.do(http.MethodPut, "/me/organization", gen.SetMyOrganizationJSONRequestBody{OrganizationId: uuid.New()}),
		http.StatusNotFound, apperr.CodeNotFound)
	got := decode[gen.Me](t, agent.do(http.MethodPut, "/me/organization", gen.SetMyOrganizationJSONRequestBody{OrganizationId: b}), http.StatusOK)
	if orgOf(got) != b {
		t.Fatalf("after picking = %+v", got)
	}
	again := session{e: e, cookie: sessionCookie(t, e.login(e.users[auth.RoleAgent], password))}
	if got := again.me(t); orgOf(got) != b {
		t.Fatalf("next sign-in works in %s, want %s", orgOf(got), b)
	}
	expectProblem(t, e.as(auth.RoleRequester).do(http.MethodPut, "/me/organization", gen.SetMyOrganizationJSONRequestBody{OrganizationId: b}),
		http.StatusForbidden, apperr.CodeForbidden)
}

// An email link to a ticket of another organization the user works in names it, so the app can
// offer to switch; someone outside that organization learns nothing.
func TestTicketOfAnotherOrganization(t *testing.T) {
	te := newTicketEnv(t)
	b := te.secondOrganization(t, "PT Mitra Data", "MD")
	tk := te.requester.report(t, incident(te.projectA, "Printer offline"))

	rec := te.admin.in(b).do(http.MethodGet, "/tickets/"+tk.Key, nil)
	expectProblem(t, rec, http.StatusConflict, apperr.CodeOtherOrganization)
	if p := decode[gen.Problem](t, rec, http.StatusConflict); p.Detail == nil || *p.Detail != te.org.String() {
		t.Fatalf("problem = %+v, want the ticket's organization", p)
	}
	// A supervisor of the other organization only doesn't learn the key exists.
	te.member(t, auth.RoleSupervisor, b, auth.RoleSupervisor)
	if err := te.st.Q().DeleteMembership(context.Background(), db.DeleteMembershipParams{
		UserID: userID(t, te.env, auth.RoleSupervisor), OrganizationID: te.org,
	}); err != nil {
		t.Fatal(err)
	}
	expectProblem(t, te.supervisor.do(http.MethodGet, "/tickets/"+tk.Key, nil), http.StatusNotFound, apperr.CodeNotFound)
}

// Settings and records of one organization can't be changed from another.
func TestSettingsStayInTheirOrganization(t *testing.T) {
	te := newTicketEnv(t)
	b := te.secondOrganization(t, "PT Mitra Data", "MD")
	admin := te.admin.in(b)
	expectProblem(t, admin.do(http.MethodPut, "/clients/"+te.clientA.String(), gen.ClientInput{Name: "Renamed"}),
		http.StatusNotFound, apperr.CodeNotFound)
	expectProblem(t, admin.do(http.MethodDelete, "/projects/"+te.projectA.String(), nil), http.StatusNotFound, apperr.CodeNotFound)
	expectProblem(t, admin.do(http.MethodPut, "/clients/"+te.clientA.String()+"/agents",
		gen.ClientAgentsInput{UserIds: []uuid.UUID{te.agentID}}), http.StatusNotFound, apperr.CodeNotFound)
	// The agent isn't one of this organization's agents, so no client here can be given to them.
	c := decode[gen.Client](t, admin.do(http.MethodPost, "/clients", gen.ClientInput{Name: "Bappeda Riau"}), http.StatusCreated)
	expectProblem(t, admin.do(http.MethodPut, "/clients/"+c.Id.String()+"/agents",
		gen.ClientAgentsInput{UserIds: []uuid.UUID{te.agentID}}), http.StatusBadRequest, apperr.CodeInvalidAgent)
	// Same client name, another organization: allowed.
	decode[gen.Client](t, te.admin.do(http.MethodPost, "/clients", gen.ClientInput{Name: "Bappeda Riau"}), http.StatusCreated)
}

// Someone who leaves an organization stops hearing about its tickets, even one still assigned
// to them.
func TestNoNotificationsAfterLeavingTheOrganization(t *testing.T) {
	te := newTicketEnv(t)
	tk := te.requester.report(t, incident(te.projectA, "Printer offline"))
	decode[gen.Ticket](t, te.admin.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	te.fanout(t)
	if err := te.st.Q().DeleteMembership(context.Background(), db.DeleteMembershipParams{UserID: te.agentID, OrganizationID: te.org}); err != nil {
		t.Fatal(err)
	}
	before := len(te.notices(t, tk.Id))
	decode[gen.TicketMessage](t, te.requester.message(tk.Key, gen.MessageVisibilityPublic, "Still broken"), http.StatusCreated)
	te.fanout(t)
	for _, n := range te.notices(t, tk.Id)[before:] {
		if n.user == te.agentID {
			t.Fatalf("former member notified: %+v", n)
		}
	}
}

// An invite carries the name of the organization it comes from.
func TestInviteNamesTheOrganization(t *testing.T) {
	e := newEnv(t)
	b := e.secondOrganization(t, "PT Mitra Data", "MD")
	const addr = "putri@example.test"
	if rec := e.as(auth.RoleAdmin).in(b).do(http.MethodPost, "/members", gen.MemberInvite{Email: addr, Name: "Putri", Role: gen.StaffRoleAgent}); rec.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body)
	}
	e.inviteToken(t, addr)
	for _, m := range e.mailer.Messages() {
		if m.To == addr && !strings.Contains(m.Subject, "PT Mitra Data") {
			t.Fatalf("invite subject = %q", m.Subject)
		}
	}
	// They joined the organization that invited them, not the first one.
	r := e.acceptInvite(e.inviteToken(t, addr), invitePassword)
	if r.code != http.StatusOK || !strings.Contains(r.body, `"name":"PT Mitra Data"`) {
		t.Fatalf("accept: %d %s", r.code, r.body)
	}
}

// Administrators list and add organizations; a new one starts with the defaults.
func TestCreateOrganization(t *testing.T) {
	e := newEnv(t)
	admin := e.as(auth.RoleAdmin)
	in := gen.OrganizationInput{Name: "PT Mitra Data", TicketPrefix: "md", Timezone: "Asia/Makassar"}
	expectProblem(t, e.as(auth.RoleSupervisor).do(http.MethodPost, "/organizations", in), http.StatusForbidden, apperr.CodeForbidden)
	created := decode[gen.OrganizationSummary](t, admin.do(http.MethodPost, "/organizations", in), http.StatusCreated)
	if created.TicketPrefix != "MD" || created.Timezone != "Asia/Makassar" {
		t.Fatalf("created = %+v", created)
	}
	expectProblem(t, admin.do(http.MethodPost, "/organizations", in), http.StatusConflict, apperr.CodeTicketPrefixTaken)
	list := decode[[]gen.OrganizationSummary](t, admin.do(http.MethodGet, "/organizations", nil), http.StatusOK)
	if len(list) != 2 {
		t.Fatalf("organizations = %+v", list)
	}
	levels := decode[[]gen.PriorityLevel](t, admin.in(created.Id).do(http.MethodGet, "/priorities", nil), http.StatusOK)
	hours := decode[gen.BusinessHours](t, admin.in(created.Id).do(http.MethodGet, "/business-hours", nil), http.StatusOK)
	if len(levels) != 4 || len(hours.Weekdays) != 5 || hours.Timezone != "Asia/Makassar" {
		t.Fatalf("defaults: %d priorities, hours %+v", len(levels), hours)
	}
}

// Taking someone off an organization keeps their account; their open tickets there go first.
func TestRemoveMember(t *testing.T) {
	te := newTicketEnv(t)
	b := te.secondOrganization(t, "PT Mitra Data", "MD")
	te.member(t, auth.RoleAgent, b, auth.RoleAgent)
	tk := te.requester.report(t, incident(te.projectA, "Printer offline"))
	decode[gen.Ticket](t, te.admin.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	path := "/members/" + te.agentID.String()

	expectProblem(t, te.admin.do(http.MethodDelete, path, nil), http.StatusConflict, apperr.CodeMemberHasOpenTickets)
	expectProblem(t, te.admin.do(http.MethodDelete, "/members/"+userID(t, te.env, auth.RoleAdmin).String(), nil),
		http.StatusConflict, apperr.CodeCannotChangeSelf)
	if rec := te.admin.in(b).do(http.MethodDelete, path, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body)
	}
	me := te.agent.me(t)
	if len(me.Organizations) != 1 || orgOf(me) != te.org {
		t.Fatalf("after removal = %+v", me)
	}
	expectProblem(t, te.admin.in(b).do(http.MethodDelete, path, nil), http.StatusNotFound, apperr.CodeNotFound)
}
