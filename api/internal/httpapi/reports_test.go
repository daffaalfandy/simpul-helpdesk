package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/report"
)

func (s session) slaReport(t *testing.T, query string) gen.SlaReport {
	t.Helper()
	return decode[gen.SlaReport](t, s.do(http.MethodGet, "/reports/sla"+query, nil), http.StatusOK)
}

// reportedAt moves a ticket's creation into another month.
func (te *ticketEnv) reportedAt(t *testing.T, key string, at time.Time) {
	t.Helper()
	te.exec(t, `UPDATE tickets SET created_at = $2 WHERE key = $1`, key, at)
}

// answeredAfter marks the first-response clock met after elapsed time on the clock.
func (te *ticketEnv) answeredAfter(t *testing.T, key string, elapsed time.Duration) {
	t.Helper()
	te.exec(t, `UPDATE ticket_sla SET first_response_met_at = now(), first_response_elapsed_ms = $2,
		first_response_running_since = NULL, first_response_due_at = NULL, first_response_at_risk_at = NULL
		WHERE ticket_id = (SELECT id FROM tickets WHERE key = $1)`, key, elapsed.Milliseconds())
}

func (te *ticketEnv) resolutionBreached(t *testing.T, key string) {
	t.Helper()
	te.exec(t, `UPDATE ticket_sla SET resolution_breached_at = now()
		WHERE ticket_id = (SELECT id FROM tickets WHERE key = $1)`, key)
}

func reportClient(t *testing.T, r gen.SlaReport, name string) gen.SlaReportClient {
	t.Helper()
	for _, c := range r.Clients {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no client %q in the report", name)
	return gen.SlaReportClient{}
}

func TestSLAReport(t *testing.T) {
	te := newTicketEnv(t)
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	mid := time.Date(2026, time.October, 15, 9, 0, 0, 0, jakarta)

	quick := te.requester.report(t, incident(te.projectA, "Printer offline"))
	slow := te.requester.report(t, incident(te.projectA, "e-Office down"))
	logged := te.agent.logFor(t, te.clientA, te.requesterID, incident(te.projectA, "Called about VPN"))
	request := incident(te.projectA2, "New mailbox")
	request.Type, request.ProjectId = gen.TicketTypeRequest, nil
	unanswered := te.requester.report(t, request)
	for _, k := range []string{quick.Key, slow.Key, logged.Key, unanswered.Key} {
		te.reportedAt(t, k, mid)
	}
	te.answeredAfter(t, quick.Key, 10*time.Minute)
	te.answeredAfter(t, slow.Key, 30*time.Minute)
	te.answeredAfter(t, logged.Key, 0)
	te.resolutionBreached(t, slow.Key)
	// Answered late as well: still one breached ticket.
	te.exec(t, `UPDATE ticket_sla SET first_response_breached_at = now() WHERE ticket_id = $1`, slow.Id)

	// Month edges are local midnight in the organisation's time zone.
	lastOctober := te.otherRequester.report(t, incident(te.projectB, "Router down"))
	te.reportedAt(t, lastOctober.Key, time.Date(2026, time.October, 31, 23, 30, 0, 0, jakarta))
	firstNovember := te.otherRequester.report(t, incident(te.projectB, "Switch down"))
	te.reportedAt(t, firstNovember.Key, time.Date(2026, time.November, 1, 0, 0, 0, 0, jakarta))
	// This ticket's project ran its clock while waiting on the client.
	te.exec(t, `UPDATE tickets SET sla_snapshot = jsonb_set(sla_snapshot, '{pauseOnWaitingClient}', 'false') WHERE key = $1`,
		lastOctober.Key)

	r := te.supervisor.slaReport(t, "?month=2026-10")
	current := report.MonthOf(time.Now(), jakarta).String()
	if r.Month != "2026-10" || r.ToDate || r.CurrentMonth != current || r.FirstMonth == nil || *r.FirstMonth != "2026-10" {
		t.Fatalf("month = %s, toDate = %v, current = %s, first = %v", r.Month, r.ToDate, r.CurrentMonth, r.FirstMonth)
	}
	if r.TargetPercent != report.TargetPercent {
		t.Errorf("target = %v", r.TargetPercent)
	}
	tot := r.Totals
	if tot.Tickets != 5 || tot.Incidents != 4 || tot.Requests != 1 || tot.Breaches != 1 {
		t.Errorf("totals = %+v, want 5 tickets (4 incidents, 1 request), 1 breach", tot)
	}
	if tot.WithinSlaPercent == nil || *tot.WithinSlaPercent != 80 {
		t.Errorf("within = %v, want 80", tot.WithinSlaPercent)
	}
	// Portal reports answered after 10 and 30 minutes; the logged ticket stays out.
	if tot.AvgFirstResponseMs == nil || *tot.AvgFirstResponseMs != (20*time.Minute).Milliseconds() {
		t.Errorf("avg first response = %v ms, want 20 min", tot.AvgFirstResponseMs)
	}
	a := reportClient(t, r, requesterClient)
	if a.Figures.Tickets != 4 || a.Figures.Breaches != 1 || !a.BelowTarget {
		t.Errorf("%s = %+v, below target %v; want 4 tickets, 1 breach, below target", a.Name, a.Figures, a.BelowTarget)
	}
	b := reportClient(t, r, otherClient)
	if b.Figures.Tickets != 1 || b.BelowTarget || b.Figures.AvgFirstResponseMs != nil {
		t.Errorf("%s = %+v; want only the 31 Oct ticket, on target, no average", b.Name, b.Figures)
	}
	if r.Clients[0].Name != requesterClient || r.Clients[1].Name != otherClient {
		t.Errorf("order = %s, %s; want the busiest client first", r.Clients[0].Name, r.Clients[1].Name)
	}
	if r.ClientsWithTickets != 2 || r.ClientsBelowTarget != 1 {
		t.Errorf("clients with tickets = %d, below target = %d", r.ClientsWithTickets, r.ClientsBelowTarget)
	}
	if r.WaitingPause == nil || *r.WaitingPause != gen.SlaWaitingPauseSome {
		t.Errorf("waitingPause = %v, want some", r.WaitingPause)
	}

	// Without a month: the current one, to date. Admins may read it too.
	now := te.admin.slaReport(t, "")
	if now.Month != current || !now.ToDate {
		t.Errorf("default month = %s (to date %v), want %s", now.Month, now.ToDate, current)
	}

	// Every client is listed, even without tickets in the month.
	empty := te.supervisor.slaReport(t, "?month=2026-09")
	if empty.Totals.Tickets != 0 || empty.Totals.WithinSlaPercent != nil || empty.WaitingPause != nil ||
		len(empty.Clients) != len(r.Clients) || empty.ClientsBelowTarget != 0 {
		t.Errorf("empty month = %+v", empty)
	}
}

func TestSLAReportAccessAndMonths(t *testing.T) {
	te := newTicketEnv(t)
	for name, s := range map[string]session{"agent": te.agent, "requester": te.requester} {
		if rec := s.do(http.MethodGet, "/reports/sla", nil); rec.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", name, rec.Code)
		}
	}
	expectProblem(t, te.supervisor.do(http.MethodGet, "/reports/sla?month=2026-13", nil), http.StatusBadRequest, apperr.CodeInvalidRequest)
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	next := report.MonthOf(time.Now(), jakarta).Next().String()
	expectProblem(t, te.supervisor.do(http.MethodGet, "/reports/sla?month="+next, nil), http.StatusBadRequest, apperr.CodeMonthInFuture)
}
