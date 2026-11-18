package httpapi_test

import (
	"bytes"
	"context"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/report"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/reports"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
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

func reportProject(t *testing.T, r gen.SlaReport, name string) gen.SlaReportProject {
	t.Helper()
	for _, p := range r.Projects {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no project %q in the report", name)
	return gen.SlaReportProject{}
}

// reportRows names the report's rows "client / project", in order.
func reportRows(r gen.SlaReport) []string {
	out := make([]string, len(r.Projects))
	for i, p := range r.Projects {
		out[i] = p.Client.Name + " / " + p.Name
	}
	return out
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
	now := report.MonthOf(time.Now(), jakarta)
	current := now.String()
	if r.Month != "2026-10" || r.ToDate || r.CurrentMonth != current || r.FirstMonth == nil || *r.FirstMonth != "2026-10" {
		t.Fatalf("month = %s, toDate = %v, current = %s, first = %v", r.Month, r.ToDate, r.CurrentMonth, r.FirstMonth)
	}
	if r.OldestMonth != report.Oldest(now).String() {
		t.Errorf("oldest = %s, want %s", r.OldestMonth, report.Oldest(now))
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
	// The service request went to the client's default project, e-Office.
	a := reportProject(t, r, "e-Office")
	if a.Client.Id != te.clientA || a.Client.Name != requesterClient || !a.Active ||
		a.Figures.Tickets != 4 || a.Figures.Breaches != 1 || !a.BelowTarget {
		t.Errorf("%s = %+v, below target %v; want %s's, 4 tickets, 1 breach, below target", a.Name, a.Figures, a.BelowTarget, requesterClient)
	}
	b := reportProject(t, r, "Network & infra support")
	if b.Figures.Tickets != 1 || b.BelowTarget || b.Figures.AvgFirstResponseMs != nil {
		t.Errorf("%s = %+v; want only the 31 Oct ticket, on target, no average", b.Name, b.Figures)
	}
	// Busiest first; a project without tickets is still listed.
	want := []string{requesterClient + " / e-Office", otherClient + " / Network & infra support", requesterClient + " / Email"}
	if got := reportRows(r); !slices.Equal(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
	if r.ProjectsWithTickets != 2 || r.ProjectsBelowTarget != 1 {
		t.Errorf("projects with tickets = %d, below target = %d", r.ProjectsWithTickets, r.ProjectsBelowTarget)
	}
	if r.WaitingPause == nil || *r.WaitingPause != gen.SlaWaitingPauseSome {
		t.Errorf("waitingPause = %v, want some", r.WaitingPause)
	}

	// Without a month: the current one, to date. Admins may read it too.
	thisMonth := te.admin.slaReport(t, "")
	if thisMonth.Month != current || !thisMonth.ToDate {
		t.Errorf("default month = %s (to date %v), want %s", thisMonth.Month, thisMonth.ToDate, current)
	}

	// Every project is listed, even without tickets in the month.
	empty := te.supervisor.slaReport(t, "?month=2026-09")
	if empty.Totals.Tickets != 0 || empty.Totals.WithinSlaPercent != nil || empty.WaitingPause != nil ||
		len(empty.Projects) != len(r.Projects) || empty.ProjectsWithTickets != 0 || empty.ProjectsBelowTarget != 0 {
		t.Errorf("empty month = %+v", empty)
	}
}

// A hidden project shows only in months it has tickets; its tickets count in the totals.
func TestSLAReportHiddenProjects(t *testing.T) {
	te := newTicketEnv(t)
	legacy := testdb.Project(t, te.st, te.clientA, "Legacy portal", false)
	archive := testdb.Project(t, te.st, te.clientB, "Archive", false)
	old := te.requester.report(t, incident(legacy, "Old portal down"))
	te.exec(t, `UPDATE projects SET active = false WHERE id = ANY($1)`, []uuid.UUID{legacy, archive})

	r := te.supervisor.slaReport(t, "")
	want := []string{requesterClient + " / Legacy portal", otherClient + " / Network & infra support",
		requesterClient + " / e-Office", requesterClient + " / Email"}
	if got := reportRows(r); !slices.Equal(got, want) {
		t.Errorf("rows = %v, want %v (hidden Archive left out)", got, want)
	}
	if p := reportProject(t, r, "Legacy portal"); p.Active || p.Figures.Tickets != 1 {
		t.Errorf("Legacy portal = active %v, %+v; want hidden with 1 ticket", p.Active, p.Figures)
	}
	if r.Totals.Tickets != 1 || r.ProjectsWithTickets != 1 {
		t.Errorf("totals = %+v, with tickets = %d", r.Totals, r.ProjectsWithTickets)
	}

	// Moved to another month, the hidden project drops off this one.
	te.reportedAt(t, old.Key, time.Now().AddDate(0, -2, 0))
	r = te.supervisor.slaReport(t, "")
	if got := reportRows(r); slices.Contains(got, requesterClient+" / Legacy portal") || len(got) != 3 {
		t.Errorf("rows without its tickets = %v", got)
	}
	// A hidden project still has its PDF.
	if rec := te.supervisor.do(http.MethodGet, "/reports/sla/projects/"+archive.String()+"/pdf", nil); rec.Code != http.StatusOK {
		t.Errorf("hidden project's PDF: status %d", rec.Code)
	}
}

func mustJakarta(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestSLAReportAccessAndMonths(t *testing.T) {
	te := newTicketEnv(t)
	for name, s := range map[string]session{"agent": te.agent, "requester": te.requester} {
		if rec := s.do(http.MethodGet, "/reports/sla", nil); rec.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", name, rec.Code)
		}
	}
	expectProblem(t, te.supervisor.do(http.MethodGet, "/reports/sla?month=2026-13", nil), http.StatusBadRequest, apperr.CodeInvalidRequest)
	now := report.MonthOf(time.Now(), mustJakarta(t))
	oldest := report.Oldest(now)
	tooOld := oldest.Prev().String()
	for _, path := range []string{"/reports/sla", "/reports/sla/excel", "/reports/sla/projects/" + te.projectA.String() + "/pdf"} {
		expectProblem(t, te.supervisor.do(http.MethodGet, path+"?month="+now.Next().String(), nil), http.StatusBadRequest, apperr.CodeMonthInFuture)
		expectProblem(t, te.supervisor.do(http.MethodGet, path+"?month="+tooOld, nil), http.StatusBadRequest, apperr.CodeMonthOutOfRange)
	}
	// The oldest month is still served.
	if r := te.supervisor.slaReport(t, "?month="+oldest.String()); r.Month != oldest.String() || r.OldestMonth != oldest.String() {
		t.Errorf("oldest month = %s (oldest %s)", r.Month, r.OldestMonth)
	}
}

func TestSLAReportExcel(t *testing.T) {
	te := newTicketEnv(t)
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	formula := te.requester.report(t, incident(te.projectA, `=HYPERLINK("http://evil.test","Click")`))
	te.reportedAt(t, formula.Key, time.Date(2026, time.October, 15, 9, 0, 0, 0, jakarta))
	te.answeredAfter(t, formula.Key, 16*time.Minute)

	for name, s := range map[string]session{"agent": te.agent, "requester": te.requester} {
		if rec := s.do(http.MethodGet, "/reports/sla/excel", nil); rec.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", name, rec.Code)
		}
	}
	rec := te.supervisor.do(http.MethodGet, "/reports/sla/excel?month=2026-10", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	h := rec.Header()
	if got := h.Get("Content-Type"); got != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := h.Get("Content-Disposition"); got != `attachment; filename=sla-report-2026-10.xlsx` {
		t.Errorf("Content-Disposition = %q", got)
	}
	if h.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", h.Get("Cache-Control"))
	}

	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if got := f.GetSheetList(); !slices.Equal(got, []string{"Summary", "Tickets"}) {
		t.Fatalf("sheets = %v", got)
	}
	cell := func(sheet, ref string) string {
		t.Helper()
		v, err := f.GetCellValue(sheet, ref)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if got := cell("Summary", "A2"); got != "SLA report · October 2026" {
		t.Errorf("heading = %q", got)
	}
	if got := cell("Summary", "F6"); got != "Projects below target" {
		t.Errorf("KPI header = %q", got)
	}
	header := []string{cell("Summary", "A9"), cell("Summary", "B9"), cell("Summary", "C9"), cell("Summary", "H9")}
	if want := []string{"Client", "Project", "Contract no.", "Within SLA"}; !slices.Equal(header, want) {
		t.Errorf("project header = %v, want %v", header, want)
	}
	// One row per project, the busiest first, then by client and project name.
	var rows []string
	for r := 10; r <= 13; r++ {
		row := strconv.Itoa(r)
		rows = append(rows, cell("Summary", "A"+row)+" / "+cell("Summary", "B"+row)+" / "+cell("Summary", "H"+row))
	}
	want := []string{
		requesterClient + " / e-Office / 100.0%", otherClient + " / Network & infra support / No tickets",
		requesterClient + " / Email / No tickets", " /  / ",
	}
	if !slices.Equal(rows, want) {
		t.Errorf("project rows = %q, want %q", rows, want)
	}
	if panes, err := f.GetPanes("Summary"); err != nil || !panes.Freeze || panes.YSplit != 9 {
		t.Errorf("Summary panes = %+v, %v; want frozen through the project header (row 9)", panes, err)
	}
	if got := cell("Tickets", "B2") + " / " + cell("Tickets", "C2"); got != requesterClient+" / e-Office" {
		t.Errorf("ticket's client and project = %q", got)
	}
	if got := cell("Tickets", "I1"); got != "Reported (Asia/Jakarta)" {
		t.Errorf("reported header = %q", got)
	}
	if got := cell("Tickets", "A2"); got != formula.Key {
		t.Errorf("first ticket = %q, want %s", got, formula.Key)
	}
	if got := cell("Tickets", "I2"); got != "2026-10-15 09:00" {
		t.Errorf("reported = %q, want local time", got)
	}
	if got := cell("Tickets", "K2"); got != "16" {
		t.Errorf("time to first response = %q, want 16", got)
	}
	// A subject that looks like a formula is stored as text.
	if got, _ := f.GetCellFormula("Tickets", "G2"); got != "" {
		t.Errorf("subject became a formula: %q", got)
	}
	if got := cell("Tickets", "G2"); got != formula.Subject {
		t.Errorf("subject = %q", got)
	}
}

func TestSLAReportPDF(t *testing.T) {
	te := newTicketEnv(t)
	jakarta := mustJakarta(t)
	tk := te.requester.report(t, incident(te.projectA, "Printer offline"))
	te.reportedAt(t, tk.Key, time.Date(2026, time.October, 15, 9, 0, 0, 0, jakarta))

	path := "/reports/sla/projects/" + te.projectA.String() + "/pdf?month=2026-10"
	for name, s := range map[string]session{"agent": te.agent, "requester": te.requester} {
		if rec := s.do(http.MethodGet, path, nil); rec.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", name, rec.Code)
		}
	}
	rec := te.supervisor.do(http.MethodGet, path, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != "attachment; filename=sla-report-2026-10-dinas-kominfo-jambi-e-office.pdf" {
		t.Errorf("Content-Disposition = %q", got)
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-")) {
		t.Errorf("body is not a PDF")
	}
	// A project without tickets still gets its report.
	if rec := te.admin.do(http.MethodGet, "/reports/sla/projects/"+te.projectB.String()+"/pdf?month=2026-10", nil); rec.Code != http.StatusOK {
		t.Errorf("project without tickets: status %d", rec.Code)
	}
	expectProblem(t, te.supervisor.do(http.MethodGet, "/reports/sla/projects/"+uuid.NewString()+"/pdf", nil), http.StatusNotFound, apperr.CodeNotFound)

	// Another organization's project is not found, even for an Administrator.
	org2 := te.secondOrganization(t, "PT Mitra Data", "MD")
	foreignClient, err := te.st.Q().UpsertClient(context.Background(), db.UpsertClientParams{OrganizationID: org2, Name: "Dinas Kesehatan"})
	if err != nil {
		t.Fatal(err)
	}
	foreign := testdb.Project(t, te.st, foreignClient, "SIMRS", true)
	for _, s := range []session{te.supervisor, te.admin} {
		expectProblem(t, s.do(http.MethodGet, "/reports/sla/projects/"+foreign.String()+"/pdf", nil), http.StatusNotFound, apperr.CodeNotFound)
	}
}

// One project's PDF holds only that project's tickets, so a client never sees another's numbers.
func TestSLAReportPDFTickets(t *testing.T) {
	te := newTicketEnv(t)
	ctx := context.Background()
	own := te.requester.report(t, incident(te.projectA, "Printer offline"))
	te.requester.report(t, incident(te.projectA2, "Mailbox full"))
	te.otherRequester.report(t, incident(te.projectB, "Router down"))

	svc := reports.NewService(te.st)
	f, err := svc.ProjectPDF(ctx, te.org, te.projectA, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(f.Body, []byte("%PDF-")) {
		t.Fatal("not a PDF")
	}
	rows, err := te.st.Q().ListReportTickets(ctx, db.ListReportTicketsParams{
		OrganizationID: te.org, MonthStart: time.Now().Add(-time.Hour), MonthEnd: time.Now().Add(time.Hour), ProjectID: &te.projectA,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Key != own.Key || rows[0].ClientName != requesterClient {
		t.Errorf("project rows = %+v, want only %s", rows, own.Key)
	}
}
