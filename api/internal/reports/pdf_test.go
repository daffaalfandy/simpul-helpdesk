package reports

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/report"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// pdfOutEnv names a directory to keep the rendered PDFs in, for a look by eye.
const pdfOutEnv = "SIMDESK_REPORT_PDF_DIR"

func testPeriod(t *testing.T) Period {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	p := Period{Month: report.Month{Year: 2026, Month: time.October}, Current: report.Month{Year: 2026, Month: time.November}, Location: loc}
	p.Start, p.End = p.Month.Bounds(loc)
	return p
}

func testProject() db.ListReportProjectsRow {
	no := "027/KOMINFO-JMB/2026"
	start, end := calendar.NewDate(2026, time.January, 1), calendar.NewDate(2026, time.December, 31)
	return db.ListReportProjectsRow{
		ID: uuid.New(), Name: "e-Office", Active: true, ClientID: uuid.New(), ClientName: "Dinas Kominfo Jambi",
		ContractNo: &no, ContractStart: &start, ContractEnd: &end,
	}
}

// testPriorities are the default priority names, with P4 left unnamed.
var testPriorities = map[sla.Priority]string{sla.P1: "Critical", sla.P2: "High", sla.P3: "Standard"}

func testRows(project db.ListReportProjectsRow, start time.Time, n int) []db.ListReportTicketsRow {
	subjects := []string{
		"Cannot sign in to e-Office after the password reset",
		"Disposisi surat tidak muncul di kotak masuk kepala bidang sejak pembaruan sistem kemarin sore 🙏",
		"=HYPERLINK(\"http://evil.test\")",
		"Short",
	}
	rows := make([]db.ListReportTicketsRow, n)
	for i := range rows {
		created := start.Add(time.Duration(i) * 7 * time.Hour)
		answered := created.Add(20 * time.Minute)
		elapsed := int64((time.Duration(i%5+1) * 10 * time.Minute) / time.Millisecond)
		row := db.ListReportTicketsRow{
			ID: uuid.New(), Key: fmt.Sprintf("HD-%d", 24000+i), ClientID: project.ClientID, ClientName: project.ClientName,
			ProjectID: project.ID, ProjectName: project.Name,
			Type: string(ticket.TypeIncident), Channel: string(ticket.ChannelPortal), Status: string(ticket.StatusClosed),
			Priority: string(sla.Priorities[i%len(sla.Priorities)]), Subject: subjects[i%len(subjects)], CreatedAt: created,
			FirstResponseAt: &answered, FirstResponseMetAt: &answered, FirstResponseElapsedMs: &elapsed,
			PausesWhileWaiting: true,
		}
		if i%3 == 0 {
			row.Type = string(ticket.TypeRequest)
		}
		if i%7 == 0 {
			row.ResolutionBreachedAt = &answered
		}
		if i%11 != 0 {
			resolved := created.Add(4 * time.Hour)
			row.ResolvedAt = &resolved
		} else {
			row.Status = string(ticket.StatusInProgress)
		}
		rows[i] = row
	}
	return rows
}

func TestRenderPDF(t *testing.T) {
	period := testPeriod(t)
	project := testProject()
	note := "Data is stored in Indonesia."
	tests := []struct {
		name      string
		rows      int
		wantPages int
	}{
		{"no tickets", 0, 1},
		{"a few tickets", 5, 1},
		{"a month that runs over several pages", 90, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := exportData{
				Period: period, Generated: time.Date(2026, time.November, 16, 7, 5, 0, 0, time.UTC),
				Org:  db.GetOrganizationRow{Name: "PT Vendor Teknologi", FooterNote: &note},
				Rows: testRows(project, period.Start, tt.rows),
			}
			body, pages, err := renderPDF(d, project, testPriorities)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(body, []byte("%PDF-")) {
				t.Fatalf("not a PDF: %q", body[:min(len(body), 16)])
			}
			if pages != tt.wantPages {
				t.Errorf("pages = %d, want %d", pages, tt.wantPages)
			}
			if dir := os.Getenv(pdfOutEnv); dir != "" {
				if err := os.WriteFile(fmt.Sprintf("%s/%d-tickets.pdf", dir, tt.rows), body, 0o600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestPDFFileName(t *testing.T) {
	period := testPeriod(t)
	tests := []struct {
		client, project, want string
	}{
		{"Dinas Kominfo Jambi", "e-Office", "sla-report-2026-10-dinas-kominfo-jambi-e-office.pdf"},
		{"Bappeda Prov. Riau", "SIPD (lama)", "sla-report-2026-10-bappeda-prov-riau-sipd-lama.pdf"},
		{"日本", "e-Office", "sla-report-2026-10-e-office.pdf"},
		{"Dinas Kominfo Jambi", "日本", "sla-report-2026-10-dinas-kominfo-jambi.pdf"},
	}
	for _, tt := range tests {
		p := testProject()
		p.ClientName, p.Name = tt.client, tt.project
		f, err := buildPDF(exportData{Period: period}, p, testPriorities)
		if err != nil {
			t.Fatal(err)
		}
		if f.Filename != tt.want {
			t.Errorf("file name for %q / %q = %q, want %q", tt.client, tt.project, f.Filename, tt.want)
		}
	}
	p := testProject()
	p.ClientName, p.Name = "日本", "東京"
	f, err := buildPDF(exportData{Period: period}, p, testPriorities)
	if err != nil {
		t.Fatal(err)
	}
	if want := "sla-report-2026-10-" + p.ID.String() + ".pdf"; f.Filename != want {
		t.Errorf("file name without ASCII letters = %q, want %q", f.Filename, want)
	}
}

func TestPriorityCells(t *testing.T) {
	w, err := newPDFWriter()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		priority sla.Priority
		name     string
		want     string
	}{
		{sla.P1, "Critical", "P1 · Critical"},
		{sla.P4, "", "P4"},
	}
	for _, tt := range tests {
		cells := w.priorityCells(report.PriorityGroup{Priority: tt.priority}, tt.name)
		if got := cells[0].lines; len(got) != 1 || got[0] != tt.want {
			t.Errorf("priority cell = %q, want %q", got, tt.want)
		}
		if len(cells) != len(priorityColumns) {
			t.Errorf("%d cells for %d columns", len(cells), len(priorityColumns))
		}
	}
	if w.err != nil || w.c.err != nil {
		t.Fatal(w.err, w.c.err)
	}
}
