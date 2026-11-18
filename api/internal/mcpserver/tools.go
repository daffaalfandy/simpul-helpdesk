package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// Tool names.
const (
	toolSearch       = "search_tickets"
	toolGet          = "get_ticket"
	toolActivity     = "get_ticket_activity"
	toolAssignees    = "list_assignees"
	toolAttachment   = "read_attachment"
	toolReply        = "reply_to_ticket"
	toolNote         = "add_internal_note"
	toolSetStatus    = "set_ticket_status"
	toolAssign       = "assign_ticket"
	toolSetPriority  = "set_ticket_priority"
	assigneeMe       = "me"
	assigneeNone     = "none"
	assigneeAnyone   = "any"
	defaultListLimit = 20
	maxListLimit     = 50
)

// errInternal is what the model sees when SimDesk itself failed; the cause is logged.
var errInternal = errors.New("SimDesk couldn't do that just now. Try again; if it keeps failing, tell your administrator")

type keyInput struct {
	Key string `json:"key" jsonschema:"The ticket key, for example HD-24871"`
}

type searchInput struct {
	Query    string   `json:"query,omitempty" jsonschema:"Words from the subject or the client's name, or a ticket key"`
	Statuses []string `json:"statuses,omitempty" jsonschema:"Only tickets in these statuses: new, assigned, in_progress, waiting_on_client, in_uat, resolved, closed"`
	Assignee string   `json:"assignee,omitempty" jsonschema:"me, unassigned, or any (the default)"`
	Limit    int      `json:"limit,omitempty" jsonschema:"How many tickets to return, 1 to 50 (default 20), most recently updated first"`
}

type attachmentInput struct {
	AttachmentID string `json:"attachment_id" jsonschema:"The attachment id from get_ticket"`
}

type messageInput struct {
	Key  string `json:"key" jsonschema:"The ticket key, for example HD-24871"`
	Body string `json:"body" jsonschema:"The message in Markdown, at most 20000 characters"`
}

type statusInput struct {
	Key    string `json:"key" jsonschema:"The ticket key, for example HD-24871"`
	Status string `json:"status" jsonschema:"The new status; get_ticket lists the moves allowed from the current one"`
	Reason string `json:"reason,omitempty" jsonschema:"Why, at most 500 characters. Needed to resolve a ticket whose Definition of Done isn't met yet"`
}

type assignInput struct {
	Key      string `json:"key" jsonschema:"The ticket key, for example HD-24871"`
	Assignee string `json:"assignee" jsonschema:"me, none to unassign, or a person's id from list_assignees (supervisors and administrators only)"`
}

type priorityInput struct {
	Key      string `json:"key" jsonschema:"The ticket key, for example HD-24871"`
	Priority string `json:"priority" jsonschema:"P1 (most urgent) to P4"`
	Reason   string `json:"reason" jsonschema:"Why the priority changes, 1 to 500 characters; it shows in the ticket's activity"`
}

// toolset holds one request's token; its methods are the tools.
type toolset struct {
	deps    Deps
	session auth.TokenSession
}

func (t *toolset) p() *auth.Principal { return t.session.Principal }

func (t *toolset) register(s *mcp.Server) {
	read := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)}
	mcp.AddTool(s, &mcp.Tool{Name: toolSearch, Annotations: read, Description: "Find tickets you can see in SimDesk. " +
		"Returns key, subject, status, priority, client and assignee for each."}, t.search)
	mcp.AddTool(s, &mcp.Tool{Name: toolGet, Annotations: read, Description: "Read one ticket: its properties, SLA, " +
		"Definition of Done, description, attachments and the whole conversation, internal notes included."}, t.get)
	mcp.AddTool(s, &mcp.Tool{Name: toolActivity, Annotations: read, Description: "Read a ticket's activity log: " +
		"status, assignee and priority changes with reasons, SLA alerts and linked commits, oldest first."}, t.activity)
	mcp.AddTool(s, &mcp.Tool{Name: toolAssignees, Annotations: read, Description: "List the people a ticket can be " +
		"assigned to, with their ids and open ticket counts."}, t.assignees)
	mcp.AddTool(s, &mcp.Tool{Name: toolAttachment, Annotations: read, Description: "Read a file attached to a ticket. " +
		"Images come back as images and text files as text; other files give a link to download them in SimDesk."}, t.attachment)
	if !t.session.CanWrite() {
		return
	}
	write := &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)}
	mcp.AddTool(s, &mcp.Tool{Name: toolReply, Annotations: write, Description: "Post a reply to the client on a ticket, " +
		"as you. The client sees it and gets it by email. Use add_internal_note for anything only staff should read."}, t.reply)
	mcp.AddTool(s, &mcp.Tool{Name: toolNote, Annotations: write, Description: "Add an internal note to a ticket, as you. " +
		"Only the vendor's staff see it; it never reaches the client."}, t.note)
	mcp.AddTool(s, &mcp.Tool{Name: toolSetStatus, Annotations: write, Description: "Move a ticket to another status, " +
		"with the same rules as the board."}, t.setStatus)
	mcp.AddTool(s, &mcp.Tool{Name: toolAssign, Annotations: write, Description: "Assign a ticket to yourself or " +
		"someone else, or unassign it. Agents can only take an unassigned ticket or release their own; supervisors " +
		"and administrators can give it to anyone from list_assignees."}, t.assign)
	mcp.AddTool(s, &mcp.Tool{Name: toolSetPriority, Annotations: write, Description: "Change a ticket's priority, " +
		"which restarts its SLA targets for the new priority."}, t.setPriority)
}

func (t *toolset) search(ctx context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, any, error) {
	f := tickets.ListFilter{Query: strings.TrimSpace(in.Query), Limit: in.Limit}
	if f.Limit <= 0 {
		f.Limit = defaultListLimit
	}
	f.Limit = min(f.Limit, maxListLimit)
	for _, s := range in.Statuses {
		st := ticket.Status(s)
		if !st.Valid() {
			return nil, nil, fmt.Errorf("unknown status %q", s)
		}
		f.Statuses = append(f.Statuses, st)
	}
	switch in.Assignee {
	case "", assigneeAnyone:
	case string(tickets.AssigneeMe), string(tickets.AssigneeUnassigned):
		f.Assignee = tickets.AssigneeFilter(in.Assignee)
	default:
		return nil, nil, fmt.Errorf("assignee must be %s, %s or %s", tickets.AssigneeMe, tickets.AssigneeUnassigned, assigneeAnyone)
	}
	list, err := t.deps.Tickets.List(ctx, t.p(), f)
	if err != nil {
		return nil, nil, t.fail(ctx, toolSearch, err)
	}
	return text(renderList(list, t.ticketURL)), nil, nil
}

func (t *toolset) get(ctx context.Context, _ *mcp.CallToolRequest, in keyInput) (*mcp.CallToolResult, any, error) {
	tk, err := t.deps.Tickets.Get(ctx, t.p(), in.Key)
	if err != nil {
		return nil, nil, t.fail(ctx, toolGet, err)
	}
	msgs, err := t.deps.Tickets.Messages(ctx, t.p(), in.Key)
	if err != nil {
		return nil, nil, t.fail(ctx, toolGet, err)
	}
	return text(renderTicket(tk, msgs, t.ticketURL(tk.Key))), nil, nil
}

func (t *toolset) activity(ctx context.Context, _ *mcp.CallToolRequest, in keyInput) (*mcp.CallToolResult, any, error) {
	entries, err := t.deps.Tickets.Activity(ctx, t.p(), in.Key)
	if err != nil {
		return nil, nil, t.fail(ctx, toolActivity, err)
	}
	return text(renderActivity(in.Key, entries)), nil, nil
}

func (t *toolset) assignees(ctx context.Context, _ *mcp.CallToolRequest, in keyInput) (*mcp.CallToolResult, any, error) {
	tk, err := t.deps.Tickets.Get(ctx, t.p(), in.Key)
	if err != nil {
		return nil, nil, t.fail(ctx, toolAssignees, err)
	}
	people, err := t.deps.Clients.Assignees(ctx, t.p(), tk.Client.ID)
	if err != nil {
		return nil, nil, t.fail(ctx, toolAssignees, err)
	}
	return text(renderAssignees(tk.Key, tk.Assignee, people, auth.Can(t.p().Role, auth.CapTicketsAssignAny))), nil, nil
}

func (t *toolset) attachment(ctx context.Context, _ *mcp.CallToolRequest, in attachmentInput) (*mcp.CallToolResult, any, error) {
	id, err := uuid.Parse(strings.TrimSpace(in.AttachmentID))
	if err != nil {
		return nil, nil, errors.New("attachment_id must be an id from get_ticket")
	}
	f, err := t.deps.Tickets.Open(ctx, t.p(), id)
	if err != nil {
		return nil, nil, t.fail(ctx, toolAttachment, err)
	}
	defer func() { _ = f.Body.Close() }()
	link := t.deps.AppURL.JoinPath(pathAttachment, id.String()).String()
	kind := classify(f.ContentType)
	limit := inlineLimit(kind)
	if limit == 0 || (kind == kindImage && f.Size > limit) {
		return nil, nil, fmt.Errorf("%s (%s, %s) can't be shown here; download it in SimDesk: %s",
			f.Filename, f.ContentType, humanBytes(f.Size), link)
	}
	data, err := io.ReadAll(io.LimitReader(f.Body, limit))
	if err != nil {
		return nil, nil, t.fail(ctx, toolAttachment, err)
	}
	if kind == kindImage {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.ImageContent{Data: data, MIMEType: f.ContentType}}}, nil, nil
	}
	return text(renderTextFile(f.Filename, string(data), f.Size > limit, link)), nil, nil
}

func (t *toolset) reply(ctx context.Context, _ *mcp.CallToolRequest, in messageInput) (*mcp.CallToolResult, any, error) {
	return t.post(ctx, toolReply, in, ticket.VisibilityPublic)
}

func (t *toolset) note(ctx context.Context, _ *mcp.CallToolRequest, in messageInput) (*mcp.CallToolResult, any, error) {
	return t.post(ctx, toolNote, in, ticket.VisibilityInternal)
}

func (t *toolset) post(ctx context.Context, tool string, in messageInput, v ticket.Visibility) (*mcp.CallToolResult, any, error) {
	if _, err := t.deps.Tickets.AddMessage(ctx, t.p(), in.Key, tickets.MessageInput{Visibility: v, Body: in.Body}); err != nil {
		return nil, nil, t.fail(ctx, tool, err)
	}
	return text(renderPosted(in.Key, v, t.ticketURL(in.Key))), nil, nil
}

func (t *toolset) setStatus(ctx context.Context, _ *mcp.CallToolRequest, in statusInput) (*mcp.CallToolResult, any, error) {
	to := ticket.Status(in.Status)
	if !to.Valid() {
		return nil, nil, fmt.Errorf("unknown status %q", in.Status)
	}
	tk, err := t.deps.Tickets.SetStatus(ctx, t.p(), in.Key, to, nil, in.Reason)
	if err != nil {
		return nil, nil, t.fail(ctx, toolSetStatus, err)
	}
	return text(renderChanged(tk)), nil, nil
}

func (t *toolset) assign(ctx context.Context, _ *mcp.CallToolRequest, in assignInput) (*mcp.CallToolResult, any, error) {
	var to *uuid.UUID
	switch a := strings.TrimSpace(in.Assignee); a {
	case assigneeMe:
		id := t.p().UserID
		to = &id
	case assigneeNone:
	default:
		id, err := uuid.Parse(a)
		if err != nil {
			return nil, nil, fmt.Errorf("assignee must be %s, %s or an id from %s", assigneeMe, assigneeNone, toolAssignees)
		}
		to = &id
	}
	tk, err := t.deps.Tickets.SetAssignee(ctx, t.p(), in.Key, to)
	if err != nil {
		return nil, nil, t.fail(ctx, toolAssign, err)
	}
	return text(renderChanged(tk)), nil, nil
}

func (t *toolset) setPriority(ctx context.Context, _ *mcp.CallToolRequest, in priorityInput) (*mcp.CallToolResult, any, error) {
	pr := sla.Priority(strings.ToUpper(strings.TrimSpace(in.Priority)))
	if !pr.Valid() {
		return nil, nil, fmt.Errorf("priority must be one of %v", sla.Priorities)
	}
	tk, err := t.deps.Tickets.SetPriority(ctx, t.p(), in.Key, pr, in.Reason)
	if err != nil {
		return nil, nil, t.fail(ctx, toolSetPriority, err)
	}
	return text(renderChanged(tk)), nil, nil
}

// fail turns a service error into what the model reads: SimDesk's own explanation for a
// domain error, so it can correct itself, and a generic line for anything else, which is logged.
func (t *toolset) fail(ctx context.Context, tool string, err error) error {
	var ae *apperr.Error
	if errors.As(err, &ae) && ae.Status < httpServerErrors {
		return errors.New(explain(ae))
	}
	t.deps.Logger.ErrorContext(ctx, "mcp tool failed", "tool", tool, "err", err)
	return errInternal
}

// httpServerErrors is the first 5xx status: domain errors below it are the caller's to fix.
const httpServerErrors = 500

func (t *toolset) ticketURL(key string) string {
	return t.deps.AppURL.JoinPath(pathTickets, key).String()
}

func text(md string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: md}}}
}
