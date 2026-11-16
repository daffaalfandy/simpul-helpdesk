package httpapi_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

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
