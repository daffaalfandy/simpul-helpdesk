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
	b, err := s.tickets.Board(ctx, p, f)
	if err != nil {
		return nil, err
	}
	out := gen.GetBoard200JSONResponse{
		Columns:       make([]gen.BoardColumn, len(b.Columns)),
		ResolvedSince: b.ResolvedSince,
		Stats:         gen.BoardStats{OpenTickets: b.Stats.OpenTickets, ResolvedThisWeek: b.Stats.ResolvedThisWeek},
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
		AllowedStatuses: make([]gen.TicketStatus, len(c.AllowedStatuses)),
	}
	for i, st := range c.AllowedStatuses {
		out.AllowedStatuses[i] = gen.TicketStatus(st)
	}
	return out
}
