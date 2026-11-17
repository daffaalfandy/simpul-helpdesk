package mcpserver

import (
	"fmt"
	"strings"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/slaengine"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// Paths under APP_URL that tool output links to.
const (
	pathTickets    = "tickets"
	pathAttachment = "api/v1/attachments"
)

// Tool output is Markdown for the client's model. Times are UTC so every reader agrees.
const (
	timeLayout  = "2006-01-02 15:04 UTC"
	nobody      = "nobody"
	none        = "none"
	listSep     = ", "
	codeOther   = apperr.CodeOtherOrganization
	detailOther = "This ticket belongs to another organization. A token works only in the organization it was made in."
)

func at(t time.Time) string { return t.UTC().Format(timeLayout) }

func refName(r *tickets.Ref) string {
	if r == nil {
		return nobody
	}
	return r.Name
}

func renderList(list []tickets.Summary, link func(string) string) string {
	if len(list) == 0 {
		return "No tickets match."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d tickets, most recently updated first:\n\n", len(list))
	for _, t := range list {
		fmt.Fprintf(&b, "- **%s** %s\n  %s · %s · %s · client %s · assignee %s · updated %s · %s\n",
			t.Key, t.Subject, t.Status, t.Priority, t.Type, t.Client.Name, refName(t.Assignee), at(t.UpdatedAt), link(t.Key))
	}
	return b.String()
}

func renderTicket(t tickets.Ticket, msgs []tickets.Message, link string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: %s\n\n", t.Key, t.Subject)
	fmt.Fprintf(&b, "- Link: %s\n", link)
	fmt.Fprintf(&b, "- Status: %s%s\n", t.Status, allowedMoves(t.AllowedStatuses))
	fmt.Fprintf(&b, "- Priority: %s\n- Type: %s\n", t.Priority, t.Type)
	fmt.Fprintf(&b, "- Client: %s\n- Project: %s\n", t.Client.Name, t.Project.Name)
	if t.CatalogItem != nil {
		fmt.Fprintf(&b, "- Request: %s\n", t.CatalogItem.Name)
	}
	fmt.Fprintf(&b, "- Requester: %s%s\n", t.Requester.Name, jobTitle(t.RequesterJobTitle))
	fmt.Fprintf(&b, "- Assignee: %s\n", refName(t.Assignee))
	fmt.Fprintf(&b, "- Created: %s via %s\n- Updated: %s\n", at(t.CreatedAt), t.Channel, at(t.UpdatedAt))
	if t.SLAClocks != nil {
		fmt.Fprintf(&b, "- SLA first response: %s\n", clock(t.SLAClocks.FirstResponse))
		fmt.Fprintf(&b, "- SLA resolution: %s\n", clock(t.SLAClocks.Resolution))
	}
	if t.DoD != nil {
		fmt.Fprintf(&b, "- Definition of Done: %s\n", doneness(t.DoD))
	}
	fmt.Fprintf(&b, "\n## Description\n\n%s\n", t.Description)
	writeAttachments(&b, t.Attachments)
	fmt.Fprintf(&b, "\n## Conversation (%d messages, oldest first)\n", len(msgs))
	for _, m := range msgs {
		fmt.Fprintf(&b, "\n### %s, %s\n\n%s\n", messageHeading(m), at(m.CreatedAt), m.Body)
		writeAttachments(&b, m.Attachments)
	}
	return b.String()
}

func allowedMoves(to []ticket.Status) string {
	if len(to) == 0 {
		return ""
	}
	moves := make([]string, len(to))
	for i, s := range to {
		moves[i] = string(s)
	}
	return " (can move to: " + strings.Join(moves, listSep) + ")"
}

func jobTitle(t *string) string {
	if t == nil || *t == "" {
		return ""
	}
	return " (" + *t + ")"
}

func clock(v slaengine.View) string {
	switch {
	case v.MetAt != nil:
		return fmt.Sprintf("%s at %s", v.Phase, at(*v.MetAt))
	case v.DueAt != nil:
		return fmt.Sprintf("%s, due %s", v.Phase, at(*v.DueAt))
	}
	return string(v.Phase)
}

func doneness(d *tickets.TicketDoD) string {
	if d.Met {
		return "met"
	}
	pending := make([]string, len(d.Pending))
	for i, p := range d.Pending {
		pending[i] = string(p)
	}
	if len(pending) == 0 {
		return "not met"
	}
	return "waiting for " + strings.Join(pending, listSep)
}

func messageHeading(m tickets.Message) string {
	switch {
	case m.Visibility == ticket.VisibilityInternal:
		return "Internal note by " + m.Author.Name + " (staff only, never shown to the client)"
	case m.FromSupport:
		return "Reply by " + m.Author.Name + " (support team)"
	}
	return "Message from " + m.Author.Name + jobTitle(m.AuthorJobTitle) + " (client)"
}

func writeAttachments(b *strings.Builder, files []tickets.Attachment) {
	if len(files) == 0 {
		return
	}
	b.WriteString("\nAttachments (read them with read_attachment):\n")
	for _, f := range files {
		fmt.Fprintf(b, "- %s (%s, %s), id %s\n", f.Filename, f.ContentType, humanBytes(f.SizeBytes), f.ID)
	}
}

func renderActivity(key string, entries []tickets.ActivityEntry) string {
	if len(entries) == 0 {
		return "No activity on " + key + " yet."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Activity on %s, oldest first:\n\n", key)
	for _, e := range entries {
		actor := "SimDesk"
		if e.ActorName != nil {
			actor = *e.ActorName
		}
		fmt.Fprintf(&b, "- %s · %s · %s", at(e.At), actor, e.Action)
		if e.From != nil || e.To != nil {
			fmt.Fprintf(&b, ": %s → %s", orNone(e.From), orNone(e.To))
		}
		if e.Cause != nil {
			fmt.Fprintf(&b, " (%s)", *e.Cause)
		}
		if e.Code != nil {
			fmt.Fprintf(&b, " · %s %s %s", e.Code.Provider, e.Code.Kind, e.Code.Ref)
		}
		if e.Reason != nil && *e.Reason != "" {
			fmt.Fprintf(&b, " · reason: %s", *e.Reason)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func orNone(s *string) string {
	if s == nil || *s == "" {
		return none
	}
	return *s
}

func renderAssignees(key string, current *tickets.Ref, people []clients.Assignee) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s is assigned to %s. People who can take it:\n\n", key, refName(current))
	if len(people) == 0 {
		b.WriteString("- nobody: no staff member works this client yet\n")
	}
	for _, p := range people {
		fmt.Fprintf(&b, "- %s%s, %s, %d open tickets, id %s\n", p.Name, jobTitle(p.JobTitle), p.Role, p.OpenTickets, p.ID)
	}
	return b.String()
}

func renderTextFile(name, content string, cut bool, link string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:\n\n```\n%s\n```\n", name, strings.TrimRight(content, "\n"))
	if cut {
		fmt.Fprintf(&b, "\nOnly the first %s are shown. The whole file: %s\n", humanBytes(maxInlineTextBytes), link)
	}
	return b.String()
}

func renderPosted(key string, v ticket.Visibility, link string) string {
	if v == ticket.VisibilityInternal {
		return fmt.Sprintf("Internal note added to %s. Only staff can see it. %s", key, link)
	}
	return fmt.Sprintf("Reply posted on %s. The client sees it and is emailed. %s", key, link)
}

func renderChanged(t tickets.Ticket) string {
	return fmt.Sprintf("%s is now %s, priority %s, assigned to %s.%s", t.Key, t.Status, t.Priority, refName(t.Assignee),
		allowedMoves(t.AllowedStatuses))
}

// explain is a domain error as the model reads it: what went wrong, how to fix it, and the
// stable code.
func explain(ae *apperr.Error) string {
	if ae.Code == codeOther {
		return detailOther + " (" + codeOther + ")"
	}
	msg := ae.Title
	if ae.Detail != "" {
		msg += ". " + ae.Detail
	}
	for _, f := range ae.Fields {
		msg += " " + f.Field + ": " + f.Message + "."
	}
	return msg + " (" + ae.Code + ")"
}
