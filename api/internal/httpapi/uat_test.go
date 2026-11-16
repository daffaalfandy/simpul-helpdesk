package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

const (
	testInstructions = "The fix is on the e-Office test server. Create a disposition letter and check the date."
	rejectMessage    = "The date is still one day behind on letters created after 16:00."
	internalPlan     = "Rollback plan if the client says no."
	sqlAgeResolved   = `UPDATE tickets SET resolved_at = resolved_at - interval '30 days' WHERE key = $1`
	sqlCloseTicket   = `UPDATE tickets SET status = 'closed', closed_at = now() WHERE key = $1`
	sqlStatusData    = `SELECT data::text FROM audit_log WHERE action = $1 AND target_id = $2 ORDER BY at DESC, id DESC LIMIT 1`
)

func (s session) answer(key string, a gen.UatAnswer, message string) *httptest.ResponseRecorder {
	in := gen.UatAnswerInput{Answer: a}
	if message != "" {
		in.Message = &message
	}
	return s.do(http.MethodPost, "/tickets/"+key+"/uat", in)
}

func (s session) reopen(key string) *httptest.ResponseRecorder {
	return s.do(http.MethodPost, "/tickets/"+key+"/reopen", nil)
}

func (s session) uatRequests(t *testing.T) []gen.UatRequest {
	t.Helper()
	return decode[[]gen.UatRequest](t, s.do(http.MethodGet, "/uat-requests", nil), http.StatusOK)
}

// toUAT has the agent post test instructions and move the ticket In UAT.
func (te *ticketEnv) toUAT(t *testing.T, key string) {
	t.Helper()
	decode[gen.TicketMessage](t, te.agent.message(key, gen.MessageVisibilityPublic, testInstructions), http.StatusCreated)
	decode[gen.Ticket](t, te.agent.setStatus(key, gen.TicketStatusInProgress, ""), http.StatusOK)
	decode[gen.Ticket](t, te.agent.setStatus(key, gen.TicketStatusInUat, ""), http.StatusOK)
}

func TestUATFlow(t *testing.T) {
	te := newTicketEnv(t)
	testdb.SetProjectDoD(t, te.st, te.projectA, uatDoD())
	colleague, _ := te.addColleague(t)

	tk := te.requester.report(t, incident(te.projectA, "Disposition letter PDF shows wrong date"))
	decode[gen.Ticket](t, te.agent.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	te.toUAT(t, tk.Key)

	waiting := te.requester.ticket(t, tk.Key)
	if u := waiting.Uat; u == nil || !u.CanAnswer || u.AutoApproveAt == nil || u.Instructions == nil || *u.Instructions != testInstructions {
		t.Fatalf("requester UAT = %+v", u)
	}
	if u := te.agent.ticket(t, tk.Key).Uat; u == nil || u.CanAnswer {
		t.Fatalf("agent UAT = %+v", u)
	}
	expectProblem(t, te.agent.answer(tk.Key, gen.UatAnswerApprove, ""), http.StatusForbidden, apperr.CodeForbidden)
	expectProblem(t, te.requester.answer(tk.Key, gen.UatAnswerReject, "  "), http.StatusBadRequest, apperr.CodeInvalidMessage)

	// A colleague who neither reported nor follows the ticket may approve it too (owner, 16 Nov).
	approved := decode[gen.Ticket](t, colleague.answer(tk.Key, gen.UatAnswerApprove, ""), http.StatusOK)
	if approved.Status != gen.TicketStatusResolved || approved.Uat != nil || approved.Reopen == nil || !approved.Reopen.CanReopen {
		t.Fatalf("approved = %s, uat %+v, reopen %+v", approved.Status, approved.Uat, approved.Reopen)
	}
	d := te.agent.ticket(t, tk.Key).Dod
	if !d.Met || d.Items[0].DoneBy == nil || d.Items[0].DoneBy.Name != "Dian Pratama" {
		t.Fatalf("DoD after approval = %+v", d)
	}
	id := te.ticketID(t, tk.Key)
	if c := te.clocks(t, id); c.ResolutionMetAt == nil || c.ResolutionRunningSince != nil {
		t.Fatalf("resolution clock after approval: met %v, running %v", c.ResolutionMetAt, c.ResolutionRunningSince)
	}
	expectProblem(t, te.requester.answer(tk.Key, gen.UatAnswerReject, rejectMessage), http.StatusConflict, apperr.CodeUATNotWaiting)

	// A rejection without an assignee goes to the New queue, with what went wrong as a reply.
	other := te.requester.report(t, incident(te.projectA, "Archive export fails"))
	te.toUAT(t, other.Key)
	rejected := decode[gen.Ticket](t, te.requester.answer(other.Key, gen.UatAnswerReject, rejectMessage), http.StatusOK)
	if rejected.Status != gen.TicketStatusNew {
		t.Fatalf("rejected status = %s", rejected.Status)
	}
	msgs := te.requester.messages(t, other.Key)
	if last := msgs[len(msgs)-1]; last.Body != rejectMessage || last.FromSupport || last.Visibility != gen.MessageVisibilityPublic {
		t.Fatalf("last message = %+v", last)
	}
	var data string
	if err := te.st.Pool().QueryRow(context.Background(), sqlStatusData, tickets.ActionStatusChanged, te.ticketID(t, other.Key)).Scan(&data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(data, `"message_id"`) || !strings.Contains(data, `"uat_rejected"`) || strings.Contains(data, "one day behind") {
		t.Fatalf("status audit data = %s", data)
	}
	activity := te.agent.activity(t, other.Key)
	if last := activity[len(activity)-1]; last.Cause == nil || *last.Cause != gen.TicketStatusCauseUatRejected {
		t.Fatalf("activity = %+v", last)
	}
}

func TestReopen(t *testing.T) {
	te := newTicketEnv(t)
	colleague, _ := te.addColleague(t)
	tk := te.requester.report(t, incident(te.projectA, "Printer offline"))
	decode[gen.Ticket](t, te.agent.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	decode[gen.Ticket](t, te.agent.setStatus(tk.Key, gen.TicketStatusResolved, ""), http.StatusOK)

	if r := te.agent.ticket(t, tk.Key).Reopen; r == nil || r.CanReopen {
		t.Fatalf("agent reopen = %+v", r)
	}
	expectProblem(t, te.agent.reopen(tk.Key), http.StatusForbidden, apperr.CodeForbidden)
	reopened := decode[gen.Ticket](t, colleague.reopen(tk.Key), http.StatusOK)
	if reopened.Status != gen.TicketStatusInProgress || reopened.Priority != tk.Priority || reopened.Reopen != nil {
		t.Fatalf("reopened = %s %s %+v", reopened.Status, reopened.Priority, reopened.Reopen)
	}
	expectProblem(t, te.requester.reopen(tk.Key), http.StatusConflict, apperr.CodeReopenNotAllowed)

	// After the window it stays resolved; once closed it takes no changes.
	decode[gen.Ticket](t, te.agent.setStatus(tk.Key, gen.TicketStatusResolved, ""), http.StatusOK)
	te.exec(t, sqlAgeResolved, tk.Key)
	if r := te.requester.ticket(t, tk.Key).Reopen; r == nil || r.CanReopen {
		t.Fatalf("reopen after the window = %+v", r)
	}
	expectProblem(t, te.requester.reopen(tk.Key), http.StatusConflict, apperr.CodeReopenNotAllowed)
	te.exec(t, sqlCloseTicket, tk.Key)
	expectProblem(t, te.requester.reopen(tk.Key), http.StatusConflict, apperr.CodeTicketClosed)
}

func TestUATRequests(t *testing.T) {
	te := newTicketEnv(t)
	colleague, colleagueID := te.addColleague(t)
	mine := te.requester.report(t, incident(te.projectA, "Letters show the wrong date"))
	theirs := colleague.report(t, incident(te.projectA, "Archive export fails"))
	for _, tk := range []gen.Ticket{mine, theirs} {
		te.toUAT(t, tk.Key)
	}
	decode[gen.TicketMessage](t, te.agent.message(mine.Key, gen.MessageVisibilityInternal, internalPlan), http.StatusCreated)

	got := te.requester.uatRequests(t)
	if len(got) != 1 || got[0].Key != mine.Key || got[0].Instructions == nil || *got[0].Instructions != testInstructions ||
		got[0].Project.Id != te.projectA {
		t.Fatalf("requester's tests = %+v", got)
	}
	// Following a colleague's ticket adds it.
	decode[[]gen.Colleague](t, colleague.do(http.MethodPost, "/tickets/"+theirs.Key+"/watchers", gen.WatcherInput{UserId: te.requesterID}), http.StatusOK)
	keys := []string{}
	for _, r := range te.requester.uatRequests(t) {
		keys = append(keys, r.Key)
	}
	if !slices.Equal(keys, []string{mine.Key, theirs.Key}) {
		t.Fatalf("tests after following = %v", keys)
	}
	if got := colleague.uatRequests(t); len(got) != 1 || got[0].Key != theirs.Key || got[0].Requester.Id != colleagueID {
		t.Fatalf("colleague's tests = %+v", got)
	}
	if got := te.otherRequester.uatRequests(t); len(got) != 0 {
		t.Fatalf("another client's requester sees %+v", got)
	}
	expectProblem(t, te.agent.do(http.MethodGet, "/uat-requests", nil), http.StatusForbidden, apperr.CodeForbidden)
}

// stored is one notification as the fan-out stored it.
type stored struct {
	event       string
	kind        *string
	messageID   *uuid.UUID
	autoApprove *string
	reopenUntil *string
	reason      *string
}

func (te *ticketEnv) notificationFor(t *testing.T, user, ticketID uuid.UUID, event string) stored {
	t.Helper()
	var n stored
	err := te.st.Pool().QueryRow(context.Background(), `
		SELECT event_type, data->>'kind', message_id, data->>'auto_approve_at', data->>'reopen_until', data->>'reason'
		FROM notifications WHERE user_id = $1 AND ticket_id = $2 AND event_type = $3
		ORDER BY created_at DESC, id DESC LIMIT 1`, user, ticketID, event).
		Scan(&n.event, &n.kind, &n.messageID, &n.autoApprove, &n.reopenUntil, &n.reason)
	if err != nil {
		t.Fatalf("%s notification for %s: %v", event, user, err)
	}
	return n
}

func TestUATNotifications(t *testing.T) {
	te := newTicketEnv(t)
	testdb.SetProjectDoD(t, te.st, te.projectA, uatDoD())
	supervisorID := userID(t, te.env, auth.RoleSupervisor)
	te.setFrequency(t, te.requester, gen.EmailFrequencyDaily)
	tk := te.requester.report(t, incident(te.projectA, "Disposition letter shows the wrong date"))
	id := te.ticketID(t, tk.Key)
	decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	te.toUAT(t, tk.Key)
	te.fanout(t)

	// The test request quotes the instructions and dates the auto-approval; it is urgent, so a
	// daily-summary requester gets it right away.
	req := te.notificationFor(t, te.requesterID, id, string(notification.EventUATRequested))
	msgs := te.requester.messages(t, tk.Key)
	if req.messageID == nil || *req.messageID != msgs[len(msgs)-1].Id || req.autoApprove == nil {
		t.Fatalf("UAT request = %+v", req)
	}
	te.age(t, notification.GroupWindow)
	te.sendEmails(t)
	if got := mailsTo(te.mailer.Messages(), te.users[auth.RoleRequester]); len(got) != 1 {
		t.Fatalf("daily requester got %v, want the UAT request right away", subjects(got))
	}

	// A rejection reaches the assignee with the client's words.
	te.fanout(t)
	decode[gen.Ticket](t, te.requester.answer(tk.Key, gen.UatAnswerReject, rejectMessage), http.StatusOK)
	te.fanout(t)
	res := te.notificationFor(t, te.agentID, id, string(notification.EventUATResult))
	msgs = te.agent.messages(t, tk.Key)
	if res.kind == nil || *res.kind != string(notification.KindRejected) || res.messageID == nil || *res.messageID != msgs[len(msgs)-1].Id {
		t.Fatalf("UAT result = %+v", res)
	}

	// Resolving without the Definition of Done tells the supervisors, with the reason; the
	// requester hears it's resolved, with the reopen date and without the reason.
	decode[gen.Ticket](t, te.agent.setStatus(tk.Key, gen.TicketStatusResolved, skipReason), http.StatusOK)
	te.fanout(t)
	sup := te.notificationFor(t, supervisorID, id, string(notification.EventDoD))
	if sup.kind == nil || *sup.kind != string(notification.KindSkipped) || sup.reason == nil || *sup.reason != skipReason {
		t.Fatalf("supervisor alert = %+v", sup)
	}
	done := te.notificationFor(t, te.requesterID, id, string(notification.EventResolved))
	if done.reopenUntil == nil || done.reason != nil {
		t.Fatalf("requester resolved = %+v", done)
	}

	// A ticket's own Definition of Done tells the supervisors too; resetting it doesn't.
	other := te.requester.report(t, incident(te.projectA, "Archive search is slow"))
	otherID := te.ticketID(t, other.Key)
	hotfix := dod.None()
	decode[gen.Ticket](t, te.agent.setDod(other.Key, &hotfix, hotfixReason), http.StatusOK)
	decode[gen.Ticket](t, te.agent.setDod(other.Key, nil, ""), http.StatusOK)
	te.fanout(t)
	n := 0
	for _, x := range te.notices(t, otherID) {
		if x.user == supervisorID && x.event == notification.EventDoD {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("supervisor DoD alerts = %d, want 1", n)
	}
}
