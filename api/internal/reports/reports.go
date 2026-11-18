// Package reports serves the SLA report over a range of months (HANDOFF §10) and its Excel and
// PDF exports.
// The rules live in the pure internal/report package; this package loads the rows.
package reports

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/report"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

type Service struct {
	store *store.Store
	now   func() time.Time
}

func NewService(st *store.Store) *Service { return &Service{store: st, now: time.Now} }

// Months are the months asked for; a nil end is the current month.
type Months struct {
	From, To *report.Month
}

// Period is the report's months placed in the organisation's time zone.
type Period struct {
	Range   report.MonthRange
	Current report.Month // this month, organisation time
	Oldest  report.Month // the earliest month the report shows
	// First is the month of the first ticket ever; nil without tickets.
	First      *report.Month
	ToDate     bool // the range reaches the current month, which hasn't ended yet
	Start, End time.Time
	Location   *time.Location
}

// SLA is the report page: every project over the range.
type SLA struct {
	Period
	report.Report
	// WaitingPause is nil without tickets.
	WaitingPause *report.WaitingPause
}

// SLA returns the organization's report over the months asked for.
func (s *Service) SLA(ctx context.Context, orgID uuid.UUID, months Months) (SLA, error) {
	q := s.store.Q()
	p, err := s.period(ctx, q, orgID, months)
	if err != nil {
		return SLA{}, err
	}
	projects, err := q.ListReportProjects(ctx, orgID)
	if err != nil {
		return SLA{}, err
	}
	rows, err := s.tickets(ctx, q, orgID, p, nil)
	if err != nil {
		return SLA{}, err
	}
	tickets := reportTickets(rows)
	out := SLA{Period: p, Report: report.Build(reportProjects(projects), tickets)}
	if wp, ok := report.WaitingPauseOf(tickets); ok {
		out.WaitingPause = &wp
	}
	return out, nil
}

// period places the months asked for in the organization's time zone. A month that hasn't
// started, one before report.Oldest, or an end before the start is refused.
func (s *Service) period(ctx context.Context, q *db.Queries, orgID uuid.UUID, months Months) (Period, error) {
	loc, err := sla.Location(ctx, q, orgID)
	if err != nil {
		return Period{}, err
	}
	current := report.MonthOf(s.now(), loc)
	r := report.SingleMonth(current)
	if months.From != nil {
		r.From = *months.From
	}
	if months.To != nil {
		r.To = *months.To
	}
	oldest := report.Oldest(current)
	if err := r.Validate(current); err != nil {
		return Period{}, rangeError(err, oldest)
	}
	p := Period{Range: r, Current: current, Oldest: oldest, ToDate: r.To == current, Location: loc}
	p.Start, p.End = r.Bounds(loc)
	first, err := q.GetFirstTicketCreatedAt(ctx, orgID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return Period{}, err
	default:
		fm := report.MonthOf(first, loc)
		p.First = &fm
	}
	return p, nil
}

// rangeError is the API error for a range report.MonthRange.Validate refused.
func rangeError(err error, oldest report.Month) error {
	switch {
	case errors.Is(err, report.ErrMonthInFuture):
		return apperr.Invalid(apperr.CodeMonthInFuture, "This month hasn't started yet", "")
	case errors.Is(err, report.ErrMonthTooOld):
		return apperr.Invalid(apperr.CodeMonthOutOfRange, "This month is too far back",
			fmt.Sprintf("The report goes back to %s.", oldest))
	case errors.Is(err, report.ErrMonthsReversed):
		return apperr.Invalid(apperr.CodeInvalidMonthRange, "The end month is before the start month", "")
	}
	return err
}

// tickets lists the range's tickets, of one project when projectID is set.
func (s *Service) tickets(ctx context.Context, q *db.Queries, orgID uuid.UUID, p Period, projectID *uuid.UUID) ([]db.ListReportTicketsRow, error) {
	return q.ListReportTickets(ctx, db.ListReportTicketsParams{OrganizationID: orgID, PeriodStart: p.Start, PeriodEnd: p.End, ProjectID: projectID})
}

func reportProjects(rows []db.ListReportProjectsRow) []report.Project {
	out := make([]report.Project, len(rows))
	for i, r := range rows {
		out[i] = report.Project{ID: r.ID, Name: r.Name, Client: report.Ref{ID: r.ClientID, Name: r.ClientName}, Active: r.Active}
	}
	return out
}

func reportTickets(rows []db.ListReportTicketsRow) []report.Ticket {
	out := make([]report.Ticket, len(rows))
	for i, r := range rows {
		out[i] = reportTicket(r)
	}
	return out
}

func reportTicket(r db.ListReportTicketsRow) report.Ticket {
	t := report.Ticket{
		ProjectID: r.ProjectID, Priority: sla.Priority(r.Priority),
		Type: ticket.Type(r.Type), Channel: ticket.Channel(r.Channel),
		FirstResponseBreached: r.FirstResponseBreachedAt != nil, ResolutionBreached: r.ResolutionBreachedAt != nil,
		PausesWhileWaiting: r.PausesWhileWaiting,
	}
	if r.FirstResponseMetAt != nil && r.FirstResponseElapsedMs != nil {
		d := time.Duration(*r.FirstResponseElapsedMs) * time.Millisecond
		t.FirstResponse = &d
	}
	return t
}

// Export is a file to download.
type Export struct {
	Filename string
	Body     []byte
}

// exportData is everything a file shows.
type exportData struct {
	Period
	Org       db.GetOrganizationRow
	Generated time.Time
	Projects  []db.ListReportProjectsRow
	Rows      []db.ListReportTicketsRow
}

// Excel returns the range for every project: a Summary sheet and a Tickets sheet.
func (s *Service) Excel(ctx context.Context, orgID uuid.UUID, months Months) (Export, error) {
	q := s.store.Q()
	d, err := s.exportData(ctx, q, orgID, months)
	if err != nil {
		return Export{}, err
	}
	if d.Projects, err = q.ListReportProjects(ctx, orgID); err != nil {
		return Export{}, err
	}
	if d.Rows, err = s.tickets(ctx, q, orgID, d.Period, nil); err != nil {
		return Export{}, err
	}
	return buildExcel(d)
}

func (s *Service) exportData(ctx context.Context, q *db.Queries, orgID uuid.UUID, months Months) (exportData, error) {
	p, err := s.period(ctx, q, orgID, months)
	if err != nil {
		return exportData{}, err
	}
	org, err := q.GetOrganization(ctx, orgID)
	if err != nil {
		return exportData{}, err
	}
	return exportData{Period: p, Org: org, Generated: s.now()}, nil
}

// ProjectPDF returns one project's range, to send to its client: the figures, a row per
// priority and every ticket. It never shows another project's numbers.
func (s *Service) ProjectPDF(ctx context.Context, orgID, projectID uuid.UUID, months Months) (Export, error) {
	q := s.store.Q()
	d, err := s.exportData(ctx, q, orgID, months)
	if err != nil {
		return Export{}, err
	}
	project, err := q.GetReportProject(ctx, db.GetReportProjectParams{ID: projectID, OrganizationID: orgID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Export{}, apperr.NotFound()
	}
	if err != nil {
		return Export{}, err
	}
	if d.Rows, err = s.tickets(ctx, q, orgID, d.Period, &projectID); err != nil {
		return Export{}, err
	}
	levels, err := q.ListPriorityLevels(ctx, orgID)
	if err != nil {
		return Export{}, err
	}
	names := make(map[sla.Priority]string, len(levels))
	for _, l := range levels {
		names[sla.Priority(l.Code)] = l.Name
	}
	return buildPDF(d, db.ListReportProjectsRow(project), names)
}
