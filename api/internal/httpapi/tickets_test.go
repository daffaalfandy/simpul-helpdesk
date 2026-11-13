package httpapi_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

const (
	otherClient         = "BPKAD Kota Batam"
	otherRequesterEmail = "requester2@example.test"
	otherAgentEmail     = "agent2@example.test"
)

// pngBytes starts like a PNG, which is all the content check looks at.
var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01")

// ticketEnv has two clients: A (the requester's, worked by the agent) and B (another
// requester's, worked by nobody), plus an agent assigned to no client.
type ticketEnv struct {
	*env
	admin, supervisor, agent, requester session
	otherAgent, otherRequester          session
	clientA, clientB                    uuid.UUID
	agentID, requesterID, otherReqID    uuid.UUID
}

func newTicketEnv(t *testing.T) *ticketEnv {
	t.Helper()
	ctx := context.Background()
	e := newEnv(t)
	te := &ticketEnv{env: e}
	a, err := e.st.Q().GetClientByName(ctx, requesterClient)
	if err != nil {
		t.Fatal(err)
	}
	te.clientA = a.ID
	if te.clientB, err = e.st.Q().UpsertClient(ctx, otherClient); err != nil {
		t.Fatal(err)
	}
	for _, u := range []auth.NewUser{
		{Email: otherAgentEmail, Name: "Unlinked Agent", Password: password, Role: auth.RoleAgent},
		{Email: otherRequesterEmail, Name: "Sri Lestari", Password: password, Role: auth.RoleRequester, ClientName: otherClient},
	} {
		if _, err := e.auth.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	te.admin, te.supervisor = e.as(auth.RoleAdmin), e.as(auth.RoleSupervisor)
	te.agent, te.requester = e.as(auth.RoleAgent), e.as(auth.RoleRequester)
	te.otherAgent = session{e: e, cookie: sessionCookie(t, e.login(otherAgentEmail, password))}
	te.otherRequester = session{e: e, cookie: sessionCookie(t, e.login(otherRequesterEmail, password))}
	te.agentID, te.requesterID = userID(t, e, auth.RoleAgent), userID(t, e, auth.RoleRequester)
	other, err := e.st.Q().GetUserByEmail(ctx, otherRequesterEmail)
	if err != nil {
		t.Fatal(err)
	}
	te.otherReqID = other.ID
	rec := te.admin.do(http.MethodPut, "/clients/"+te.clientA.String()+"/agents", gen.ClientAgentsInput{UserIds: []uuid.UUID{te.agentID}})
	decode[gen.Client](t, rec, http.StatusOK)
	return te
}

func incident(subject string) gen.TicketInput {
	return gen.TicketInput{Type: gen.TicketTypeIncident, Priority: gen.PriorityP3, Subject: subject, Description: "It broke."}
}

// report opens a ticket from the portal.
func (s session) report(t *testing.T, in gen.TicketInput) gen.Ticket {
	t.Helper()
	return decode[gen.Ticket](t, s.do(http.MethodPost, "/tickets", in), http.StatusCreated)
}

// logFor opens a ticket as staff for a requester of the client.
func (s session) logFor(t *testing.T, client, requester uuid.UUID, in gen.TicketInput) gen.Ticket {
	t.Helper()
	phone := gen.TicketChannelPhone
	in.ClientId, in.RequesterId, in.Channel = &client, &requester, &phone
	return s.report(t, in)
}

func (s session) uploadFile(name string, content []byte) *httptest.ResponseRecorder {
	s.e.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", name)
	if err != nil {
		s.e.t.Fatal(err)
	}
	_, _ = fw.Write(content)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, apiBase+"/attachments", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(s.cookie)
	rec := httptest.NewRecorder()
	s.e.handler.ServeHTTP(rec, req)
	return rec
}

func (s session) attach(t *testing.T, name string) gen.Attachment {
	t.Helper()
	return decode[gen.Attachment](t, s.uploadFile(name, pngBytes), http.StatusCreated)
}

func (s session) message(key string, vis gen.MessageVisibility, body string, files ...uuid.UUID) *httptest.ResponseRecorder {
	in := gen.TicketMessageInput{Visibility: vis, Body: body}
	if len(files) > 0 {
		in.AttachmentIds = &files
	}
	return s.do(http.MethodPost, "/tickets/"+key+"/messages", in)
}

func (s session) messages(t *testing.T, key string) []gen.TicketMessage {
	t.Helper()
	return decode[[]gen.TicketMessage](t, s.do(http.MethodGet, "/tickets/"+key+"/messages", nil), http.StatusOK)
}

func (s session) ticket(t *testing.T, key string) gen.Ticket {
	t.Helper()
	return decode[gen.Ticket](t, s.do(http.MethodGet, "/tickets/"+key, nil), http.StatusOK)
}

func (s session) keys(t *testing.T, query string) []string {
	t.Helper()
	list := decode[[]gen.TicketSummary](t, s.do(http.MethodGet, "/tickets"+query, nil), http.StatusOK)
	out := make([]string, len(list))
	for i, tk := range list {
		out[i] = tk.Key
	}
	return out
}

// auditActions lists the actions recorded against a ticket, oldest first.
func (e *env) auditActions(t *testing.T, ticketID uuid.UUID) []string {
	t.Helper()
	rows, err := e.st.Pool().Query(context.Background(),
		`SELECT action FROM audit_log WHERE target_type = 'ticket' AND target_id = $1 ORDER BY at, id`, ticketID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		_ = rows.Scan(&a)
		out = append(out, a)
	}
	return out
}

// scopedOps are the operations that act on one ticket or file, with the status each outsider
// gets. 404 hides the ticket; 403 means the role may never do this.
var scopedOps = map[string]struct{ otherRequester, otherAgent int }{
	"getTicket":           {http.StatusNotFound, http.StatusNotFound},
	"listTicketMessages":  {http.StatusNotFound, http.StatusNotFound},
	"createTicketMessage": {http.StatusNotFound, http.StatusNotFound},
	"setTicketStatus":     {http.StatusForbidden, http.StatusNotFound},
	"setTicketAssignee":   {http.StatusForbidden, http.StatusNotFound},
	"setTicketPriority":   {http.StatusForbidden, http.StatusNotFound},
	"listTicketActivity":  {http.StatusForbidden, http.StatusNotFound},
	"downloadAttachment":  {http.StatusNotFound, http.StatusNotFound},
}

func TestTicketScope(t *testing.T) {
	te := newTicketEnv(t)
	file := te.requester.attach(t, "screen.png")
	in := incident("Login fails")
	in.AttachmentIds = &[]uuid.UUID{file.Id}
	tk := te.requester.report(t, in)
	path := "/tickets/" + tk.Key

	calls := map[string]func(s session) *httptest.ResponseRecorder{
		"getTicket":          func(s session) *httptest.ResponseRecorder { return s.do(http.MethodGet, path, nil) },
		"listTicketMessages": func(s session) *httptest.ResponseRecorder { return s.do(http.MethodGet, path+"/messages", nil) },
		"createTicketMessage": func(s session) *httptest.ResponseRecorder {
			return s.message(tk.Key, gen.MessageVisibilityPublic, "hello")
		},
		"setTicketStatus": func(s session) *httptest.ResponseRecorder {
			return s.do(http.MethodPut, path+"/status", gen.TicketStatusInput{Status: gen.TicketStatusInProgress})
		},
		"setTicketAssignee": func(s session) *httptest.ResponseRecorder {
			return s.do(http.MethodPut, path+"/assignee", gen.TicketAssigneeInput{AssigneeId: nil})
		},
		"setTicketPriority": func(s session) *httptest.ResponseRecorder {
			return s.do(http.MethodPut, path+"/priority", gen.TicketPriorityInput{Priority: gen.PriorityP1, Reason: "x"})
		},
		"listTicketActivity": func(s session) *httptest.ResponseRecorder { return s.do(http.MethodGet, path+"/activity", nil) },
		"downloadAttachment": func(s session) *httptest.ResponseRecorder {
			return s.do(http.MethodGet, "/attachments/"+file.Id.String(), nil)
		},
	}

	// Every operation on one ticket or file must be in the matrix.
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	for p, item := range spec.Paths.Map() {
		if !strings.Contains(p, "{ticketKey}") && !strings.Contains(p, "{attachmentId}") {
			continue
		}
		for _, op := range item.Operations() {
			id := strings.ToLower(op.OperationID[:1]) + op.OperationID[1:]
			if _, ok := scopedOps[id]; !ok {
				t.Errorf("%s is not in the scope matrix", id)
			}
		}
	}

	for op, want := range scopedOps {
		if rec := calls[op](te.otherRequester); rec.Code != want.otherRequester {
			t.Errorf("%s as requester of another client: %d, want %d", op, rec.Code, want.otherRequester)
		}
		if rec := calls[op](te.otherAgent); rec.Code != want.otherAgent {
			t.Errorf("%s as unassigned agent: %d, want %d", op, rec.Code, want.otherAgent)
		}
	}
	for name, s := range map[string]session{"other requester": te.otherRequester, "other agent": te.otherAgent} {
		if slices.Contains(s.keys(t, ""), tk.Key) {
			t.Errorf("%s sees %s in the list", name, tk.Key)
		}
	}
	for _, s := range []session{te.agent, te.supervisor, te.admin, te.requester} {
		if !slices.Contains(s.keys(t, ""), tk.Key) {
			t.Errorf("an allowed user doesn't see %s", tk.Key)
		}
	}

	clientA := "/clients/" + te.clientA.String()
	for _, p := range []string{"/intake?clientId=" + te.clientA.String(), clientA + "/requesters", clientA + "/assignees"} {
		if rec := te.otherAgent.do(http.MethodGet, p, nil); rec.Code != http.StatusNotFound {
			t.Errorf("unassigned agent GET %s: %d", p, rec.Code)
		}
	}
	for _, p := range []string{clientA + "/requesters", clientA + "/assignees"} {
		if rec := te.requester.do(http.MethodGet, p, nil); rec.Code != http.StatusForbidden {
			t.Errorf("requester GET %s: %d", p, rec.Code)
		}
	}
	rec := te.otherAgent.do(http.MethodPost, "/tickets", func() gen.TicketInput {
		in := incident("Not mine")
		phone := gen.TicketChannelPhone
		in.ClientId, in.RequesterId, in.Channel = &te.clientA, &te.requesterID, &phone
		return in
	}())
	expectProblem(t, rec, http.StatusBadRequest, "client_not_found")

	// Unlinking the agent takes their access away at once.
	decode[gen.Client](t, te.admin.do(http.MethodPut, clientA+"/agents", gen.ClientAgentsInput{UserIds: []uuid.UUID{}}), http.StatusOK)
	if rec := te.agent.do(http.MethodGet, path, nil); rec.Code != http.StatusNotFound {
		t.Errorf("unlinked agent: %d", rec.Code)
	}
}

func TestInternalNotesNeverReachRequesters(t *testing.T) {
	te := newTicketEnv(t)
	tk := te.requester.report(t, incident("Printer jam"))
	before := te.requester.ticket(t, tk.Key)
	listBefore := te.requester.keys(t, "")

	note := te.agent.attach(t, "plan.png")
	rec := te.agent.message(tk.Key, gen.MessageVisibilityInternal, "Rollback plan: restore yesterday's image.", note.Id)
	decode[gen.TicketMessage](t, rec, http.StatusCreated)

	for _, m := range te.requester.messages(t, tk.Key) {
		if m.Visibility == gen.MessageVisibilityInternal || strings.Contains(m.Body, "Rollback") {
			t.Fatalf("requester sees internal note %+v", m)
		}
	}
	if got := len(te.agent.messages(t, tk.Key)); got != 1 {
		t.Fatalf("agent sees %d messages", got)
	}
	if rec := te.requester.do(http.MethodGet, "/attachments/"+note.Id.String(), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("requester downloads a note's file: %d", rec.Code)
	}
	if rec := te.agent.do(http.MethodGet, "/attachments/"+note.Id.String(), nil); rec.Code != http.StatusOK {
		t.Fatalf("agent download: %d", rec.Code)
	}
	after := te.requester.ticket(t, tk.Key)
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("internal note moved updatedAt from %v to %v", before.UpdatedAt, after.UpdatedAt)
	}
	if !slices.Equal(listBefore, te.requester.keys(t, "")) {
		t.Fatal("the requester's list changed")
	}
	if rec := te.requester.message(tk.Key, gen.MessageVisibilityInternal, "sneaky"); rec.Code != http.StatusForbidden {
		t.Fatalf("requester posting internal: %d", rec.Code)
	}
}

func TestCreateTickets(t *testing.T) {
	ctx := context.Background()
	te := newTicketEnv(t)
	plan := defaultPlan(t, te.admin)

	file := te.requester.attach(t, "error.png")
	in := incident("e-Office login fails")
	in.AttachmentIds = &[]uuid.UUID{file.Id}
	// Staff-only fields are ignored for requesters.
	in.ClientId, in.RequesterId = &te.clientB, &te.otherReqID
	tk := te.requester.report(t, in)
	if !strings.HasPrefix(tk.Key, "HD-") || tk.Status != gen.TicketStatusNew || tk.Channel != gen.TicketChannelPortal ||
		tk.Client.Id != te.clientA || tk.Requester.Id != te.requesterID || tk.LoggedByStaff {
		t.Fatalf("portal ticket = %+v", tk)
	}
	if len(tk.Attachments) != 1 || tk.Attachments[0].Id != file.Id {
		t.Fatalf("attachments = %+v", tk.Attachments)
	}
	if len(tk.SlaTargets) != len(plan.Targets) || tk.Sla.ResolutionMinutes != plan.Targets[2].ResolveMinutes {
		t.Fatalf("sla = %+v", tk.Sla)
	}
	if len(tk.AllowedStatuses) != 0 || tk.Progress[0].State != gen.TicketProgressStateCurrent {
		t.Fatalf("requester view: allowed %v progress %+v", tk.AllowedStatuses, tk.Progress)
	}
	if got := te.auditActions(t, tk.Id); !slices.Equal(got, []string{"ticket.created"}) {
		t.Fatalf("audit = %v", got)
	}

	// A catalogue item's delivery time becomes the P3 request target.
	item := decode[gen.CatalogItem](t, te.admin.do(http.MethodPost, "/catalog-items", gen.CatalogItemInput{
		Name: "VPN / remote access", DeliveryMinutes: 1080, SortOrder: 10, Active: true,
	}), http.StatusCreated)
	req := gen.TicketInput{Type: gen.TicketTypeRequest, Priority: gen.PriorityP3, Subject: "VPN", Description: "Please", CatalogItemId: &item.Id}
	rt := te.requester.report(t, req)
	if !rt.Sla.FromCatalog || rt.Sla.ResolutionMinutes != 1080 || rt.CatalogItem == nil {
		t.Fatalf("request sla = %+v", rt.Sla)
	}

	// The snapshot is frozen: editing the plan later doesn't change the ticket.
	edited := planInput(plan.Name)
	decode[gen.SlaPlan](t, te.admin.do(http.MethodPut, "/sla-plans/"+plan.Id.String(), edited), http.StatusOK)
	if got := te.agent.ticket(t, tk.Key).Sla; got != tk.Sla {
		t.Fatalf("SLA after plan edit = %+v, want %+v", got, tk.Sla)
	}

	// Agents log for a requester, and can take it straight away.
	logged := incident("Called in")
	logged.AssignToMe = new(bool)
	*logged.AssignToMe = true
	lt := te.agent.logFor(t, te.clientA, te.requesterID, logged)
	if lt.Status != gen.TicketStatusAssigned || lt.Assignee == nil || lt.Assignee.Id != te.agentID ||
		lt.Channel != gen.TicketChannelPhone || !lt.LoggedByStaff || lt.CreatedBy.Id != te.agentID {
		t.Fatalf("logged ticket = %+v", lt)
	}

	cases := map[string]struct {
		s    session
		in   func() gen.TicketInput
		code string
	}{
		"blank subject": {te.requester, func() gen.TicketInput { return incident("   ") }, "invalid_subject"},
		"system on a request": {te.requester, func() gen.TicketInput {
			in := req
			in.SystemId, in.CatalogItemId = &item.Id, nil
			return in
		}, "invalid_system"},
		"unknown system": {te.requester, func() gen.TicketInput {
			in := incident("x")
			id := uuid.New()
			in.SystemId = &id
			return in
		}, "invalid_system"},
		"staff without client": {te.agent, func() gen.TicketInput { return incident("x") }, "client_required"},
		"portal channel by staff": {te.agent, func() gen.TicketInput {
			in := incident("x")
			portal := gen.TicketChannelPortal
			in.ClientId, in.RequesterId, in.Channel = &te.clientA, &te.requesterID, &portal
			return in
		}, "invalid_channel"},
		"requester of another client": {te.admin, func() gen.TicketInput {
			in := incident("x")
			phone := gen.TicketChannelPhone
			in.ClientId, in.RequesterId, in.Channel = &te.clientA, &te.otherReqID, &phone
			return in
		}, "invalid_requester"},
		"someone else's file": {te.requester, func() gen.TicketInput {
			in := incident("x")
			other := te.agent.attach(t, "theirs.png")
			in.AttachmentIds = &[]uuid.UUID{other.Id}
			return in
		}, "invalid_attachment"},
		"file already used": {te.requester, func() gen.TicketInput {
			in := incident("x")
			in.AttachmentIds = &[]uuid.UUID{file.Id}
			return in
		}, "invalid_attachment"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			expectProblem(t, c.s.do(http.MethodPost, "/tickets", c.in()), http.StatusBadRequest, c.code)
		})
	}

	expired := te.requester.attach(t, "old.png")
	if _, err := te.st.Pool().Exec(ctx, `UPDATE attachments SET created_at = now() - interval '2 days' WHERE id = $1`, expired.Id); err != nil {
		t.Fatal(err)
	}
	in = incident("late")
	in.AttachmentIds = &[]uuid.UUID{expired.Id}
	expectProblem(t, te.requester.do(http.MethodPost, "/tickets", in), http.StatusBadRequest, "invalid_attachment")
}

func TestTicketActions(t *testing.T) {
	te := newTicketEnv(t)
	tk := te.requester.report(t, incident("Network down"))
	path := "/tickets/" + tk.Key
	status := func(s gen.TicketStatus) *httptest.ResponseRecorder {
		return te.agent.do(http.MethodPut, path+"/status", gen.TicketStatusInput{Status: s})
	}
	assign := func(id *uuid.UUID) *httptest.ResponseRecorder {
		return te.agent.do(http.MethodPut, path+"/assignee", gen.TicketAssigneeInput{AssigneeId: id})
	}

	got := decode[gen.Ticket](t, assign(&te.agentID), http.StatusOK)
	if got.Status != gen.TicketStatusAssigned || got.Assignee.Id != te.agentID {
		t.Fatalf("after assign: %+v", got)
	}
	got = decode[gen.Ticket](t, assign(nil), http.StatusOK)
	if got.Status != gen.TicketStatusNew || got.Assignee != nil {
		t.Fatalf("after unassign: %+v", got)
	}
	expectProblem(t, assign(&te.requesterID), http.StatusBadRequest, "invalid_assignee")
	otherAgent := func() uuid.UUID {
		u, _ := te.st.Q().GetUserByEmail(context.Background(), otherAgentEmail)
		return u.ID
	}()
	expectProblem(t, assign(&otherAgent), http.StatusBadRequest, "invalid_assignee")
	supervisor := userID(t, te.env, auth.RoleSupervisor)
	decode[gen.Ticket](t, assign(&supervisor), http.StatusOK)

	got = decode[gen.Ticket](t, status(gen.TicketStatusInProgress), http.StatusOK)
	if !slices.Contains(got.AllowedStatuses, gen.TicketStatusWaitingOnClient) || slices.Contains(got.AllowedStatuses, gen.TicketStatusClosed) {
		t.Fatalf("allowed = %v", got.AllowedStatuses)
	}
	expectProblem(t, status(gen.TicketStatusClosed), http.StatusConflict, "invalid_status_change")
	expectProblem(t, status(gen.TicketStatusNew), http.StatusConflict, "invalid_status_change")
	got = decode[gen.Ticket](t, status(gen.TicketStatusResolved), http.StatusOK)
	if got.ResolvedAt == nil {
		t.Fatal("resolvedAt not set")
	}
	got = decode[gen.Ticket](t, status(gen.TicketStatusInProgress), http.StatusOK)
	if got.ResolvedAt != nil {
		t.Fatal("resolvedAt kept after reopen")
	}
	decode[gen.Ticket](t, status(gen.TicketStatusInProgress), http.StatusOK) // no-op

	pri := func(p gen.Priority, reason string) *httptest.ResponseRecorder {
		return te.agent.do(http.MethodPut, path+"/priority", gen.TicketPriorityInput{Priority: p, Reason: reason})
	}
	expectProblem(t, pri(gen.PriorityP1, "   "), http.StatusBadRequest, "reason_required")
	got = decode[gen.Ticket](t, pri(gen.PriorityP1, "Whole office is down"), http.StatusOK)
	if got.Priority != gen.PriorityP1 || got.Sla.Priority != gen.PriorityP1 || got.Sla.FirstResponseMinutes != got.SlaTargets[0].FirstResponseMinutes {
		t.Fatalf("after priority change: %+v", got.Sla)
	}
	var reason string
	if err := te.st.Pool().QueryRow(context.Background(),
		`SELECT reason FROM audit_log WHERE action = 'ticket.priority_changed' AND target_id = $1`, tk.Id).Scan(&reason); err != nil || reason != "Whole office is down" {
		t.Fatalf("reason = %q, %v", reason, err)
	}

	want := []string{
		"ticket.created",
		"ticket.assignee_changed", "ticket.status_changed", // assign: new -> assigned
		"ticket.assignee_changed", "ticket.status_changed", // unassign: assigned -> new
		"ticket.assignee_changed", "ticket.status_changed", // supervisor: new -> assigned
		"ticket.status_changed", "ticket.status_changed", "ticket.status_changed", // in progress, resolved, reopened
		"ticket.priority_changed",
	}
	if got := te.auditActions(t, tk.Id); !slices.Equal(got, want) {
		t.Fatalf("audit =\n%v\nwant\n%v", got, want)
	}
	activity := decode[[]gen.TicketActivity](t, te.agent.do(http.MethodGet, path+"/activity", nil), http.StatusOK)
	if len(activity) != len(want) || activity[0].Action != gen.TicketActivityActionCreated ||
		*activity[0].Channel != gen.TicketChannelPortal {
		t.Fatalf("activity = %+v", activity)
	}
	first := activity[1]
	if first.Action != gen.TicketActivityActionAssigneeChanged || first.From != nil || *first.To != "User agent" {
		t.Fatalf("assignee entry = %+v", first)
	}
	if c := activity[2]; *c.From != "new" || *c.To != "assigned" || *c.Cause != gen.TicketStatusCauseAssignment {
		t.Fatalf("status entry = %+v", c)
	}
	if last := activity[len(activity)-1]; *last.Reason != "Whole office is down" || *last.From != "P3" || *last.To != "P1" {
		t.Fatalf("priority entry = %+v", last)
	}

	// Closed tickets take nothing.
	if _, err := te.st.Pool().Exec(context.Background(),
		`UPDATE tickets SET status = 'closed', resolved_at = now(), closed_at = now() WHERE id = $1`, tk.Id); err != nil {
		t.Fatal(err)
	}
	expectProblem(t, status(gen.TicketStatusInProgress), http.StatusConflict, "ticket_closed")
	expectProblem(t, te.requester.message(tk.Key, gen.MessageVisibilityPublic, "hello?"), http.StatusConflict, "ticket_closed")
}

func TestMessagesAndFirstResponse(t *testing.T) {
	te := newTicketEnv(t)
	tk := te.requester.report(t, incident("Slow VPN"))
	decode[gen.TicketMessage](t, te.agent.message(tk.Key, gen.MessageVisibilityInternal, "Check the tunnel"), http.StatusCreated)
	if te.agent.ticket(t, tk.Key).FirstResponseAt != nil {
		t.Fatal("an internal note counted as the first response")
	}
	file := te.agent.attach(t, "steps.png")
	reply := decode[gen.TicketMessage](t, te.agent.message(tk.Key, gen.MessageVisibilityPublic, "Please restart the client", file.Id), http.StatusCreated)
	if !reply.FromSupport || len(reply.Attachments) != 1 {
		t.Fatalf("reply = %+v", reply)
	}
	first := te.agent.ticket(t, tk.Key).FirstResponseAt
	if first == nil {
		t.Fatal("first response not recorded")
	}
	decode[gen.TicketMessage](t, te.agent.message(tk.Key, gen.MessageVisibilityPublic, "Any luck?"), http.StatusCreated)
	if again := te.agent.ticket(t, tk.Key).FirstResponseAt; !again.Equal(*first) {
		t.Fatal("first response moved")
	}

	decode[gen.Ticket](t, te.agent.do(http.MethodPut, "/tickets/"+tk.Key+"/status",
		gen.TicketStatusInput{Status: gen.TicketStatusWaitingOnClient}), http.StatusOK)
	decode[gen.TicketMessage](t, te.requester.message(tk.Key, gen.MessageVisibilityPublic, "Restarted, still slow"), http.StatusCreated)
	got := te.agent.ticket(t, tk.Key)
	if got.Status != gen.TicketStatusInProgress {
		t.Fatalf("status after requester reply = %s", got.Status)
	}
	msgs := te.requester.messages(t, tk.Key)
	if len(msgs) != 3 || msgs[0].Body != "Please restart the client" || msgs[2].FromSupport {
		t.Fatalf("requester messages = %+v", msgs)
	}
	expectProblem(t, te.requester.message(tk.Key, gen.MessageVisibilityPublic, "   "), http.StatusBadRequest, "invalid_message")
}

func TestListTickets(t *testing.T) {
	te := newTicketEnv(t)
	a := te.requester.report(t, incident("Printer offline"))
	b := te.requester.report(t, incident("Email bounce 50% of the time"))
	c := te.otherRequester.report(t, incident("Other client"))
	decode[gen.Ticket](t, te.admin.do(http.MethodPut, "/tickets/"+a.Key+"/assignee", gen.TicketAssigneeInput{AssigneeId: &te.agentID}), http.StatusOK)
	decode[gen.Ticket](t, te.admin.do(http.MethodPut, "/tickets/"+b.Key+"/status", gen.TicketStatusInput{Status: gen.TicketStatusResolved}), http.StatusOK)

	checks := map[string]struct {
		s     session
		query string
		want  []string
	}{
		"admin, all":          {te.admin, "", []string{c.Key, b.Key, a.Key}},
		"agent, own clients":  {te.agent, "", []string{b.Key, a.Key}},
		"open":                {te.admin, "?status=new&status=assigned", []string{c.Key, a.Key}},
		"resolved":            {te.requester, "?status=resolved&status=closed", []string{b.Key}},
		"mine":                {te.agent, "?assignee=me", []string{a.Key}},
		"unassigned":          {te.admin, "?assignee=unassigned", []string{c.Key, b.Key}},
		"client":              {te.admin, "?clientId=" + te.clientB.String(), []string{c.Key}},
		"key search":          {te.admin, "?q=" + strings.ToLower(a.Key), []string{a.Key}},
		"subject search":      {te.admin, "?q=printer", []string{a.Key}},
		"literal percent":     {te.admin, "?q=50%25", []string{b.Key}},
		"first page":          {te.admin, "?limit=2", []string{c.Key, b.Key}},
		"next page":           {te.admin, "?limit=2&before=" + b.Id.String(), []string{a.Key}},
		"requester, own only": {te.otherRequester, "", []string{c.Key}},
	}
	for name, ch := range checks {
		if got := ch.s.keys(t, ch.query); !slices.Equal(got, ch.want) {
			t.Errorf("%s: %v, want %v", name, got, ch.want)
		}
	}
}

func TestAttachments(t *testing.T) {
	ctx := context.Background()
	te := newTicketEnv(t)
	up := te.requester.attach(t, "C:\\Users\\hendra\\Desktop\\layar galat.png")
	if up.Filename != "layar galat.png" || up.ContentType != "image/png" || up.SizeBytes != int64(len(pngBytes)) {
		t.Fatalf("upload = %+v", up)
	}
	if rec := te.requester.do(http.MethodGet, "/attachments/"+up.Id.String(), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("pending file downloadable: %d", rec.Code)
	}
	in := incident("With file")
	in.AttachmentIds = &[]uuid.UUID{up.Id}
	te.requester.report(t, in)

	rec := te.requester.do(http.MethodGet, "/attachments/"+up.Id.String(), nil)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), pngBytes) {
		t.Fatalf("download: %d %q", rec.Code, rec.Body.Bytes())
	}
	h := rec.Header()
	if h.Get("Content-Type") != "application/octet-stream" || h.Get("Cache-Control") != "no-store" ||
		h.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") ||
		h.Get("Content-Disposition") != `attachment; filename="layar galat.png"` {
		t.Fatalf("download headers = %v", h)
	}

	expectProblem(t, te.requester.uploadFile("drawing.svg", []byte("<svg/>")), http.StatusBadRequest, "attachment_type_not_allowed")
	expectProblem(t, te.requester.uploadFile("fake.png", []byte("MZ\x90\x00")), http.StatusBadRequest, "attachment_type_not_allowed")
	expectProblem(t, te.requester.uploadFile("empty.png", nil), http.StatusBadRequest, "attachment_empty")

	big := append(slices.Clone(pngBytes), make([]byte, 10<<20)...)
	expectProblem(t, te.requester.uploadFile("big.png", big), http.StatusRequestEntityTooLarge, "attachment_too_large")
	var rows int
	if err := te.st.Pool().QueryRow(ctx, `SELECT count(*) FROM attachments WHERE filename = 'big.png'`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("rows for rejected file = %d, %v", rows, err)
	}
	if n, err := te.files.PurgeTemp(ctx, 0); err != nil || n != 0 {
		t.Fatalf("temp files left = %d, %v", n, err)
	}

	// The pending cap stops one user filling the disk.
	for i := 0; ; i++ {
		rec := te.otherRequester.uploadFile("f.png", pngBytes)
		if rec.Code == http.StatusConflict {
			expectProblem(t, rec, http.StatusConflict, "too_many_pending_attachments")
			if i != 20 {
				t.Fatalf("cap hit after %d uploads", i)
			}
			break
		}
		decode[gen.Attachment](t, rec, http.StatusCreated)
	}

	huge := strings.Repeat("a", 1<<20)
	rec = te.requester.do(http.MethodPost, "/tickets", incident(huge))
	expectProblem(t, rec, http.StatusRequestEntityTooLarge, "request_too_large")

	policy := decode[gen.AttachmentPolicy](t, te.requester.do(http.MethodGet, "/attachment-policy", nil), http.StatusOK)
	if policy.MaxBytes != 10<<20 || policy.MaxPerMessage != 5 || !slices.Contains(policy.Extensions, ".png") {
		t.Fatalf("policy = %+v", policy)
	}
}

func TestCatalog(t *testing.T) {
	te := newTicketEnv(t)
	desc := "Letters, dispositions, archives"
	sys := decode[gen.System](t, te.admin.do(http.MethodPost, "/systems", gen.SystemInput{Name: "e-Office", Description: &desc, SortOrder: 10, Active: true}), http.StatusCreated)
	hidden := decode[gen.System](t, te.admin.do(http.MethodPost, "/systems", gen.SystemInput{Name: "Old app", SortOrder: 20, Active: false}), http.StatusCreated)
	expectProblem(t, te.admin.do(http.MethodPost, "/systems", gen.SystemInput{Name: "E-OFFICE", SortOrder: 1, Active: true}), http.StatusConflict, "system_name_taken")
	if rec := te.agent.do(http.MethodPost, "/systems", gen.SystemInput{Name: "x", SortOrder: 1, Active: true}); rec.Code != http.StatusForbidden {
		t.Fatalf("agent create: %d", rec.Code)
	}
	if list := decode[[]gen.System](t, te.agent.do(http.MethodGet, "/systems", nil), http.StatusOK); len(list) != 2 {
		t.Fatalf("agent list = %+v", list)
	}
	intake := decode[gen.Intake](t, te.requester.do(http.MethodGet, "/intake", nil), http.StatusOK)
	if len(intake.Systems) != 1 || intake.Systems[0].Id != sys.Id || intake.ClientId != te.clientA ||
		len(intake.Targets) != 4 || intake.BusinessHours.WorkingDayMinutes == 0 {
		t.Fatalf("intake = %+v", intake)
	}
	expectProblem(t, te.agent.do(http.MethodGet, "/intake", nil), http.StatusBadRequest, "client_required")
	in := incident("Uses hidden")
	in.SystemId = &hidden.Id
	expectProblem(t, te.requester.do(http.MethodPost, "/tickets", in), http.StatusBadRequest, "invalid_system")

	in.SystemId = &sys.Id
	te.requester.report(t, in)
	expectProblem(t, te.admin.do(http.MethodDelete, "/systems/"+sys.Id.String(), nil), http.StatusConflict, "system_in_use")
	if rec := te.admin.do(http.MethodDelete, "/systems/"+hidden.Id.String(), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete unused: %d", rec.Code)
	}
	updated := decode[gen.System](t, te.admin.do(http.MethodPut, "/systems/"+sys.Id.String(),
		gen.SystemInput{Name: "e-Office", SortOrder: 10, Active: false}), http.StatusOK)
	if updated.Active || updated.TicketCount != 1 || updated.Description != nil {
		t.Fatalf("updated = %+v", updated)
	}

	item := decode[gen.CatalogItem](t, te.admin.do(http.MethodPost, "/catalog-items", gen.CatalogItemInput{
		Name: "New email account", DeliveryMinutes: 540, SortOrder: 10, Active: true}), http.StatusCreated)
	expectProblem(t, te.admin.do(http.MethodPut, "/catalog-items/"+item.Id.String(), gen.CatalogItemInput{
		Name: "New email account", DeliveryMinutes: 0, SortOrder: 10, Active: true}), http.StatusBadRequest, "invalid_request")

	var actions []string
	rows, _ := te.st.Pool().Query(context.Background(),
		`SELECT action FROM audit_log WHERE target_type IN ('system', 'catalog_item') ORDER BY at, id`)
	for rows.Next() {
		var a string
		_ = rows.Scan(&a)
		actions = append(actions, a)
	}
	rows.Close()
	want := []string{"system.created", "system.created", "system.deleted", "system.updated", "catalog_item.created"}
	if !slices.Equal(actions, want) {
		t.Fatalf("audit = %v", actions)
	}
}

func TestRequestersAndAssignees(t *testing.T) {
	te := newTicketEnv(t)
	base := "/clients/" + te.clientA.String()
	r := decode[gen.Requester](t, te.agent.do(http.MethodPost, base+"/requesters", gen.RequesterInput{
		Name: "Siti Rahma", Email: " Siti@Kominfo-Jambi.test "}), http.StatusCreated)
	if r.Status != gen.UserStatusInvited || r.Email != "siti@kominfo-jambi.test" {
		t.Fatalf("requester = %+v", r)
	}
	if rec := te.login("siti@kominfo-jambi.test", password); rec.Code != http.StatusUnauthorized {
		t.Fatalf("invited user signs in: %d", rec.Code)
	}
	expectProblem(t, te.agent.do(http.MethodPost, base+"/requesters", gen.RequesterInput{Name: "Again", Email: "siti@kominfo-jambi.test"}),
		http.StatusConflict, "email_taken")
	expectProblem(t, te.agent.do(http.MethodPost, base+"/requesters", gen.RequesterInput{Name: "No email", Email: "nope"}),
		http.StatusBadRequest, "invalid_email")
	list := decode[[]gen.Requester](t, te.agent.do(http.MethodGet, base+"/requesters", nil), http.StatusOK)
	if len(list) != 2 {
		t.Fatalf("requesters = %+v", list)
	}
	// A new requester can be named on a ticket straight away.
	te.agent.logFor(t, te.clientA, r.Id, incident("Phoned in"))

	names := []string{}
	for _, a := range decode[[]gen.Assignee](t, te.agent.do(http.MethodGet, base+"/assignees", nil), http.StatusOK) {
		names = append(names, a.Name)
	}
	if !slices.Equal(names, []string{"User admin", "User agent", "User supervisor"}) {
		t.Fatalf("assignees = %v", names)
	}
}

func TestClientWithTicketsCantBeDeleted(t *testing.T) {
	te := newTicketEnv(t)
	te.otherRequester.report(t, incident("x"))
	if _, err := te.st.Pool().Exec(context.Background(), `DELETE FROM users WHERE email = $1`, otherRequesterEmail); err == nil {
		t.Fatal("deleted a requester with tickets")
	}
	expectProblem(t, te.admin.do(http.MethodDelete, "/clients/"+te.clientB.String(), nil), http.StatusConflict, "client_in_use")
}

func TestDownloadStreamsWholeFile(t *testing.T) {
	te := newTicketEnv(t)
	content := append(slices.Clone(pngBytes), bytes.Repeat([]byte{7}, 3<<20)...)
	up := decode[gen.Attachment](t, te.requester.uploadFile("large.png", content), http.StatusCreated)
	in := incident("Big screenshot")
	in.AttachmentIds = &[]uuid.UUID{up.Id}
	te.requester.report(t, in)
	rec := te.agent.do(http.MethodGet, "/attachments/"+up.Id.String(), nil)
	body, _ := io.ReadAll(rec.Body)
	if rec.Code != http.StatusOK || !bytes.Equal(body, content) {
		t.Fatalf("download: %d, %d bytes", rec.Code, len(body))
	}
}
