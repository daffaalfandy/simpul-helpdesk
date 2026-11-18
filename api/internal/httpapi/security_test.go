package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/richtext"
)

// Tests from the M10 security review.

// resetToken asks for a reset link for email and returns the token from the email.
func (e *env) resetToken(t *testing.T, email string) string {
	t.Helper()
	ctx := context.Background()
	before := len(e.mailer.Messages())
	if rec := e.do(request{method: http.MethodPost, path: apiBase + "/auth/password/forgot", body: gen.ForgotPasswordRequest{Email: email}}); rec.Code != http.StatusAccepted {
		t.Fatalf("forgot: %d %s", rec.Code, rec.Body)
	}
	var encoded []byte
	if err := e.st.Pool().QueryRow(ctx, `SELECT args FROM river_job WHERE kind = $1 ORDER BY id DESC LIMIT 1`,
		auth.JobKindPasswordReset).Scan(&encoded); err != nil {
		t.Fatal(err)
	}
	var args auth.SendPasswordResetArgs
	if err := json.Unmarshal(encoded, &args); err != nil {
		t.Fatal(err)
	}
	if err := e.worker.Work(ctx, &river.Job[auth.SendPasswordResetArgs]{Args: args}); err != nil {
		t.Fatal(err)
	}
	msgs := e.mailer.Messages()
	if len(msgs) != before+1 {
		t.Fatalf("no reset email for %s", email)
	}
	_, token, found := strings.Cut(msgs[len(msgs)-1].Text, auth.ResetTokenFragment)
	if !found {
		t.Fatal("no reset link in the email")
	}
	return strings.Fields(token)[0]
}

func (e *env) resetPassword(token, pw string) *httptest.ResponseRecorder {
	return e.do(request{method: http.MethodPost, path: apiBase + "/auth/password/reset", body: gen.ResetPasswordRequest{Token: token, Password: pw}})
}

// A burst of parallel guesses is counted one by one: only the limit's worth get an answer.
func TestParallelGuessesAreCountedOneByOne(t *testing.T) {
	e := newEnv(t)
	const guesses = 4 * auth.DefaultMaxFailuresPerEmail
	codes := make(chan int, guesses)
	var wg sync.WaitGroup
	for i := range guesses {
		wg.Go(func() { codes <- e.login(e.users[auth.RoleAgent], fmt.Sprintf("wrong-password-%d", i)).Code })
	}
	wg.Wait()
	close(codes)
	answered := 0
	for code := range codes {
		switch code {
		case http.StatusUnauthorized:
			answered++
		case http.StatusTooManyRequests:
		default:
			t.Fatalf("unexpected status %d", code)
		}
	}
	if answered != auth.DefaultMaxFailuresPerEmail {
		t.Fatalf("%d guesses were checked, want %d", answered, auth.DefaultMaxFailuresPerEmail)
	}
}

// A successful sign-in doesn't count against the address: a shared office address isn't locked
// out by its own colleagues signing in.
func TestSuccessfulSignInsDontCount(t *testing.T) {
	e := newEnv(t)
	for range auth.DefaultMaxFailuresPerIP + 1 {
		e.sessionFor(auth.RoleAgent)
	}
}

// Someone locked out by wrong guesses can sign in as soon as they reset their password.
func TestResetEndsLockout(t *testing.T) {
	e := newEnv(t)
	email := e.users[auth.RoleAgent]
	for range auth.DefaultMaxFailuresPerEmail {
		e.login(email, "wrong-password")
	}
	if rec := e.login(email, password); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("locked out: %d", rec.Code)
	}
	const newPassword = "brand-new-password"
	if rec := e.resetPassword(e.resetToken(t, email), newPassword); rec.Code != http.StatusNoContent {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body)
	}
	if rec := e.login(email, newPassword); rec.Code != http.StatusOK {
		t.Fatalf("sign in after reset: %d %s", rec.Code, rec.Body)
	}
}

// Made-up reset links count against the caller's address (and are refused before any password
// is hashed).
func TestWrongResetLinksAreRateLimited(t *testing.T) {
	e := newEnv(t)
	madeUp := func() string {
		raw, _, _ := auth.NewToken()
		return raw
	}
	for range auth.DefaultMaxFailuresPerIP {
		expectProblem(t, e.resetPassword(madeUp(), "brand-new-password"), http.StatusBadRequest, apperr.CodeInvalidToken)
	}
	expectProblem(t, e.resetPassword(madeUp(), "brand-new-password"), http.StatusTooManyRequests, apperr.CodeRateLimited)
}

// Suspending someone voids their reset links, and a link never reactivates a suspended account.
func TestSuspensionVoidsResetLinks(t *testing.T) {
	te := newTicketEnv(t)
	email := te.users[auth.RoleAgent]
	token := te.resetToken(t, email)
	if rec := te.admin.do(http.MethodPost, "/members/"+te.agentID.String()+"/suspend", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("suspend: %d %s", rec.Code, rec.Body)
	}
	expectProblem(t, te.resetPassword(token, "brand-new-password"), http.StatusBadRequest, apperr.CodeInvalidToken)

	// A link that somehow outlives the suspension still doesn't work.
	supervisor := te.users[auth.RoleSupervisor]
	token = te.resetToken(t, supervisor)
	if _, err := te.st.Pool().Exec(context.Background(), `UPDATE users SET status = 'suspended' WHERE email = $1`, supervisor); err != nil {
		t.Fatal(err)
	}
	expectProblem(t, te.resetPassword(token, "brand-new-password"), http.StatusBadRequest, apperr.CodeInvalidToken)
}

// An administrator moved to a role in one organization keeps only that organization's tokens:
// tokens made elsewhere must not come back if they join that organization later.
func TestDemotedAdministratorKeepsOnlyTheirOrganizationsTokens(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	other := e.secondOrganization(t, "PT Mitra Data", "MD")
	admin := e.as(auth.RoleAdmin)
	admin.tokenFor(t, gen.TokenAccessWrite)
	admin.in(other).tokenFor(t, gen.TokenAccessWrite)

	second := auth.NewUser{Email: "admin2@example.test", Name: "Second Admin", Password: password, Role: auth.RoleAdmin}
	if _, err := e.auth.CreateUser(ctx, second); err != nil {
		t.Fatal(err)
	}
	rec := e.login(second.Email, password)
	secondAdmin := session{e: e, cookie: sessionCookie(t, rec)}.in(e.org)
	adminID := userID(t, e, auth.RoleAdmin)
	if rec := secondAdmin.do(http.MethodPut, "/members/"+adminID.String()+"/role", gen.MemberRoleInput{Role: gen.StaffRoleAgent}); rec.Code != http.StatusNoContent {
		t.Fatalf("demote: %d %s", rec.Code, rec.Body)
	}
	var here, elsewhere int
	if err := e.st.Pool().QueryRow(ctx, `SELECT count(*) FILTER (WHERE organization_id = $2), count(*) FILTER (WHERE organization_id <> $2)
		FROM personal_access_tokens WHERE user_id = $1`, adminID, e.org).Scan(&here, &elsewhere); err != nil {
		t.Fatal(err)
	}
	if here != 1 || elsewhere != 0 {
		t.Fatalf("tokens here %d, elsewhere %d; want 1 and 0", here, elsewhere)
	}
}

// Bodies past the rich-text limits are refused when written, whoever writes them.
func TestTooComplexTextIsRefused(t *testing.T) {
	te := newTicketEnv(t)
	tk := te.requester.report(t, incident(te.projectA, "Printer offline"))
	deep := strings.Repeat("> ", richtext.MaxNestingDepth+1) + "deep"
	wide := strings.Repeat("|a", 500) + "|\n" + strings.Repeat("|-", 500) + "|\n" + strings.Repeat("|\n", richtext.MaxTableCells/500)
	for _, body := range []string{deep, wide} {
		expectProblem(t, te.requester.do(http.MethodPost, "/tickets/"+tk.Key+"/messages",
			gen.TicketMessageInput{Body: body, Visibility: gen.MessageVisibilityPublic}), http.StatusBadRequest, apperr.CodeTextTooComplex)
	}
	in := incident(te.projectA, "Printer offline")
	in.Description = deep
	expectProblem(t, te.requester.do(http.MethodPost, "/tickets", in), http.StatusBadRequest, apperr.CodeTextTooComplex)
}

// The hourly purge deletes expired sessions, spent reset links and old sign-in attempts, and
// keeps live ones.
func TestAuthPurge(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	agentID := userID(t, e, auth.RoleAgent)
	now := time.Now()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := e.st.Pool().Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, expires := range []time.Time{now.Add(-time.Minute), now.Add(time.Hour)} {
		_, hash, _ := auth.NewToken()
		exec(`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`, hash, agentID, expires)
		_, hash, _ = auth.NewToken()
		exec(`INSERT INTO password_reset_tokens (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`, hash, agentID, expires)
	}
	for i, at := range []time.Time{now.Add(-25 * time.Hour), now.Add(-time.Hour)} {
		exec(`INSERT INTO login_attempts (key, at) VALUES ($1, $2)`, fmt.Sprintf("test:%d", i), at)
	}
	_, hash, _ := auth.NewToken()
	exec(`INSERT INTO password_reset_tokens (token_hash, user_id, expires_at, used_at) VALUES ($1, $2, $3, $4)`,
		hash, agentID, now.Add(time.Hour), now.Add(-time.Minute))

	if err := auth.NewPurgeWorker(e.st).Work(ctx, &river.Job[auth.PurgeArgs]{}); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int{"sessions": 1, "password_reset_tokens": 1, "login_attempts": 1} {
		var n int
		if err := e.st.Pool().QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("%s: %d rows left, want %d", table, n, want)
		}
	}
}
