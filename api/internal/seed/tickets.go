package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// refs resolves the names used in the demo data to seeded IDs.
type refs struct {
	clients  map[string]uuid.UUID
	users    map[string]uuid.UUID            // by email
	projects map[string]map[string]uuid.UUID // by client, then project name
	items    map[string]uuid.UUID            // catalogue items by name
}

// defaultProject is the client's first demo project, which seedProjects makes its default.
func (r refs) defaultProject(client string) uuid.UUID {
	for _, p := range demoProjects {
		if p.client == client {
			return r.projects[client][p.name]
		}
	}
	panic("seed: no project for " + client)
}

func (r refs) user(email string) tickets.Ref {
	for _, p := range people {
		if p.email == email {
			return tickets.Ref{ID: r.users[email], Name: p.name}
		}
	}
	panic("seed: unknown user " + email)
}

// demoStep and demoMessage happen the given minutes after the ticket was opened.
type demoStep struct {
	after    int
	actor    string
	assignee string        // email; empty keeps the assignee
	status   ticket.Status // empty keeps the status
}

type demoMessage struct {
	after    int
	author   string
	internal bool
	body     string
}

type demoTicket struct {
	number                     int64
	typ                        ticket.Type
	priority                   sla.Priority
	channel                    ticket.Channel
	client, requester, creator string
	what                       string // project (incident) or catalogue item (request)
	subject, description       string
	age                        int // minutes before the seed ran
	steps                      []demoStep
	messages                   []demoMessage
}

// Minutes, for readable ages.
const (
	minutesPerHour = 60
	minutesPerDay  = 24 * minutesPerHour
)

// demoTickets are the mock's example tickets (HANDOFF §16), oldest first. Code activity
// arrives with M7.
var demoTickets = []demoTicket{
	{
		number: 24826, typ: ticket.TypeIncident, priority: sla.P2, channel: ticket.ChannelPortal,
		client: clientBatam, requester: emailSri, creator: emailSri, what: projectFinanceApps,
		subject:     "SIPD export to Excel times out",
		description: "Exporting the monthly budget realisation to Excel stops with a timeout after about a minute. Smaller exports still work.",
		age:         2 * minutesPerDay,
		steps:       []demoStep{{15, emailYoga, emailYoga, ""}, {40, emailYoga, "", ticket.StatusInProgress}},
		messages: []demoMessage{
			{12, emailYoga, false, "Hi Bu Sri, we can reproduce the timeout on exports over 5,000 rows. We're working on it."},
		},
	},
	{
		number: 24830, typ: ticket.TypeIncident, priority: sla.P3, channel: ticket.ChannelPortal,
		client: RequesterClient, requester: emailHendra, creator: emailHendra, what: projectEOffice,
		subject:     "Disposition letter PDF shows wrong date",
		description: "Disposition letters printed to PDF after 17:00 show tomorrow's date.",
		age:         40 * minutesPerHour,
		steps: []demoStep{
			{10, emailRina, emailAdi, ""}, {30, emailAdi, "", ticket.StatusInProgress},
			{15 * minutesPerHour, emailAdi, "", ticket.StatusInUAT},
		},
		messages: []demoMessage{
			{25, emailAdi, false, "Thanks Pak Hendra. The PDF uses the server's UTC date; we're switching it to WIB."},
			{15 * minutesPerHour, emailAdi, false, "The fix is on the e-Office test server. Could you print a few letters and check the date?"},
		},
	},
	{
		number: 24834, typ: ticket.TypeIncident, priority: sla.P3, channel: ticket.ChannelPhone,
		client: RequesterClient, requester: emailDian, creator: emailRina, what: projectWebsite,
		subject:     "News page still shows last month's banner",
		description: "Bu Dian called: the news page shows the old event banner although the new one was uploaded yesterday.",
		age:         3 * minutesPerDay,
		steps: []demoStep{
			{5, emailRina, emailRina, ""}, {20, emailRina, "", ticket.StatusInProgress},
			{200, emailRina, "", ticket.StatusResolved},
		},
		messages: []demoMessage{
			{18, emailRina, false, "We cleared the website cache and the new banner shows now. Please refresh the page."},
		},
	},
	{
		number: 24859, typ: ticket.TypeRequest, priority: sla.P3, channel: ticket.ChannelPortal,
		client: RequesterClient, requester: emailHendra, creator: emailHendra, what: "VPN / remote access",
		subject:     "VPN access for the new finance officer",
		description: "Our new finance officer starts next week and needs VPN access to the budgeting system.",
		age:         minutesPerDay,
		steps: []demoStep{
			{30, emailRina, emailAdi, ""}, {60, emailAdi, "", ticket.StatusInProgress},
			{90, emailAdi, "", ticket.StatusWaitingOnClient},
		},
		messages: []demoMessage{
			{85, emailAdi, false, "Could you send the officer's name, work email and the date access should start?"},
		},
	},
	{
		number: 24866, typ: ticket.TypeIncident, priority: sla.P2, channel: ticket.ChannelEmail,
		client: RequesterClient, requester: emailHendra, creator: emailRina, what: projectEmail,
		subject:     "Shared mailbox stopped receiving mail",
		description: "Pak Hendra emailed: the public complaints mailbox has received nothing since this morning.",
		age:         minutesPerHour,
	},
	{
		number: 24871, typ: ticket.TypeIncident, priority: sla.P1, channel: ticket.ChannelPortal,
		client: RequesterClient, requester: emailHendra, creator: emailHendra, what: projectEOffice,
		subject:     "e-Office login fails for all staff after this morning's update",
		description: "Since 08:00 nobody can sign in to e-Office. The page says \"Session expired\" right after the password is entered. Every department is affected.",
		age:         3 * minutesPerHour,
		steps:       []demoStep{{2, emailRina, emailRina, ""}, {38, emailRina, "", ticket.StatusInProgress}},
		messages: []demoMessage{
			{8, emailRina, false, "Thanks Pak Hendra, we're on it. The problem started with this morning's single sign-on update. Next update within 30 minutes."},
			{17, emailRina, true, "Matches last night's SSO patch: the token lifetime dropped to 5 minutes. Adi is preparing a revert."},
		},
	},
}

// seedCatalog upserts the mock's catalogue items.
func seedCatalog(ctx context.Context, q *db.Queries) (map[string]uuid.UUID, error) {
	out := map[string]uuid.UUID{}
	for i, e := range catalogItems {
		id, err := q.UpsertCatalogItemByName(ctx, db.UpsertCatalogItemByNameParams{
			Name: e.name, Description: optional(e.description), SortOrder: int32((i + 1) * sortStep),
		})
		if err != nil {
			return nil, fmt.Errorf("catalogue item %s: %w", e.name, err)
		}
		out[e.name] = id
	}
	return out, nil
}

// seedTickets adds the demo tickets that don't exist yet, then continues numbering after them.
func seedTickets(ctx context.Context, q *db.Queries, now time.Time, r refs) error {
	org, err := q.GetOrganization(ctx)
	if err != nil {
		return err
	}
	for _, d := range demoTickets {
		created := now.Add(-time.Duration(d.age) * time.Minute)
		at := func(after int) time.Time { return created.Add(time.Duration(after) * time.Minute) }
		t := tickets.SeedTicket{
			Number: d.number, Prefix: org.TicketPrefix, Type: d.typ, Priority: d.priority, Channel: d.channel,
			ClientID: r.clients[d.client], Requester: r.user(d.requester), CreatedBy: r.user(d.creator),
			Subject: d.subject, Description: d.description, CreatedAt: created,
		}
		if d.typ == ticket.TypeIncident {
			t.ProjectID = r.projects[d.client][d.what]
		} else {
			item := r.items[d.what]
			t.ProjectID, t.CatalogItemID = r.defaultProject(d.client), &item
		}
		for _, st := range d.steps {
			step := tickets.SeedStep{At: at(st.after), Actor: r.user(st.actor), Status: st.status}
			if st.assignee != "" {
				a := r.user(st.assignee)
				step.Assignee = &a
			}
			t.Steps = append(t.Steps, step)
		}
		for _, m := range d.messages {
			author := r.user(m.author)
			msg := tickets.SeedMessage{
				At: at(m.after), Author: author, FromSupport: author.ID != t.Requester.ID, Body: m.body,
				Visibility: ticket.VisibilityPublic,
			}
			if m.internal {
				msg.Visibility = ticket.VisibilityInternal
			}
			t.Messages = append(t.Messages, msg)
		}
		if _, err := tickets.InsertSeed(ctx, q, t); err != nil {
			return fmt.Errorf("ticket %d: %w", d.number, err)
		}
	}
	return q.SetTicketNumberFloor(ctx)
}
