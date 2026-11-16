package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// A month on: past every test's auto-approval and every reopen window.
const monthLater = 30 * 24 * time.Hour

const (
	sqlBreakDoD     = `UPDATE projects SET dod_config = '{"uat": "yes"}' WHERE id = $1`
	sqlSystemItems  = `SELECT count(*) FROM ticket_dod_items i JOIN tickets t ON t.id = i.ticket_id WHERE t.key = $1 AND i.done_by IS NULL`
	sqlClosedAtSet  = `SELECT closed_at IS NOT NULL FROM tickets WHERE key = $1`
	sqlLifecycleNum = `SELECT count(*) FROM audit_log WHERE actor_name = $1`
)

// lifecycleAt runs the lifecycle scan as if the clock read at.
func (e *env) lifecycleAt(t *testing.T, at time.Time) {
	t.Helper()
	e.lifecycle.SetClock(func() time.Time { return at })
	if err := e.lifecycle.Work(context.Background(), &river.Job[tickets.LifecycleScanArgs]{}); err != nil {
		t.Fatal(err)
	}
}

func (te *ticketEnv) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := te.st.Pool().QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLifecycleScan(t *testing.T) {
	te := newTicketEnv(t)
	testdb.SetProjectDoD(t, te.st, te.projectA, uatDoD())
	never := uatDoD()
	never.UAT.AutoApproveWorkingDays = dod.AutoApproveNever
	testdb.SetProjectDoD(t, te.st, te.projectA2, never)

	tested := te.requester.report(t, incident(te.projectA, "Letters show the wrong date"))
	decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, "/tickets/"+tested.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	te.toUAT(t, tested.Key)
	waits := te.requester.report(t, incident(te.projectA2, "Mailbox quota"))
	te.toUAT(t, waits.Key)
	reopened := te.otherRequester.report(t, incident(te.projectB, "Wi-Fi drops"))
	te.move(t, reopened.Key, gen.TicketStatusResolved)
	te.move(t, reopened.Key, gen.TicketStatusInProgress)
	te.fanout(t)

	// Not yet due: nothing moves.
	te.lifecycleAt(t, time.Now().Add(time.Hour))
	if n := te.count(t, sqlLifecycleNum, tickets.ActorNameLifecycle); n != 0 {
		t.Fatalf("lifecycle audit entries before the deadline = %d", n)
	}

	// A month on the overdue test is approved by the system; a test that never auto-approves waits.
	te.lifecycleAt(t, time.Now().Add(monthLater))
	got := te.agent.ticket(t, tested.Key)
	if got.Status != gen.TicketStatusResolved || !got.Dod.Met || got.Dod.Items[0].DoneBy != nil {
		t.Fatalf("auto-approved = %s %+v", got.Status, got.Dod)
	}
	if n := te.count(t, sqlSystemItems, tested.Key); n != 1 {
		t.Fatalf("system-ticked items = %d", n)
	}
	activity := te.agent.activity(t, tested.Key)
	if last := activity[len(activity)-1]; last.Cause == nil || *last.Cause != gen.TicketStatusCauseUatAutoApproved {
		t.Fatalf("activity = %+v", last)
	}
	if s := te.agent.ticket(t, waits.Key).Status; s != gen.TicketStatusInUat {
		t.Fatalf("never-auto-approve ticket = %s", s)
	}
	if s := te.admin.ticket(t, reopened.Key).Status; s != gen.TicketStatusInProgress {
		t.Fatalf("reopened ticket = %s", s)
	}
	te.fanout(t)
	id := te.ticketID(t, tested.Key)
	if res := te.notificationFor(t, te.agentID, id, string(notification.EventUATResult)); res.kind == nil || *res.kind != string(notification.KindAutoApproved) {
		t.Fatalf("assignee = %+v", res)
	}
	te.notificationFor(t, te.requesterID, id, string(notification.EventResolved))

	// Resolved only just now: the window is still open.
	te.lifecycleAt(t, time.Now().Add(time.Hour))
	if s := te.agent.ticket(t, tested.Key).Status; s != gen.TicketStatusResolved {
		t.Fatalf("closed inside the window: %s", s)
	}
	// After it, the ticket closes.
	te.lifecycleAt(t, time.Now().Add(monthLater))
	closed := te.requester.ticket(t, tested.Key)
	if closed.Status != gen.TicketStatusClosed || closed.ClosedAt == nil || closed.Reopen != nil {
		t.Fatalf("closed = %s %v %+v", closed.Status, closed.ClosedAt, closed.Reopen)
	}
	te.fanout(t)
	if n := te.notificationFor(t, te.requesterID, id, string(notification.EventResolved)); n.kind != nil {
		t.Fatalf("closed notice = %+v", n)
	}
}

// One ticket that can't be read never holds back the others.
func TestLifecycleScanSkipsBrokenTickets(t *testing.T) {
	te := newTicketEnv(t)
	testdb.SetProjectDoD(t, te.st, te.projectA, uatDoD())
	broken := te.requester.report(t, incident(te.projectA, "Letters show the wrong date"))
	te.toUAT(t, broken.Key)
	te.exec(t, sqlBreakDoD, te.projectA)
	fine := te.requester.report(t, incident(te.projectA2, "Mailbox quota"))
	te.move(t, fine.Key, gen.TicketStatusResolved)

	te.lifecycleAt(t, time.Now().Add(monthLater))
	if s := te.admin.ticket(t, fine.Key).Status; s != gen.TicketStatusClosed {
		t.Fatalf("healthy ticket = %s", s)
	}
	var closedAt bool
	if err := te.st.Pool().QueryRow(context.Background(), sqlClosedAtSet, fine.Key).Scan(&closedAt); err != nil || !closedAt {
		t.Fatalf("closed_at set = %v, %v", closedAt, err)
	}
	if n := te.count(t, `SELECT count(*) FROM tickets WHERE key = $1 AND status = 'in_uat'`, broken.Key); n != 1 {
		t.Fatal("the broken ticket moved")
	}
}
