package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

const mcpPath = "/api/mcp"

// tokenFor makes a personal token for the session's user.
func (s session) tokenFor(t *testing.T, access gen.TokenAccess) gen.CreatedPersonalToken {
	t.Helper()
	in := gen.PersonalTokenInput{Name: "Claude Code", Access: access, ExpiresInDays: gen.PersonalTokenInputExpiresInDaysN90}
	return decode[gen.CreatedPersonalToken](t, s.do(http.MethodPost, "/me/tokens", in), http.StatusCreated)
}

// bearerTransport adds a personal token to every request.
type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

// mcpClient connects an MCP client to the app with a personal token.
func (e *env) mcpClient(t *testing.T, token string) *mcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(e.handler)
	t.Cleanup(srv.Close)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: srv.URL + mcpPath, HTTPClient: &http.Client{Transport: bearerTransport{token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func toolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

// call runs a tool and returns its text and whether it reported an error.
func call(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

func mustCall(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) string {
	t.Helper()
	out, isErr := call(t, cs, tool, args)
	if isErr {
		t.Fatalf("%s failed: %s", tool, out)
	}
	return out
}

func TestPersonalTokens(t *testing.T) {
	e := newEnv(t)
	agent, requester := e.as(auth.RoleAgent), e.as(auth.RoleRequester)

	made := agent.tokenFor(t, gen.TokenAccessWrite)
	if !strings.HasPrefix(made.Secret, auth.TokenPrefix) || !strings.HasSuffix(made.Secret, made.Token.LastFour) {
		t.Fatalf("secret %q doesn't match the token %+v", made.Secret, made.Token)
	}
	list := decode[gen.PersonalTokens](t, agent.do(http.MethodGet, "/me/tokens", nil), http.StatusOK)
	if list.McpUrl != appURL+mcpPath || len(list.Items) != 1 || list.Items[0].Id != made.Token.Id {
		t.Fatalf("list = %+v", list)
	}
	if strings.Contains(agent.do(http.MethodGet, "/me/tokens", nil).Body.String(), made.Secret) {
		t.Fatal("the list must never show the secret")
	}

	expectProblem(t, requester.do(http.MethodGet, "/me/tokens", nil), http.StatusForbidden, apperr.CodeForbidden)
	expectProblem(t, requester.do(http.MethodPost, "/me/tokens",
		gen.PersonalTokenInput{Name: "x", Access: gen.TokenAccessRead, ExpiresInDays: gen.PersonalTokenInputExpiresInDaysN30}), http.StatusForbidden, apperr.CodeForbidden)
	expectProblem(t, agent.do(http.MethodPost, "/me/tokens", map[string]any{"name": "x", "access": "read", "expiresInDays": 7}),
		http.StatusBadRequest, apperr.CodeInvalidRequest)

	// Someone else's token is a 404; the owner's revoke is final.
	supervisor := e.as(auth.RoleSupervisor)
	expectProblem(t, supervisor.do(http.MethodDelete, "/me/tokens/"+made.Token.Id.String(), nil), http.StatusNotFound, apperr.CodeNotFound)
	if rec := agent.do(http.MethodDelete, "/me/tokens/"+made.Token.Id.String(), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d %s", rec.Code, rec.Body)
	}
	expectProblem(t, agent.do(http.MethodDelete, "/me/tokens/"+made.Token.Id.String(), nil), http.StatusNotFound, apperr.CodeNotFound)

	for range auth.MaxTokensPerUser {
		agent.tokenFor(t, gen.TokenAccessRead)
	}
	expectProblem(t, agent.do(http.MethodPost, "/me/tokens", gen.PersonalTokenInput{Name: "one more", Access: gen.TokenAccessRead, ExpiresInDays: gen.PersonalTokenInputExpiresInDaysN30}),
		http.StatusConflict, apperr.CodeTokenLimit)
}

func TestMCPNeedsAPersonalToken(t *testing.T) {
	e := newEnv(t)
	agent := e.as(auth.RoleAgent)
	token := agent.tokenFor(t, gen.TokenAccessRead)
	post := func(header map[string]string, cookie *http.Cookie) *httptest.ResponseRecorder {
		return e.do(request{method: http.MethodPost, path: mcpPath, body: map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"},
			header: header, cookie: cookie})
	}
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"no token":       post(nil, nil),
		"session cookie": post(nil, agent.cookie),
		"unknown token":  post(map[string]string{"Authorization": "Bearer " + auth.TokenPrefix + "nope"}, nil),
		"basic scheme":   post(map[string]string{"Authorization": "Basic " + token.Secret}, nil),
	} {
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("%s: %d %q", name, rec.Code, rec.Header().Get("WWW-Authenticate"))
		}
	}

	// Revoked and suspended both end the token.
	cs := e.mcpClient(t, token.Secret)
	if len(toolNames(t, cs)) == 0 {
		t.Fatal("a live token lists tools")
	}
	if rec := agent.do(http.MethodDelete, "/me/tokens/"+token.Token.Id.String(), nil); rec.Code != http.StatusNoContent {
		t.Fatal(rec.Body)
	}
	if rec := post(map[string]string{"Authorization": "Bearer " + token.Secret}, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token: %d", rec.Code)
	}
	other := agent.tokenFor(t, gen.TokenAccessRead)
	admin := e.as(auth.RoleAdmin)
	agentID := userID(t, e, auth.RoleAgent)
	if rec := admin.do(http.MethodPost, "/members/"+agentID.String()+"/suspend", nil); rec.Code >= http.StatusBadRequest {
		t.Fatalf("suspend: %d %s", rec.Code, rec.Body)
	}
	if rec := post(map[string]string{"Authorization": "Bearer " + other.Secret}, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("suspended user's token: %d", rec.Code)
	}
}

func TestMCPUnknownTokensAreRateLimited(t *testing.T) {
	e := newEnv(t)
	bad := map[string]string{"Authorization": "Bearer " + auth.TokenPrefix + "guess"}
	for range auth.DefaultMaxFailuresPerIP {
		e.do(request{method: http.MethodPost, path: mcpPath, body: map[string]any{}, header: bad})
	}
	if rec := e.do(request{method: http.MethodPost, path: mcpPath, body: map[string]any{}, header: bad}); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("after %d unknown tokens: %d", auth.DefaultMaxFailuresPerIP, rec.Code)
	}
}

func TestMCPToolsFollowTheTokensAccess(t *testing.T) {
	e := newEnv(t)
	agent := e.as(auth.RoleAgent)
	read := toolNames(t, e.mcpClient(t, agent.tokenFor(t, gen.TokenAccessRead).Secret))
	write := toolNames(t, e.mcpClient(t, agent.tokenFor(t, gen.TokenAccessWrite).Secret))
	wantRead := []string{"get_ticket", "get_ticket_activity", "list_assignees", "read_attachment", "search_tickets"}
	if !slices.Equal(read, wantRead) {
		t.Errorf("read token tools = %v, want %v", read, wantRead)
	}
	wantWrite := append(slices.Clone(wantRead), "add_internal_note", "assign_ticket", "reply_to_ticket", "set_ticket_priority", "set_ticket_status")
	slices.Sort(wantWrite)
	if !slices.Equal(write, wantWrite) {
		t.Errorf("write token tools = %v, want %v", write, wantWrite)
	}
}

func TestMCPWorksTicketsAsTheUser(t *testing.T) {
	te := newTicketEnv(t)
	in := incident(te.projectA, "e-Office login fails")
	in.Description = "Since 08:00 **nobody** can sign in."
	file := te.requester.attach(t, "screen.png")
	in.AttachmentIds = &[]uuid.UUID{file.Id}
	tk := te.requester.report(t, in)
	theirs := te.otherRequester.report(t, incident(te.projectB, "Printer offline"))
	cs := te.mcpClient(t, te.agent.tokenFor(t, gen.TokenAccessWrite).Secret)

	if out := mustCall(t, cs, "search_tickets", map[string]any{"query": "login"}); !strings.Contains(out, tk.Key) || strings.Contains(out, theirs.Key) {
		t.Errorf("search shows only the agent's clients' tickets:\n%s", out)
	}
	got := mustCall(t, cs, "get_ticket", map[string]any{"key": tk.Key})
	for _, want := range []string{"# " + tk.Key, "Since 08:00 **nobody** can sign in.", "screen.png", file.Id.String(), appURL + "/tickets/" + tk.Key} {
		if !strings.Contains(got, want) {
			t.Errorf("get_ticket lacks %q:\n%s", want, got)
		}
	}
	if out, isErr := call(t, cs, "get_ticket", map[string]any{"key": theirs.Key}); !isErr || !strings.Contains(out, apperr.CodeNotFound) {
		t.Errorf("another client's ticket: %v %s", isErr, out)
	}

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "read_attachment", Arguments: map[string]any{"attachment_id": file.Id.String()}})
	if err != nil || res.IsError || len(res.Content) != 1 {
		t.Fatalf("read_attachment: %v %+v", err, res)
	}
	if img, ok := res.Content[0].(*mcp.ImageContent); !ok || img.MIMEType != "image/png" || string(img.Data) != string(pngBytes) {
		t.Errorf("read_attachment returned %+v", res.Content[0])
	}

	mustCall(t, cs, "assign_ticket", map[string]any{"key": tk.Key, "assignee": "me"})
	mustCall(t, cs, "set_ticket_status", map[string]any{"key": tk.Key, "status": "in_progress"})
	mustCall(t, cs, "set_ticket_priority", map[string]any{"key": tk.Key, "priority": "p2", "reason": "Only one office affected"})
	mustCall(t, cs, "reply_to_ticket", map[string]any{"key": tk.Key, "body": "Fixed on **staging**, please check."})
	mustCall(t, cs, "add_internal_note", map[string]any{"key": tk.Key, "body": "Reverted `token_ttl`."})

	now := decode[gen.Ticket](t, te.agent.do(http.MethodGet, "/tickets/"+tk.Key, nil), http.StatusOK)
	if now.Status != gen.TicketStatusInProgress || now.Priority != gen.PriorityP2 || now.Assignee == nil || now.Assignee.Id != te.agentID {
		t.Errorf("after the tools: %s %s %+v", now.Status, now.Priority, now.Assignee)
	}
	// The requester sees the reply, not the note.
	var bodies []string
	for _, m := range te.requester.messages(t, tk.Key) {
		bodies = append(bodies, m.Body)
	}
	if !slices.Equal(bodies, []string{"Fixed on **staging**, please check."}) {
		t.Errorf("requester sees %q", bodies)
	}
	activity := mustCall(t, cs, "get_ticket_activity", map[string]any{"key": tk.Key})
	for _, want := range []string{"User agent", "priority_changed", "Only one office affected"} {
		if !strings.Contains(activity, want) {
			t.Errorf("activity lacks %q:\n%s", want, activity)
		}
	}
	if out := mustCall(t, cs, "list_assignees", map[string]any{"key": tk.Key}); !strings.Contains(out, te.agentID.String()) {
		t.Errorf("list_assignees:\n%s", out)
	}
	if out, isErr := call(t, cs, "set_ticket_status", map[string]any{"key": tk.Key, "status": "done"}); !isErr || !strings.Contains(out, "unknown status") {
		t.Errorf("bad status: %v %s", isErr, out)
	}
}

// An agent's token follows the agent's assignment rule, and the tool says why it refuses.
func TestMCPAssignFollowsTheAgentRule(t *testing.T) {
	te := newTicketEnv(t)
	tk := te.requester.report(t, incident(te.projectA, "Printer offline"))
	supervisorID := userID(t, te.env, auth.RoleSupervisor)
	const note = "Only a supervisor or administrator can give it to someone else"

	agent := te.mcpClient(t, te.agent.tokenFor(t, gen.TokenAccessWrite).Secret)
	if out := mustCall(t, agent, "list_assignees", map[string]any{"key": tk.Key}); !strings.Contains(out, note) {
		t.Errorf("list_assignees for an agent lacks the rule:\n%s", out)
	}
	out, isErr := call(t, agent, "assign_ticket", map[string]any{"key": tk.Key, "assignee": supervisorID.String()})
	if !isErr || !strings.Contains(out, apperr.CodeAssigneeNotAllowed) || !strings.Contains(out, "Ask a supervisor") {
		t.Fatalf("agent assigns a colleague: %v %s", isErr, out)
	}
	mustCall(t, agent, "assign_ticket", map[string]any{"key": tk.Key, "assignee": "me"})

	supervisor := te.mcpClient(t, te.supervisor.tokenFor(t, gen.TokenAccessWrite).Secret)
	if out := mustCall(t, supervisor, "list_assignees", map[string]any{"key": tk.Key}); strings.Contains(out, note) {
		t.Errorf("list_assignees for a supervisor:\n%s", out)
	}
	mustCall(t, supervisor, "assign_ticket", map[string]any{"key": tk.Key, "assignee": supervisorID.String()})
	if got := te.admin.ticket(t, tk.Key); got.Assignee == nil || got.Assignee.Id != supervisorID {
		t.Fatalf("after the supervisor's tool: %+v", got.Assignee)
	}
}

func TestMCPResolveExplainsTheDoD(t *testing.T) {
	te := newTicketEnv(t)
	testdb.SetProjectDoD(t, te.st, te.projectA, uatDoD())
	tk := te.requester.report(t, incident(te.projectA, "Disposition letter shows the wrong date"))
	cs := te.mcpClient(t, te.agent.tokenFor(t, gen.TokenAccessWrite).Secret)
	mustCall(t, cs, "assign_ticket", map[string]any{"key": tk.Key, "assignee": "me"})
	mustCall(t, cs, "set_ticket_status", map[string]any{"key": tk.Key, "status": "in_progress"})
	out, isErr := call(t, cs, "set_ticket_status", map[string]any{"key": tk.Key, "status": "resolved"})
	if !isErr || !strings.Contains(out, apperr.CodeDoDUnmet) || !strings.Contains(out, "reason") {
		t.Fatalf("resolve without a reason: %v %s", isErr, out)
	}
	mustCall(t, cs, "set_ticket_status", map[string]any{"key": tk.Key, "status": "resolved", "reason": "Hotfix, client confirmed by phone"})
}
