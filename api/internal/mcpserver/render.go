package mcpserver

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
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
	timeLayout = "2006-01-02 15:04 UTC"
	nobody     = "nobody"
	none       = "none"
	listSep    = ", "
	// guidanceOnly follows a Definition of Done that isn't mandatory: resolving needs no reason.
	guidanceOnly = " (guidance only)"
	codeOnBranch = "code on %s"
	codeOther    = apperr.CodeOtherOrganization
	detailOther  = "This ticket belongs to another organization. A token works only in the organization it was made in."
)

// Fences around text people wrote (see quoted).
const (
	fenceChar   = '`'
	minFenceLen = 3
	// Unicode's line and paragraph separators, which some readers also break lines on.
	lineSeparator      = '\u2028'
	paragraphSeparator = '\u2029'
	space              = ' '
)

func at(t time.Time) string { return t.UTC().Format(timeLayout) }

func refName(r *tickets.Ref) string {
	if r == nil {
		return nobody
	}
	return oneLine(r.Name)
}

// quoted fences text someone wrote (a description, a message, a file) with a fence longer than
// any backtick run inside it, so nothing in it can close the fence and pass for this output's
// own structure, such as a made-up "Internal note" heading.
func quoted(text string) string {
	longest, run := 0, 0
	for _, r := range text {
		if r == fenceChar {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	fence := strings.Repeat(string(fenceChar), max(minFenceLen, longest+1))
	return fence + "\n" + strings.TrimRight(text, "\n") + "\n" + fence
}

// oneLine keeps a short field (a subject, a name, a file name) on one line: line breaks and
// other control characters become spaces, so the field can't start a heading of its own.
func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == lineSeparator || r == paragraphSeparator {
			return space
		}
		return r
	}, s)
}

func renderList(list []tickets.Summary, link func(string) string) string {
	if len(list) == 0 {
		return "No tickets match."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d tickets, most recently updated first:\n\n", len(list))
	for _, t := range list {
		fmt.Fprintf(&b, "- **%s** %s\n  %s · %s · %s · client %s · assignee %s · updated %s · %s\n",
			t.Key, oneLine(t.Subject), t.Status, t.Priority, t.Type, oneLine(t.Client.Name), refName(t.Assignee), at(t.UpdatedAt), link(t.Key))
	}
	return b.String()
}

func renderTicket(t tickets.Ticket, msgs []tickets.Message, link string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: %s\n\n", t.Key, oneLine(t.Subject))
	fmt.Fprintf(&b, "- Link: %s\n", link)
	fmt.Fprintf(&b, "- Status: %s%s\n", t.Status, allowedMoves(t.AllowedStatuses))
	fmt.Fprintf(&b, "- Priority: %s\n- Type: %s\n", t.Priority, t.Type)
	fmt.Fprintf(&b, "- Client: %s\n- Project: %s\n", oneLine(t.Client.Name), oneLine(t.Project.Name))
	if t.CatalogItem != nil {
		fmt.Fprintf(&b, "- Request: %s\n", oneLine(t.CatalogItem.Name))
	}
	fmt.Fprintf(&b, "- Requester: %s%s\n", oneLine(t.Requester.Name), jobTitle(t.RequesterJobTitle))
	fmt.Fprintf(&b, "- Assignee: %s\n", refName(t.Assignee))
	fmt.Fprintf(&b, "- Created: %s via %s\n- Updated: %s\n", at(t.CreatedAt), t.Channel, at(t.UpdatedAt))
	if t.SLAClocks != nil {
		fmt.Fprintf(&b, "- SLA first response: %s\n", clock(t.SLAClocks.FirstResponse))
		fmt.Fprintf(&b, "- SLA resolution: %s\n", clock(t.SLAClocks.Resolution))
	}
	if t.DoD != nil {
		fmt.Fprintf(&b, "- Definition of Done: %s\n", doneness(t.DoD))
	}
	fmt.Fprintf(&b, "\n## Description\n\n%s\n", quoted(t.Description))
	writeAttachments(&b, t.Attachments)
	fmt.Fprintf(&b, "\n## Conversation (%d messages, oldest first)\n", len(msgs))
	for _, m := range msgs {
		fmt.Fprintf(&b, "\n### %s, %s\n\n%s\n", messageHeading(m), at(m.CreatedAt), quoted(m.Body))
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
	return " (" + oneLine(*t) + ")"
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
	suffix := ""
	if !d.Mandatory {
		suffix = guidanceOnly
	}
	if d.Met {
		return "met" + suffix
	}
	pending := make([]string, len(d.Pending))
	for i, p := range d.Pending {
		pending[i] = string(p)
		if p == dod.ItemCodeOnBranch {
			pending[i] = fmt.Sprintf(codeOnBranch, d.Config.CodeOnBranch.Branch)
		}
	}
	if len(pending) == 0 {
		return "not met" + suffix
	}
	return "waiting for " + strings.Join(pending, listSep) + suffix
}

func messageHeading(m tickets.Message) string {
	switch {
	case m.Visibility == ticket.VisibilityInternal:
		return "Internal note by " + oneLine(m.Author.Name) + " (staff only, never shown to the client)"
	case m.FromSupport:
		return "Reply by " + oneLine(m.Author.Name) + " (support team)"
	}
	return "Message from " + oneLine(m.Author.Name) + jobTitle(m.AuthorJobTitle) + " (client)"
}

func writeAttachments(b *strings.Builder, files []tickets.Attachment) {
	if len(files) == 0 {
		return
	}
	b.WriteString("\nAttachments (read them with read_attachment):\n")
	for _, f := range files {
		fmt.Fprintf(b, "- %s (%s, %s), id %s\n", oneLine(f.Filename), f.ContentType, humanBytes(f.SizeBytes), f.ID)
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
			actor = oneLine(*e.ActorName)
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
			fmt.Fprintf(&b, " · reason: %s", oneLine(*e.Reason))
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

// noteSelfAssignOnly tells an agent what assign_ticket lets them do (ticket.SelfAssignment).
const noteSelfAssignOnly = "\nAs an agent you can take this ticket while it is unassigned (assignee me) or release it " +
	"while it is yours (assignee none). Only a supervisor or administrator can give it to someone else.\n"

// renderAssignees lists who can work the ticket; canAssignAny is false for agents, who only take
// or release.
func renderAssignees(key string, current *tickets.Ref, people []clients.Assignee, canAssignAny bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s is assigned to %s. People who can take it:\n\n", key, refName(current))
	if len(people) == 0 {
		b.WriteString("- nobody: no staff member works this client yet\n")
	}
	for _, p := range people {
		fmt.Fprintf(&b, "- %s%s, %s, %d open tickets, id %s\n", oneLine(p.Name), jobTitle(p.JobTitle), p.Role, p.OpenTickets, p.ID)
	}
	if !canAssignAny {
		b.WriteString(noteSelfAssignOnly)
	}
	return b.String()
}

func renderTextFile(name, content string, cut bool, link string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:\n\n%s\n", oneLine(name), quoted(content))
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
