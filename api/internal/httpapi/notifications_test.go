package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
)

const colleagueEmail = "colleague@example.test"

// runJobs works every queued job of the worker's kind, oldest first, and removes them, as a
// worker process would. It returns how many ran.
func runJobs[T river.JobArgs](t *testing.T, e *env, w river.Worker[T]) int {
	t.Helper()
	ctx := context.Background()
	var zero T
	rows, err := e.st.Pool().Query(ctx, `SELECT id, args FROM river_job WHERE kind = $1 ORDER BY id`, zero.Kind())
	if err != nil {
		t.Fatal(err)
	}
	type queued struct {
		id   int64
		args T
	}
	var jobs []queued
	for rows.Next() {
		var q queued
		var raw []byte
		if err := rows.Scan(&q.id, &raw); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &q.args); err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, q)
	}
	rows.Close()
	for _, q := range jobs {
		if err := w.Work(ctx, &river.Job[T]{Args: q.args}); err != nil {
			t.Fatalf("%s: %v", zero.Kind(), err)
		}
		if _, err := e.st.Pool().Exec(ctx, `DELETE FROM river_job WHERE id = $1`, q.id); err != nil {
			t.Fatal(err)
		}
	}
	return len(jobs)
}

// fanout runs the queued notification fan-outs.
func (e *env) fanout(t *testing.T) { t.Helper(); runJobs(t, e, e.fanoutWorker) }

// notice is one stored notification, reduced to what tests check.
type notice struct {
	user  uuid.UUID
	event notification.Event
}

// notices lists a ticket's notifications in the order they were made.
func (e *env) notices(t *testing.T, ticketID uuid.UUID) []notice {
	t.Helper()
	rows, err := e.st.Pool().Query(context.Background(),
		`SELECT user_id, event_type FROM notifications WHERE ticket_id = $1 ORDER BY created_at, id`, ticketID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []notice
	for rows.Next() {
		var n notice
		if err := rows.Scan(&n.user, &n.event); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}

// addColleague creates another requester of the requester's client and signs them in.
func (te *ticketEnv) addColleague(t *testing.T) (session, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if _, err := te.auth.CreateUser(ctx, auth.NewUser{
		Email: colleagueEmail, Name: "Dian Pratama", Password: password, Role: auth.RoleRequester, ClientName: requesterClient,
	}); err != nil {
		t.Fatal(err)
	}
	u, err := te.st.Q().GetUserByEmail(ctx, colleagueEmail)
	if err != nil {
		t.Fatal(err)
	}
	return session{e: te.env, cookie: sessionCookie(t, te.login(colleagueEmail, password))}, u.ID
}

func TestFanoutFollowsTheTicket(t *testing.T) {
	te := newTicketEnv(t)
	supervisorID := userID(t, te.env, auth.RoleSupervisor)
	tk := te.requester.report(t, incident(te.projectA, "VPN down"))
	te.fanout(t)
	if got := te.notices(t, tk.Id); len(got) != 0 {
		t.Fatalf("a portal report notifies nobody (HANDOFF §13 Q7), got %v", got)
	}

	steps := []struct {
		name string
		act  func()
		want []notice
	}{
		{"supervisor assigns the agent", func() {
			decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee",
				gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
		}, []notice{{te.agentID, notification.EventAssignment}, {te.requesterID, notification.EventStatus}}},
		{"requester replies", func() {
			decode[gen.TicketMessage](t, te.requester.message(tk.Key, gen.MessageVisibilityPublic, "Still down"), http.StatusCreated)
		}, []notice{{te.agentID, notification.EventComment}}},
		{"supervisor adds an internal note", func() {
			decode[gen.TicketMessage](t, te.supervisor.message(tk.Key, gen.MessageVisibilityInternal, "Check the tunnel"), http.StatusCreated)
		}, []notice{{te.agentID, notification.EventInternalNote}}},
		{"agent replies and asks a question", func() {
			decode[gen.TicketMessage](t, te.agent.message(tk.Key, gen.MessageVisibilityPublic, "Which office?"), http.StatusCreated)
			decode[gen.Ticket](t, te.agent.do(http.MethodPut, "/tickets/"+tk.Key+"/status",
				gen.TicketStatusInput{Status: gen.TicketStatusWaitingOnClient}), http.StatusOK)
		}, []notice{{te.requesterID, notification.EventReply}, {te.requesterID, notification.EventQuestion}}},
		{"supervisor raises the priority", func() {
			decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, "/tickets/"+tk.Key+"/priority",
				gen.TicketPriorityInput{Priority: gen.PriorityP2, Reason: "Whole office"}), http.StatusOK)
		}, []notice{{te.agentID, notification.EventPriority}}},
	}
	var want []notice
	for _, s := range steps {
		s.act()
		te.fanout(t)
		want = append(want, s.want...)
		if got := te.notices(t, tk.Id); !slices.Equal(got, want) {
			t.Fatalf("after %q\n got %v\nwant %v", s.name, got, want)
		}
	}
	for _, n := range te.notices(t, tk.Id) {
		if n.user == supervisorID {
			t.Errorf("the actor was notified: %v", n)
		}
	}
	// A retried fan-out adds nothing.
	var count int
	if err := te.st.Pool().QueryRow(context.Background(), `SELECT count(*) FROM notifications`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(want) {
		t.Fatalf("notifications = %d, want %d", count, len(want))
	}
}

func TestLoggedTicketNotifiesTheRequester(t *testing.T) {
	te := newTicketEnv(t)
	tk := te.agent.logFor(t, te.clientA, te.requesterID, incident(te.projectA, "Called about email"))
	te.fanout(t)
	want := []notice{{te.requesterID, notification.EventTicketLogged}}
	if got := te.notices(t, tk.Id); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestInternalNotesNeverNotifyClientStaff(t *testing.T) {
	te := newTicketEnv(t)
	_, colleagueID := te.addColleague(t)
	tk := te.requester.report(t, incident(te.projectA, "Disk full"))
	decode[[]gen.Colleague](t, te.requester.do(http.MethodPost, "/tickets/"+tk.Key+"/watchers",
		gen.WatcherInput{UserId: colleagueID}), http.StatusOK)
	te.fanout(t)
	before := len(te.notices(t, tk.Id))
	decode[gen.TicketMessage](t, te.agent.message(tk.Key, gen.MessageVisibilityInternal, "Secret plan"), http.StatusCreated)
	te.fanout(t)
	for _, n := range te.notices(t, tk.Id)[before:] {
		if n.user == te.requesterID || n.user == colleagueID {
			t.Fatalf("internal note reached client staff: %v", n)
		}
	}
}

func TestWatchers(t *testing.T) {
	te := newTicketEnv(t)
	colleague, colleagueID := te.addColleague(t)
	tk := te.requester.report(t, incident(te.projectA, "Projector"))
	path := "/tickets/" + tk.Key

	candidates := decode[[]gen.Colleague](t, te.requester.do(http.MethodGet, path+"/watcher-candidates", nil), http.StatusOK)
	if len(candidates) != 1 || candidates[0].Id != colleagueID {
		t.Fatalf("candidates = %+v, want only the colleague", candidates)
	}
	if none := decode[[]gen.Colleague](t, te.requester.do(http.MethodGet, path+"/watcher-candidates?q=zzz", nil), http.StatusOK); len(none) != 0 {
		t.Fatalf("search: %+v", none)
	}
	expectProblem(t, te.agent.do(http.MethodGet, path+"/watcher-candidates", nil), http.StatusForbidden, apperr.CodeForbidden)
	expectProblem(t, te.requester.do(http.MethodPost, path+"/watchers", gen.WatcherInput{UserId: te.otherReqID}),
		http.StatusBadRequest, apperr.CodeInvalidWatcher)
	expectProblem(t, te.requester.do(http.MethodPost, path+"/watchers", gen.WatcherInput{UserId: te.requesterID}),
		http.StatusBadRequest, apperr.CodeInvalidWatcher)
	expectProblem(t, te.agent.do(http.MethodPost, path+"/watchers", gen.WatcherInput{UserId: colleagueID}),
		http.StatusForbidden, apperr.CodeForbidden)

	list := decode[[]gen.Colleague](t, te.requester.do(http.MethodPost, path+"/watchers", gen.WatcherInput{UserId: colleagueID}), http.StatusOK)
	if len(list) != 1 || list[0].Id != colleagueID || list[0].Name != "Dian Pratama" {
		t.Fatalf("watchers = %+v", list)
	}
	// Adding twice changes nothing and audits once.
	decode[[]gen.Colleague](t, te.requester.do(http.MethodPost, path+"/watchers", gen.WatcherInput{UserId: colleagueID}), http.StatusOK)
	te.fanout(t)
	if got, want := te.notices(t, tk.Id), []notice{{colleagueID, notification.EventColleague}}; !slices.Equal(got, want) {
		t.Fatalf("adding notifies the colleague: got %v, want %v", got, want)
	}
	if got := decode[[]gen.Colleague](t, te.agent.do(http.MethodGet, path+"/watchers", nil), http.StatusOK); len(got) != 1 {
		t.Fatalf("staff see who is notified: %+v", got)
	}

	// The colleague now hears about replies from the team, and the requester about the colleague's.
	decode[gen.TicketMessage](t, te.agent.message(tk.Key, gen.MessageVisibilityPublic, "On our way"), http.StatusCreated)
	decode[gen.TicketMessage](t, colleague.message(tk.Key, gen.MessageVisibilityPublic, "Room 2"), http.StatusCreated)
	te.fanout(t)
	want := []notice{
		{colleagueID, notification.EventColleague},
		{te.requesterID, notification.EventReply}, {colleagueID, notification.EventReply},
		{te.requesterID, notification.EventColleague},
	}
	if got := te.notices(t, tk.Id); !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}

	if rec := te.requester.do(http.MethodDelete, path+"/watchers/"+colleagueID.String(), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body)
	}
	expectProblem(t, te.requester.do(http.MethodDelete, path+"/watchers/"+colleagueID.String(), nil), http.StatusNotFound, apperr.CodeNotFound)
	if got := te.auditActions(t, tk.Id); slices.Index(got, "ticket.watcher_added") < 0 ||
		slices.Index(got, "ticket.watcher_removed") < 0 || countOf(got, "ticket.watcher_added") != 1 {
		t.Fatalf("audit = %v", got)
	}
}

func countOf(list []string, s string) int {
	n := 0
	for _, v := range list {
		if v == s {
			n++
		}
	}
	return n
}
