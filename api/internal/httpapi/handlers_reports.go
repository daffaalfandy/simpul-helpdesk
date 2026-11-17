package httpapi

import (
	"bytes"
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/report"
)

func (s *server) GetSlaReport(ctx context.Context, req gen.GetSlaReportRequestObject) (gen.GetSlaReportResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapReportsView)
	if err != nil {
		return nil, err
	}
	month, err := reportMonth(req.Params.Month)
	if err != nil {
		return nil, err
	}
	r, err := s.reports.SLA(ctx, p.OrganizationID, month)
	if err != nil {
		return nil, err
	}
	out := gen.GetSlaReport200JSONResponse{
		Month: r.Month.String(), CurrentMonth: r.Current.String(), ToDate: r.ToDate,
		TargetPercent: report.TargetPercent, Totals: apiSlaFigures(r.Totals),
		ClientsWithTickets: r.ClientsWithTickets, ClientsBelowTarget: r.ClientsBelowTarget,
		Clients: make([]gen.SlaReportClient, len(r.Clients)),
	}
	if r.First != nil {
		first := r.First.String()
		out.FirstMonth = &first
	}
	if r.WaitingPause != nil {
		wp := gen.SlaWaitingPause(*r.WaitingPause)
		out.WaitingPause = &wp
	}
	for i, c := range r.Clients {
		out.Clients[i] = gen.SlaReportClient{Id: c.ID, Name: c.Name, BelowTarget: c.BelowTarget(), Figures: apiSlaFigures(c.Figures)}
	}
	return out, nil
}

func (s *server) ExportSlaReportExcel(ctx context.Context, req gen.ExportSlaReportExcelRequestObject) (gen.ExportSlaReportExcelResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapReportsView)
	if err != nil {
		return nil, err
	}
	month, err := reportMonth(req.Params.Month)
	if err != nil {
		return nil, err
	}
	f, err := s.reports.Excel(ctx, p.OrganizationID, month)
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

func (s *server) ExportSlaReportPdf(ctx context.Context, req gen.ExportSlaReportPdfRequestObject) (gen.ExportSlaReportPdfResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapReportsView)
	if err != nil {
		return nil, err
	}
	month, err := reportMonth(req.Params.Month)
	if err != nil {
		return nil, err
	}
	f, err := s.reports.ClientPDF(ctx, p.OrganizationID, req.ClientId, month)
	if err != nil {
		return nil, err
	}
	disposition, cache, csp := downloadHeaders(f.Filename)
	return gen.ExportSlaReportPdf200ApplicationpdfResponse{
		Body: bytes.NewReader(f.Body), ContentLength: int64(len(f.Body)),
		Headers: gen.ExportSlaReportPdf200ResponseHeaders{
			ContentDisposition: &disposition, CacheControl: &cache, ContentSecurityPolicy: &csp,
		},
	}, nil
}

// reportMonth reads the optional month parameter; the spec's pattern has already checked it.
func reportMonth(param *gen.ReportMonth) (*report.Month, error) {
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
