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

func (e *env) install(t *testing.T) gen.Install {
	t.Helper()
	return decode[gen.Install](t, e.do(request{method: http.MethodGet, path: apiBase + "/install"}), http.StatusOK)
}

func TestFirstRunSetup(t *testing.T) {
	e := newEnv(t)
	// newEnv makes its admin with the CLI path, which closes setup like the form does.
	if e.install(t).SetupRequired {
		t.Fatal("setup still open after the CLI created an administrator")
	}
	in := gen.SetupRequest{Name: "Dewi Wulandari", Email: "Dewi@Vendor.test", Password: invitePassword}
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
	if !e.install(t).SetupRequired {
		t.Fatal("setup not open on a fresh install")
	}
	for _, bad := range []struct {
		body gen.SetupRequest
		code string
	}{
		{gen.SetupRequest{Name: "  ", Email: in.Email, Password: in.Password}, apperr.CodeInvalidName},
		{gen.SetupRequest{Name: in.Name, Email: "nope", Password: in.Password}, apperr.CodeInvalidEmail},
		{gen.SetupRequest{Name: in.Name, Email: e.users[auth.RoleAgent], Password: in.Password}, apperr.CodeEmailTaken},
	} {
		if r := setup(bad.body); !strings.Contains(r.body, bad.code) {
			t.Errorf("want %s: %d %s", bad.code, r.code, r.body)
		}
	}
	if !e.install(t).SetupRequired {
		t.Fatal("a refused form closed setup")
	}

	organizations := e.count(t, `SELECT count(*) FROM organizations`)
	r := setup(in)
	if r.code != http.StatusOK || !strings.Contains(r.cookie, "sd_session") || !strings.Contains(r.body, `"role":"admin"`) {
		t.Fatalf("setup: %d %s", r.code, r.body)
	}
	if e.install(t).SetupRequired {
		t.Fatal("setup still open after it ran")
	}
	// Setup makes the administrator only; they create the first organization next.
	if n := e.count(t, `SELECT count(*) FROM organizations`); n != organizations {
		t.Fatalf("organizations after setup = %d, want %d", n, organizations)
	}
	admin := session{e: e, cookie: sessionCookie(t, e.login("dewi@vendor.test", invitePassword))}
	created := decode[gen.OrganizationSummary](t, admin.do(http.MethodPost, "/organizations",
		gen.OrganizationInput{Name: "PT Vendor Teknologi", TicketPrefix: "VT", Timezone: "Asia/Makassar"}), http.StatusCreated)
	if me := admin.in(created.Id).me(t); !me.IsAdmin || orgOf(me) != created.Id {
		t.Fatalf("new admin in the organization they made = %+v", me)
	}
	if r := setup(gen.SetupRequest{Name: "Mallory", Email: "mallory@example.test", Password: invitePassword}); r.code != http.StatusConflict {
		t.Fatalf("second setup: %d %s", r.code, r.body)
	}
	if n := e.count(t, `SELECT count(*) FROM audit_log WHERE action = $1 AND actor_name = 'setup'`, auth.ActionSetupCompleted); n != 1 {
		t.Errorf("setup audit entries = %d", n)
	}
}
