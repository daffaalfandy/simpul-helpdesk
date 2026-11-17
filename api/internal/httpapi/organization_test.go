package httpapi_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/organization"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// uploadLogo sends one file as the logo.
func (s session) uploadLogo(name string, content []byte) *httptest.ResponseRecorder {
	s.e.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", name)
	if err != nil {
		s.e.t.Fatal(err)
	}
	_, _ = fw.Write(content)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPut, apiBase+"/organization/logo", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(s.cookie)
	rec := httptest.NewRecorder()
	s.e.handler.ServeHTTP(rec, req)
	return rec
}

func TestOrganizationSettings(t *testing.T) {
	e := newEnv(t)
	admin := e.as(auth.RoleAdmin)

	org := decode[gen.Organization](t, e.do(request{method: http.MethodGet, path: apiBase + "/organization"}), http.StatusOK)
	if org.TicketPrefix != "HD" || !strings.HasPrefix(org.AppUrl, "http") || org.LogoUrl != nil {
		t.Fatalf("organisation = %+v", org)
	}
	if apiBase != organization.APIBasePath {
		t.Fatalf("emails would link the logo under %s, the API is at %s", organization.APIBasePath, apiBase)
	}

	in := gen.OrganizationInput{Name: "  PT Vendor Teknologi ", TicketPrefix: "ops", Timezone: "Asia/Makassar"}
	expectProblem(t, e.as(auth.RoleSupervisor).do(http.MethodPut, "/organization", in), http.StatusForbidden, apperr.CodeForbidden)
	got := decode[gen.Organization](t, admin.do(http.MethodPut, "/organization", in), http.StatusOK)
	if got.Name != "PT Vendor Teknologi" || got.TicketPrefix != "OPS" || got.Timezone != "Asia/Makassar" {
		t.Fatalf("saved = %+v", got)
	}
	if n := e.countRows(t, `SELECT count(*) FROM audit_log WHERE action = $1`, organization.ActionUpdated); n != 1 {
		t.Errorf("audit entries = %d", n)
	}
	// A new time zone recomputes open tickets' SLA clocks.
	if n := e.countRows(t, `SELECT count(*) FROM river_job WHERE kind = $1`, sla.JobKindRecompute); n != 1 {
		t.Errorf("recompute jobs = %d", n)
	}
	// Saving the same values again changes nothing.
	decode[gen.Organization](t, admin.do(http.MethodPut, "/organization", in), http.StatusOK)
	if n := e.countRows(t, `SELECT count(*) FROM audit_log WHERE action = $1`, organization.ActionUpdated); n != 1 {
		t.Errorf("audit entries after a no-op = %d", n)
	}

	for name, bad := range map[string]gen.OrganizationInput{
		"unknown zone": {Name: "X", TicketPrefix: "HD", Timezone: "Mars/Olympus"},
		"server zone":  {Name: "X", TicketPrefix: "HD", Timezone: "Local"},
		"blank name":   {Name: "   ", TicketPrefix: "HD", Timezone: "Asia/Jakarta"},
	} {
		rec := admin.do(http.MethodPut, "/organization", bad)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), apperr.CodeInvalidOrganization) {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
}

func TestOrganizationLogo(t *testing.T) {
	e := newEnv(t)
	admin := e.as(auth.RoleAdmin)

	expectProblem(t, e.as(auth.RoleAgent).uploadLogo("logo.png", pngBytes), http.StatusForbidden, apperr.CodeForbidden)
	expectProblem(t, admin.uploadLogo("logo.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)), http.StatusBadRequest, apperr.CodeLogoTypeNotAllowed)
	big := append(slices.Clone(pngBytes), bytes.Repeat([]byte{0}, organization.MaxLogoBytes)...)
	expectProblem(t, admin.uploadLogo("big.png", big), http.StatusRequestEntityTooLarge, apperr.CodeLogoTooLarge)

	got := decode[gen.Organization](t, admin.uploadLogo("logo.png", pngBytes), http.StatusOK)
	if got.LogoUrl == nil || !strings.HasPrefix(*got.LogoUrl, apiBase+"/organization/logo?v=") {
		t.Fatalf("logoUrl = %v", got.LogoUrl)
	}
	// Public, inline, cached for good under its versioned URL.
	rec := e.do(request{method: http.MethodGet, path: *got.LogoUrl})
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != organization.LogoPNG ||
		!strings.Contains(rec.Header().Get("Cache-Control"), "immutable") || !bytes.Equal(rec.Body.Bytes(), pngBytes) {
		t.Fatalf("logo: %d %v", rec.Code, rec.Header())
	}
	first := e.logoKey(t)

	// A new logo replaces the file.
	decode[gen.Organization](t, admin.uploadLogo("logo2.png", pngBytes), http.StatusOK)
	if _, _, err := e.files.Open(t.Context(), first); err == nil {
		t.Error("the old logo file is still stored")
	}

	removed := decode[gen.Organization](t, admin.do(http.MethodDelete, "/organization/logo", nil), http.StatusOK)
	if removed.LogoUrl != nil {
		t.Fatalf("logoUrl after removing = %v", removed.LogoUrl)
	}
	expectProblem(t, e.do(request{method: http.MethodGet, path: apiBase + "/organization/logo"}), http.StatusNotFound, apperr.CodeNotFound)
	for _, action := range []string{organization.ActionLogoUpdated, organization.ActionLogoRemoved} {
		if n := e.countRows(t, `SELECT count(*) FROM audit_log WHERE action = $1`, action); n == 0 {
			t.Errorf("no %s audit entry", action)
		}
	}
}

func (e *env) logoKey(t *testing.T) string {
	t.Helper()
	var key string
	if err := e.st.Pool().QueryRow(t.Context(), `SELECT logo_key FROM organization`).Scan(&key); err != nil {
		t.Fatal(err)
	}
	return key
}

func (e *env) countRows(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.st.Pool().QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
