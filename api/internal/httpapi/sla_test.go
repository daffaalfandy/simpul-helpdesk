package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
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
