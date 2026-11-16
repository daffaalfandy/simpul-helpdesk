package httpapi_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// The agent's account email, so their commits are theirs.
const agentEmail = "agent@example.test"

// codeEnv is a ticket environment whose e-Office project reads code from a GitHub repository.
type codeEnv struct {
	*ticketEnv
	gh       hook
	delivery int
}

func newCodeEnv(t *testing.T, c dod.Config) *codeEnv {
	t.Helper()
	te := newTicketEnv(t)
	ce := &codeEnv{ticketEnv: te, gh: te.hook(t, gen.GitProviderGithub, eOfficeURL, te.projectA)}
	testdb.SetProjectDoD(t, te.st, te.projectA, c)
	return ce
}

// send delivers a signed GitHub event and processes it like the job would.
func (ce *codeEnv) send(t *testing.T, event, body string) {
	t.Helper()
	ce.delivery++
	if rec := ce.gh.github(event, fmt.Sprint("delivery-", ce.delivery), body, ce.gh.secret); rec.Code != http.StatusAccepted {
		t.Fatalf("%s: %d %s", event, rec.Code, rec.Body)
	}
	runJobs(t, ce.env, ce.process)
}

const repoJSON = `"repository":{"id":7001,"full_name":"vendor-id/eoffice-jambi","html_url":"https://github.com/vendor-id/eoffice-jambi"}`

func commitJSON(sha, msg, email, at string) string {
	return `{"id":"` + sha + `","message":` + quote(msg) + `,"timestamp":"` + at + `","url":"https://github.com/vendor-id/eoffice-jambi/commit/` + sha +
		`","author":{"name":"Someone","email":"` + email + `","username":"someone"}}`
}

func pushJSON(branch string, commits ...string) string {
	return `{"ref":"refs/heads/` + branch + `","commits":[` + strings.Join(commits, ",") + `],` + repoJSON + `}`
}

func prJSON(action string, merged bool, number int, title, head, base, at string) string {
	return fmt.Sprintf(`{"action":%q,"pull_request":{"number":%d,"html_url":"https://github.com/vendor-id/eoffice-jambi/pull/%d","title":%s,`+
		`"body":"","merged":%t,"created_at":%q,"updated_at":%q,"closed_at":%q,"merged_at":%q,"merge_commit_sha":"m%d",`+
		`"user":{"login":"someone"},"merged_by":{"login":"dewi.w"},"head":{"ref":%q,"sha":"h"},"base":{"ref":%q}},%s}`,
		action, number, number, quote(title), merged, at, at, at, at, number, head, base, repoJSON)
}

func runJSON(id int, branch, at string) string {
	return fmt.Sprintf(`{"action":"completed","workflow_run":{"id":%d,"name":"CI","run_number":%d,"head_branch":%q,"head_sha":"s",`+
		`"conclusion":"success","html_url":"https://github.com/vendor-id/eoffice-jambi/actions/runs/%d","created_at":%q,"run_started_at":%q},%s}`,
		id, id, branch, id, at, at, repoJSON)
}

func deployJSON(id int, at string) string {
	return fmt.Sprintf(`{"deployment_status":{"state":"success","environment":"production","created_at":%q},`+
		`"deployment":{"id":%d,"sha":"s","ref":"main","environment":"production","production_environment":true},%s}`, at, id, repoJSON)
}

func sha(n int) string { return fmt.Sprintf("%040d", n) }

func itemsDone(tk gen.Ticket) []gen.DodItem {
	var out []gen.DodItem
	if tk.Dod == nil {
		return nil
	}
	for _, it := range tk.Dod.Items {
		if it.DoneAt != nil {
			out = append(out, it.Item)
		}
	}
	return out
}

func eOfficeDoD() dod.Config {
	c := dod.Default()
	c.Merged = dod.Merged{Enabled: true, Branch: dod.BranchMain}
	return c
}

func TestCodeMovesTickets(t *testing.T) {
	ce := newCodeEnv(t, eOfficeDoD())
	tk := ce.requester.report(t, incident(ce.projectA, "Login fails"))

	// The agent's commit hands the new ticket to them and starts the work.
	ce.send(t, "push", pushJSON("fix/"+tk.Key+"-sso",
		commitJSON(sha(1), "refs "+tk.Key+" add logging", agentEmail, "2026-11-16T08:40:00+07:00")))
	got := ce.agent.ticket(t, tk.Key)
	if got.Status != gen.TicketStatusInProgress || got.Assignee == nil || got.Assignee.Id != ce.agentID {
		t.Fatalf("after refs: %s, assignee %+v", got.Status, got.Assignee)
	}
	activity := decode[[]gen.TicketActivity](t, ce.agent.do(http.MethodGet, "/tickets/"+tk.Key+"/activity", nil), http.StatusOK)
	var code *gen.TicketActivityCode
	for _, a := range activity {
		if a.Action == gen.TicketActivityActionCodeLinked {
			code = a.Code
		}
	}
	if code == nil || code.Provider != gen.GitProviderGithub || code.Kind != gen.CodeEventKindCommit || code.Ref != sha(1)[:7] ||
		code.Outcome != gen.CodeOutcomeLinked || code.Branch != "fix/"+tk.Key+"-sso" {
		t.Fatalf("code activity = %+v", code)
	}
	if n := ce.count(t, `SELECT count(*) FROM audit_log WHERE target_id = $1 AND actor_kind = $2`, tk.Id, audit.ActorGit); n < 3 {
		t.Fatalf("git audit entries = %d", n)
	}

	// A fix commit ticks Fix committed; the ticket waits for the merge.
	ce.send(t, "push", pushJSON("fix/"+tk.Key+"-sso",
		commitJSON(sha(2), "Fix "+tk.Key+": revert TTL", agentEmail, "2026-11-16T08:52:00+07:00")))
	got = ce.agent.ticket(t, tk.Key)
	if done := itemsDone(got); !slices.Equal(done, []gen.DodItem{gen.DodItemFixCommitted}) || got.Status != gen.TicketStatusInProgress {
		t.Fatalf("after fix: %s %v", got.Status, done)
	}

	// The pull request merged into main ticks merged and asks the client to test.
	merged := prJSON("closed", true, 142, "Revert TTL", "fix/"+tk.Key+"-sso", "main", "2026-11-16T05:40:00Z")
	ce.send(t, "pull_request", merged)
	got = ce.agent.ticket(t, tk.Key)
	if got.Status != gen.TicketStatusInUat || !slices.Contains(itemsDone(got), gen.DodItemMerged) {
		t.Fatalf("after merge: %s %v", got.Status, itemsDone(got))
	}
	ce.fanout(t)
	if n := ce.notificationFor(t, ce.requesterID, tk.Id, string(notification.EventUATRequested)); n.event == "" {
		t.Fatal("requester not asked to test")
	}

	// The same merge delivered again changes nothing.
	links := ce.count(t, `SELECT count(*) FROM code_event_links WHERE ticket_id = $1`, tk.Id)
	ce.send(t, "pull_request", merged)
	if again := ce.count(t, `SELECT count(*) FROM code_event_links WHERE ticket_id = $1`, tk.Id); again != links {
		t.Fatalf("links %d -> %d after a redelivery", links, again)
	}

	// "reopen" takes it back to work and forgets the merge, so the next merge asks again.
	ce.send(t, "push", pushJSON("main", commitJSON(sha(3), "reopen "+tk.Key+" token still expires", agentEmail, "2026-11-16T14:00:00+07:00")))
	got = ce.agent.ticket(t, tk.Key)
	if got.Status != gen.TicketStatusInProgress || slices.Contains(itemsDone(got), gen.DodItemMerged) {
		t.Fatalf("after reopen: %s %v", got.Status, itemsDone(got))
	}
	ce.send(t, "pull_request", prJSON("closed", true, 143, "Really fix TTL", "fix/"+tk.Key+"-again", "main", "2026-11-16T08:00:00Z"))
	if got = ce.agent.ticket(t, tk.Key); got.Status != gen.TicketStatusInUat {
		t.Fatalf("after the second merge: %s", got.Status)
	}
}

func TestCodeIgnoresOtherTickets(t *testing.T) {
	ce := newCodeEnv(t, eOfficeDoD())
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
	if got := ce.admin.ticket(t, other.Key); got.Status != gen.TicketStatusNew {
		t.Errorf("another repository's ticket moved to %s", got.Status)
	}
	// An author the helpdesk doesn't know leaves a new ticket New, linked.
	tk := ce.requester.report(t, incident(ce.projectA, "Printer"))
	ce.send(t, "push", pushJSON("fix/x", commitJSON(sha(2), "refs "+tk.Key, "stranger@example.com", "2026-11-16T09:00:00+07:00")))
	if got := ce.agent.ticket(t, tk.Key); got.Status != gen.TicketStatusNew || got.Assignee != nil {
		t.Fatalf("unknown author: %s %+v", got.Status, got.Assignee)
	}
	if n := ce.count(t, `SELECT count(*) FROM code_event_links WHERE ticket_id = $1 AND outcome = 'linked'`, tk.Id); n != 1 {
		t.Fatalf("links = %d", n)
	}
}

func TestCodeCIAndDeploy(t *testing.T) {
	ci := dod.None()
	ci.Merged = dod.Merged{Enabled: true, Branch: dod.BranchRelease}
	ci.CIPassed = true
	ce := newCodeEnv(t, ci)
	tk := ce.requester.report(t, incident(ce.projectA, "Export dates"))
	ce.agent.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &ce.agentID})

	// CI on the release branch reaches the server before the merge does; it still counts.
	ce.send(t, "workflow_run", runJSON(881, "release/2026.11", "2026-11-16T05:41:00Z"))
	ce.send(t, "pull_request", prJSON("closed", true, 57, "Dates", "fix/"+tk.Key, "release/2026.11", "2026-11-16T05:40:00Z"))
	got := ce.agent.ticket(t, tk.Key)
	if got.Status != gen.TicketStatusResolved || !slices.Contains(itemsDone(got), gen.DodItemCiPassed) {
		t.Fatalf("after merge and CI: %s %v", got.Status, itemsDone(got))
	}

	// Merged into main and deployed: resolved by the production deployment after the merge.
	web := dod.None()
	web.Merged = dod.Merged{Enabled: true, Branch: dod.BranchMain}
	web.DeployedProduction = true
	testdb.SetProjectDoD(t, ce.st, ce.projectA, web)
	tk2 := ce.requester.report(t, incident(ce.projectA, "Homepage banner"))
	ce.agent.do(http.MethodPut, "/tickets/"+tk2.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &ce.agentID})
	ce.send(t, "pull_request", prJSON("closed", true, 91, "Banner", "fix/"+tk2.Key, "main", "2026-11-16T06:00:00Z"))
	if got := ce.agent.ticket(t, tk2.Key); got.Status != gen.TicketStatusInProgress {
		t.Fatalf("merged but not deployed: %s", got.Status)
	}
	ce.send(t, "deployment_status", deployJSON(3301, "2026-11-16T06:30:00Z"))
	got = ce.agent.ticket(t, tk2.Key)
	if got.Status != gen.TicketStatusResolved || !slices.Contains(itemsDone(got), gen.DodItemDeployed) {
		t.Fatalf("after deploy: %s %v", got.Status, itemsDone(got))
	}
	var cause string
	if err := ce.st.Pool().QueryRow(t.Context(), `SELECT data->>'cause' FROM audit_log WHERE target_id = $1 AND action = $2
		ORDER BY at DESC, id DESC LIMIT 1`, tk2.Id, tickets.ActionStatusChanged).Scan(&cause); err != nil || cause != "code_done" {
		t.Fatalf("resolve cause = %q, %v", cause, err)
	}
}

func TestCodeNotifications(t *testing.T) {
	ce := newCodeEnv(t, eOfficeDoD())
	tk := ce.requester.report(t, incident(ce.projectA, "Login fails"))
	ce.agent.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &ce.agentID})
	ce.fanout(t)
	count := func(user string) int {
		return ce.count(t, `SELECT count(*) FROM notifications n JOIN users u ON u.id = n.user_id
			WHERE u.email = $1 AND n.ticket_id = $2 AND n.event_type = $3`, user, tk.Id, string(notification.EventCode))
	}

	// A commit that starts the work: the assignee hears of the status change, with the code
	// folded in, so the status email still goes out.
	ce.send(t, "push", pushJSON("fix/x", commitJSON(sha(9), "refs "+tk.Key, "stranger@example.com", "2026-11-16T08:30:00+07:00")))
	ce.fanout(t)
	var change string
	if err := ce.st.Pool().QueryRow(t.Context(), `SELECT data->'also'->0->>'change' FROM notifications n JOIN users u ON u.id = n.user_id
		WHERE u.email = $1 AND n.ticket_id = $2 AND n.event_type = 'status' AND n.data->>'cause' = 'code_activity'`,
		agentEmail, tk.Id).Scan(&change); err != nil || change != string(notification.ChangeCode) {
		t.Fatalf("status notification folds %q, %v", change, err)
	}

	// Someone else's commit on a ticket in progress: the assignee hears about it, the client
	// doesn't.
	ce.send(t, "push", pushJSON("fix/x", commitJSON(sha(1), "refs "+tk.Key, "stranger@example.com", "2026-11-16T08:40:00+07:00")))
	ce.fanout(t)
	if n := count(agentEmail); n != 1 {
		t.Fatalf("assignee code notifications = %d", n)
	}
	if n := count(ce.users["requester"]); n != 0 {
		t.Fatalf("requester got %d code notifications", n)
	}
	var provider, kind string
	if err := ce.st.Pool().QueryRow(t.Context(), `SELECT data->'code'->>'provider', data->'code'->>'kind' FROM notifications
		WHERE ticket_id = $1 AND event_type = 'code'`, tk.Id).Scan(&provider, &kind); err != nil || provider != "github" || kind != "commit" {
		t.Fatalf("stored code = %q %q, %v", provider, kind, err)
	}
	inbox := decode[gen.NotificationPage](t, ce.agent.do(http.MethodGet, "/notifications?filter=code", nil), http.StatusOK)
	if len(inbox.Items) != 1 || inbox.Items[0].Code == nil || inbox.Items[0].Code.Ref != sha(1)[:7] {
		t.Fatalf("code inbox = %+v", inbox.Items)
	}

	// The assignee's own commit never notifies them.
	ce.send(t, "push", pushJSON("fix/x", commitJSON(sha(2), "refs "+tk.Key, agentEmail, "2026-11-16T08:50:00+07:00")))
	ce.fanout(t)
	if n := count(agentEmail); n != 1 {
		t.Fatalf("own commit notified the author: %d", n)
	}
}

func TestCodeReads(t *testing.T) {
	ce := newCodeEnv(t, eOfficeDoD())
	tk := ce.requester.report(t, incident(ce.projectA, "Login fails"))
	ce.agent.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &ce.agentID})
	branch := "fix/" + tk.Key + "-sso"
	ce.send(t, "push", pushJSON(branch,
		commitJSON(sha(1), "refs "+tk.Key+" add logging", agentEmail, "2026-11-16T08:40:00+07:00"),
		commitJSON(sha(2), "Fix "+tk.Key+": revert TTL", agentEmail, "2026-11-16T08:52:00+07:00")))
	ce.send(t, "pull_request", prJSON("opened", false, 142, "Revert TTL", branch, "main", "2026-11-16T02:05:00Z"))

	// The board chip shows the pull request in review.
	board := decode[gen.Board](t, ce.agent.do(http.MethodGet, "/board", nil), http.StatusOK)
	var chip *gen.BoardCardCode
	for _, col := range board.Columns {
		for _, c := range col.Cards {
			if c.Key == tk.Key {
				chip = c.Code
			}
		}
	}
	if chip == nil || chip.Request == nil || chip.Request.Number != 142 || chip.Request.State != gen.CodeRequestStateOpen ||
		chip.Commits != 2 || chip.Branch != branch || chip.Deployed {
		t.Fatalf("board chip = %+v", chip)
	}

	// Merged: the request's state follows, and the merged item says how it was met.
	ce.send(t, "pull_request", prJSON("closed", true, 142, "Revert TTL", branch, "main", "2026-11-16T05:40:00Z"))
	code := decode[gen.TicketCode](t, ce.agent.do(http.MethodGet, "/tickets/"+tk.Key+"/code", nil), http.StatusOK)
	if code.Repository == nil || code.Repository.FullName != "vendor-id/eoffice-jambi" || len(code.Requests) != 1 ||
		code.Requests[0].State != gen.CodeRequestStateMerged || code.Requests[0].Author != "dewi.w" || len(code.Commits) != 2 ||
		code.Commits[0].Sha != sha(2)[:7] || code.Commits[0].Title != "Fix "+tk.Key+": revert TTL" || code.Commits[0].Author != "User agent" {
		t.Fatalf("code = %+v", code)
	}
	got := ce.agent.ticket(t, tk.Key)
	if got.Dod == nil || !got.Dod.HasRepository {
		t.Fatal("ticket DoD lacks its repository")
	}
	for _, it := range got.Dod.Items {
		switch it.Item {
		case gen.DodItemMerged:
			if it.Source == nil || it.Source.Kind != gen.CodeEventKindPrMerged || it.Source.Ref != "142" || it.Source.Author != "dewi.w" {
				t.Errorf("merged source = %+v", it.Source)
			}
		case gen.DodItemFixCommitted:
			if it.Source == nil || it.Source.Kind != gen.CodeEventKindCommit || it.Source.Ref != sha(2)[:7] {
				t.Errorf("fix source = %+v", it.Source)
			}
		}
	}
	// Commit messages are internal.
	expectProblem(t, ce.requester.do(http.MethodGet, "/tickets/"+tk.Key+"/code", nil), http.StatusForbidden, apperr.CodeForbidden)

	// Settings: the tester reads messages like the connector; the log lists what happened.
	admin := ce.admin
	test := decode[gen.CommitTest](t, admin.do(http.MethodPost, "/integrations/commit-test",
		gen.CommitTestInput{Message: "Closes " + tk.Key + ", refs HD-99999"}), http.StatusOK)
	want := []gen.CommitTestRef{{Key: tk.Key, Action: gen.CommitRefActionFix, TicketExists: true}, {Key: "HD-99999", Action: gen.CommitRefActionLink}}
	if !slices.Equal(test.Refs, want) {
		t.Fatalf("tester = %+v", test.Refs)
	}
	log := decode[[]gen.CodeEventLog](t, admin.do(http.MethodGet, "/integrations/events", nil), http.StatusOK)
	if len(log) == 0 || log[0].Kind != gen.CodeEventKindPrMerged || log[0].Outcome != gen.CodeOutcomeMerged ||
		log[0].StatusTo == nil || *log[0].StatusTo != gen.TicketStatusInUat || log[0].Repository != "vendor-id/eoffice-jambi" {
		t.Fatalf("log = %+v", log)
	}
	expectProblem(t, ce.agent.do(http.MethodGet, "/integrations/events", nil), http.StatusForbidden, apperr.CodeForbidden)
}
