package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

// ListClients lists the clients the user may see.
func (s *server) ListClients(ctx context.Context, _ gen.ListClientsRequestObject) (gen.ListClientsResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	list, err := s.clients.List(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListClients200JSONResponse, len(list))
	for i, c := range list {
		out[i] = apiClient(c)
	}
	return out, nil
}

func (s *server) CreateClient(ctx context.Context, req gen.CreateClientRequestObject) (gen.CreateClientResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	c, err := s.clients.Create(ctx, p.AuditActor(), clientInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return gen.CreateClient201JSONResponse(apiClient(c)), nil
}

func (s *server) GetClient(ctx context.Context, req gen.GetClientRequestObject) (gen.GetClientResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	c, err := s.clients.Get(ctx, p, req.ClientId)
	if err != nil {
		return nil, err
	}
	return gen.GetClient200JSONResponse(apiClient(c)), nil
}

func (s *server) UpdateClient(ctx context.Context, req gen.UpdateClientRequestObject) (gen.UpdateClientResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	c, err := s.clients.Update(ctx, p.AuditActor(), req.ClientId, clientInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return gen.UpdateClient200JSONResponse(apiClient(c)), nil
}

func (s *server) DeleteClient(ctx context.Context, req gen.DeleteClientRequestObject) (gen.DeleteClientResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	if err := s.clients.Delete(ctx, p.AuditActor(), req.ClientId); err != nil {
		return nil, err
	}
	return gen.DeleteClient204Response{}, nil
}

func (s *server) SetClientAgents(ctx context.Context, req gen.SetClientAgentsRequestObject) (gen.SetClientAgentsResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	c, err := s.clients.SetAgents(ctx, p.AuditActor(), req.ClientId, req.Body.UserIds)
	if err != nil {
		return nil, err
	}
	return gen.SetClientAgents200JSONResponse(apiClient(c)), nil
}

func clientInput(b gen.ClientInput) clients.Input {
	return clients.Input{
		Name: b.Name, ContractNo: b.ContractNo, ContractStart: domainDatePtr(b.ContractStart),
		ContractEnd: domainDatePtr(b.ContractEnd), PicName: b.PicName, PicEmail: b.PicEmail, SlaPlanID: b.SlaPlanId,
	}
}

func apiClient(c clients.Client) gen.Client {
	agents := make([]gen.Ref, len(c.Agents))
	for i, a := range c.Agents {
		agents[i] = gen.Ref{Id: a.ID, Name: a.Name}
	}
	return gen.Client{
		Id: c.ID, Name: c.Name, ContractNo: c.ContractNo, ContractStart: apiDatePtr(c.ContractStart),
		ContractEnd: apiDatePtr(c.ContractEnd), PicName: c.PicName, PicEmail: c.PicEmail,
		SlaPlan: gen.Ref{Id: c.SlaPlan.ID, Name: c.SlaPlan.Name}, RequesterCount: c.RequesterCount,
		Agents: agents, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func (s *server) ListClientRequesters(ctx context.Context, req gen.ListClientRequestersRequestObject) (gen.ListClientRequestersResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	list, err := s.clients.Requesters(ctx, p, req.ClientId)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListClientRequesters200JSONResponse, len(list))
	for i, r := range list {
		out[i] = apiRequester(r)
	}
	return out, nil
}

// CreateClientRequester adds an invited requester to a client.
func (s *server) CreateClientRequester(ctx context.Context, req gen.CreateClientRequesterRequestObject) (gen.CreateClientRequesterResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	r, err := s.clients.CreateRequester(ctx, p, req.ClientId, clients.RequesterInput{
		Name: req.Body.Name, Email: req.Body.Email, JobTitle: req.Body.JobTitle,
	})
	if err != nil {
		return nil, err
	}
	return gen.CreateClientRequester201JSONResponse(apiRequester(r)), nil
}

func (s *server) ListClientAssignees(ctx context.Context, req gen.ListClientAssigneesRequestObject) (gen.ListClientAssigneesResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	list, err := s.clients.Assignees(ctx, p, req.ClientId)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListClientAssignees200JSONResponse, len(list))
	for i, a := range list {
		out[i] = gen.Assignee{Id: a.ID, Name: a.Name, Role: gen.Role(a.Role), JobTitle: a.JobTitle, OpenTicketCount: a.OpenTickets}
	}
	return out, nil
}

func apiRequester(r clients.Requester) gen.Requester {
	return gen.Requester{Id: r.ID, Name: r.Name, Email: r.Email, JobTitle: r.JobTitle, Status: gen.UserStatus(r.Status)}
}
