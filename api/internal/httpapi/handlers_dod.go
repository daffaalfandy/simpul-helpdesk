package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

func apiDodConfig(c dod.Config) gen.DodConfig {
	return gen.DodConfig{
		CodeOnBranch:      gen.DodCodeOnBranch{Enabled: c.CodeOnBranch.Enabled, Branch: c.CodeOnBranch.Branch},
		Uat:               gen.DodUat{Enabled: c.UAT.Enabled, AutoApproveWorkingDays: gen.AutoApproveDays(c.UAT.AutoApproveWorkingDays)},
		RequesterConfirms: c.RequesterConfirms,
	}
}

func domainDodConfig(c gen.DodConfig) dod.Config {
	return dod.Config{
		CodeOnBranch:      dod.CodeOnBranch{Enabled: c.CodeOnBranch.Enabled, Branch: c.CodeOnBranch.Branch},
		UAT:               dod.UAT{Enabled: c.Uat.Enabled, AutoApproveWorkingDays: int(c.Uat.AutoApproveWorkingDays)},
		RequesterConfirms: c.RequesterConfirms,
	}
}

// SetTicketDod is staff only; the service checks the project's override switch.
func (s *server) SetTicketDod(ctx context.Context, req gen.SetTicketDodRequestObject) (gen.SetTicketDodResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	var cfg *dod.Config
	if req.Body.Dod != nil {
		c := domainDodConfig(*req.Body.Dod)
		cfg = &c
	}
	reason := ""
	if req.Body.Reason != nil {
		reason = *req.Body.Reason
	}
	t, err := s.tickets.SetDoD(ctx, p, req.TicketKey, cfg, reason)
	if err != nil {
		return nil, err
	}
	return gen.SetTicketDod200JSONResponse(apiTicket(t)), nil
}

func apiTicketDod(d tickets.TicketDoD) gen.TicketDod {
	out := gen.TicketDod{
		Source: gen.DodSource(d.Source), Config: apiDodConfig(d.Config), ProjectConfig: apiDodConfig(d.ProjectConfig),
		OverrideReason: d.OverrideReason, Items: make([]gen.TicketDodItem, len(d.Items)), Met: d.Met,
		Pending: apiDodItems(d.Pending), Mandatory: d.Mandatory, HasRepository: d.HasRepository, CanOverride: d.CanOverride,
	}
	for i, it := range d.Items {
		out.Items[i] = gen.TicketDodItem{Item: gen.DodItem(it.Item), DoneAt: it.DoneAt, DoneBy: apiRefPtr(it.DoneBy)}
		if it.Commit != nil {
			out.Items[i].Commit = &gen.TicketDodCommit{Sha: it.Commit.SHA, Url: it.Commit.URL}
		}
	}
	return out
}

func apiDodItems(items []dod.Item) []gen.DodItem {
	out := make([]gen.DodItem, len(items))
	for i, it := range items {
		out[i] = gen.DodItem(it)
	}
	return out
}

// AnswerTicketUat is for client staff; the service scopes the ticket to their client.
func (s *server) AnswerTicketUat(ctx context.Context, req gen.AnswerTicketUatRequestObject) (gen.AnswerTicketUatResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapPortal)
	if err != nil {
		return nil, err
	}
	message := ""
	if req.Body.Message != nil {
		message = *req.Body.Message
	}
	t, err := s.tickets.AnswerUAT(ctx, p, req.TicketKey, ticket.UATAnswer(req.Body.Answer), message)
	if err != nil {
		return nil, err
	}
	return gen.AnswerTicketUat200JSONResponse(apiTicket(t)), nil
}

func (s *server) ReopenTicket(ctx context.Context, req gen.ReopenTicketRequestObject) (gen.ReopenTicketResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapPortal)
	if err != nil {
		return nil, err
	}
	t, err := s.tickets.Reopen(ctx, p, req.TicketKey)
	if err != nil {
		return nil, err
	}
	return gen.ReopenTicket200JSONResponse(apiTicket(t)), nil
}

func (s *server) ListUatRequests(ctx context.Context, _ gen.ListUatRequestsRequestObject) (gen.ListUatRequestsResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapPortal)
	if err != nil {
		return nil, err
	}
	list, err := s.tickets.UATRequests(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListUatRequests200JSONResponse, len(list))
	for i, r := range list {
		out[i] = gen.UatRequest{
			Key: r.Key, Subject: r.Subject, Project: apiRef(r.Project), Requester: apiRef(r.Requester),
			AutoApproveAt: r.AutoApproveAt, Instructions: r.Instructions,
		}
	}
	return out, nil
}

func apiTicketUat(u *tickets.TicketUAT) *gen.TicketUat {
	if u == nil {
		return nil
	}
	return &gen.TicketUat{AutoApproveAt: u.AutoApproveAt, CanAnswer: u.CanAnswer, Instructions: u.Instructions}
}

func apiTicketReopen(r *tickets.TicketReopen) *gen.TicketReopen {
	if r == nil {
		return nil
	}
	return &gen.TicketReopen{Until: r.Until, CanReopen: r.CanReopen}
}

func apiCardUat(u *tickets.CardUAT) *gen.BoardCardUat {
	if u == nil {
		return nil
	}
	return &gen.BoardCardUat{Day: u.Day, Days: u.Days}
}
