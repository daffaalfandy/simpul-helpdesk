package httpapi_test

import (
	"bytes"
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/connector"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

const eOfficeURL = "https://github.com/vendor-id/eoffice-jambi"

func repoInput(p gen.GitProvider, url string, projects ...uuid.UUID) gen.RepositoryCreate {
	if projects == nil {
		projects = []uuid.UUID{}
	}
	return gen.RepositoryCreate{Provider: p, Url: url, ProjectIds: projects}
}

func repoPath(r gen.Repository) string { return "/repositories/" + r.Id.String() }

// connectRepo connects a repository as admin and returns it with its webhook secret.
func (e *env) connectRepo(t *testing.T, p gen.GitProvider, url string, projects ...uuid.UUID) gen.RepositoryWithSecret {
	t.Helper()
	return decode[gen.RepositoryWithSecret](t, e.as(auth.RoleAdmin).do(http.MethodPost, "/repositories", repoInput(p, url, projects...)), http.StatusCreated)
}

func TestRepositories(t *testing.T) {
	e := newEnv(t)
	admin := e.as(auth.RoleAdmin)
	sup := e.as(auth.RoleSupervisor)
	client := newClient(t, e, "Dinas Kominfo Jambi")
	eOffice := decode[gen.Project](t, sup.do(http.MethodPost, "/projects", projectInput(client, "e-Office")), http.StatusCreated)
	website := decode[gen.Project](t, sup.do(http.MethodPost, "/projects", projectInput(client, "Public website")), http.StatusCreated)

	created := e.connectRepo(t, gen.GitProviderGithub, eOfficeURL+".git/", eOffice.Id)
	r := created.Repository
	if r.FullName != "vendor-id/eoffice-jambi" || r.Host != "github.com" || r.WebUrl != eOfficeURL ||
		r.Status != gen.RepositoryStatusWaiting || r.LastDeliveryAt != nil || r.LastError != nil {
		t.Fatalf("created = %+v", r)
	}
	if r.WebhookUrl != appURL+connector.WebhookPath+r.Id.String() {
		t.Errorf("webhook URL = %q", r.WebhookUrl)
	}
	if len(created.Secret) < 20 {
		t.Errorf("secret %q too short", created.Secret)
	}
	if len(r.Projects) != 1 || r.Projects[0].Id != eOffice.Id || r.Projects[0].Client.Id != client {
		t.Fatalf("projects = %+v", r.Projects)
	}

	// The secret is stored sealed.
	var sealed []byte
	if err := e.st.Pool().QueryRow(context.Background(), `SELECT webhook_secret FROM repositories WHERE id = $1`, r.Id).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte(created.Secret)) {
		t.Fatal("webhook secret stored in plain text")
	}

	// The project now reads commits from it.
	p := decode[gen.Project](t, sup.do(http.MethodGet, projectPath(eOffice), nil), http.StatusOK)
	if p.Repository == nil || p.Repository.Id != r.Id || p.Repository.Provider != gen.GitProviderGithub {
		t.Fatalf("project repository = %+v", p.Repository)
	}

	ov := decode[gen.Integrations](t, admin.do(http.MethodGet, "/integrations", nil), http.StatusOK)
	if ov.TicketPrefix != "HD" || len(ov.Repositories) != 1 || ov.Repositories[0].Id != r.Id {
		t.Fatalf("overview = %+v", ov)
	}

	// Adding the same repository again is refused, whatever the spelling.
	expectProblem(t, admin.do(http.MethodPost, "/repositories", repoInput(gen.GitProviderGithub, "https://GITHUB.com/Vendor-ID/eoffice-jambi")), http.StatusConflict, apperr.CodeRepositoryTaken)
	expectProblem(t, admin.do(http.MethodPost, "/repositories", repoInput(gen.GitProviderGithub, "https://github.com/vendor-id")), http.StatusBadRequest, apperr.CodeInvalidRepositoryURL)
	expectProblem(t, admin.do(http.MethodPost, "/repositories", repoInput(gen.GitProviderGitlab, "https://gitlab.com/g/p", uuid.New())), http.StatusBadRequest, apperr.CodeInvalidProject)

	both := decode[gen.Repository](t, admin.do(http.MethodPatch, repoPath(r), gen.RepositoryUpdate{ProjectIds: []uuid.UUID{website.Id, eOffice.Id, website.Id}}), http.StatusOK)
	if len(both.Projects) != 2 {
		t.Fatalf("linked = %+v", both.Projects)
	}

	// A second repository takes a project over: a project reads from one repository.
	gl := e.connectRepo(t, gen.GitProviderGitlab, "https://gitlab.vendor.co.id/gov/sipd-connector/-/tree/main", website.Id).Repository
	if gl.FullName != "gov/sipd-connector" || gl.Host != "gitlab.vendor.co.id" {
		t.Fatalf("GitLab = %+v", gl)
	}
	ov = decode[gen.Integrations](t, admin.do(http.MethodGet, "/integrations", nil), http.StatusOK)
	if len(ov.Repositories) != 2 || len(ov.Repositories[0].Projects) != 1 || ov.Repositories[0].Projects[0].Id != eOffice.Id {
		t.Fatalf("after the move = %+v", ov.Repositories)
	}

	// A new secret replaces the old one.
	rotated := decode[gen.RepositoryWithSecret](t, admin.do(http.MethodPost, repoPath(r)+"/secret", nil), http.StatusOK)
	if rotated.Secret == created.Secret || rotated.Repository.Id != r.Id {
		t.Fatalf("rotated = %+v", rotated)
	}

	// A repository without code events is deleted; adding it again works.
	if rec := admin.do(http.MethodDelete, repoPath(gl), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	expectProblem(t, admin.do(http.MethodPatch, repoPath(gl), gen.RepositoryUpdate{ProjectIds: []uuid.UUID{}}), http.StatusNotFound, apperr.CodeNotFound)
	if p := decode[gen.Project](t, sup.do(http.MethodGet, projectPath(website), nil), http.StatusOK); p.Repository != nil {
		t.Fatal("website still has the deleted repository")
	}
	e.connectRepo(t, gen.GitProviderGitlab, "https://gitlab.vendor.co.id/gov/sipd-connector")

	actions := e.auditActionsLike(t, "repository.%")
	for _, want := range []string{connector.ActionRepositoryCreated, connector.ActionRepositoryUpdated,
		connector.ActionRepositorySecretRotated, connector.ActionRepositoryRemoved} {
		if !slices.Contains(actions, want) {
			t.Errorf("audit lacks %s: %v", want, actions)
		}
	}
}

func TestRepositoriesAreAdminOnly(t *testing.T) {
	e := newEnv(t)
	r := e.connectRepo(t, gen.GitProviderGithub, eOfficeURL).Repository
	for _, role := range []auth.Role{auth.RoleSupervisor, auth.RoleAgent, auth.RoleRequester} {
		s := e.as(role)
		for _, c := range []struct{ method, path string }{
			{http.MethodGet, "/integrations"},
			{http.MethodPost, "/repositories"},
			{http.MethodPatch, repoPath(r)},
			{http.MethodDelete, repoPath(r)},
			{http.MethodPost, repoPath(r) + "/secret"},
		} {
			var body any
			switch {
			case c.method == http.MethodPost && !strings.HasSuffix(c.path, "/secret"):
				body = repoInput(gen.GitProviderGithub, "https://github.com/a/b")
			case c.method == http.MethodPatch:
				body = gen.RepositoryUpdate{ProjectIds: []uuid.UUID{}}
			}
			expectProblem(t, s.do(c.method, c.path, body), http.StatusForbidden, apperr.CodeForbidden)
		}
	}
}
