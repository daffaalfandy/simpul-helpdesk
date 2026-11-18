package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

func (s *server) GetBoard(ctx context.Context, req gen.GetBoardRequestObject) (gen.GetBoardResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	f := tickets.BoardFilter{}
	if req.Params.Assignee != nil {
		f.Assignee = tickets.AssigneeFilter(*req.Params.Assignee)
	}
	if req.Params.Q != nil {
		f.Query = *req.Params.Q
	}
	f.SLAAtRisk = req.Params.Sla != nil && *req.Params.Sla == gen.BoardSlaFilterAtRisk
	f.Reported = tickets.ReportedRange{From: domainDatePtr(req.Params.From), To: domainDatePtr(req.Params.To)}
	b, err := s.tickets.Board(ctx, p, f)
	if err != nil {
		return nil, err
	}
	out := gen.GetBoard200JSONResponse{
		Columns:  make([]gen.BoardColumn, len(b.Columns)),
		DoneFrom: apiDate(b.DoneFrom),
		DoneTo:   apiDate(b.DoneTo),
		Stats:    apiBoardStats(b.Stats),
	}
	for i, c := range b.Columns {
		col := gen.BoardColumn{Id: gen.BoardColumnId(c.ID), Total: c.Total, Cards: make([]gen.BoardCard, len(c.Cards))}
		for j, card := range c.Cards {
			col.Cards[j] = apiBoardCard(card)
		}
		out.Columns[i] = col
	}
	return out, nil
}

func apiBoardCard(c tickets.BoardCard) gen.BoardCard {
	out := gen.BoardCard{
		Id: c.ID, Key: c.Key, Type: gen.TicketType(c.Type), Status: gen.TicketStatus(c.Status),
		Priority: gen.Priority(c.Priority), Subject: c.Subject, Client: apiRef(c.Client), Assignee: apiRefPtr(c.Assignee),
		AllowedStatuses: make([]gen.TicketStatus, len(c.AllowedStatuses)), DodPending: apiDodItems(c.DoDPending),
		DodMandatory: c.DoDMandatory, Uat: apiCardUat(c.UAT), Code: apiCardCode(c.Code),
	}
	for i, st := range c.AllowedStatuses {
		out.AllowedStatuses[i] = gen.TicketStatus(st)
	}
	if c.SLA != nil {
		out.Sla = &gen.BoardCardSla{
			Clock: gen.SlaClockName(c.SLA.Clock), Phase: gen.SlaPhase(c.SLA.View.Phase),
			RemainingMs: c.SLA.View.Remaining.Milliseconds(), DayMs: c.SLA.View.Day.Milliseconds(),
		}
	}
	return out
}

func apiBoardStats(st tickets.BoardStats) gen.BoardStats {
	out := gen.BoardStats{
		OpenTickets: st.OpenTickets, ResolvedThisWeek: st.ResolvedThisWeek, SlaBreached: st.SLABreached,
		SlaAtRisk: st.SLAAtRisk, WithinSlaPercent: st.WithinSLAPercent,
	}
	if st.SLAOldest != nil {
		out.SlaOldest = &gen.SlaProblem{Key: st.SLAOldest.Key, ClientName: st.SLAOldest.ClientName}
	}
	if st.AvgFirstResponse != nil {
		ms := st.AvgFirstResponse.Milliseconds()
		out.AvgFirstResponseMs = &ms
	}
	return out
}

func apiCardCode(c *tickets.CardCode) *gen.BoardCardCode {
	if c == nil {
		return nil
	}
	return &gen.BoardCardCode{Provider: gen.GitProvider(c.Provider), Commits: c.Commits, Sha: c.SHA, Branch: c.Branch}
}
