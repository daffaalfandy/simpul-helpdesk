package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/report"
)

func (s *server) GetSlaReport(ctx context.Context, req gen.GetSlaReportRequestObject) (gen.GetSlaReportResponseObject, error) {
	if _, err := auth.Require(ctx, auth.CapReportsView); err != nil {
		return nil, err
	}
	month, err := reportMonth(req.Params.Month)
	if err != nil {
		return nil, err
	}
	r, err := s.reports.SLA(ctx, month)
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
