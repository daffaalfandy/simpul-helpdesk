package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/mail"
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
		OrganizationID: te.org,
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

func (s session) inbox(t *testing.T, query string) gen.NotificationPage {
	t.Helper()
	return decode[gen.NotificationPage](t, s.do(http.MethodGet, "/notifications"+query, nil), http.StatusOK)
}

func (s session) unread(t *testing.T) int {
	t.Helper()
	return decode[gen.UnreadCount](t, s.do(http.MethodGet, "/notifications/unread-count", nil), http.StatusOK).Count
}

func TestInbox(t *testing.T) {
	te := newTicketEnv(t)
	tk := te.requester.report(t, incident(te.projectA, "Login fails"))
	path := "/tickets/" + tk.Key
	decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, path+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	decode[gen.TicketMessage](t, te.requester.message(tk.Key, gen.MessageVisibilityPublic, "Two of three staff can sign in now."), http.StatusCreated)
	decode[gen.TicketMessage](t, te.supervisor.message(tk.Key, gen.MessageVisibilityInternal, "Timeout is on their side."), http.StatusCreated)
	decode[gen.TicketMessage](t, te.agent.message(tk.Key, gen.MessageVisibilityPublic, "Which accounts still fail?"), http.StatusCreated)
	decode[gen.Ticket](t, te.agent.do(http.MethodPut, path+"/status", gen.TicketStatusInput{Status: gen.TicketStatusWaitingOnClient}), http.StatusOK)
	te.fanout(t)

	page := te.agent.inbox(t, "")
	var events []gen.NotificationEvent
	for _, it := range page.Items {
		events = append(events, it.Event)
	}
	if want := []gen.NotificationEvent{gen.NotificationEventInternalNote, gen.NotificationEventComment, gen.NotificationEventAssignment}; !slices.Equal(events, want) {
		t.Fatalf("agent inbox = %v, want %v (newest first)", events, want)
	}
	note, reply := page.Items[0], page.Items[1]
	if note.Quote == nil || *note.Quote != "Timeout is on their side." || note.ActorName == nil || *note.ActorName != "User supervisor" {
		t.Errorf("note item = %+v", note)
	}
	if reply.Quote == nil || *reply.Quote != "Two of three staff can sign in now." || reply.Ticket.Key != tk.Key || reply.Ticket.Subject != "Login fails" {
		t.Errorf("reply item = %+v", reply)
	}
	if page.UnreadCount != 3 || te.agent.unread(t) != 3 || page.NextBefore != nil {
		t.Errorf("unread = %d / %d, next = %v", page.UnreadCount, te.agent.unread(t), page.NextBefore)
	}
	if got := te.agent.inbox(t, "?filter=comments").Items; len(got) != 2 {
		t.Errorf("comments filter = %d items", len(got))
	}
	if got := te.agent.inbox(t, "?filter=assigned").Items; len(got) != 1 || *got[0].Kind != gen.NotificationKindAssigned {
		t.Errorf("assigned filter = %+v", got)
	}

	// The requester's question quotes the team's reply, never the note.
	portal := te.requester.inbox(t, "?filter=replies")
	if len(portal.Items) != 2 || portal.Items[0].Event != gen.NotificationEventQuestion ||
		portal.Items[0].Quote == nil || *portal.Items[0].Quote != "Which accounts still fail?" {
		t.Fatalf("requester replies = %+v", portal.Items)
	}

	// Paging.
	first := te.agent.inbox(t, "?limit=2")
	if len(first.Items) != 2 || first.NextBefore == nil {
		t.Fatalf("first page = %+v", first)
	}
	if rest := te.agent.inbox(t, "?limit=2&before="+first.NextBefore.String()); len(rest.Items) != 1 || rest.NextBefore != nil {
		t.Fatalf("second page = %+v", rest)
	}

	// Read state.
	readPath := "/notifications/" + note.Id.String() + "/read"
	if rec := te.agent.do(http.MethodPut, readPath, gen.NotificationReadInput{Read: true}); rec.Code != http.StatusNoContent {
		t.Fatalf("read: %d %s", rec.Code, rec.Body)
	}
	if te.agent.unread(t) != 2 || len(te.agent.inbox(t, "?filter=unread").Items) != 2 {
		t.Error("read item still unread")
	}
	expectProblem(t, te.requester.do(http.MethodPut, readPath, gen.NotificationReadInput{Read: false}), http.StatusNotFound, apperr.CodeNotFound)
	if rec := te.agent.do(http.MethodPut, readPath, gen.NotificationReadInput{Read: false}); rec.Code != http.StatusNoContent || te.agent.unread(t) != 3 {
		t.Fatalf("unread again: %d, count %d", rec.Code, te.agent.unread(t))
	}
	if rec := te.agent.do(http.MethodPost, "/notifications/read-all", nil); rec.Code != http.StatusNoContent || te.agent.unread(t) != 0 {
		t.Fatalf("read all: %d, count %d", rec.Code, te.agent.unread(t))
	}
	if te.requester.unread(t) == 0 {
		t.Error("read-all touched another user's inbox")
	}

	// An agent unlinked from the client keeps the items about the ticket still assigned to them
	// (owner, 17 Nov); once they release it, the items go.
	decode[gen.Client](t, te.admin.do(http.MethodPut, "/clients/"+te.clientA.String()+"/agents",
		gen.ClientAgentsInput{UserIds: []uuid.UUID{}}), http.StatusOK)
	if got := te.agent.inbox(t, ""); len(got.Items) != 3 {
		t.Fatalf("unlinked assignee inbox = %+v", got)
	}
	decode[gen.Ticket](t, te.agent.do(http.MethodPut, path+"/assignee", gen.TicketAssigneeInput{AssigneeId: nil}), http.StatusOK)
	if got := te.agent.inbox(t, ""); len(got.Items) != 0 || got.UnreadCount != 0 {
		t.Fatalf("inbox after releasing the ticket = %+v", got)
	}
}

func TestNotificationSettings(t *testing.T) {
	te := newTicketEnv(t)
	settings := decode[gen.NotificationSettings](t, te.agent.do(http.MethodGet, "/notification-settings", nil), http.StatusOK)
	if settings.Audience != gen.AudienceTeam || settings.EmailFrequency != gen.EmailFrequencyInstant || len(settings.Events) != 8 {
		t.Fatalf("agent settings = %+v", settings)
	}
	for _, e := range settings.Events {
		locked := e.Event == gen.NotificationEventAssignment || e.Event == gen.NotificationEventSla
		if e.Locked != locked {
			t.Errorf("%s locked = %v", e.Event, e.Locked)
		}
	}
	set := func(s session, event string, web, email bool) *httptest.ResponseRecorder {
		return s.do(http.MethodPut, "/notification-settings/events/"+event, gen.NotificationPreferenceInput{Web: web, Email: email})
	}
	expectProblem(t, set(te.agent, "sla", true, false), http.StatusBadRequest, apperr.CodeNotificationLocked)
	expectProblem(t, set(te.agent, "reply", false, false), http.StatusBadRequest, apperr.CodeNotificationEventUnknown)
	expectProblem(t, set(te.requester, "ticket_logged", false, false), http.StatusBadRequest, apperr.CodeNotificationEventUnknown)
	updated := decode[gen.NotificationSettings](t, set(te.agent, "comment", false, false), http.StatusOK)
	if updated.Events[0].Event != gen.NotificationEventComment || updated.Events[0].Web || updated.Events[0].Email {
		t.Fatalf("comment switched off = %+v", updated.Events[0])
	}
	freq := decode[gen.NotificationSettings](t, te.agent.do(http.MethodPut, "/notification-settings/email-frequency",
		gen.EmailFrequencyInput{EmailFrequency: gen.EmailFrequencyHourly}), http.StatusOK)
	if freq.EmailFrequency != gen.EmailFrequencyHourly {
		t.Fatalf("frequency = %s", freq.EmailFrequency)
	}
	portal := decode[gen.NotificationSettings](t, te.requester.do(http.MethodGet, "/notification-settings", nil), http.StatusOK)
	if portal.Audience != gen.AudiencePortal || len(portal.Events) != 6 {
		t.Fatalf("portal settings = %+v", portal)
	}
	for _, e := range portal.Events {
		if e.Locked != (e.Event == gen.NotificationEventUatRequested) {
			t.Errorf("portal %s locked = %v", e.Event, e.Locked)
		}
	}
	expectProblem(t, set(te.requester, "uat_requested", true, false), http.StatusBadRequest, apperr.CodeNotificationLocked)
	expectProblem(t, set(te.supervisor, "dod", false, false), http.StatusBadRequest, apperr.CodeNotificationEventUnknown)
	if got := countOf(te.auditActionsLike(t, "notification_settings.%"), "notification_settings.updated"); got != 2 {
		t.Errorf("settings audit entries = %d, want 2", got)
	}

	// A switched-off event notifies nobody.
	tk := te.requester.report(t, incident(te.projectA, "Quiet"))
	decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	decode[gen.TicketMessage](t, te.requester.message(tk.Key, gen.MessageVisibilityPublic, "Hello?"), http.StatusCreated)
	te.fanout(t)
	for _, n := range te.notices(t, tk.Id) {
		if n.user == te.agentID && n.event == notification.EventComment {
			t.Fatal("comment notified an agent who switched it off")
		}
	}
}

// age backdates every notification, as if the clock had moved on.
func (e *env) age(t *testing.T, d time.Duration) {
	t.Helper()
	if _, err := e.st.Pool().Exec(context.Background(),
		`UPDATE notifications SET created_at = created_at - make_interval(secs => $1)`, d.Seconds()); err != nil {
		t.Fatal(err)
	}
}

// sendEmails runs the dispatcher (a periodic job, so never queued here) and the emails it queued.
func (e *env) sendEmails(t *testing.T) {
	t.Helper()
	if err := e.dispatch.Work(context.Background(), &river.Job[notification.DispatchArgs]{}); err != nil {
		t.Fatal(err)
	}
	runJobs(t, e, e.ticketEmail)
	runJobs(t, e, e.digest)
}

func (e *env) setFrequency(t *testing.T, s session, f gen.EmailFrequency) {
	t.Helper()
	decode[gen.NotificationSettings](t, s.do(http.MethodPut, "/notification-settings/email-frequency",
		gen.EmailFrequencyInput{EmailFrequency: f}), http.StatusOK)
}

func mailsTo(msgs []mail.Message, to string) []mail.Message {
	var out []mail.Message
	for _, m := range msgs {
		if m.To == to {
			out = append(out, m)
		}
	}
	return out
}

func TestTicketEmailsAreGrouped(t *testing.T) {
	te := newTicketEnv(t)
	agentEmail, requesterEmail := te.users[auth.RoleAgent], te.users[auth.RoleRequester]
	tk := te.requester.report(t, incident(te.projectA, "Login fails"))
	path := "/tickets/" + tk.Key
	decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, path+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	decode[gen.TicketMessage](t, te.agent.message(tk.Key, gen.MessageVisibilityInternal, "Secret: their cert expired"), http.StatusCreated)
	decode[gen.TicketMessage](t, te.agent.message(tk.Key, gen.MessageVisibilityPublic, "Which accounts fail?"), http.StatusCreated)
	decode[gen.Ticket](t, te.agent.do(http.MethodPut, path+"/status", gen.TicketStatusInput{Status: gen.TicketStatusWaitingOnClient}), http.StatusOK)
	te.fanout(t)

	te.sendEmails(t)
	if got := te.mailer.Messages(); len(got) != 0 {
		t.Fatalf("emails sent inside the group window: %d", len(got))
	}
	te.age(t, notification.GroupWindow)
	te.sendEmails(t)
	msgs := te.mailer.Messages()
	toRequester := mailsTo(msgs, requesterEmail)
	if len(toRequester) != 1 {
		t.Fatalf("requester got %d emails, want one grouped email", len(toRequester))
	}
	m := toRequester[0]
	if want := "[" + tk.Key + "] A question for you: Login fails"; m.Subject != want {
		t.Errorf("subject = %q, want %q", m.Subject, want)
	}
	for _, want := range []string{"Which accounts fail?", "Also changed:", "Status changed: New → Assigned", "you reported " + tk.Key,
		appURL + "/portal/tickets/" + tk.Key, appURL + "/portal/notifications#settings"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("requester email lacks %q:\n%s", want, m.Text)
		}
	}
	if strings.Contains(m.Text+m.HTML, "Secret") {
		t.Fatal("an internal note reached the requester's email")
	}
	if m.Headers[mail.HeaderAutoSubmitted] != mail.AutoSubmittedGenerated {
		t.Errorf("headers = %v", m.Headers)
	}
	if got := mailsTo(msgs, agentEmail); len(got) != 1 || !strings.Contains(got[0].Subject, "Assigned to you") {
		t.Fatalf("agent emails = %+v", got)
	}
	// Everything sent is marked, so nothing goes twice.
	te.age(t, notification.GroupWindow)
	te.sendEmails(t)
	if n := len(te.mailer.Messages()); n != len(msgs) {
		t.Fatalf("emails after a second run = %d, want %d", n, len(msgs))
	}
}

func TestSummaryUsersWaitButUrgentGoesNow(t *testing.T) {
	te := newTicketEnv(t)
	agentEmail := te.users[auth.RoleAgent]
	te.setFrequency(t, te.agent, gen.EmailFrequencyHourly)
	routine := te.requester.report(t, incident(te.projectA, "Printer"))
	urgentIn := incident(te.projectA, "Whole office down")
	urgentIn.Priority = gen.PriorityP1
	urgent := te.requester.report(t, urgentIn)
	for _, tk := range []gen.Ticket{routine, urgent} {
		decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
		decode[gen.TicketMessage](t, te.requester.message(tk.Key, gen.MessageVisibilityPublic, "Any news?"), http.StatusCreated)
	}
	te.fanout(t)
	te.age(t, notification.GroupWindow)
	te.sendEmails(t)
	got := mailsTo(te.mailer.Messages(), agentEmail)
	if len(got) != 1 || !strings.Contains(got[0].Subject, urgent.Key) {
		t.Fatalf("hourly agent got %v, want only the P1 ticket's email", subjects(got))
	}

	// Once the hour has turned, the routine items go out as one summary.
	te.age(t, time.Hour)
	te.sendEmails(t)
	got = mailsTo(te.mailer.Messages(), agentEmail)
	if len(got) != 2 || !strings.Contains(got[1].Subject, "Your hourly summary") || !strings.Contains(got[1].Text, routine.Key) {
		t.Fatalf("after the hour: %v", subjects(got))
	}
	if strings.Contains(got[1].Text, urgent.Key) {
		t.Error("the summary repeats the urgent ticket")
	}
}

func TestDailySummary(t *testing.T) {
	te := newTicketEnv(t)
	te.setFrequency(t, te.requester, gen.EmailFrequencyDaily)
	tk := te.requester.report(t, incident(te.projectA, "Email quota"))
	decode[gen.Ticket](t, te.agent.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	decode[gen.TicketMessage](t, te.agent.message(tk.Key, gen.MessageVisibilityPublic, "Quota raised."), http.StatusCreated)
	te.fanout(t)
	te.age(t, 2*24*time.Hour) // older than the latest 08:00, whatever time the test runs
	te.sendEmails(t)
	got := mailsTo(te.mailer.Messages(), te.users[auth.RoleRequester])
	if len(got) != 1 || !strings.Contains(got[0].Subject, "Your daily summary") ||
		!strings.Contains(got[0].Text, "2 updates on 1 ticket") {
		t.Fatalf("daily summary = %v", subjects(got))
	}
}

func subjects(msgs []mail.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Subject
	}
	return out
}

// Summary rows that aren't due never reach the dispatcher's batch, so they can't crowd out
// emails that are.
func TestDispatchReadsOnlyDueRows(t *testing.T) {
	te := newTicketEnv(t)
	te.setFrequency(t, te.agent, gen.EmailFrequencyDaily)
	tk := te.requester.report(t, incident(te.projectA, "Quota"))
	decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	decode[gen.TicketMessage](t, te.agent.message(tk.Key, gen.MessageVisibilityPublic, "On it"), http.StatusCreated)
	te.fanout(t)
	te.age(t, notification.GroupWindow)
	now := time.Now()
	rows, err := te.st.Q().ListDueEmails(context.Background(), db.ListDueEmailsParams{
		OrganizationID: te.org, StaleBefore: now.Add(-notification.ClaimTimeout), GroupCutoff: now.Add(-notification.GroupWindow),
		Instant: string(notification.FrequencyInstant), Hourly: string(notification.FrequencyHourly),
		Daily: string(notification.FrequencyDaily), HourlyCutoff: now.Add(-time.Hour), DailyCutoff: now.Add(-24 * time.Hour),
		RowLimit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.UserID == te.agentID {
			t.Fatalf("a daily user's fresh rows are in the batch: %+v", r)
		}
	}
	if len(rows) == 0 {
		t.Fatal("the instant requester's due row is missing")
	}
}

func (e *env) pendingEmails(t *testing.T, user uuid.UUID) (pending, skipped int) {
	t.Helper()
	if err := e.st.Pool().QueryRow(context.Background(), `SELECT
		count(*) FILTER (WHERE email AND emailed_at IS NULL AND email_claimed_at IS NULL),
		count(*) FILTER (WHERE email_skipped) FROM notifications WHERE user_id = $1`, user).Scan(&pending, &skipped); err != nil {
		t.Fatal(err)
	}
	return pending, skipped
}

// A send that keeps failing is retried by River, then given up so it isn't queued forever.
func TestFailingEmailIsGivenUpAfterTheLastAttempt(t *testing.T) {
	te := newTicketEnv(t)
	tk := te.requester.report(t, incident(te.projectA, "Bounce"))
	decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	te.fanout(t)
	te.age(t, notification.GroupWindow)
	te.mailer.Fail = errors.New("mailbox unavailable")
	ctx := context.Background()
	args := notification.TicketEmailArgs{UserID: te.agentID, TicketID: tk.Id}
	work := func(attempt int) error {
		return te.ticketEmail.Work(ctx, &river.Job[notification.TicketEmailArgs]{
			JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: 3}, Args: args,
		})
	}
	if err := work(1); err == nil {
		t.Fatal("a failed send must fail the job so River retries it")
	}
	if pending, skipped := te.pendingEmails(t, te.agentID); pending == 0 || skipped != 0 {
		t.Fatalf("after a retryable failure: pending %d, skipped %d", pending, skipped)
	}
	_ = work(3)
	if pending, skipped := te.pendingEmails(t, te.agentID); pending != 0 || skipped == 0 {
		t.Fatalf("after the last attempt: pending %d, skipped %d", pending, skipped)
	}
}

// A row that lands after its period's summary went out gets a catch-up summary, not a whole
// period's wait.
func TestLateSummaryRowsCatchUp(t *testing.T) {
	te := newTicketEnv(t)
	te.setFrequency(t, te.agent, gen.EmailFrequencyHourly)
	tk := te.requester.report(t, incident(te.projectA, "Printer"))
	decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	te.fanout(t)
	te.age(t, time.Hour)
	te.sendEmails(t)
	if got := mailsTo(te.mailer.Messages(), te.users[auth.RoleAgent]); len(got) != 1 {
		t.Fatalf("first summary: %v", subjects(got))
	}
	// A change dated before the same boundary, fanned out late.
	decode[gen.TicketMessage](t, te.requester.message(tk.Key, gen.MessageVisibilityPublic, "Still jammed"), http.StatusCreated)
	te.fanout(t)
	if _, err := te.st.Pool().Exec(context.Background(),
		`UPDATE notifications SET created_at = now() - interval '1 hour' WHERE emailed_at IS NULL`); err != nil {
		t.Fatal(err)
	}
	te.sendEmails(t)
	if got := mailsTo(te.mailer.Messages(), te.users[auth.RoleAgent]); len(got) != 2 {
		t.Fatalf("catch-up summary: %v", subjects(got))
	}
}
