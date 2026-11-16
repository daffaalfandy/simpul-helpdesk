package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/connector"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

func (s *server) GetIntegrations(ctx context.Context, _ gen.GetIntegrationsRequestObject) (gen.GetIntegrationsResponseObject, error) {
	if _, err := auth.Require(ctx, auth.CapSettingsManage); err != nil {
		return nil, err
	}
	o, err := s.connector.Overview(ctx)
	if err != nil {
		return nil, err
	}
	repos := make([]gen.Repository, len(o.Repositories))
	for i, r := range o.Repositories {
		repos[i] = apiRepository(r)
	}
	return gen.GetIntegrations200JSONResponse{Repositories: repos, TicketPrefix: o.TicketPrefix}, nil
}

func (s *server) CreateRepository(ctx context.Context, req gen.CreateRepositoryRequestObject) (gen.CreateRepositoryResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	r, secret, err := s.connector.Create(ctx, p.AuditActor(), p.UserID, connector.CreateInput{
		Provider: gitevent.Provider(req.Body.Provider), URL: req.Body.Url, ProjectIDs: req.Body.ProjectIds,
	})
	if err != nil {
		return nil, err
	}
	return gen.CreateRepository201JSONResponse{Repository: apiRepository(r), Secret: secret}, nil
}

func (s *server) UpdateRepository(ctx context.Context, req gen.UpdateRepositoryRequestObject) (gen.UpdateRepositoryResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	r, err := s.connector.SetProjects(ctx, p.AuditActor(), req.RepositoryId, req.Body.ProjectIds)
	if err != nil {
		return nil, err
	}
	return gen.UpdateRepository200JSONResponse(apiRepository(r)), nil
}

func (s *server) RotateRepositorySecret(ctx context.Context, req gen.RotateRepositorySecretRequestObject) (gen.RotateRepositorySecretResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	r, secret, err := s.connector.RotateSecret(ctx, p.AuditActor(), req.RepositoryId)
	if err != nil {
		return nil, err
	}
	return gen.RotateRepositorySecret200JSONResponse{Repository: apiRepository(r), Secret: secret}, nil
}

func (s *server) DeleteRepository(ctx context.Context, req gen.DeleteRepositoryRequestObject) (gen.DeleteRepositoryResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	if err := s.connector.Remove(ctx, p.AuditActor(), req.RepositoryId); err != nil {
		return nil, err
	}
	return gen.DeleteRepository204Response{}, nil
}

func apiRepository(r connector.Repository) gen.Repository {
	projects := make([]gen.LinkedProject, len(r.Projects))
	for i, p := range r.Projects {
		projects[i] = gen.LinkedProject{Id: p.ID, Name: p.Name, Client: gen.Ref{Id: p.Client.ID, Name: p.Client.Name}}
	}
	var lastError *gen.RepositoryError
	if r.LastError != nil {
		e := gen.RepositoryError(*r.LastError)
		lastError = &e
	}
	return gen.Repository{
		Id: r.ID, Provider: gen.GitProvider(r.Provider), Host: r.Location.Host, FullName: r.Location.FullName,
		WebUrl: r.Location.WebURL, WebhookUrl: r.WebhookURL, Status: gen.RepositoryStatus(r.Status),
		LastDeliveryAt: r.LastDeliveryAt, LastError: lastError, LastErrorAt: r.LastErrorAt, Projects: projects,
	}
}
