package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

// progressStep is one step of the requester's tracker.
func progressStep(t *testing.T, tk gen.Ticket, step gen.TicketProgressStep) gen.TicketProgress {
	t.Helper()
	i := slices.IndexFunc(tk.Progress, func(p gen.TicketProgress) bool { return p.Step == step })
	if i < 0 {
		t.Fatalf("no %s step in %+v", step, tk.Progress)
	}
	return tk.Progress[i]
}

func (s session) assign(key string, id *uuid.UUID) *httptest.ResponseRecorder {
	return s.do(http.MethodPut, "/tickets/"+key+"/assignee", gen.TicketAssigneeInput{AssigneeId: id})
}

// A ticket moved to Waiting on client with nobody on it never passes Assigned; assigning it there
// still marks the Assigned step done on the portal (#11).
func TestAssigningAWaitingTicketShowsOnTheTracker(t *testing.T) {
	te := newTicketEnv(t)
	tk := te.requester.report(t, incident(te.projectA, "Scanner driver missing"))
	te.move(t, tk.Key, gen.TicketStatusWaitingOnClient)

	before := te.requester.ticket(t, tk.Key)
	if p := progressStep(t, before, gen.TicketProgressStepAssigned); p.State != gen.TicketProgressStateSkipped || p.At != nil {
		t.Fatalf("assigned step before = %+v", p)
	}

	decode[gen.Ticket](t, te.supervisor.assign(tk.Key, &te.agentID), http.StatusOK)
	after := te.requester.ticket(t, tk.Key)
	if after.Status != gen.TicketStatusWaitingOnClient {
		t.Fatalf("status after assigning = %s", after.Status)
	}
	if p := progressStep(t, after, gen.TicketProgressStepAssigned); p.State != gen.TicketProgressStateDone || p.At == nil {
		t.Fatalf("assigned step after = %+v", p)
	}
	if p := progressStep(t, after, gen.TicketProgressStepInProgress); p.State != gen.TicketProgressStateCurrent {
		t.Fatalf("in progress step = %+v", p)
	}
}

// A requester's reply hands a ticket waiting on them back to its assignee, or to the New queue
// when nobody is on it, never to an "In progress" nobody works.
func TestRequesterReplyResumesTheTicket(t *testing.T) {
	te := newTicketEnv(t)
	reply := func(key string) {
		t.Helper()
		decode[gen.TicketMessage](t, te.requester.message(key, gen.MessageVisibilityPublic, "Here is the screenshot."), http.StatusCreated)
	}

	alone := te.requester.report(t, incident(te.projectA, "Scanner driver missing"))
	te.move(t, alone.Key, gen.TicketStatusWaitingOnClient)
	reply(alone.Key)
	got := te.agent.ticket(t, alone.Key)
	if got.Status != gen.TicketStatusNew || got.Assignee != nil {
		t.Fatalf("unassigned after the reply: %s %+v", got.Status, got.Assignee)
	}
	activity := te.agent.activity(t, alone.Key)
	if last := activity[len(activity)-1]; last.Cause == nil || *last.Cause != gen.TicketStatusCauseRequesterReply || *last.To != string(gen.TicketStatusNew) {
		t.Fatalf("last activity = %+v", last)
	}

	worked := te.requester.report(t, incident(te.projectA, "Mailbox full"))
	decode[gen.Ticket](t, te.supervisor.assign(worked.Key, &te.agentID), http.StatusOK)
	te.move(t, worked.Key, gen.TicketStatusWaitingOnClient)
	reply(worked.Key)
	if s := te.agent.ticket(t, worked.Key).Status; s != gen.TicketStatusInProgress {
		t.Fatalf("assigned after the reply: %s", s)
	}
}

// Agents take unassigned tickets and release their own; only supervisors and administrators give
// a ticket to someone else (owner, 18 Nov).
func TestWhoMayAssign(t *testing.T) {
	te := newTicketEnv(t)
	colleague, err := te.st.Q().GetUserByEmail(context.Background(), otherAgentEmail)
	if err != nil {
		t.Fatal(err)
	}
	colleagueID := colleague.ID
	decode[gen.Client](t, te.admin.do(http.MethodPut, "/clients/"+te.clientA.String()+"/agents",
		gen.ClientAgentsInput{UserIds: []uuid.UUID{te.agentID, colleagueID}}), http.StatusOK)
	tk := te.requester.report(t, incident(te.projectA, "Network down"))
	notAllowed := func(rec *httptest.ResponseRecorder) {
		t.Helper()
		expectProblem(t, rec, http.StatusForbidden, apperr.CodeAssigneeNotAllowed)
	}

	// The agent takes the unassigned ticket and releases it, but can't give it to a colleague.
	if got := decode[gen.Ticket](t, te.agent.assign(tk.Key, &te.agentID), http.StatusOK); got.Assignee == nil || got.Assignee.Id != te.agentID {
		t.Fatalf("taken: %+v", got.Assignee)
	}
	notAllowed(te.agent.assign(tk.Key, &colleagueID))
	if got := decode[gen.Ticket](t, te.agent.assign(tk.Key, nil), http.StatusOK); got.Assignee != nil {
		t.Fatalf("released: %+v", got.Assignee)
	}
	notAllowed(te.agent.assign(tk.Key, &colleagueID))

	// On a colleague's ticket the agent can't take it over, unassign it or pass it on.
	decode[gen.Ticket](t, te.otherAgent.assign(tk.Key, &colleagueID), http.StatusOK)
	notAllowed(te.agent.assign(tk.Key, &te.agentID))
	notAllowed(te.agent.assign(tk.Key, nil))
	notAllowed(te.otherAgent.assign(tk.Key, &te.agentID))
	// The same assignee again changes nothing, so there is nothing to refuse.
	decode[gen.Ticket](t, te.agent.assign(tk.Key, &colleagueID), http.StatusOK)

	const changed = "ticket.assignee_changed"
	want := []string{"ticket.created", changed, changed, changed} // taken, released, the colleague's

	// Supervisors and administrators assign anyone who works the client.
	for _, s := range []session{te.supervisor, te.admin} {
		if got := decode[gen.Ticket](t, s.assign(tk.Key, &te.agentID), http.StatusOK); got.Assignee == nil || got.Assignee.Id != te.agentID {
			t.Fatalf("supervisor or admin: %+v", got.Assignee)
		}
		decode[gen.Ticket](t, s.assign(tk.Key, &colleagueID), http.StatusOK)
		decode[gen.Ticket](t, s.assign(tk.Key, nil), http.StatusOK)
		want = append(want, changed, changed, changed)
	}

	// Refused changes leave no trace in the audit log.
	var got []string
	for _, a := range te.auditActions(t, tk.Id) {
		if a != "ticket.status_changed" {
			got = append(got, a)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("audit =\n%v\nwant\n%v", got, want)
	}
}
