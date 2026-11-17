// Package reports serves the monthly SLA report (HANDOFF §10) and its Excel and PDF exports.
// The rules live in the pure internal/report package; this package loads the rows.
package reports

import (
	"context"
	"errors"
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

// Period is a report month placed in the organisation's time zone.
type Period struct {
	Month   report.Month
	Current report.Month // this month, organisation time
	// First is the month of the first ticket ever; nil without tickets.
	First      *report.Month
	ToDate     bool // the month hasn't ended yet
	Start, End time.Time
	Location   *time.Location
}

// SLA is the report page: every client for one month.
type SLA struct {
	Period
	report.Report
	// WaitingPause is nil without tickets.
	WaitingPause *report.WaitingPause
}

// SLA returns the organization's report for month, or for the current month when month is nil.
func (s *Service) SLA(ctx context.Context, orgID uuid.UUID, month *report.Month) (SLA, error) {
	q := s.store.Q()
	p, err := s.period(ctx, q, orgID, month)
	if err != nil {
		return SLA{}, err
	}
	clients, err := q.ListReportClients(ctx, orgID)
	if err != nil {
		return SLA{}, err
	}
	rows, err := s.tickets(ctx, q, orgID, p, nil)
	if err != nil {
		return SLA{}, err
	}
	refs := make([]report.Ref, len(clients))
	for i, c := range clients {
		refs[i] = report.Ref{ID: c.ID, Name: c.Name}
	}
	tickets := reportTickets(rows)
	out := SLA{Period: p, Report: report.Build(refs, tickets)}
	if wp, ok := report.WaitingPauseOf(tickets); ok {
		out.WaitingPause = &wp
	}
	return out, nil
}

// period places month in the organization's time zone. A month that hasn't started is refused.
func (s *Service) period(ctx context.Context, q *db.Queries, orgID uuid.UUID, month *report.Month) (Period, error) {
	loc, err := sla.Location(ctx, q, orgID)
	if err != nil {
		return Period{}, err
	}
	current := report.MonthOf(s.now(), loc)
	m := current
	if month != nil {
		m = *month
	}
	if m.After(current) {
		return Period{}, apperr.Invalid(apperr.CodeMonthInFuture, "This month hasn't started yet", "")
	}
	p := Period{Month: m, Current: current, ToDate: m == current, Location: loc}
	p.Start, p.End = m.Bounds(loc)
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

func (s *Service) tickets(ctx context.Context, q *db.Queries, orgID uuid.UUID, p Period, clientID *uuid.UUID) ([]db.ListReportTicketsRow, error) {
	return q.ListReportTickets(ctx, db.ListReportTicketsParams{OrganizationID: orgID, MonthStart: p.Start, MonthEnd: p.End, ClientID: clientID})
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
		ClientID: r.ClientID, ProjectID: r.ProjectID, ProjectName: r.ProjectName,
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
	Clients   []db.ListReportClientsRow
	Rows      []db.ListReportTicketsRow
}

// Excel returns the month for every client: a Summary sheet and a Tickets sheet.
func (s *Service) Excel(ctx context.Context, orgID uuid.UUID, month *report.Month) (Export, error) {
	q := s.store.Q()
	d, err := s.exportData(ctx, q, orgID, month)
	if err != nil {
		return Export{}, err
	}
	if d.Clients, err = q.ListReportClients(ctx, orgID); err != nil {
		return Export{}, err
	}
	if d.Rows, err = s.tickets(ctx, q, orgID, d.Period, nil); err != nil {
		return Export{}, err
	}
	return buildExcel(d)
}

func (s *Service) exportData(ctx context.Context, q *db.Queries, orgID uuid.UUID, month *report.Month) (exportData, error) {
	p, err := s.period(ctx, q, orgID, month)
	if err != nil {
		return exportData{}, err
	}
	org, err := q.GetOrganization(ctx, orgID)
	if err != nil {
		return exportData{}, err
	}
	return exportData{Period: p, Org: org, Generated: s.now()}, nil
}

// ClientPDF returns one client's month: the figures, a row per project and every ticket. It
// never shows another client's numbers.
func (s *Service) ClientPDF(ctx context.Context, orgID, clientID uuid.UUID, month *report.Month) (Export, error) {
	q := s.store.Q()
	d, err := s.exportData(ctx, q, orgID, month)
	if err != nil {
		return Export{}, err
	}
	client, err := q.GetReportClient(ctx, db.GetReportClientParams{ID: clientID, OrganizationID: orgID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Export{}, apperr.NotFound()
	}
	if err != nil {
		return Export{}, err
	}
	if d.Rows, err = s.tickets(ctx, q, orgID, d.Period, &clientID); err != nil {
		return Export{}, err
	}
	return buildPDF(d, db.ListReportClientsRow(client))
}
