package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

const (
	skipReason    = "Client confirmed by phone; UAT done on their side"
	hotfixReason  = "P1 outage, hotfix approved by the client PIC by phone"
	sqlTicketID   = `SELECT id FROM tickets WHERE key = $1`
	sqlTickUAT    = `INSERT INTO ticket_dod_items (ticket_id, item) SELECT id, 'uat_approved' FROM tickets WHERE key = $1`
	sqlDoDItems   = `SELECT count(*) FROM ticket_dod_items i JOIN tickets t ON t.id = i.ticket_id WHERE t.key = $1`
	sqlNoOverride = `UPDATE projects SET allow_ticket_override = false WHERE id = $1`
)

// uatDoD asks the client to test, approved automatically after 3 working days.
func uatDoD() dod.Config {
	c := dod.None()
	c.UAT.Enabled = true
	return c
}

func (s session) setStatus(key string, to gen.TicketStatus, reason string) *httptest.ResponseRecorder {
	in := gen.TicketStatusInput{Status: to}
	if reason != "" {
		in.Reason = &reason
	}
	return s.do(http.MethodPut, "/tickets/"+key+"/status", in)
}

func (s session) setDod(key string, c *dod.Config, reason string) *httptest.ResponseRecorder {
	in := gen.TicketDodInput{}
	if c != nil {
		api := apiDod(*c)
		in.Dod = &api
	}
	if reason != "" {
		in.Reason = &reason
	}
	return s.do(http.MethodPut, "/tickets/"+key+"/dod", in)
}

func (te *ticketEnv) ticketID(t *testing.T, key string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := te.st.Pool().QueryRow(context.Background(), sqlTicketID, key).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (s session) activity(t *testing.T, key string) []gen.TicketActivity {
	t.Helper()
	return decode[[]gen.TicketActivity](t, s.do(http.MethodGet, "/tickets/"+key+"/activity", nil), http.StatusOK)
}

func TestResolveGuard(t *testing.T) {
	te := newTicketEnv(t)
	testdb.SetProjectDoD(t, te.st, te.projectA, uatDoD())
	tk := te.requester.report(t, incident(te.projectA, "Disposition letter shows the wrong date"))
	decode[gen.Ticket](t, te.agent.do(http.MethodPut, "/tickets/"+tk.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	te.move(t, tk.Key, gen.TicketStatusInProgress)
	id := te.ticketID(t, tk.Key)

	got := te.agent.ticket(t, tk.Key)
	d := got.Dod
	if d == nil || d.Source != gen.DodSourceProject || d.Met || !d.CanOverride ||
		!slices.Equal(d.Pending, []gen.DodItem{gen.DodItemUatApproved}) || len(d.Items) != 1 || d.Items[0].DoneAt != nil {
		t.Fatalf("DoD = %+v", d)
	}
	if card := card(te.agent.board(t, ""), tk.Key); !slices.Equal(card.DodPending, []gen.DodItem{gen.DodItemUatApproved}) {
		t.Fatalf("card pending = %v", card.DodPending)
	}
	if te.requester.ticket(t, tk.Key).Dod != nil {
		t.Fatal("the requester sees the Definition of Done")
	}

	// Without a reason nothing changes.
	before := te.auditActions(t, id)
	expectProblem(t, te.supervisor.setStatus(tk.Key, gen.TicketStatusResolved, ""), http.StatusConflict, apperr.CodeDoDUnmet)
	expectProblem(t, te.supervisor.setStatus(tk.Key, gen.TicketStatusResolved, "   "), http.StatusConflict, apperr.CodeDoDUnmet)
	if after := te.auditActions(t, id); !slices.Equal(before, after) || te.agent.ticket(t, tk.Key).Status != gen.TicketStatusInProgress {
		t.Fatalf("a refused resolve changed the ticket: %v", after)
	}

	// With one it resolves, and the reason stays with the team.
	te.fanout(t)
	resolved := decode[gen.Ticket](t, te.supervisor.setStatus(tk.Key, gen.TicketStatusResolved, skipReason), http.StatusOK)
	if resolved.Status != gen.TicketStatusResolved {
		t.Fatalf("status = %s", resolved.Status)
	}
	activity := te.agent.activity(t, tk.Key)
	last := activity[len(activity)-1]
	if last.Cause == nil || *last.Cause != gen.TicketStatusCauseDodSkipped || last.Reason == nil || *last.Reason != skipReason {
		t.Fatalf("activity = %+v", last)
	}
	te.fanout(t)
	reasons := map[uuid.UUID]*string{}
	rows, err := te.st.Pool().Query(context.Background(),
		`SELECT user_id, data->>'reason' FROM notifications WHERE ticket_id = $1 AND data->>'status_to' = 'resolved'`, id)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var u uuid.UUID
		var r *string
		if err := rows.Scan(&u, &r); err != nil {
			t.Fatal(err)
		}
		reasons[u] = r
	}
	rows.Close()
	if r, ok := reasons[te.requesterID]; !ok || r != nil {
		t.Errorf("requester notification reason = %v (sent %v)", r, ok)
	}
	if r := reasons[te.agentID]; r == nil || *r != skipReason {
		t.Errorf("assignee notification reason = %v", r)
	}

	// A met Definition of Done needs no reason.
	other := te.requester.report(t, incident(te.projectA2, "Mailbox full"))
	te.move(t, other.Key, gen.TicketStatusInProgress)
	decode[gen.Ticket](t, te.agent.setStatus(other.Key, gen.TicketStatusResolved, ""), http.StatusOK)
}

func TestTicketDoDOverride(t *testing.T) {
	te := newTicketEnv(t)
	testdb.SetProjectDoD(t, te.st, te.projectA, uatDoD())
	tk := te.requester.report(t, incident(te.projectA, "e-Office login fails for all staff"))
	hotfix := dod.None()

	expectProblem(t, te.agent.setDod(tk.Key, &hotfix, ""), http.StatusBadRequest, apperr.CodeReasonRequired)
	code := dod.None()
	code.Merged.Enabled = true
	expectProblem(t, te.agent.setDod(tk.Key, &code, hotfixReason), http.StatusBadRequest, apperr.CodeInvalidDoD)

	got := decode[gen.Ticket](t, te.agent.setDod(tk.Key, &hotfix, hotfixReason), http.StatusOK)
	if d := got.Dod; d.Source != gen.DodSourceTicket || !d.Met || len(d.Items) != 0 || d.OverrideReason == nil ||
		*d.OverrideReason != hotfixReason || d.ProjectConfig != apiDod(uatDoD()) {
		t.Fatalf("custom DoD = %+v", d)
	}
	activity := te.agent.activity(t, tk.Key)
	last := activity[len(activity)-1]
	if last.Action != gen.TicketActivityActionDodChanged || *last.From != string(dod.SourceProject) ||
		*last.To != string(dod.SourceTicket) || *last.Reason != hotfixReason {
		t.Fatalf("activity = %+v", last)
	}
	// The ticket's own Definition of Done has no UAT, so it resolves without a reason.
	te.move(t, tk.Key, gen.TicketStatusInProgress)
	if card := card(te.agent.board(t, ""), tk.Key); len(card.DodPending) != 0 {
		t.Fatalf("card pending = %v", card.DodPending)
	}

	// Back to the project's.
	got = decode[gen.Ticket](t, te.agent.setDod(tk.Key, nil, ""), http.StatusOK)
	if got.Dod.Source != gen.DodSourceProject || got.Dod.OverrideReason != nil || got.Dod.Met {
		t.Fatalf("reset DoD = %+v", got.Dod)
	}
	id := te.ticketID(t, tk.Key)
	n := len(te.auditActions(t, id))
	decode[gen.Ticket](t, te.agent.setDod(tk.Key, nil, ""), http.StatusOK)
	if len(te.auditActions(t, id)) != n {
		t.Fatal("resetting twice wrote an audit entry")
	}

	// A project can keep agents from it; supervisors still may.
	te.exec(t, sqlNoOverride, te.projectA)
	if te.agent.ticket(t, tk.Key).Dod.CanOverride || !te.supervisor.ticket(t, tk.Key).Dod.CanOverride {
		t.Fatal("canOverride ignores the project's switch")
	}
	expectProblem(t, te.agent.setDod(tk.Key, &hotfix, hotfixReason), http.StatusForbidden, apperr.CodeDoDOverrideNotAllowed)
	decode[gen.Ticket](t, te.supervisor.setDod(tk.Key, &hotfix, hotfixReason), http.StatusOK)

	expectProblem(t, te.requester.setDod(tk.Key, nil, ""), http.StatusForbidden, apperr.CodeForbidden)
	if !slices.Contains(te.auditActions(t, id), tickets.ActionDoDChanged) {
		t.Fatal("no audit entry")
	}
}

// Reopening a resolved ticket forgets the client's answer, so the next resolution asks again.
func TestReopenClearsClientItems(t *testing.T) {
	te := newTicketEnv(t)
	testdb.SetProjectDoD(t, te.st, te.projectA, uatDoD())
	tk := te.requester.report(t, incident(te.projectA, "Archive search is slow"))
	te.move(t, tk.Key, gen.TicketStatusInProgress)
	decode[gen.Ticket](t, te.agent.setStatus(tk.Key, gen.TicketStatusResolved, skipReason), http.StatusOK)
	te.exec(t, sqlTickUAT, tk.Key)
	if d := te.agent.ticket(t, tk.Key).Dod; !d.Met || d.Items[0].DoneAt == nil {
		t.Fatalf("ticked DoD = %+v", d)
	}

	te.move(t, tk.Key, gen.TicketStatusInProgress)
	var n int
	if err := te.st.Pool().QueryRow(context.Background(), sqlDoDItems, tk.Key).Scan(&n); err != nil || n != 0 {
		t.Fatalf("items left: %d, %v", n, err)
	}
	if !slices.Contains(te.auditActions(t, te.ticketID(t, tk.Key)), tickets.ActionDoDItemsCleared) {
		t.Fatal("clearing the items wasn't audited")
	}
	if d := te.agent.ticket(t, tk.Key).Dod; d.Met {
		t.Fatalf("DoD after reopening = %+v", d)
	}
}
