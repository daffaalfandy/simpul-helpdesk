package httpapi_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/projects"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

const (
	branchMain = "main"
	// sqlCodeItem reads a ticket's code item: the commit that met it and who did.
	sqlCodeItem = `SELECT count(*) FROM ticket_dod_items WHERE ticket_id = $1 AND item = 'code_on_branch'
		AND code_event_id IS NOT NULL AND done_by IS NULL`
	sqlCodeTicks = `SELECT count(*) FROM audit_log WHERE target_id = $1 AND action = $2 AND actor_kind = $3 AND actor_name = $4`
)

// codeAndUAT asks for the fix on branch, then the client's test.
func codeAndUAT(branch string) dod.Config {
	c := uatDoD()
	c.CodeOnBranch = dod.CodeOnBranch{Enabled: true, Branch: branch}
	return c
}

func projectDodPath(id uuid.UUID) string { return "/projects/" + id.String() + "/dod" }

// setProjectDod saves a project's Definition of Done as a supervisor.
func (te *ticketEnv) setProjectDod(t *testing.T, id uuid.UUID, c dod.Config, mandatory bool) gen.Project {
	t.Helper()
	return decode[gen.Project](t, te.supervisor.do(http.MethodPut, projectDodPath(id),
		gen.ProjectDodInput{Dod: apiDod(c), Mandatory: mandatory, AllowTicketOverride: true}), http.StatusOK)
}

func (te *ticketEnv) userID(t *testing.T, role auth.Role) uuid.UUID {
	t.Helper()
	u, err := te.st.Q().GetUserByEmail(context.Background(), te.users[role])
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

// startWork assigns the ticket to the agent and starts it.
func (te *ticketEnv) startWork(t *testing.T, key string) {
	t.Helper()
	decode[gen.Ticket](t, te.agent.do(http.MethodPut, "/tickets/"+key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	te.move(t, key, gen.TicketStatusInProgress)
}

// A Definition of Done that isn't mandatory is guidance: anyone resolves without a reason, no
// dod_skipped, no supervisor alert. Mandatory keeps the reason and the alert.
func TestDoDMandatorySwitch(t *testing.T) {
	te := newTicketEnv(t)
	supervisorID := te.userID(t, auth.RoleSupervisor)
	// What the supervisor hears about a ticket besides its arrival (unassigned, so theirs).
	supervisorNotices := func(ticketID uuid.UUID) int {
		te.fanout(t)
		n := 0
		for _, x := range te.notices(t, ticketID) {
			if x.user == supervisorID && x.event != notification.EventNewTicket {
				n++
			}
		}
		return n
	}

	p := te.setProjectDod(t, te.projectA, uatDoD(), true)
	if !p.DodMandatory {
		t.Fatal("saved project isn't mandatory")
	}
	strict := te.requester.report(t, incident(te.projectA, "Archive search is slow"))
	te.startWork(t, strict.Key)
	if d := te.agent.ticket(t, strict.Key).Dod; !d.Mandatory || d.Met {
		t.Fatalf("mandatory DoD = %+v", d)
	}
	expectProblem(t, te.agent.setStatus(strict.Key, gen.TicketStatusResolved, ""), http.StatusConflict, apperr.CodeDoDUnmet)
	decode[gen.Ticket](t, te.agent.setStatus(strict.Key, gen.TicketStatusResolved, skipReason), http.StatusOK)
	if last := lastActivity(te.agent.activity(t, strict.Key)); last.Cause == nil || *last.Cause != gen.TicketStatusCauseDodSkipped {
		t.Fatalf("mandatory resolve = %+v", last)
	}
	if n := supervisorNotices(strict.Id); n != 1 {
		t.Fatalf("supervisor notices for a skipped DoD = %d, want 1", n)
	}

	// Guidance only: the checklist still shows, the resolve is a plain move.
	p = te.setProjectDod(t, te.projectA, uatDoD(), false)
	if p.DodMandatory {
		t.Fatal("saved project is still mandatory")
	}
	var data map[string]any
	if err := te.st.Pool().QueryRow(context.Background(),
		`SELECT data FROM audit_log WHERE action = $1 AND target_id = $2 ORDER BY at DESC, id DESC LIMIT 1`,
		projects.ActionDoDUpdated, te.projectA).Scan(&data); err != nil {
		t.Fatal(err)
	}
	if _, ok := data["dod_mandatory"]; !ok {
		t.Fatalf("audit data lacks dod_mandatory: %v", data)
	}
	free := te.requester.report(t, incident(te.projectA, "Disposition letter shows the wrong date"))
	te.startWork(t, free.Key)
	d := te.agent.ticket(t, free.Key).Dod
	if d.Mandatory || d.Met || !slices.Equal(d.Pending, []gen.DodItem{gen.DodItemUatApproved}) {
		t.Fatalf("guidance DoD = %+v", d)
	}
	if c := card(te.agent.board(t, ""), free.Key); c.DodMandatory || len(c.DodPending) != 1 {
		t.Fatalf("card = mandatory %v, pending %v", c.DodMandatory, c.DodPending)
	}
	te.fanout(t)
	resolved := decode[gen.Ticket](t, te.agent.setStatus(free.Key, gen.TicketStatusResolved, ""), http.StatusOK)
	if resolved.Status != gen.TicketStatusResolved {
		t.Fatalf("status = %s", resolved.Status)
	}
	if last := lastActivity(te.agent.activity(t, free.Key)); last.Cause != nil || last.Reason != nil {
		t.Fatalf("guidance resolve = %+v", last)
	}
	if n := supervisorNotices(free.Id); n != 0 {
		t.Fatalf("supervisor notices for a guidance DoD = %d", n)
	}

	// A reason sent anyway is dropped.
	other := te.requester.report(t, incident(te.projectA, "Mailbox full"))
	te.startWork(t, other.Key)
	decode[gen.Ticket](t, te.agent.setStatus(other.Key, gen.TicketStatusResolved, skipReason), http.StatusOK)
	if last := lastActivity(te.agent.activity(t, other.Key)); last.Cause != nil || last.Reason != nil {
		t.Fatalf("guidance resolve with a reason = %+v", last)
	}
	if n := supervisorNotices(other.Id); n != 0 {
		t.Fatalf("supervisor notices = %d", n)
	}

	// A ticket's own Definition of Done keeps the project's switch.
	own := te.requester.report(t, incident(te.projectA, "Login page slow"))
	confirm := dod.None()
	confirm.RequesterConfirms = true
	got := decode[gen.Ticket](t, te.agent.setDod(own.Key, &confirm, hotfixReason), http.StatusOK)
	if got.Dod.Source != gen.DodSourceTicket || got.Dod.Mandatory {
		t.Fatalf("own DoD = %+v", got.Dod)
	}
}

func lastActivity(a []gen.TicketActivity) gen.TicketActivity { return a[len(a)-1] }

// Code reaching the project's branch ticks the item and never moves the ticket.
func TestCodeReachedBranch(t *testing.T) {
	ce := newCodeEnv(t)
	p := ce.setProjectDod(t, ce.projectA, codeAndUAT(branchMain), true)
	if !p.Dod.CodeOnBranch.Enabled || p.Dod.CodeOnBranch.Branch != branchMain {
		t.Fatalf("saved = %+v", p.Dod)
	}
	tk := ce.requester.report(t, incident(ce.projectA, "Login fails"))
	ce.startWork(t, tk.Key)
	d := ce.agent.ticket(t, tk.Key).Dod
	if !d.HasRepository || len(d.Items) != 2 || d.Items[0].Item != gen.DodItemCodeOnBranch ||
		!slices.Equal(d.Pending, []gen.DodItem{gen.DodItemCodeOnBranch, gen.DodItemUatApproved}) {
		t.Fatalf("DoD = %+v", d)
	}
	ticks := func(id uuid.UUID) int {
		return ce.count(t, sqlCodeTicks, id, tickets.ActionDoDItemsDone, audit.ActorSystem, tickets.ActorNameCodeBranch)
	}
	fix := commitJSON(sha(1), "Fix "+tk.Key+": refresh the token", agentEmail, "2026-11-16T08:40:00+07:00")

	// The fix branch isn't the branch.
	ce.send(t, "push", pushJSON("fix/"+tk.Key, fix))
	if n := ce.count(t, sqlCodeItem, tk.Id); n != 0 {
		t.Fatalf("a push to the fix branch ticked %d items", n)
	}

	// The same commit merged into main ticks it, as Git, and the ticket stays in progress.
	ce.send(t, "push", pushJSON(branchMain, fix))
	got := ce.agent.ticket(t, tk.Key)
	item := got.Dod.Items[0]
	if got.Status != gen.TicketStatusInProgress || item.DoneAt == nil || item.DoneBy != nil || item.Commit == nil ||
		item.Commit.Sha != sha(1)[:shortSHALen] || item.Commit.Url == "" ||
		!slices.Equal(got.Dod.Pending, []gen.DodItem{gen.DodItemUatApproved}) {
		t.Fatalf("after the merge: %s, %+v", got.Status, got.Dod)
	}
	if n := ce.count(t, sqlCodeItem, tk.Id); n != 1 {
		t.Fatalf("code items = %d", n)
	}
	if n := ticks(tk.Id); n != 1 {
		t.Fatalf("system ticks = %d", n)
	}
	if c := card(ce.agent.board(t, ""), tk.Key); !slices.Equal(c.DodPending, []gen.DodItem{gen.DodItemUatApproved}) {
		t.Fatalf("card pending = %v", c.DodPending)
	}

	// The same push delivered again does nothing.
	ce.send(t, "push", pushJSON(branchMain, fix))
	if n := ticks(tk.Id); n != 1 {
		t.Fatalf("a redelivery ticked again: %d", n)
	}

	// Branch names match exactly.
	cased := ce.requester.report(t, incident(ce.projectA, "Archive search"))
	ce.send(t, "push", pushJSON("Main", commitJSON(sha(2), "refs "+cased.Key, agentEmail, "2026-11-16T09:00:00+07:00")))
	if n := ce.count(t, sqlCodeItem, cased.Id); n != 0 {
		t.Fatal("Main counted as main")
	}

	// A first push straight to main ticks too.
	direct := ce.requester.report(t, incident(ce.projectA, "Typo on the letterhead"))
	ce.send(t, "push", pushJSON(branchMain, commitJSON(sha(3), "Fix "+direct.Key, agentEmail, "2026-11-16T09:10:00+07:00")))
	if n := ce.count(t, sqlCodeItem, direct.Id); n != 1 || ce.agent.ticket(t, direct.Key).Status != gen.TicketStatusNew {
		t.Fatalf("direct push: items %d", n)
	}

	// A closed ticket isn't ticked.
	closed := ce.requester.report(t, incident(ce.projectA, "Old issue"))
	ce.exec(t, `UPDATE tickets SET status = 'closed', closed_at = now(), resolved_at = now() WHERE id = $1`, closed.Id)
	ce.send(t, "push", pushJSON(branchMain, commitJSON(sha(4), "refs "+closed.Key, agentEmail, "2026-11-16T09:20:00+07:00")))
	if n := ce.count(t, sqlCodeItem, closed.Id); n != 0 {
		t.Fatal("a closed ticket was ticked")
	}

	// The client rejects the test: the fix goes, and pushing the old commit to main again
	// doesn't bring it back. A new commit does.
	ce.move(t, tk.Key, gen.TicketStatusInUat)
	decode[gen.Ticket](t, ce.requester.answer(tk.Key, gen.UatAnswerReject, rejectMessage), http.StatusOK)
	if n := ce.count(t, `SELECT count(*) FROM ticket_dod_items WHERE ticket_id = $1`, tk.Id); n != 0 {
		t.Fatalf("items left after the rejection: %d", n)
	}
	ce.send(t, "push", pushJSON(branchMain, fix))
	if n := ce.count(t, sqlCodeItem, tk.Id); n != 0 {
		t.Fatal("the old commit ticked again")
	}
	ce.send(t, "push", pushJSON(branchMain, commitJSON(sha(5), "Fix "+tk.Key+": the real fix", agentEmail, "2026-11-16T10:00:00+07:00")))
	if got := ce.agent.ticket(t, tk.Key).Dod.Items[0]; got.Commit == nil || got.Commit.Sha != sha(5)[:shortSHALen] {
		t.Fatalf("after a new commit: %+v", got)
	}
}

// Turning the condition on re-checks commits already linked; a ticket's own Definition of Done is
// re-checked at once. Without a repository the condition can't be turned on and doesn't apply.
func TestCodeBranchSync(t *testing.T) {
	ce := newCodeEnv(t)
	merged := ce.requester.report(t, incident(ce.projectA, "Login fails"))
	branchOnly := ce.requester.report(t, incident(ce.projectA, "Archive search"))
	own := ce.requester.report(t, incident(ce.projectA, "Letterhead"))
	ce.send(t, "push", pushJSON(branchMain,
		commitJSON(sha(1), "Fix "+merged.Key, agentEmail, "2026-11-16T08:40:00+07:00"),
		commitJSON(sha(3), "Fix "+own.Key, agentEmail, "2026-11-16T08:45:00+07:00")))
	ce.send(t, "push", pushJSON("fix/x", commitJSON(sha(2), "refs "+branchOnly.Key, agentEmail, "2026-11-16T08:50:00+07:00")))
	if n := ce.count(t, `SELECT count(*) FROM ticket_dod_items`); n != 0 {
		t.Fatalf("items before the condition: %d", n)
	}

	// A ticket's own Definition of Done asking for main ticks from its linked commit right away.
	decode[gen.Ticket](t, ce.agent.setDod(own.Key, new(codeAndUAT(" refs/heads/main ")), hotfixReason), http.StatusOK)
	if got := ce.agent.ticket(t, own.Key).Dod; got.Config.CodeOnBranch.Branch != branchMain || got.Items[0].DoneAt == nil {
		t.Fatalf("own DoD = %+v", got)
	}

	ce.setProjectDod(t, ce.projectA, codeAndUAT(branchMain), true)
	if n := runJobs(t, ce.env, ce.branchSync); n != 1 {
		t.Fatalf("sync jobs = %d", n)
	}
	if n := ce.count(t, sqlCodeItem, merged.Id); n != 1 {
		t.Fatal("the merged ticket wasn't ticked")
	}
	if n := ce.count(t, sqlCodeItem, branchOnly.Id); n != 0 {
		t.Fatal("a commit on another branch ticked")
	}
	// Saving the same again queues nothing; running it twice changes nothing.
	ce.setProjectDod(t, ce.projectA, codeAndUAT(branchMain), true)
	if n := runJobs(t, ce.env, ce.branchSync); n != 0 {
		t.Fatalf("unchanged save queued %d jobs", n)
	}
	if n := ce.count(t, sqlCodeTicks, merged.Id, tickets.ActionDoDItemsDone, audit.ActorSystem, tickets.ActorNameCodeBranch); n != 1 {
		t.Fatalf("ticks = %d", n)
	}

	// Errors: a bad branch, and the condition without a repository.
	bad := apiDod(codeAndUAT("fix me"))
	expectProblem(t, ce.supervisor.do(http.MethodPut, projectDodPath(ce.projectA),
		gen.ProjectDodInput{Dod: bad, Mandatory: true}), http.StatusBadRequest, apperr.CodeInvalidBranch)
	empty := apiDod(codeAndUAT(""))
	expectProblem(t, ce.supervisor.do(http.MethodPut, projectDodPath(ce.projectA),
		gen.ProjectDodInput{Dod: empty, Mandatory: true}), http.StatusBadRequest, apperr.CodeInvalidBranch)
	expectProblem(t, ce.supervisor.do(http.MethodPut, projectDodPath(ce.projectA2),
		gen.ProjectDodInput{Dod: apiDod(codeAndUAT(branchMain)), Mandatory: true}), http.StatusBadRequest, apperr.CodeInvalidDoD)
	expectProblem(t, ce.agent.setDod(ce.requester.report(t, incident(ce.projectA2, "Mailbox")).Key, new(codeAndUAT(branchMain)), hotfixReason),
		http.StatusBadRequest, apperr.CodeInvalidDoD)

	// A project that loses its repository stops asking for code.
	testdb.SetProjectDoD(t, ce.st, ce.projectA2, codeAndUAT(branchMain))
	noRepo := ce.requester.report(t, incident(ce.projectA2, "Mailbox full"))
	if d := ce.agent.ticket(t, noRepo.Key).Dod; d.HasRepository || !d.Config.CodeOnBranch.Enabled ||
		!slices.Equal(d.Pending, []gen.DodItem{gen.DodItemUatApproved}) || len(d.Items) != 1 {
		t.Fatalf("DoD without a repository = %+v", d)
	}
}
