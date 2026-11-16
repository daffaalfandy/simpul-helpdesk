package httpapi

import (
	"context"
	"io"
	"net/http"

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

// ReceiveWebhook takes a delivery from GitHub or GitLab. It is public: the repository's secret
// authenticates it, so the body is read exactly as sent (the spec's x-raw-body).
func (s *server) ReceiveWebhook(ctx context.Context, req gen.ReceiveWebhookRequestObject) (gen.ReceiveWebhookResponseObject, error) {
	body, err := io.ReadAll(req.Body)
	if tooLargeError(err) {
		return nil, s.connector.RefuseTooLarge(ctx, req.RepositoryId)
	}
	if err != nil {
		return nil, err
	}
	receipt, err := s.connector.Receive(ctx, req.RepositoryId, webhookHeader(req.Params), body)
	if err != nil {
		return nil, err
	}
	if receipt == connector.ReceiptQueued {
		return gen.ReceiveWebhook202Response{}, nil
	}
	return gen.ReceiveWebhook200Response{}, nil
}

// webhookHeader puts back the headers the providers send, as the spec declares them.
func webhookHeader(p gen.ReceiveWebhookParams) http.Header {
	h := http.Header{}
	for name, v := range map[string]*string{
		gitevent.HeaderGitHubEvent: p.XGitHubEvent, gitevent.HeaderGitHubDelivery: p.XGitHubDelivery,
		gitevent.HeaderGitHubSignature: p.XHubSignature256, gitevent.HeaderGitLabEvent: p.XGitlabEvent,
		gitevent.HeaderGitLabToken: p.XGitlabToken, gitevent.HeaderGitLabEventUUID: p.XGitlabEventUUID,
		gitevent.HeaderIdempotencyKey: p.IdempotencyKey,
	} {
		if v != nil {
			h.Set(name, *v)
		}
	}
	return h
}

func (s *server) TestCommitMessage(ctx context.Context, req gen.TestCommitMessageRequestObject) (gen.TestCommitMessageResponseObject, error) {
	if _, err := auth.Require(ctx, auth.CapSettingsManage); err != nil {
		return nil, err
	}
	refs, err := s.connector.TestMessage(ctx, req.Body.Message)
	if err != nil {
		return nil, err
	}
	out := gen.TestCommitMessage200JSONResponse{Refs: make([]gen.CommitTestRef, len(refs))}
	for i, r := range refs {
		out.Refs[i] = gen.CommitTestRef{Key: r.Key, Action: gen.CommitRefAction(r.Action), TicketExists: r.TicketExists}
	}
	return out, nil
}

func (s *server) ListCodeEvents(ctx context.Context, _ gen.ListCodeEventsRequestObject) (gen.ListCodeEventsResponseObject, error) {
	if _, err := auth.Require(ctx, auth.CapSettingsManage); err != nil {
		return nil, err
	}
	log, err := s.connector.Log(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListCodeEvents200JSONResponse, len(log))
	for i, l := range log {
		items := make([]gen.DodItem, len(l.Items))
		for j, it := range l.Items {
			items[j] = gen.DodItem(it)
		}
		out[i] = gen.CodeEventLog{
			Id: l.ID, At: l.At, Provider: gen.GitProvider(l.Provider), Repository: l.Repository, Kind: gen.CodeEventKind(l.Kind),
			Ref: l.Ref, Title: l.Title, Branch: l.Branch, Target: l.Target, Url: l.URL, TicketKey: l.TicketKey,
			TicketFound: l.TicketFound, Outcome: gen.CodeOutcome(l.Outcome), Items: items,
			StatusFrom: (*gen.TicketStatus)(l.StatusFrom), StatusTo: (*gen.TicketStatus)(l.StatusTo),
		}
	}
	return out, nil
}
