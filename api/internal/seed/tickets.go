package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/codeflow"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
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
	cause    ticket.Cause  // why the status moved, when it wasn't by hand
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
	age                        int  // minutes before the seed ran
	endsToday                  bool // the last step lands earlier today (local time), ignoring age
	steps                      []demoStep
	messages                   []demoMessage
	code                       []demoCode
}

// Minutes, for readable ages.
const (
	minutesPerHour = 60
	minutesPerDay  = 24 * minutesPerHour
)

// demoTickets are the mock's example tickets (HANDOFF §16 and the board mock), by number, with
// their code activity (Ticket and Integrations mocks). Resolved tickets stay inside their reopen
// window when the seed runs: resolved under 2 days ago is always under 3 working days.
var demoTickets = []demoTicket{
	{
		number: 24826, typ: ticket.TypeIncident, priority: sla.P2, channel: ticket.ChannelPortal,
		client: clientBatam, requester: emailSri, creator: emailSri, what: projectFinanceApps,
		subject:     "SIPD export to Excel times out",
		description: "Exporting the monthly budget realisation to Excel stops with a timeout after about a minute. Smaller exports still work.",
		age:         2 * minutesPerDay,
		steps: []demoStep{
			{15, emailYoga, emailYoga, "", ""}, {40, emailYoga, "", ticket.StatusInProgress, ""},
			{26 * minutesPerHour, emailDewi, "", ticket.StatusInUAT, ticket.CauseCodeDone},
		},
		messages: []demoMessage{
			{12, emailYoga, false, "Hi Bu Sri, we can reproduce the timeout on exports over 5,000 rows. We're working on it."},
			{26*minutesPerHour + 5, emailYoga, false, "Large exports now stream instead of building in memory. Could you export last month's realisation and check it finishes?"},
		},
		code: []demoCode{
			{20 * minutesPerHour, repoSIPD, gitevent.KindCommit, "da1560886d4f094c3e6c9ef40349f7d38b5d27d7",
				"Closes %s: stream large Excel exports", "fix/HD-24826-export", "", emailYoga,
				codeflow.OutcomeFixCommitted, []dod.Item{dod.ItemFixCommitted}, "", ""},
			{26 * minutesPerHour, repoSIPD, gitevent.KindPRMerged, "57", "SIPD export streams large files",
				"fix/HD-24826-export", "release/2026.11", emailDewi,
				codeflow.OutcomeMerged, []dod.Item{dod.ItemMerged}, ticket.StatusInProgress, ticket.StatusInUAT},
		},
	},
	{
		number: 24830, typ: ticket.TypeIncident, priority: sla.P3, channel: ticket.ChannelPortal,
		client: RequesterClient, requester: emailHendra, creator: emailHendra, what: projectEOffice,
		subject:     "Disposition letter PDF shows wrong date",
		description: "Disposition letters printed to PDF after 17:00 show tomorrow's date.",
		age:         40 * minutesPerHour,
		steps: []demoStep{
			{10, emailRina, emailAdi, "", ""}, {30, emailAdi, "", ticket.StatusInProgress, ""},
			{15 * minutesPerHour, emailDewi, "", ticket.StatusInUAT, ticket.CauseCodeDone},
		},
		messages: []demoMessage{
			{25, emailAdi, false, "Thanks Pak Hendra. The PDF uses the server's UTC date; we're switching it to WIB."},
			{15*minutesPerHour + 5, emailAdi, false, "The fix is on the e-Office test server. Could you print a few letters and check the date?"},
		},
		code: []demoCode{
			{14 * minutesPerHour, repoEOffice, gitevent.KindCommit, "c4e1b20f9a8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b",
				"Fix %s: print disposition dates in WIB", "fix/HD-24830-wib", "", emailAdi,
				codeflow.OutcomeFixCommitted, []dod.Item{dod.ItemFixCommitted}, "", ""},
			{14*minutesPerHour + 10, repoEOffice, gitevent.KindPROpened, "138", "Disposition PDF uses WIB dates",
				"fix/HD-24830-wib", "main", emailAdi, codeflow.OutcomeInReview, nil, "", ""},
			{15 * minutesPerHour, repoEOffice, gitevent.KindPRMerged, "138", "Disposition PDF uses WIB dates",
				"fix/HD-24830-wib", "main", emailDewi,
				codeflow.OutcomeMerged, []dod.Item{dod.ItemMerged}, ticket.StatusInProgress, ticket.StatusInUAT},
		},
	},
	{
		number: 24834, typ: ticket.TypeIncident, priority: sla.P3, channel: ticket.ChannelPhone,
		client: RequesterClient, requester: emailDian, creator: emailRina, what: projectWebsite,
		subject:     "News page still shows last month's banner",
		description: "Bu Dian called: the news page shows the old event banner although the new one was uploaded yesterday.",
		age:         2 * minutesPerDay,
		steps: []demoStep{
			{5, emailRina, emailRina, "", ""}, {20, emailRina, "", ticket.StatusInProgress, ""},
			{200, emailRina, "", ticket.StatusResolved, ""},
		},
		messages: []demoMessage{
			{18, emailRina, false, "We cleared the website cache and the new banner shows now. Please refresh the page."},
		},
	},
	{
		number: 24842, typ: ticket.TypeIncident, priority: sla.P3, channel: ticket.ChannelPortal,
		client: clientBatam, requester: emailSri, creator: emailSri, what: projectNetwork,
		subject:     "Shared drive permissions reset",
		description: "Since this morning the finance team can't open the Finance folder on the shared drive. Other folders still work.",
		endsToday:   true,
		steps: []demoStep{
			{10, emailYoga, emailYoga, "", ""}, {25, emailYoga, "", ticket.StatusInProgress, ""},
			{110, emailYoga, "", ticket.StatusInUAT, ""},
			{120, emailSri, "", ticket.StatusResolved, ticket.CauseUATApproved},
		},
		messages: []demoMessage{
			{110, emailYoga, false, "Permissions are back as they were before the weekend. Please check you can open the Finance folder."},
		},
	},
	{
		number: 24851, typ: ticket.TypeRequest, priority: sla.P4, channel: ticket.ChannelPhone,
		client: clientPANRB, requester: emailArif, creator: emailRina, what: "Install licensed software",
		subject:     "Printer driver for new laptops",
		description: "Pak Arif called: the 12 new laptops in the planning division can't print to the shared printer and need its driver.",
		age:         2 * minutesPerDay,
		steps:       []demoStep{{5, emailRina, emailRina, "", ""}},
	},
	{
		number: 24855, typ: ticket.TypeRequest, priority: sla.P2, channel: ticket.ChannelEmail,
		client: clientBintan, requester: emailMaya, creator: emailYoga, what: "Something else",
		subject:     "Renew website SSL certificate",
		description: "Bu Maya emailed: the browser warns that the website's SSL certificate expires in 5 days.",
		age:         minutesPerDay,
		steps: []demoStep{
			{20, emailYoga, emailYoga, "", ""}, {40, emailYoga, "", ticket.StatusInProgress, ""},
			{65, emailYoga, "", ticket.StatusWaitingOnClient, ""},
		},
		messages: []demoMessage{
			{60, emailYoga, false, "Hi Bu Maya, the certificate renews through your domain registrar. Could you add us as the technical contact there?"},
		},
	},
	{
		number: 24859, typ: ticket.TypeRequest, priority: sla.P3, channel: ticket.ChannelPortal,
		client: RequesterClient, requester: emailHendra, creator: emailHendra, what: "VPN / remote access",
		subject:     "VPN access for the new finance officer",
		description: "Our new finance officer starts next week and needs VPN access to the budgeting system.",
		age:         minutesPerDay,
		steps: []demoStep{
			{30, emailRina, emailAdi, "", ""}, {60, emailAdi, "", ticket.StatusInProgress, ""},
			{90, emailAdi, "", ticket.StatusWaitingOnClient, ""},
		},
		messages: []demoMessage{
			{85, emailAdi, false, "Could you send the officer's name, work email and the date access should start?"},
		},
		code: []demoCode{
			{70, repoEOffice, gitevent.KindCommit, "5b2e7aa4c3d2e1f0a9b8c7d6e5f4a3b2c1d0e9f8",
				"refs %s add VPN profile template", "fix/HD-24859", "", emailAdi, codeflow.OutcomeLinked, nil, "", ""},
			{80, repoEOffice, gitevent.KindCommit, "9d03c11e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c",
				"refs %s add retry", "fix/HD-24859", "", emailAdi, codeflow.OutcomeLinked, nil, "", ""},
		},
	},
	{
		number: 24860, typ: ticket.TypeIncident, priority: sla.P3, channel: ticket.ChannelWhatsApp,
		client: RequesterClient, requester: emailDian, creator: emailRina, what: projectEmail,
		subject:     "Mailbox full for Head of Office",
		description: "Bu Dian sent a WhatsApp message: the Head of Office's mailbox is full and new mail bounces.",
		age:         5 * minutesPerHour,
		steps:       []demoStep{{5, emailRina, emailRina, "", ""}, {15, emailRina, "", ticket.StatusInProgress, ""}},
		messages: []demoMessage{
			{14, emailRina, false, "Hi Bu Dian, the mailbox is at its size limit. We're archiving last year's mail to free space."},
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
		steps:       []demoStep{{2, emailRina, emailRina, "", ""}, {38, emailRina, "", ticket.StatusInProgress, ""}},
		messages: []demoMessage{
			{8, emailRina, false, "Thanks Pak Hendra, we're on it. The problem started with this morning's single sign-on update. Next update within 30 minutes."},
			{17, emailRina, true, "Matches last night's SSO patch: the token lifetime dropped to 5 minutes. Adi is preparing a revert."},
		},
		code: []demoCode{
			{40, repoEOffice, gitevent.KindCommit, "7c3d019b2a3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e",
				"refs %s add logging around token refresh", "fix/HD-24871-sso", "", emailAdi, codeflow.OutcomeLinked, nil, "", ""},
			{52, repoEOffice, gitevent.KindCommit, "a1f9c2e5d7b8c9a0b1c2d3e4f5a6b7c8d9e0f1a2",
				"Fix %s: revert SSO token TTL to 8h", "fix/HD-24871-sso", "", emailAdi,
				codeflow.OutcomeFixCommitted, []dod.Item{dod.ItemFixCommitted}, "", ""},
			{65, repoEOffice, gitevent.KindPROpened, "142", "Revert SSO token TTL change", "fix/HD-24871-sso", "main",
				emailAdi, codeflow.OutcomeInReview, nil, "", ""},
		},
	},
	{
		number: 24888, typ: ticket.TypeIncident, priority: sla.P2, channel: ticket.ChannelEmail,
		client: clientBintan, requester: emailMaya, creator: emailYoga, what: projectBintanWeb,
		subject:     "Website shows 502 error on and off",
		description: "Bu Maya emailed: visitors get a 502 Bad Gateway error every few minutes, mostly on the news pages.",
		endsToday:   true,
		steps: []demoStep{
			{10, emailYoga, emailYoga, "", ""}, {20, emailYoga, "", ticket.StatusInProgress, ""},
			{150, emailDewi, "", ticket.StatusResolved, ticket.CauseCodeDone},
		},
		messages: []demoMessage{
			{25, emailYoga, false, "Hi Bu Maya, the proxy in front of the website gives up too early on slow news pages. We're raising its timeout."},
		},
		code: []demoCode{
			{100, repoBintan, gitevent.KindCommit, "e81f4d2a3b4c5d6e7f8091a2b3c4d5e6f7a8b9c0",
				"Fix %s: raise the proxy's upstream timeout", "fix/HD-24888-502", "", emailYoga,
				codeflow.OutcomeFixCommitted, []dod.Item{dod.ItemFixCommitted}, "", ""},
			{120, repoBintan, gitevent.KindPRMerged, "91", "Raise proxy upstream timeout", "fix/HD-24888-502", "main",
				emailDewi, codeflow.OutcomeMerged, []dod.Item{dod.ItemMerged}, "", ""},
			{150, repoBintan, gitevent.KindDeployment, "3301", "production", "main", "", emailDewi,
				codeflow.OutcomeDeployed, []dod.Item{dod.ItemDeployed}, ticket.StatusInProgress, ticket.StatusResolved},
		},
	},
	{
		number: 24890, typ: ticket.TypeRequest, priority: sla.P3, channel: ticket.ChannelEmail,
		client: clientRiau, requester: emailFajar, creator: emailYoga, what: "VPN / remote access",
		subject:     "VPN access for 3 new contractors",
		description: "Pak Fajar emailed: three contractors join the SIPD team on Monday and need VPN access until the end of the year.",
		age:         25,
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

// seedTickets adds the demo tickets that don't exist yet, with their code activity, then
// continues numbering after them.
func seedTickets(ctx context.Context, q *db.Queries, now time.Time, r refs, repos map[string]demoRepoRef) error {
	org, err := q.GetOrganization(ctx)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(org.Timezone)
	if err != nil {
		return fmt.Errorf("organisation time zone: %w", err)
	}
	local := now.In(loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	// earlierToday is halfway between local midnight and now, so "today" holds wherever the
	// seed runs.
	earlierToday := midnight.Add(now.Sub(midnight) / 2)
	for _, d := range demoTickets {
		created := now.Add(-time.Duration(d.age) * time.Minute)
		if d.endsToday && len(d.steps) > 0 {
			created = earlierToday.Add(-time.Duration(d.steps[len(d.steps)-1].after) * time.Minute)
		}
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
			step := tickets.SeedStep{At: at(st.after), Actor: r.user(st.actor), Status: st.status, Cause: st.cause}
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
		key := ticket.FormatKey(org.TicketPrefix, d.number)
		for _, c := range d.code {
			e, err := seedCodeEvent(ctx, q, repos, r, c, key, at(c.after))
			if err != nil {
				return fmt.Errorf("ticket %d code: %w", d.number, err)
			}
			t.Code = append(t.Code, tickets.SeedCode{At: at(c.after), Event: e, Outcome: c.outcome, Items: c.items, From: c.from, To: c.to})
		}
		if _, err := tickets.InsertSeed(ctx, q, t); err != nil {
			return fmt.Errorf("ticket %d: %w", d.number, err)
		}
	}
	return q.SetTicketNumberFloor(ctx)
}
