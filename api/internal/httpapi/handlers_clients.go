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

// CreateClient adds a client.
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

// GetClient returns one client the user may see.
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

// UpdateClient replaces a client's details.
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

// DeleteClient removes a client without requesters.
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

// SetClientAgents replaces a client's assigned agents.
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
