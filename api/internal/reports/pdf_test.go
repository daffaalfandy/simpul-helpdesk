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

func testClient() db.ListReportClientsRow {
	no := "027/KOMINFO-JMB/2026"
	start, end := calendar.NewDate(2026, time.January, 1), calendar.NewDate(2026, time.December, 31)
	return db.ListReportClientsRow{ID: uuid.New(), Name: "Dinas Kominfo Jambi", ContractNo: &no, ContractStart: &start, ContractEnd: &end}
}

func testRows(client uuid.UUID, start time.Time, n int) []db.ListReportTicketsRow {
	projects := []struct {
		id   uuid.UUID
		name string
	}{{uuid.New(), "e-Office"}, {uuid.New(), "Public website"}}
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
		p := projects[i%len(projects)]
		row := db.ListReportTicketsRow{
			ID: uuid.New(), Key: fmt.Sprintf("HD-%d", 24000+i), ClientID: client, ProjectID: p.id, ProjectName: p.name,
			Type: string(ticket.TypeIncident), Channel: string(ticket.ChannelPortal), Priority: "P3",
			Status: string(ticket.StatusClosed), Subject: subjects[i%len(subjects)], CreatedAt: created,
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
	client := testClient()
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
				Rows: testRows(client.ID, period.Start, tt.rows),
			}
			body, pages, err := renderPDF(d, client)
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
		client, want string
	}{
		{"Dinas Kominfo Jambi", "sla-report-2026-10-dinas-kominfo-jambi.pdf"},
		{"Bappeda Prov. Riau", "sla-report-2026-10-bappeda-prov-riau.pdf"},
	}
	for _, tt := range tests {
		cl := testClient()
		cl.Name = tt.client
		f, err := buildPDF(exportData{Period: period}, cl)
		if err != nil {
			t.Fatal(err)
		}
		if f.Filename != tt.want {
			t.Errorf("file name for %q = %q, want %q", tt.client, f.Filename, tt.want)
		}
	}
	cl := testClient()
	cl.Name = "日本"
	f, err := buildPDF(exportData{Period: period}, cl)
	if err != nil {
		t.Fatal(err)
	}
	if want := "sla-report-2026-10-" + cl.ID.String() + ".pdf"; f.Filename != want {
		t.Errorf("file name without ASCII letters = %q, want %q", f.Filename, want)
	}
}
