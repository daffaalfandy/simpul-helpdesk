package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

const secondColleagueEmail = "colleague2@example.test"

// requesterProjectsPath is the administrator's endpoint for one requester of client A.
func (te *ticketEnv) requesterProjectsPath(user uuid.UUID) string {
	return "/clients/" + te.clientA.String() + "/requesters/" + user.String() + "/projects"
}

// link sets which of client A's projects a requester reports on, as the administrator.
func (te *ticketEnv) link(t *testing.T, user uuid.UUID, projects ...uuid.UUID) gen.Requester {
	t.Helper()
	return decode[gen.Requester](t, te.admin.do(http.MethodPut, te.requesterProjectsPath(user),
		gen.RequesterProjectsInput{ProjectIds: append([]uuid.UUID{}, projects...)}), http.StatusOK)
}

// sameIDs reports whether got holds exactly want, in any order.
func sameIDs(got []uuid.UUID, want ...uuid.UUID) bool {
	byString := func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) }
	return slices.Equal(slices.SortedFunc(slices.Values(got), byString), slices.SortedFunc(slices.Values(want), byString))
}

// projectsChange is the audit data of user.projects_changed.
type projectsChange struct {
	Client  uuid.UUID   `json:"client_id"`
	Added   []uuid.UUID `json:"added"`
	Removed []uuid.UUID `json:"removed"`
}

// countFor counts the notifications that went to user.
func countFor(list []notice, user uuid.UUID) int {
	n := 0
	for _, x := range list {
		if x.user == user {
			n++
		}
	}
	return n
}

func ticketKeys(list []gen.TicketSummary) []string {
	out := make([]string, len(list))
	for i, tk := range list {
		out[i] = tk.Key
	}
	return out
}

// A requester narrowed to one project loses a colleague's ticket on another project everywhere
// (owner, 18 Nov), keeps the tickets they reported, and stops hearing about the one they follow.
func TestRequesterSeesOnlyTheirProjects(t *testing.T) {
	te := newTicketEnv(t)
	testdb.SetProjectDoD(t, te.st, te.projectA2, uatDoD())
	colleague, _ := te.addColleague(t)
	requesterEmail := te.users[auth.RoleRequester]

	file := colleague.attach(t, "mailbox.png")
	in := incident(te.projectA2, "Mailbox full")
	in.AttachmentIds = &[]uuid.UUID{file.Id}
	theirs := colleague.report(t, in)
	decode[[]gen.Colleague](t, colleague.do(http.MethodPost, "/tickets/"+theirs.Key+"/watchers",
		gen.WatcherInput{UserId: te.requesterID}), http.StatusOK)
	own := te.requester.report(t, incident(te.projectA2, "Signature missing"))
	shared := colleague.report(t, incident(te.projectA, "Letters are slow"))
	te.toUAT(t, theirs.Key)
	decode[gen.TicketMessage](t, te.agent.message(own.Key, gen.MessageVisibilityPublic, "Which signature?"), http.StatusCreated)
	te.fanout(t)

	path := "/tickets/" + theirs.Key
	decode[gen.Ticket](t, te.requester.do(http.MethodGet, path, nil), http.StatusOK)
	if got := te.requester.uatRequests(t); len(got) != 1 || got[0].Key != theirs.Key {
		t.Fatalf("followed test before narrowing = %+v", got)
	}
	about := func(page gen.NotificationPage, key string) int {
		n := 0
		for _, it := range page.Items {
			if it.Ticket.Key == key {
				n++
			}
		}
		return n
	}
	before := te.requester.inbox(t, "")
	followed := about(before, theirs.Key)
	if followed == 0 || about(before, own.Key) == 0 {
		t.Fatalf("inbox before narrowing = %+v", before.Items)
	}
	unread := te.requester.unread(t)

	te.link(t, te.requesterID, te.projectA)

	for name, rec := range map[string]int{
		"ticket":       te.requester.do(http.MethodGet, path, nil).Code,
		"messages":     te.requester.do(http.MethodGet, path+"/messages", nil).Code,
		"reply":        te.requester.message(theirs.Key, gen.MessageVisibilityPublic, "Me too").Code,
		"attachment":   te.requester.do(http.MethodGet, "/attachments/"+file.Id.String(), nil).Code,
		"watchers":     te.requester.do(http.MethodGet, path+"/watchers", nil).Code,
		"candidates":   te.requester.do(http.MethodGet, path+"/watcher-candidates", nil).Code,
		"test answer":  te.requester.answer(theirs.Key, gen.UatAnswerApprove, "").Code,
		"reopen (UAT)": te.requester.reopen(theirs.Key).Code,
	} {
		if rec != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", name, rec)
		}
	}
	keys := ticketKeys(decode[[]gen.TicketSummary](t, te.requester.do(http.MethodGet, "/tickets", nil), http.StatusOK))
	if slices.Contains(keys, theirs.Key) || !slices.Contains(keys, own.Key) || !slices.Contains(keys, shared.Key) {
		t.Fatalf("list = %v, want %s and %s without %s", keys, own.Key, shared.Key, theirs.Key)
	}
	// Their own ticket on that project stays theirs.
	decode[gen.Ticket](t, te.requester.do(http.MethodGet, "/tickets/"+own.Key, nil), http.StatusOK)
	decode[gen.TicketMessage](t, te.requester.message(own.Key, gen.MessageVisibilityPublic, "The new one"), http.StatusCreated)
	if got := te.requester.uatRequests(t); len(got) != 0 {
		t.Fatalf("a followed test on an unlinked project = %+v", got)
	}

	// The inbox and the bell drop what's about it, and no email goes out about it.
	if n := about(te.requester.inbox(t, ""), theirs.Key); n != 0 {
		t.Fatalf("inbox keeps %d items about %s", n, theirs.Key)
	}
	if got := te.requester.unread(t); got != unread-followed {
		t.Fatalf("unread = %d, want %d", got, unread-followed)
	}
	te.age(t, notification.GroupWindow)
	te.sendEmails(t)
	sent := subjects(mailsTo(te.mailer.Messages(), requesterEmail))
	if !slices.ContainsFunc(sent, func(s string) bool { return strings.Contains(s, own.Key) }) ||
		slices.ContainsFunc(sent, func(s string) bool { return strings.Contains(s, theirs.Key) }) {
		t.Fatalf("requester emails = %v, want %s only", sent, own.Key)
	}

	// The colleague goes on alone: the unlinked follower is neither listed nor told.
	if got := decode[[]gen.Colleague](t, colleague.do(http.MethodGet, path+"/watchers", nil), http.StatusOK); len(got) != 0 {
		t.Fatalf("followers = %+v", got)
	}
	toldBefore := countFor(te.notices(t, theirs.Id), te.requesterID)
	decode[gen.Ticket](t, colleague.answer(theirs.Key, gen.UatAnswerApprove, ""), http.StatusOK)
	te.fanout(t)
	if got := te.requester.reopen(theirs.Key).Code; got != http.StatusNotFound {
		t.Fatalf("reopen (resolved): %d, want 404", got)
	}
	if got := countFor(te.notices(t, theirs.Id), te.requesterID); got != toldBefore {
		t.Fatalf("unlinked follower told about the resolve: %v", te.notices(t, theirs.Id))
	}

	// Linked again, they see it and follow it again.
	te.link(t, te.requesterID, te.projectA, te.projectA2)
	decode[gen.Ticket](t, te.requester.do(http.MethodGet, path, nil), http.StatusOK)
	if got := decode[[]gen.Colleague](t, colleague.do(http.MethodGet, path+"/watchers", nil), http.StatusOK); len(got) != 1 {
		t.Fatalf("followers after linking again = %+v", got)
	}
}

// The report form offers only the requester's projects, and the API holds them to it.
func TestRequesterReportsOnTheirProjects(t *testing.T) {
	te := newTicketEnv(t)
	intake := func() gen.Intake {
		t.Helper()
		return decode[gen.Intake](t, te.requester.do(http.MethodGet, "/intake", nil), http.StatusOK)
	}
	projectIDs := func(in gen.Intake) []uuid.UUID {
		out := []uuid.UUID{}
		for _, p := range in.Projects {
			out = append(out, p.Id)
		}
		return out
	}
	request := gen.TicketInput{Type: gen.TicketTypeRequest, Priority: gen.PriorityP3, Subject: "New account", Description: "For the intern."}

	te.link(t, te.requesterID, te.projectA)
	got := intake()
	if !sameIDs(projectIDs(got), te.projectA) || got.DefaultProjectId == nil || *got.DefaultProjectId != te.projectA {
		t.Fatalf("intake linked to the default = %+v", got)
	}
	expectProblem(t, te.requester.do(http.MethodPost, "/tickets", incident(te.projectA2, "Mail down")),
		http.StatusBadRequest, apperr.CodeInvalidProject)
	te.requester.report(t, request)

	te.link(t, te.requesterID, te.projectA2)
	got = intake()
	if !sameIDs(projectIDs(got), te.projectA2) || got.DefaultProjectId != nil {
		t.Fatalf("intake without the default = %+v", got)
	}
	expectProblem(t, te.requester.do(http.MethodPost, "/tickets", request), http.StatusBadRequest, apperr.CodeInvalidProject)
	expectProblem(t, te.requester.do(http.MethodPost, "/tickets", incident(te.projectA, "Letters")),
		http.StatusBadRequest, apperr.CodeInvalidProject)
	te.requester.report(t, incident(te.projectA2, "Mail down"))

	te.link(t, te.requesterID)
	if got = intake(); len(got.Projects) != 0 || got.DefaultProjectId != nil {
		t.Fatalf("intake with no project = %+v", got)
	}
	// Staff still log for them on any project of the client, and they see that ticket.
	logged := te.agent.logFor(t, te.clientA, te.requesterID, incident(te.projectA, "Phoned in"))
	decode[gen.Ticket](t, te.requester.do(http.MethodGet, "/tickets/"+logged.Key, nil), http.StatusOK)
	staff := decode[gen.Intake](t, te.agent.do(http.MethodGet, "/intake?clientId="+te.clientA.String(), nil), http.StatusOK)
	if !sameIDs(projectIDs(staff), te.projectA, te.projectA2) || staff.DefaultProjectId == nil {
		t.Fatalf("staff intake = %+v", staff)
	}
}

// Only colleagues linked to the ticket's project can follow it.
func TestWatchersAreLinkedToTheProject(t *testing.T) {
	te := newTicketEnv(t)
	_, colleagueID := te.addColleague(t)
	if _, err := te.auth.CreateUser(context.Background(), auth.NewUser{
		Email: secondColleagueEmail, Name: "Bayu Saputra", Password: password, Role: auth.RoleRequester,
		ClientName: requesterClient, OrganizationID: te.org,
	}); err != nil {
		t.Fatal(err)
	}
	second, err := te.st.Q().GetUserByEmail(context.Background(), secondColleagueEmail)
	if err != nil {
		t.Fatal(err)
	}
	te.link(t, second.ID, te.projectA2)
	tk := te.requester.report(t, incident(te.projectA, "Projector"))
	path := "/tickets/" + tk.Key

	candidates := decode[[]gen.Colleague](t, te.requester.do(http.MethodGet, path+"/watcher-candidates", nil), http.StatusOK)
	if len(candidates) != 1 || candidates[0].Id != colleagueID {
		t.Fatalf("candidates = %+v, want only the linked colleague", candidates)
	}
	expectProblem(t, te.requester.do(http.MethodPost, path+"/watchers", gen.WatcherInput{UserId: second.ID}),
		http.StatusBadRequest, apperr.CodeInvalidWatcher)
	decode[[]gen.Colleague](t, te.requester.do(http.MethodPost, path+"/watchers", gen.WatcherInput{UserId: colleagueID}), http.StatusOK)
}

// Only administrators set a requester's projects, only to their client's projects, and each
// change is audited with what was added and removed.
func TestSetRequesterProjects(t *testing.T) {
	te := newTicketEnv(t)
	path := te.requesterProjectsPath(te.requesterID)
	body := gen.RequesterProjectsInput{ProjectIds: []uuid.UUID{te.projectA}}
	for _, s := range []session{te.supervisor, te.agent, te.requester} {
		expectProblem(t, s.do(http.MethodPut, path, body), http.StatusForbidden, apperr.CodeForbidden)
	}
	for _, ids := range [][]uuid.UUID{{te.projectB}, {te.projectA, uuid.New()}} {
		expectProblem(t, te.admin.do(http.MethodPut, path, gen.RequesterProjectsInput{ProjectIds: ids}),
			http.StatusBadRequest, apperr.CodeInvalidProject)
	}
	// Someone who isn't a requester of the client.
	for _, u := range []uuid.UUID{te.otherReqID, te.agentID} {
		expectProblem(t, te.admin.do(http.MethodPut, te.requesterProjectsPath(u), body), http.StatusNotFound, apperr.CodeNotFound)
	}
	if got := te.auditActionsLike(t, auth.ActionProjectsChanged); len(got) != 0 {
		t.Fatalf("refused changes audited: %v", got)
	}

	r := te.link(t, te.requesterID, te.projectA)
	if r.Id != te.requesterID || !sameIDs(r.ProjectIds, te.projectA) {
		t.Fatalf("requester = %+v", r)
	}
	te.link(t, te.requesterID, te.projectA) // no change, no entry
	te.link(t, te.requesterID, te.projectA2)
	var entries []projectsChange
	rows, err := te.st.Pool().Query(context.Background(), `SELECT data FROM audit_log
		WHERE action = $1 AND target_type = $2 AND target_id = $3 AND organization_id = $4 ORDER BY at, id`,
		auth.ActionProjectsChanged, auth.TargetUser, te.requesterID, te.org)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var e projectsChange
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	rows.Close()
	if len(entries) != 2 ||
		entries[0].Client != te.clientA || len(entries[0].Added) != 0 || !sameIDs(entries[0].Removed, te.projectA2) ||
		!sameIDs(entries[1].Added, te.projectA2) || !sameIDs(entries[1].Removed, te.projectA) {
		t.Fatalf("audit = %+v", entries)
	}
	list := decode[[]gen.Requester](t, te.admin.do(http.MethodGet, "/clients/"+te.clientA.String()+"/requesters", nil), http.StatusOK)
	if i := slices.IndexFunc(list, func(x gen.Requester) bool { return x.Id == te.requesterID }); i < 0 || !sameIDs(list[i].ProjectIds, te.projectA2) {
		t.Fatalf("list = %+v", list)
	}
}

// A new client's requesters can be invited before it has a project; its first project takes them
// all, later ones only whoever the administrator links.
func TestFirstProjectTakesTheClientsRequesters(t *testing.T) {
	te := newTicketEnv(t)
	c := decode[gen.Client](t, te.admin.do(http.MethodPost, "/clients", gen.ClientInput{Name: "Bappeda Riau"}), http.StatusCreated)
	base := "/clients/" + c.Id.String() + "/requesters"
	r := decode[gen.Requester](t, te.admin.do(http.MethodPost, base, gen.RequesterInput{Name: "Fajar", Email: "fajar@riau.test"}),
		http.StatusCreated)
	if len(r.ProjectIds) != 0 {
		t.Fatalf("invited before any project = %+v", r)
	}
	first := decode[gen.Project](t, te.supervisor.do(http.MethodPost, "/projects", projectInput(c.Id, "SIPD connector")), http.StatusCreated)
	decode[gen.Project](t, te.supervisor.do(http.MethodPost, "/projects", projectInput(c.Id, "Website")), http.StatusCreated)
	list := decode[[]gen.Requester](t, te.admin.do(http.MethodGet, base, nil), http.StatusOK)
	if len(list) != 1 || !sameIDs(list[0].ProjectIds, first.Id) {
		t.Fatalf("requesters = %+v, want linked to the first project only", list)
	}
	var linked []uuid.UUID
	if err := te.st.Pool().QueryRow(context.Background(),
		`SELECT data->'linked_requester_ids' FROM audit_log WHERE action = 'project.created' AND target_id = $1`, first.Id).
		Scan(&linked); err != nil || !sameIDs(linked, r.Id) {
		t.Fatalf("audit linked = %v (err %v)", linked, err)
	}
}
