package httpapi_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

func (e *env) clocks(t *testing.T, ticketID uuid.UUID) db.TicketSla {
	t.Helper()
	row, err := e.st.Q().GetTicketSLA(context.Background(), ticketID)
	if err != nil {
		t.Fatalf("ticket_sla: %v", err)
	}
	return row
}

func (e *env) updatedAt(t *testing.T, ticketID uuid.UUID) time.Time {
	t.Helper()
	var at time.Time
	if err := e.st.Pool().QueryRow(context.Background(), `SELECT updated_at FROM tickets WHERE id = $1`, ticketID).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func TestSLAClocksFollowTheTicket(t *testing.T) {
	te := newTicketEnv(t)
	// On this project P1 resolves in 2 hours around the clock; the other priorities in 4 business hours.
	if _, err := te.st.Pool().Exec(context.Background(), `UPDATE project_sla_targets
		SET resolve_value = 2, resolve_unit = 'h', calendar = 'always' WHERE project_id = $1 AND priority = 'P1'`, te.projectA); err != nil {
		t.Fatal(err)
	}
	tk := te.requester.report(t, incident(te.projectA, "Printer offline"))
	path := "/tickets/" + tk.Key
	c := te.clocks(t, tk.Id)
	if c.ResolutionRunningSince == nil || c.ResolutionBudgetMs != (4*time.Hour).Milliseconds() ||
		c.ResolutionCalendar != string(sla.CalendarBusinessHours) || c.FirstResponseRunningSince == nil {
		t.Fatalf("new ticket clocks = %+v", c)
	}

	decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, path+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	decode[gen.TicketMessage](t, te.agent.message(tk.Key, gen.MessageVisibilityPublic, "Looking now"), http.StatusCreated)
	if c := te.clocks(t, tk.Id); c.FirstResponseMetAt == nil || c.FirstResponseRunningSince != nil || c.FirstResponseDueAt != nil {
		t.Fatalf("after the first reply = %+v", c)
	}

	move := func(to gen.TicketStatus) {
		t.Helper()
		decode[gen.Ticket](t, te.agent.do(http.MethodPut, path+"/status", gen.TicketStatusInput{Status: to}), http.StatusOK)
	}
	move(gen.TicketStatusWaitingOnClient)
	if c := te.clocks(t, tk.Id); c.ResolutionRunningSince != nil || c.ResolutionDueAt != nil || c.ResolutionAtRiskAt != nil {
		t.Fatalf("waiting on client should pause resolution: %+v", c)
	}
	got := decode[gen.Ticket](t, te.agent.do(http.MethodGet, path, nil), http.StatusOK)
	if got.Sla.ResolutionClock == nil || got.Sla.ResolutionClock.Phase != gen.SlaPhasePaused {
		t.Fatalf("paused clock in the API = %+v", got.Sla.ResolutionClock)
	}
	move(gen.TicketStatusInProgress)
	if c := te.clocks(t, tk.Id); c.ResolutionRunningSince == nil || c.ResolutionDueAt == nil {
		t.Fatalf("back in progress should resume: %+v", c)
	}

	decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, path+"/priority",
		gen.TicketPriorityInput{Priority: gen.PriorityP1, Reason: "Whole office"}), http.StatusOK)
	if c := te.clocks(t, tk.Id); c.ResolutionBudgetMs != (2*time.Hour).Milliseconds() || c.ResolutionCalendar != string(sla.CalendarAlways) {
		t.Fatalf("P1 should swap the target: %+v", c)
	}

	move(gen.TicketStatusResolved)
	if c := te.clocks(t, tk.Id); c.ResolutionMetAt == nil || c.ResolutionRunningSince != nil {
		t.Fatalf("resolved should stop the clock: %+v", c)
	}
}

func TestCalendarEditRecomputesOpenClocks(t *testing.T) {
	te := newTicketEnv(t)
	tk := te.requester.report(t, incident(te.projectA, "VPN slow"))
	before, updated := te.clocks(t, tk.Id), te.updatedAt(t, tk.Id)
	day := time.Now().AddDate(0, 1, 0).Format(time.DateOnly)
	rec := te.admin.do(http.MethodPost, "/holidays", map[string]string{"date": day, "name": "Office closed"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create holiday: %d %s", rec.Code, rec.Body)
	}
	if n := runJobs(t, te.env, te.recompute); n != 1 {
		t.Fatalf("recompute jobs run = %d, want 1", n)
	}
	after := te.clocks(t, tk.Id)
	if !after.ComputedAt.After(before.ComputedAt) {
		t.Fatalf("computed_at %v not after %v", after.ComputedAt, before.ComputedAt)
	}
	if got := te.updatedAt(t, tk.Id); !got.Equal(updated) {
		t.Fatalf("replaying moved the ticket's updated_at: %v → %v", updated, got)
	}
}

// scanAt runs the SLA scan as if the clock read at.
func (e *env) scanAt(t *testing.T, at time.Time) {
	t.Helper()
	e.scan.SetClock(func() time.Time { return at })
	if err := e.scan.Work(context.Background(), &river.Job[tickets.SLAScanArgs]{}); err != nil {
		t.Fatal(err)
	}
}

const picEmail = "head.it@kominfo-jambi.test"

func TestSLAScanFlagsAndAlerts(t *testing.T) {
	te := newTicketEnv(t)
	ctx := context.Background()
	supervisorID := userID(t, te.env, auth.RoleSupervisor)
	// P1 here runs around the clock (so the test doesn't depend on today's hours): 15 min to
	// respond, 1 h to resolve, at risk at 75%. The client's contact hears about P1 breaches.
	for _, sql := range []string{
		`UPDATE project_sla_targets SET first_response_value = 15, first_response_unit = 'min',
		   resolve_value = 1, resolve_unit = 'h', calendar = 'always' WHERE project_id = $1 AND priority = 'P1'`,
		`UPDATE projects SET sla_notify_pic_on_p1_breach = true WHERE id = $1`,
	} {
		if _, err := te.st.Pool().Exec(ctx, sql, te.projectA); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := te.st.Pool().Exec(ctx, `UPDATE clients SET pic_name = 'Head of IT', pic_email = $2 WHERE id = $1`, te.clientA, picEmail); err != nil {
		t.Fatal(err)
	}
	in := incident(te.projectA, "Whole office offline")
	in.Priority = gen.PriorityP1
	tk := te.requester.report(t, in)
	decode[gen.Ticket](t, te.supervisor.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	te.fanout(t)
	start := tk.CreatedAt

	// 50 minutes in: first response breached, resolution at risk (45 min is 75% of an hour).
	te.scanAt(t, start.Add(50*time.Minute))
	te.fanout(t)
	if got := te.auditActionsLike(t, "ticket.sla_%"); !slices.Equal(got, []string{"ticket.sla_breached", "ticket.sla_at_risk"}) {
		t.Fatalf("audit = %v", got)
	}
	c := te.clocks(t, tk.Id)
	if c.FirstResponseBreachedAt == nil || c.ResolutionAtRiskFlaggedAt == nil || c.ResolutionBreachedAt != nil {
		t.Fatalf("flags = %+v", c)
	}
	var slaNotices []notice
	for _, n := range te.notices(t, tk.Id) {
		if n.event == notification.EventSLA {
			slaNotices = append(slaNotices, n)
		}
	}
	want := []notice{{te.agentID, notification.EventSLA}, {supervisorID, notification.EventSLA}, {te.agentID, notification.EventSLA}}
	if !slices.Equal(slaNotices, want) {
		t.Fatalf("SLA notices = %v, want %v (breach: assignee and supervisor; at risk: assignee)", slaNotices, want)
	}

	// Again at the same time: nothing new.
	te.scanAt(t, start.Add(50*time.Minute))
	if got := te.auditActionsLike(t, "ticket.sla_%"); len(got) != 2 {
		t.Fatalf("a second scan added entries: %v", got)
	}

	// Past the hour: resolution breached, and the client's contact is told.
	te.scanAt(t, start.Add(70*time.Minute))
	if n := runJobs(t, te.env, te.pic); n != 1 {
		t.Fatalf("PIC emails queued = %d, want 1", n)
	}
	msgs := mailsTo(te.mailer.Messages(), picEmail)
	if len(msgs) != 1 || !strings.Contains(msgs[0].Subject, "P1 missed its fix target") ||
		!strings.Contains(msgs[0].Text, "fix target of 1 hour") || !strings.Contains(msgs[0].Text, "/portal/tickets/"+tk.Key) {
		t.Fatalf("PIC email = %+v", msgs)
	}

	// The board filters and counts what the scan flagged.
	board := decode[gen.Board](t, te.agent.do(http.MethodGet, "/board?sla=at_risk", nil), http.StatusOK)
	var keys []string
	for _, col := range board.Columns {
		for _, card := range col.Cards {
			keys = append(keys, card.Key)
			if card.Sla == nil || card.Sla.Phase != gen.SlaPhaseBreached {
				t.Errorf("card %s sla = %+v", card.Key, card.Sla)
			}
		}
	}
	if !slices.Equal(keys, []string{tk.Key}) || board.Stats.SlaBreached != 1 || board.Stats.SlaOldest == nil || board.Stats.SlaOldest.Key != tk.Key {
		t.Fatalf("at-risk board = %v, stats %+v", keys, board.Stats)
	}
	activity := decode[[]gen.TicketActivity](t, te.agent.do(http.MethodGet, "/tickets/"+tk.Key+"/activity", nil), http.StatusOK)
	last := activity[len(activity)-1]
	if last.Action != gen.TicketActivityActionSlaBreached || last.To == nil || *last.To != "resolution" {
		t.Fatalf("last activity = %+v", last)
	}
}

// A ticket the engine sees for the first time gets its past flags without alerts.
func TestSLABackfillIsSilent(t *testing.T) {
	te := newTicketEnv(t)
	tk := te.requester.report(t, incident(te.projectA, "Old ticket"))
	te.fanout(t)
	if _, err := te.st.Pool().Exec(context.Background(), `DELETE FROM ticket_sla WHERE ticket_id = $1`, tk.Id); err != nil {
		t.Fatal(err)
	}
	before := len(te.notices(t, tk.Id))
	te.scanAt(t, tk.CreatedAt.Add(30*24*time.Hour))
	c := te.clocks(t, tk.Id)
	if c.FirstResponseBreachedAt == nil || c.ResolutionBreachedAt == nil {
		t.Fatalf("backfill should flag the past breaches: %+v", c)
	}
	te.fanout(t)
	if got := te.auditActionsLike(t, "ticket.sla_%"); len(got) != 0 {
		t.Fatalf("backfill wrote alerts: %v", got)
	}
	if n := len(te.notices(t, tk.Id)); n != before {
		t.Fatalf("backfill notified: %d → %d", before, n)
	}
}

// A ticket the engine can't replay is skipped; the scan still handles the others.
func TestSLAScanSkipsABrokenTicket(t *testing.T) {
	te := newTicketEnv(t)
	ctx := context.Background()
	broken := te.requester.report(t, incident(te.projectA, "Broken snapshot"))
	fine := te.requester.report(t, incident(te.projectA, "Fine"))
	for _, sql := range []string{
		`DELETE FROM ticket_sla`,
		`UPDATE tickets SET sla_snapshot = '{"version": 99}' WHERE id = '` + broken.Id.String() + `'`,
	} {
		if _, err := te.st.Pool().Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	te.scanAt(t, time.Now())
	if _, err := te.st.Q().GetTicketSLA(ctx, fine.Id); err != nil {
		t.Fatalf("the healthy ticket wasn't replayed: %v", err)
	}
	if _, err := te.st.Q().GetTicketSLA(ctx, broken.Id); err == nil {
		t.Fatal("the broken ticket shouldn't have clocks")
	}
}
