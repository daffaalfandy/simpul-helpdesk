package httpapi_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// The agent's account email, so their commits are theirs.
const agentEmail = "agent@example.test"

// codeEnv is a ticket environment whose e-Office project reads commits from a GitHub repository.
type codeEnv struct {
	*ticketEnv
	gh       hook
	delivery int
}

func newCodeEnv(t *testing.T) *codeEnv {
	t.Helper()
	te := newTicketEnv(t)
	return &codeEnv{ticketEnv: te, gh: te.hook(t, gen.GitProviderGithub, eOfficeURL, te.projectA)}
}

// deliver sends a signed GitHub event and processes it like the job would.
func (ce *codeEnv) deliver(t *testing.T, event, body string) int {
	t.Helper()
	ce.delivery++
	rec := ce.gh.github(event, fmt.Sprint("delivery-", ce.delivery), body, ce.gh.secret)
	runJobs(t, ce.env, ce.process)
	return rec.Code
}

// send delivers an event the connector reads.
func (ce *codeEnv) send(t *testing.T, event, body string) {
	t.Helper()
	if code := ce.deliver(t, event, body); code != http.StatusAccepted {
		t.Fatalf("%s: %d", event, code)
	}
}

const repoJSON = `"repository":{"id":7001,"full_name":"vendor-id/eoffice-jambi","html_url":"https://github.com/vendor-id/eoffice-jambi"}`

func commitJSON(sha, msg, email, at string) string {
	return `{"id":"` + sha + `","message":` + quote(msg) + `,"timestamp":"` + at + `","url":"https://github.com/vendor-id/eoffice-jambi/commit/` + sha +
		`","author":{"name":"Someone","email":"` + email + `","username":"someone"}}`
}

func pushJSON(branch string, commits ...string) string {
	return `{"ref":"refs/heads/` + branch + `","commits":[` + strings.Join(commits, ",") + `],` + repoJSON + `}`
}

// sha is a 40-character commit id whose short form (the first 7) differs per n.
func sha(n int) string { return fmt.Sprintf("%07d", n) + strings.Repeat("0", shaLen-shortSHALen) }

const (
	shaLen      = 40
	shortSHALen = 7
)

func TestCommitsLinkWithoutMovingTickets(t *testing.T) {
	ce := newCodeEnv(t)
	tk := ce.requester.report(t, incident(ce.projectA, "Login fails"))
	branch := "fix/" + tk.Key + "-sso"

	// The agent's commit links the new ticket; it stays New and unassigned.
	ce.send(t, "push", pushJSON(branch, commitJSON(sha(1), "refs "+tk.Key+" add logging", agentEmail, "2026-11-16T08:40:00+07:00")))
	got := ce.agent.ticket(t, tk.Key)
	if got.Status != gen.TicketStatusNew || got.Assignee != nil {
		t.Fatalf("after a commit: %s, assignee %+v", got.Status, got.Assignee)
	}
	activity := decode[[]gen.TicketActivity](t, ce.agent.do(http.MethodGet, "/tickets/"+tk.Key+"/activity", nil), http.StatusOK)
	var code *gen.TicketActivityCode
	for _, a := range activity {
		if a.Action == gen.TicketActivityActionCodeLinked {
			code = a.Code
		}
	}
	if code == nil || code.Provider != gen.GitProviderGithub || code.Kind != gen.CodeEventKindCommit || code.Ref != sha(1)[:7] ||
		code.Outcome != gen.CodeOutcomeLinked || code.Branch != branch {
		t.Fatalf("code activity = %+v", code)
	}
	if n := ce.count(t, `SELECT count(*) FROM audit_log WHERE target_id = $1 AND actor_kind = $2`, tk.Id, audit.ActorGit); n != 1 {
		t.Fatalf("git audit entries = %d, want the link only", n)
	}

	// Fix and reopen words are just words: an assigned ticket in progress stays put.
	ce.agent.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &ce.agentID})
	ce.agent.do(http.MethodPut, "/tickets/"+tk.Key+"/status", gen.TicketStatusInput{Status: gen.TicketStatusInProgress})
	ce.send(t, "push", pushJSON(branch,
		commitJSON(sha(2), "Fixes "+tk.Key+": revert TTL", agentEmail, "2026-11-16T08:52:00+07:00"),
		commitJSON(sha(3), "reopen "+tk.Key, agentEmail, "2026-11-16T08:53:00+07:00")))
	if got = ce.agent.ticket(t, tk.Key); got.Status != gen.TicketStatusInProgress {
		t.Fatalf("after fix and reopen words: %s", got.Status)
	}
	if n := ce.count(t, `SELECT count(*) FROM ticket_dod_items WHERE ticket_id = $1`, tk.Id); n != 0 {
		t.Fatalf("commits ticked %d Definition of Done items", n)
	}

	// The fix reaches main: the commit gains the branch, without a second link or audit entry.
	links := ce.count(t, `SELECT count(*) FROM code_event_links WHERE ticket_id = $1`, tk.Id)
	ce.send(t, "push", pushJSON("main", commitJSON(sha(2), "Fixes "+tk.Key+": revert TTL", agentEmail, "2026-11-16T08:52:00+07:00")))
	if again := ce.count(t, `SELECT count(*) FROM code_event_links WHERE ticket_id = $1`, tk.Id); again != links {
		t.Fatalf("links %d -> %d after the push to main", links, again)
	}
	code2 := decode[gen.TicketCode](t, ce.agent.do(http.MethodGet, "/tickets/"+tk.Key+"/code", nil), http.StatusOK)
	var fix gen.CodeCommit
	for _, c := range code2.Commits {
		if c.Sha == sha(2)[:7] {
			fix = c
		}
	}
	if !slices.Equal(fix.Branches, []string{branch, "main"}) {
		t.Fatalf("branches = %v", fix.Branches)
	}
	if got = ce.agent.ticket(t, tk.Key); got.Status != gen.TicketStatusInProgress {
		t.Fatalf("after the merge: %s", got.Status)
	}
}

func TestOnlyPushesAreRead(t *testing.T) {
	ce := newCodeEnv(t)
	tk := ce.requester.report(t, incident(ce.projectA, "Login fails"))
	for event, body := range map[string]string{
		"pull_request": `{"action":"closed","pull_request":{"number":142,"title":"` + tk.Key + `","merged":true},` + repoJSON + `}`,
		"workflow_run": `{"action":"completed","workflow_run":{"id":881,"conclusion":"success","head_branch":"main"},` + repoJSON + `}`,
		"deployment_status": `{"deployment_status":{"state":"success","environment":"production"},` +
			`"deployment":{"id":3301,"ref":"main","production_environment":true},` + repoJSON + `}`,
	} {
		if code := ce.deliver(t, event, body); code != http.StatusOK {
			t.Errorf("%s: %d, want 200 (acknowledged, not read)", event, code)
		}
	}
	if n := ce.count(t, `SELECT count(*) FROM code_events`); n != 0 {
		t.Fatalf("stored %d events from requests, CI or deployments", n)
	}
}

func TestCodeIgnoresOtherTickets(t *testing.T) {
	ce := newCodeEnv(t)
	other := ce.otherRequester.report(t, incident(ce.projectB, "VPN down"))
	closed := ce.requester.report(t, incident(ce.projectA, "Old issue"))
	if _, err := ce.st.Pool().Exec(t.Context(), `UPDATE tickets SET status = 'closed', closed_at = now(), resolved_at = now() WHERE id = $1`, closed.Id); err != nil {
		t.Fatal(err)
	}
	ce.send(t, "push", pushJSON("main", commitJSON(sha(1),
		"refs HD-99999, "+other.Key+" and "+closed.Key, agentEmail, "2026-11-16T08:40:00+07:00")))
	for key, outcome := range map[string]string{"HD-99999": "unknown_ticket", other.Key: "other_repository", closed.Key: "ticket_closed"} {
		if n := ce.count(t, `SELECT count(*) FROM code_event_links WHERE ticket_key = $1 AND outcome = $2`, key, outcome); n != 1 {
			t.Errorf("%s: %s rows = %d", key, outcome, n)
		}
	}
	if n := ce.count(t, `SELECT count(*) FROM audit_log WHERE target_id = $1 AND action = $2`, closed.Id, tickets.ActionCodeLinked); n != 0 {
		t.Errorf("a closed ticket got %d activity lines", n)
	}
	// An author the helpdesk doesn't know still links.
	tk := ce.requester.report(t, incident(ce.projectA, "Printer"))
	ce.send(t, "push", pushJSON("fix/x", commitJSON(sha(2), "refs "+tk.Key, "stranger@example.com", "2026-11-16T09:00:00+07:00")))
	if n := ce.count(t, `SELECT count(*) FROM code_event_links WHERE ticket_id = $1 AND outcome = 'linked'`, tk.Id); n != 1 {
		t.Fatalf("links = %d", n)
	}
}

func TestCodeNotifications(t *testing.T) {
	ce := newCodeEnv(t)
	tk := ce.requester.report(t, incident(ce.projectA, "Login fails"))
	ce.agent.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &ce.agentID})
	ce.fanout(t)
	count := func(user string) int {
		return ce.count(t, `SELECT count(*) FROM notifications n JOIN users u ON u.id = n.user_id
			WHERE u.email = $1 AND n.ticket_id = $2 AND n.event_type = $3`, user, tk.Id, string(notification.EventCode))
	}

	// Someone else's commit: the assignee hears about it, the client doesn't.
	ce.send(t, "push", pushJSON("fix/x", commitJSON(sha(1), "refs "+tk.Key, "stranger@example.com", "2026-11-16T08:40:00+07:00")))
	ce.fanout(t)
	if n := count(agentEmail); n != 1 {
		t.Fatalf("assignee code notifications = %d", n)
	}
	if n := count(ce.users["requester"]); n != 0 {
		t.Fatalf("requester got %d code notifications", n)
	}
	var provider, kind, outcome string
	if err := ce.st.Pool().QueryRow(t.Context(), `SELECT data->'code'->>'provider', data->'code'->>'kind', data->'code'->>'outcome'
		FROM notifications WHERE ticket_id = $1 AND event_type = 'code'`, tk.Id).Scan(&provider, &kind, &outcome); err != nil ||
		provider != "github" || kind != "commit" || outcome != "linked" {
		t.Fatalf("stored code = %q %q %q, %v", provider, kind, outcome, err)
	}
	inbox := decode[gen.NotificationPage](t, ce.agent.do(http.MethodGet, "/notifications?filter=code", nil), http.StatusOK)
	if len(inbox.Items) != 1 || inbox.Items[0].Code == nil || inbox.Items[0].Code.Ref != sha(1)[:7] {
		t.Fatalf("code inbox = %+v", inbox.Items)
	}

	// The assignee's own commit never notifies them, and a branch the commit reaches later is
	// no news.
	ce.send(t, "push", pushJSON("fix/x", commitJSON(sha(2), "refs "+tk.Key, agentEmail, "2026-11-16T08:50:00+07:00")))
	ce.send(t, "push", pushJSON("main", commitJSON(sha(1), "refs "+tk.Key, "stranger@example.com", "2026-11-16T08:40:00+07:00")))
	ce.fanout(t)
	if n := count(agentEmail); n != 1 {
		t.Fatalf("code notifications = %d, want still 1", n)
	}
}

func TestCodeReads(t *testing.T) {
	ce := newCodeEnv(t)
	tk := ce.requester.report(t, incident(ce.projectA, "Login fails"))
	branch := "fix/" + tk.Key + "-sso"
	ce.send(t, "push", pushJSON(branch,
		commitJSON(sha(1), "refs "+tk.Key+" add logging", agentEmail, "2026-11-16T08:40:00+07:00"),
		commitJSON(sha(2), "Fix "+tk.Key+": revert TTL", agentEmail, "2026-11-16T08:52:00+07:00")))
	ce.send(t, "push", pushJSON("main", commitJSON(sha(2), "Fix "+tk.Key+": revert TTL", agentEmail, "2026-11-16T08:52:00+07:00")))

	// The board chip: two commits, the latest one now on main.
	board := decode[gen.Board](t, ce.agent.do(http.MethodGet, "/board", nil), http.StatusOK)
	var chip *gen.BoardCardCode
	for _, col := range board.Columns {
		for _, c := range col.Cards {
			if c.Key == tk.Key {
				chip = c.Code
			}
		}
	}
	if chip == nil || chip.Commits != 2 || chip.Sha != sha(2)[:7] || chip.Branch != "main" {
		t.Fatalf("board chip = %+v", chip)
	}

	code := decode[gen.TicketCode](t, ce.agent.do(http.MethodGet, "/tickets/"+tk.Key+"/code", nil), http.StatusOK)
	if code.Repository == nil || code.Repository.FullName != "vendor-id/eoffice-jambi" || len(code.Commits) != 2 ||
		code.Commits[0].Sha != sha(2)[:7] || code.Commits[0].Title != "Fix "+tk.Key+": revert TTL" ||
		code.Commits[0].Author != "User agent" || !slices.Equal(code.Commits[1].Branches, []string{branch}) {
		t.Fatalf("code = %+v", code)
	}
	// Commit messages are internal.
	expectProblem(t, ce.requester.do(http.MethodGet, "/tickets/"+tk.Key+"/code", nil), http.StatusForbidden, apperr.CodeForbidden)

	// Settings: the tester lists the keys a message names; the log lists the latest links.
	admin := ce.admin
	test := decode[gen.CommitTest](t, admin.do(http.MethodPost, "/integrations/commit-test",
		gen.CommitTestInput{Message: "Closes " + tk.Key + ", refs HD-99999"}), http.StatusOK)
	want := []gen.CommitTestRef{{Key: tk.Key, TicketExists: true}, {Key: "HD-99999"}}
	if !slices.Equal(test.Refs, want) {
		t.Fatalf("tester = %+v", test.Refs)
	}
	log := decode[[]gen.CodeEventLog](t, admin.do(http.MethodGet, "/integrations/events", nil), http.StatusOK)
	if len(log) != 2 || log[0].Outcome != gen.CodeOutcomeLinked || log[0].Repository != "vendor-id/eoffice-jambi" ||
		log[0].TicketKey != tk.Key {
		t.Fatalf("log = %+v", log)
	}
	expectProblem(t, ce.agent.do(http.MethodGet, "/integrations/events", nil), http.StatusForbidden, apperr.CodeForbidden)
}
