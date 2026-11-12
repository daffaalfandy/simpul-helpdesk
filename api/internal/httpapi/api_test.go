package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertest"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/config"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/mail"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

const (
	password  = "correct-horse-battery"
	appURL    = "http://localhost:5173"
	spaMarker = "<!doctype html><title>spa</title>"
	apiBase   = "/api/v1"
)

var cheapParams = auth.Params{Memory: 64, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}

type env struct {
	t       *testing.T
	st      *store.Store
	handler http.Handler
	auth    *auth.Service
	mailer  *mail.Recorder
	worker  *auth.ResetEmailWorker
	users   map[auth.Role]string // role -> email
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	st := testdb.New(t)
	u, _ := url.Parse(appURL)
	cfg := config.Config{Env: config.EnvDev, AppURL: u}
	log := slog.New(slog.DiscardHandler)
	mailer := &mail.Recorder{}
	worker := auth.NewResetEmailWorker(st, mailer, u, auth.DefaultResetTokenTTL)
	workers := river.NewWorkers()
	river.AddWorker(workers, worker)
	jobs, err := river.NewClient(riverpgxv5.New(st.Pool()), &river.Config{Workers: workers})
	if err != nil {
		t.Fatal(err)
	}
	svc := auth.NewService(st, auth.NewHasher(cheapParams, auth.DefaultMaxConcurrentHashes), jobs, auth.DefaultOptions(), log)
	h, err := httpapi.New(httpapi.Deps{
		Config: cfg, Store: st, Auth: svc, Logger: log,
		WebUI: fstest.MapFS{"index.html": {Data: []byte(spaMarker)}, "assets/app.js": {Data: []byte("//js")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Q().UpsertClient(ctx, "Dinas Kominfo Jambi"); err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, st: st, handler: h, auth: svc, mailer: mailer, worker: worker, users: map[auth.Role]string{}}
	for _, r := range []auth.Role{auth.RoleAdmin, auth.RoleSupervisor, auth.RoleAgent, auth.RoleRequester} {
		in := auth.NewUser{Email: string(r) + "@example.test", Name: "User " + string(r), Password: password, Role: r}
		if r == auth.RoleRequester {
			in.ClientName, in.JobTitle = "Dinas Kominfo Jambi", "IT Officer"
		}
		if _, err := svc.CreateUser(ctx, in); err != nil {
			t.Fatalf("create %s: %v", r, err)
		}
		e.users[r] = in.Email
	}
	return e
}

type request struct {
	method, path string
	body         any
	cookie       *http.Cookie
	header       map[string]string
}

func (e *env) do(r request) *httptest.ResponseRecorder {
	e.t.Helper()
	var body io.Reader
	if r.body != nil {
		b, _ := json.Marshal(r.body)
		body = strings.NewReader(string(b))
	}
	req := httptest.NewRequest(r.method, r.path, body)
	if r.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range r.header {
		req.Header.Set(k, v)
	}
	if r.cookie != nil {
		req.AddCookie(r.cookie)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func (e *env) login(email, pw string) *httptest.ResponseRecorder {
	return e.do(request{method: http.MethodPost, path: apiBase + "/auth/login", body: gen.LoginRequest{Email: email, Password: pw}})
}

func (e *env) sessionFor(role auth.Role) *http.Cookie {
	e.t.Helper()
	rec := e.login(e.users[role], password)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("login %s: %d %s", role, rec.Code, rec.Body)
	}
	return sessionCookie(e.t, rec)
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "sd_session" {
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func problemCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type = %q, want problem+json (body %s)", ct, rec.Body)
	}
	var p gen.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.Code
}

func TestOrganizationIsPublic(t *testing.T) {
	e := newEnv(t)
	rec := e.do(request{method: http.MethodGet, path: apiBase + "/organization"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var org gen.Organization
	_ = json.Unmarshal(rec.Body.Bytes(), &org)
	if org.Name == "" {
		t.Fatal("organization name missing")
	}
}

func TestLoginSetsSecureSessionCookie(t *testing.T) {
	e := newEnv(t)
	rec := e.login("  AGENT@Example.test ", password)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	c := sessionCookie(t, rec)
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Domain != "" {
		t.Fatalf("cookie attributes wrong: %+v", c)
	}
	var me gen.Me
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Role != gen.Agent || me.Email != e.users[auth.RoleAgent] {
		t.Fatalf("me = %+v", me)
	}
}

func TestWrongPasswordAndUnknownEmailLookTheSame(t *testing.T) {
	e := newEnv(t)
	for _, rec := range []*httptest.ResponseRecorder{
		e.login(e.users[auth.RoleAgent], "wrong-password"),
		e.login("nobody@example.test", "wrong-password"),
	} {
		if rec.Code != http.StatusUnauthorized || problemCode(t, rec) != apperr.CodeInvalidCredentials {
			t.Fatalf("got %d %s", rec.Code, rec.Body)
		}
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	e := newEnv(t)
	email := e.users[auth.RoleAgent]
	for i := 0; i < auth.DefaultMaxFailuresPerEmail; i++ {
		if rec := e.login(email, "wrong-password"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i+1, rec.Code)
		}
	}
	rec := e.login(email, password) // even the right password is refused now
	if rec.Code != http.StatusTooManyRequests || problemCode(t, rec) != apperr.CodeRateLimited {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
}

func TestSuspendedUserIsLockedOut(t *testing.T) {
	e := newEnv(t)
	cookie := e.sessionFor(auth.RoleAgent)
	if _, err := e.st.Pool().Exec(context.Background(),
		`UPDATE users SET status = 'suspended' WHERE email = $1`, e.users[auth.RoleAgent]); err != nil {
		t.Fatal(err)
	}
	if rec := e.do(request{method: http.MethodGet, path: apiBase + "/auth/me", cookie: cookie}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("existing session: %d", rec.Code)
	}
	if rec := e.login(e.users[auth.RoleAgent], password); rec.Code != http.StatusUnauthorized {
		t.Fatalf("new login: %d", rec.Code)
	}
}

func TestLogoutEndsSession(t *testing.T) {
	e := newEnv(t)
	cookie := e.sessionFor(auth.RoleAgent)
	if rec := e.do(request{method: http.MethodPost, path: apiBase + "/auth/logout", cookie: cookie}); rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body)
	}
	rec := e.do(request{method: http.MethodGet, path: apiBase + "/auth/me", cookie: cookie})
	if rec.Code != http.StatusUnauthorized || problemCode(t, rec) != apperr.CodeUnauthenticated {
		t.Fatalf("me after logout: %d", rec.Code)
	}
}

func TestCrossSitePostIsBlocked(t *testing.T) {
	e := newEnv(t)
	cookie := e.sessionFor(auth.RoleAgent)
	rec := e.do(request{method: http.MethodPost, path: apiBase + "/auth/logout", cookie: cookie,
		header: map[string]string{"Sec-Fetch-Site": "cross-site"}})
	if rec.Code != http.StatusForbidden || problemCode(t, rec) != apperr.CodeCrossOrigin {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestInvalidBodyReturnsFieldErrors(t *testing.T) {
	e := newEnv(t)
	rec := e.do(request{method: http.MethodPost, path: apiBase + "/auth/login", body: map[string]string{"email": "x"}})
	if rec.Code != http.StatusBadRequest || problemCode(t, rec) != apperr.CodeInvalidRequest {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "Schema") {
		t.Fatalf("problem leaks the schema: %s", rec.Body)
	}
}

func TestMeIncludesClientForRequesters(t *testing.T) {
	e := newEnv(t)
	rec := e.do(request{method: http.MethodGet, path: apiBase + "/auth/me", cookie: e.sessionFor(auth.RoleRequester)})
	var me gen.Me
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Role != gen.Requester || me.Client == nil || me.Client.Name != "Dinas Kominfo Jambi" {
		t.Fatalf("me = %+v", me)
	}
}

func TestListUsersIsAdminOnly(t *testing.T) {
	e := newEnv(t)
	if rec := e.do(request{method: http.MethodGet, path: apiBase + "/users"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	for _, r := range []auth.Role{auth.RoleSupervisor, auth.RoleAgent, auth.RoleRequester} {
		rec := e.do(request{method: http.MethodGet, path: apiBase + "/users", cookie: e.sessionFor(r)})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s: %d", r, rec.Code)
		}
	}
	rec := e.do(request{method: http.MethodGet, path: apiBase + "/users", cookie: e.sessionFor(auth.RoleAdmin)})
	var users []gen.User
	_ = json.Unmarshal(rec.Body.Bytes(), &users)
	if rec.Code != http.StatusOK || len(users) != len(e.users) {
		t.Fatalf("admin: %d, %d users", rec.Code, len(users))
	}
}

// Every operation not marked public in the spec must reject anonymous calls.
func TestOperationsRequireSessionByDefault(t *testing.T) {
	e := newEnv(t)
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op.Security != nil && len(*op.Security) == 0 {
				continue
			}
			var body any
			if op.RequestBody != nil {
				body = map[string]string{}
			}
			rec := e.do(request{method: method, path: apiBase + path, body: body})
			if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusBadRequest {
				t.Errorf("%s %s anonymous: %d", method, path, rec.Code)
			}
		}
	}
}

func TestPasswordResetFlow(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	email := e.users[auth.RoleAgent]
	oldSession := e.sessionFor(auth.RoleAgent)

	rec := e.do(request{method: http.MethodPost, path: apiBase + "/auth/password/forgot", body: gen.ForgotPasswordRequest{Email: email}})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("forgot: %d %s", rec.Code, rec.Body)
	}
	job := rivertest.RequireInserted(ctx, t, riverpgxv5.New(e.st.Pool()), &auth.SendPasswordResetArgs{}, nil)
	if err := e.worker.Work(ctx, &river.Job[auth.SendPasswordResetArgs]{Args: *job.Args}); err != nil {
		t.Fatal(err)
	}
	msgs := e.mailer.Messages()
	if len(msgs) != 1 || msgs[0].To != email {
		t.Fatalf("mails = %+v", msgs)
	}
	_, token, found := strings.Cut(msgs[0].Text, auth.ResetTokenFragment)
	if !found {
		t.Fatalf("no reset link in %q", msgs[0].Text)
	}
	token = strings.Fields(token)[0]
	if strings.Contains(msgs[0].Text, appURL+"/"+auth.ResetPasswordPath+"?") {
		t.Fatal("token must be in the fragment, not the query")
	}

	reset := func(tok, pw string) *httptest.ResponseRecorder {
		return e.do(request{method: http.MethodPost, path: apiBase + "/auth/password/reset",
			body: gen.ResetPasswordRequest{Token: tok, Password: pw}})
	}
	const newPassword = "brand-new-password"
	if rec := reset(token, newPassword); rec.Code != http.StatusNoContent {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body)
	}
	if rec := reset(token, "another-password"); problemCode(t, rec) != apperr.CodeInvalidToken {
		t.Fatalf("token reuse: %d %s", rec.Code, rec.Body)
	}
	if rec := e.do(request{method: http.MethodGet, path: apiBase + "/auth/me", cookie: oldSession}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old session survived reset: %d", rec.Code)
	}
	if rec := e.login(email, password); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old password still works: %d", rec.Code)
	}
	if rec := e.login(email, newPassword); rec.Code != http.StatusOK {
		t.Fatalf("new password: %d", rec.Code)
	}
}

func TestForgotUnknownEmailQueuesNothing(t *testing.T) {
	e := newEnv(t)
	rec := e.do(request{method: http.MethodPost, path: apiBase + "/auth/password/forgot", body: gen.ForgotPasswordRequest{Email: "ghost@example.test"}})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("forgot: %d", rec.Code)
	}
	rivertest.RequireNotInserted(context.Background(), t, riverpgxv5.New(e.st.Pool()), &auth.SendPasswordResetArgs{}, nil)
}

func TestExpiredResetTokenIsRejected(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	raw, hash, _ := auth.NewToken()
	err := e.st.Tx(ctx, func(tx store.Tx) error {
		var userID string
		if err := tx.PG.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, e.users[auth.RoleAgent]).Scan(&userID); err != nil {
			return err
		}
		_, err := tx.PG.Exec(ctx, `INSERT INTO password_reset_tokens (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
			hash, userID, time.Now().Add(-time.Minute))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := e.do(request{method: http.MethodPost, path: apiBase + "/auth/password/reset",
		body: gen.ResetPasswordRequest{Token: raw, Password: "brand-new-password"}})
	if problemCode(t, rec) != apperr.CodeInvalidToken {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestAuditTrailRecordsSignIns(t *testing.T) {
	e := newEnv(t)
	e.sessionFor(auth.RoleAgent)
	e.login(e.users[auth.RoleAgent], "wrong-password")
	var actions []string
	rows, err := e.st.Pool().Query(context.Background(), `SELECT action FROM audit_log WHERE action LIKE 'auth.%' ORDER BY at`)
	if err != nil {
		t.Fatal(err)
	}
	actions, err = pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(actions, ",") != auth.ActionLogin+","+auth.ActionLoginFailed {
		t.Fatalf("audit actions = %v", actions)
	}
}

func TestCreateUserValidation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	cases := map[string]auth.NewUser{
		apperr.CodeClientRequired:   {Email: "r@x.test", Name: "R", Password: password, Role: auth.RoleRequester},
		apperr.CodeClientNotAllowed: {Email: "a@x.test", Name: "A", Password: password, Role: auth.RoleAgent, ClientName: "Dinas Kominfo Jambi"},
		apperr.CodeClientNotFound:   {Email: "q@x.test", Name: "Q", Password: password, Role: auth.RoleRequester, ClientName: "Nope"},
		apperr.CodeInvalidRole:      {Email: "o@x.test", Name: "O", Password: password, Role: "owner"},
		apperr.CodeWeakPassword:     {Email: "w@x.test", Name: "W", Password: "short", Role: auth.RoleAgent},
	}
	for code, in := range cases {
		_, err := e.auth.CreateUser(ctx, in)
		var ae *apperr.Error
		if !asAppErr(err, &ae) || ae.Code != code {
			t.Errorf("%s: err = %v", code, err)
		}
	}
}

func TestSPAFallbackAndAPINotFound(t *testing.T) {
	e := newEnv(t)
	rec := e.do(request{method: http.MethodGet, path: "/board"})
	if rec.Code != http.StatusOK || rec.Body.String() != spaMarker || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("SPA route: %d %q", rec.Code, rec.Body)
	}
	rec = e.do(request{method: http.MethodGet, path: "/assets/app.js"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset: %d %v", rec.Code, rec.Header())
	}
	for _, p := range []string{apiBase + "/nope", "/api/nope"} {
		rec := e.do(request{method: http.MethodGet, path: p})
		if rec.Code != http.StatusNotFound || problemCode(t, rec) != apperr.CodeNotFound {
			t.Fatalf("%s: %d %s", p, rec.Code, rec.Body)
		}
	}
	if rec.Header().Get("Content-Security-Policy") == "" || rec.Header().Get("X-Request-Id") == "" {
		t.Fatal("security headers or request ID missing")
	}
}

func asAppErr(err error, target **apperr.Error) bool {
	ae, ok := err.(*apperr.Error)
	if ok {
		*target = ae
	}
	return ok
}
