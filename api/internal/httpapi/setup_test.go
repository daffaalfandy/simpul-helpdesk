package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

func (e *env) organization(t *testing.T) gen.Organization {
	t.Helper()
	return decode[gen.Organization](t, e.do(request{method: http.MethodGet, path: apiBase + "/organization"}), http.StatusOK)
}

func TestFirstRunSetup(t *testing.T) {
	e := newEnv(t)
	// newEnv makes its admin with the CLI path, which closes setup like the form does.
	if e.organization(t).SetupRequired {
		t.Fatal("setup still open after the CLI created an administrator")
	}
	in := gen.SetupRequest{OrganizationName: " PT Vendor Teknologi ", Name: "Dewi Wulandari", Email: "Dewi@Vendor.test", Password: invitePassword}
	setup := func(body gen.SetupRequest) *httpResponse {
		rec := e.do(request{method: http.MethodPost, path: apiBase + "/setup", body: body})
		return &httpResponse{code: rec.Code, body: rec.Body.String(), cookie: rec.Header().Get("Set-Cookie")}
	}
	if r := setup(in); r.code != http.StatusConflict || !strings.Contains(r.body, apperr.CodeSetupClosed) {
		t.Fatalf("closed setup: %d %s", r.code, r.body)
	}

	// A fresh install: setup is open.
	if _, err := e.st.Pool().Exec(context.Background(), `UPDATE install SET setup_completed_at = NULL`); err != nil {
		t.Fatal(err)
	}
	if !e.organization(t).SetupRequired {
		t.Fatal("setup not open on a fresh install")
	}
	for _, bad := range []struct {
		body gen.SetupRequest
		code string
	}{
		{gen.SetupRequest{OrganizationName: "  ", Name: in.Name, Email: in.Email, Password: in.Password}, apperr.CodeInvalidOrganization},
		{gen.SetupRequest{OrganizationName: in.OrganizationName, Name: in.Name, Email: "nope", Password: in.Password}, apperr.CodeInvalidEmail},
		{gen.SetupRequest{OrganizationName: in.OrganizationName, Name: in.Name, Email: e.users[auth.RoleAgent], Password: in.Password}, apperr.CodeEmailTaken},
	} {
		if r := setup(bad.body); !strings.Contains(r.body, bad.code) {
			t.Errorf("want %s: %d %s", bad.code, r.code, r.body)
		}
	}
	if !e.organization(t).SetupRequired {
		t.Fatal("a refused form closed setup")
	}

	r := setup(in)
	if r.code != http.StatusOK || !strings.Contains(r.cookie, "sd_session") || !strings.Contains(r.body, `"role":"admin"`) {
		t.Fatalf("setup: %d %s", r.code, r.body)
	}
	org := e.organization(t)
	if org.SetupRequired || org.Name != "PT Vendor Teknologi" {
		t.Fatalf("organisation after setup = %+v", org)
	}
	if rec := e.login("dewi@vendor.test", invitePassword); rec.Code != http.StatusOK {
		t.Fatalf("new admin signs in: %d", rec.Code)
	}
	if r := setup(gen.SetupRequest{OrganizationName: "Again", Name: "Mallory", Email: "mallory@example.test", Password: invitePassword}); r.code != http.StatusConflict {
		t.Fatalf("second setup: %d %s", r.code, r.body)
	}
	if n := e.count(t, `SELECT count(*) FROM audit_log WHERE action = $1 AND actor_name = 'setup'`, auth.ActionSetupCompleted); n != 1 {
		t.Errorf("setup audit entries = %d", n)
	}
}
