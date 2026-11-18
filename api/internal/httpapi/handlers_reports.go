package httpapi

import (
	"bytes"
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/report"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/reports"
)

func (s *server) GetSlaReport(ctx context.Context, req gen.GetSlaReportRequestObject) (gen.GetSlaReportResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapReportsView)
	if err != nil {
		return nil, err
	}
	months, err := reportMonths(req.Params.From, req.Params.To)
	if err != nil {
		return nil, err
	}
	r, err := s.reports.SLA(ctx, p.OrganizationID, months)
	if err != nil {
		return nil, err
	}
	out := gen.GetSlaReport200JSONResponse{
		From: r.Range.From.String(), To: r.Range.To.String(), ToDate: r.ToDate,
		CurrentMonth: r.Current.String(), OldestMonth: r.Oldest.String(),
		TargetPercent: report.TargetPercent, Totals: apiSlaFigures(r.Totals),
		ProjectsWithTickets: r.ProjectsWithTickets, ProjectsBelowTarget: r.ProjectsBelowTarget,
		Projects: make([]gen.SlaReportProject, len(r.Projects)),
	}
	if r.First != nil {
		first := r.First.String()
		out.FirstMonth = &first
	}
	if r.WaitingPause != nil {
		wp := gen.SlaWaitingPause(*r.WaitingPause)
		out.WaitingPause = &wp
	}
	for i, g := range r.Projects {
		out.Projects[i] = gen.SlaReportProject{
			Id: g.ID, Name: g.Name, Client: gen.Ref{Id: g.Client.ID, Name: g.Client.Name}, Active: g.Active,
			BelowTarget: g.BelowTarget(), Figures: apiSlaFigures(g.Figures),
		}
	}
	return out, nil
}

func (s *server) ExportSlaReportExcel(ctx context.Context, req gen.ExportSlaReportExcelRequestObject) (gen.ExportSlaReportExcelResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapReportsView)
	if err != nil {
		return nil, err
	}
	months, err := reportMonths(req.Params.From, req.Params.To)
	if err != nil {
		return nil, err
	}
	f, err := s.reports.Excel(ctx, p.OrganizationID, months)
	if err != nil {
		return nil, err
	}
	disposition, cache, csp := downloadHeaders(f.Filename)
	return gen.ExportSlaReportExcel200ApplicationvndOpenxmlformatsOfficedocumentSpreadsheetmlSheetResponse{
		Body: bytes.NewReader(f.Body), ContentLength: int64(len(f.Body)),
		Headers: gen.ExportSlaReportExcel200ResponseHeaders{
			ContentDisposition: &disposition, CacheControl: &cache, ContentSecurityPolicy: &csp,
		},
	}, nil
}

func (s *server) ExportSlaProjectPdf(ctx context.Context, req gen.ExportSlaProjectPdfRequestObject) (gen.ExportSlaProjectPdfResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapReportsView)
	if err != nil {
		return nil, err
	}
	months, err := reportMonths(req.Params.From, req.Params.To)
	if err != nil {
		return nil, err
	}
	f, err := s.reports.ProjectPDF(ctx, p.OrganizationID, req.ProjectId, months)
	if err != nil {
		return nil, err
	}
	disposition, cache, csp := downloadHeaders(f.Filename)
	return gen.ExportSlaProjectPdf200ApplicationpdfResponse{
		Body: bytes.NewReader(f.Body), ContentLength: int64(len(f.Body)),
		Headers: gen.ExportSlaProjectPdf200ResponseHeaders{
			ContentDisposition: &disposition, CacheControl: &cache, ContentSecurityPolicy: &csp,
		},
	}, nil
}

// reportMonths reads the optional from and to months; the spec's pattern has already checked them.
func reportMonths(from *gen.ReportFrom, to *gen.ReportTo) (reports.Months, error) {
	var out reports.Months
	var err error
	if out.From, err = optionalMonth(from); err != nil {
		return reports.Months{}, err
	}
	if out.To, err = optionalMonth(to); err != nil {
		return reports.Months{}, err
	}
	return out, nil
}

func optionalMonth(param *string) (*report.Month, error) {
	if param == nil {
		return nil, nil
	}
	m, err := report.ParseMonth(*param)
	if err != nil {
		return nil, apperr.InvalidRequest(err.Error())
	}
	return &m, nil
}

func apiSlaFigures(f report.Figures) gen.SlaFigures {
	out := gen.SlaFigures{
		Tickets: f.Tickets, Incidents: f.Incidents, Requests: f.Requests, Breaches: f.Breaches,
		WithinSlaPercent: f.WithinPercent,
	}
	if f.AvgFirstResponse != nil {
		ms := f.AvgFirstResponse.Milliseconds()
		out.AvgFirstResponseMs = &ms
	}
	return out
}
