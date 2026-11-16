package httpapi_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/connector"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

// hook is a connected repository the tests send deliveries for.
type hook struct {
	e      *env
	repo   gen.Repository
	secret string
}

func (e *env) hook(t *testing.T, p gen.GitProvider, url string, projects ...uuid.UUID) hook {
	t.Helper()
	r := e.connectRepo(t, p, url, projects...)
	return hook{e: e, repo: r.Repository, secret: r.Secret}
}

// githubPush is a minimal push to the e-Office repository.
func githubPush(branch string, commits ...string) string {
	var parts []string
	for i, msg := range commits {
		parts = append(parts, `{"id":"`+strings.Repeat(string(rune('a'+i)), 40)+`","message":`+quote(msg)+
			`,"timestamp":"2026-11-16T08:40:00+07:00","url":"https://github.com/vendor-id/eoffice-jambi/commit/x",`+
			`"author":{"name":"User agent","email":"agent@example.test","username":"agent"}}`)
	}
	return `{"ref":"refs/heads/` + branch + `","commits":[` + strings.Join(parts, ",") + `],` +
		`"repository":{"id":7001,"full_name":"vendor-id/eoffice-jambi","html_url":"https://github.com/vendor-id/eoffice-jambi"}}`
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// github sends a GitHub delivery signed with secret.
func (h hook) github(event, delivery, body, secret string) *httptest.ResponseRecorder {
	h.e.t.Helper()
	header := map[string]string{gitevent.HeaderGitHubEvent: event, gitevent.HeaderGitHubDelivery: delivery}
	if secret != "" {
		header[gitevent.HeaderGitHubSignature] = sign(secret, body)
	}
	return h.post(body, header)
}

func (h hook) gitlab(event, uuid, body, token string) *httptest.ResponseRecorder {
	h.e.t.Helper()
	return h.post(body, map[string]string{gitevent.HeaderGitLabEvent: event, gitevent.HeaderGitLabEventUUID: uuid, gitevent.HeaderGitLabToken: token})
}

func (h hook) post(body string, header map[string]string) *httptest.ResponseRecorder {
	h.e.t.Helper()
	req := httptest.NewRequest(http.MethodPost, apiBase+"/webhooks/"+h.repo.Id.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.e.handler.ServeHTTP(rec, req)
	return rec
}

func (h hook) status(t *testing.T) gen.Repository {
	t.Helper()
	for _, r := range decode[gen.Integrations](t, h.e.as(auth.RoleAdmin).do(http.MethodGet, "/integrations", nil), http.StatusOK).Repositories {
		if r.Id == h.repo.Id {
			return r
		}
	}
	t.Fatal("repository not listed")
	return gen.Repository{}
}

func (e *env) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.st.Pool().QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestWebhookIntake(t *testing.T) {
	e := newEnv(t)
	h := e.hook(t, gen.GitProviderGithub, eOfficeURL)
	ping := `{"zen":"z","hook_id":1,"repository":{"id":7001,"full_name":"vendor-id/eoffice-jambi"}}`

	// Wrong or missing secrets are refused and shown on the Integrations page.
	expectProblem(t, h.github("ping", "d-0", ping, "wrong"), http.StatusUnauthorized, apperr.CodeInvalidSignature)
	expectProblem(t, h.github("ping", "d-0", ping, ""), http.StatusUnauthorized, apperr.CodeInvalidSignature)
	if r := h.status(t); r.Status != gen.RepositoryStatusFailing || r.LastError == nil || *r.LastError != gen.RepositoryErrorBadSignature {
		t.Fatalf("after a bad secret = %+v", r)
	}

	// A ping connects the repository and teaches its id.
	if rec := h.github("ping", "d-1", ping, h.secret); rec.Code != http.StatusOK {
		t.Fatalf("ping: %d %s", rec.Code, rec.Body)
	}
	if r := h.status(t); r.Status != gen.RepositoryStatusConnected || r.LastDeliveryAt == nil {
		t.Fatalf("after a ping = %+v", r)
	}
	if n := e.count(t, `SELECT count(*) FROM repositories WHERE external_id = '7001'`); n != 1 {
		t.Fatal("external id not learnt")
	}

	// A push is stored once and processed in the background.
	push := githubPush("fix/HD-1", "refs HD-1 add logging", "update readme")
	if rec := h.github("push", "d-2", push, h.secret); rec.Code != http.StatusAccepted {
		t.Fatalf("push: %d %s", rec.Code, rec.Body)
	}
	if rec := h.github("push", "d-2", push, h.secret); rec.Code != http.StatusOK {
		t.Fatalf("redelivery: %d %s", rec.Code, rec.Body)
	}
	if n := runJobs(t, e, e.process); n != 1 {
		t.Fatalf("process jobs = %d", n)
	}
	if n := e.count(t, `SELECT count(*) FROM code_events WHERE kind = 'commit'`); n != 1 {
		t.Fatalf("commits stored = %d, want only the one naming a ticket", n)
	}
	if n := e.count(t, `SELECT count(*) FROM webhook_deliveries WHERE processed_at IS NOT NULL`); n != 1 {
		t.Fatalf("processed deliveries = %d", n)
	}

	// GitHub's form encoding works too; the signature covers the body as sent.
	form := "payload=" + url.QueryEscape(githubPush("main", "refs HD-2"))
	req := map[string]string{gitevent.HeaderGitHubEvent: "push", gitevent.HeaderGitHubDelivery: "d-3", gitevent.HeaderGitHubSignature: sign(h.secret, form)}
	if rec := h.post(form, req); rec.Code != http.StatusAccepted {
		t.Fatalf("form: %d %s", rec.Code, rec.Body)
	}

	// Events the connector doesn't read are acknowledged and dropped.
	if rec := h.github("issues", "d-4", `{"repository":{"full_name":"x/y"}}`, h.secret); rec.Code != http.StatusOK {
		t.Fatalf("issues: %d %s", rec.Code, rec.Body)
	}
	if n := e.count(t, `SELECT count(*) FROM webhook_deliveries`); n != 2 {
		t.Fatalf("deliveries = %d", n)
	}

	// A webhook added to another repository is refused.
	other := strings.ReplaceAll(push, `"id":7001`, `"id":9999`)
	expectProblem(t, h.github("push", "d-5", other, h.secret), http.StatusConflict, apperr.CodeRepositoryMismatch)
	expectProblem(t, h.github("push", "d-6", `{"ref":`, h.secret), http.StatusBadRequest, apperr.CodeInvalidRequest)

	// Too large: refused and recorded.
	big := `{"pad":"` + strings.Repeat("x", connector.MaxDeliveryBytes) + `"}`
	expectProblem(t, h.github("push", "d-7", big, h.secret), http.StatusRequestEntityTooLarge, apperr.CodeRequestTooLarge)
	if r := h.status(t); r.LastError == nil || *r.LastError != gen.RepositoryErrorTooLarge {
		t.Fatalf("after a large body = %+v", r)
	}

	// Unknown and removed repositories are not found.
	missing := hook{e: e, repo: gen.Repository{Id: uuid.New()}, secret: h.secret}
	expectProblem(t, missing.github("ping", "d-8", ping, h.secret), http.StatusNotFound, apperr.CodeNotFound)
	if rec := e.as(auth.RoleAdmin).do(http.MethodDelete, repoPath(h.repo), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d", rec.Code)
	}
	expectProblem(t, h.github("ping", "d-9", ping, h.secret), http.StatusNotFound, apperr.CodeNotFound)
}

func TestWebhookGitLab(t *testing.T) {
	e := newEnv(t)
	h := e.hook(t, gen.GitProviderGitlab, "https://gitlab.vendor.co.id/gov/sipd-connector")
	body := `{"ref":"refs/heads/main","after":"b","commits":[{"id":"` + strings.Repeat("c", 40) +
		`","message":"Closes HD-3","timestamp":"2026-11-16T08:20:00+07:00","author":{"name":"A","email":"a@example.test"}}],` +
		`"project":{"id":15,"path_with_namespace":"gov/sipd-connector","web_url":"https://gitlab.vendor.co.id/gov/sipd-connector"}}`
	expectProblem(t, h.gitlab("Push Hook", "u-1", body, "nope"), http.StatusUnauthorized, apperr.CodeInvalidSignature)
	if rec := h.gitlab("Push Hook", "u-1", body, h.secret); rec.Code != http.StatusAccepted {
		t.Fatalf("push: %d %s", rec.Code, rec.Body)
	}
	if rec := h.gitlab("Push Hook", "u-1", body, h.secret); rec.Code != http.StatusOK {
		t.Fatalf("redelivery: %d %s", rec.Code, rec.Body)
	}
	if rec := h.gitlab("Tag Push Hook", "u-2", body, h.secret); rec.Code != http.StatusOK {
		t.Fatalf("tag push: %d %s", rec.Code, rec.Body)
	}
	if r := h.status(t); r.Status != gen.RepositoryStatusConnected {
		t.Fatalf("status = %+v", r)
	}
}

func TestWebhookPurge(t *testing.T) {
	e := newEnv(t)
	h := e.hook(t, gen.GitProviderGithub, eOfficeURL)
	if rec := h.github("push", "old", githubPush("main", "refs HD-1"), h.secret); rec.Code != http.StatusAccepted {
		t.Fatalf("push: %d %s", rec.Code, rec.Body)
	}
	runJobs(t, e, e.process)
	ctx := context.Background()
	if _, err := e.st.Pool().Exec(ctx, `UPDATE webhook_deliveries SET received_at = now() - interval '31 days'`); err != nil {
		t.Fatal(err)
	}
	if err := connector.NewPurgeWorker(e.st).Work(ctx, &river.Job[connector.PurgeArgs]{}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(t, `SELECT count(*) FROM webhook_deliveries`); n != 0 {
		t.Fatalf("deliveries left = %d", n)
	}
	if n := e.count(t, `SELECT count(*) FROM code_events WHERE delivery_id IS NULL`); n != 1 {
		t.Fatalf("code events kept without their delivery = %d", n)
	}
	if n := e.count(t, `SELECT count(*) FROM audit_log WHERE action = $1`, connector.ActionDeliveriesPurge); n != 1 {
		t.Fatalf("purge audits = %d", n)
	}
}

// The webhook URL shown to admins is the spec's webhook path under the App URL.
func TestWebhookPathMatchesSpec(t *testing.T) {
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	if spec.Paths.Find("/webhooks/{repositoryId}") == nil || apiBase+"/webhooks/" != connector.WebhookPath {
		t.Fatalf("connector.WebhookPath %q doesn't match the spec", connector.WebhookPath)
	}
}
