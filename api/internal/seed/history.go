package seed

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/report"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// History: closed tickets over the two months before this one and this month so far, so the
// SLA report has months to compare. Nothing here notifies anyone: the notification replay reads
// the demo tickets only, and the SLA scan backfills these clocks silently.

// historyMonths is how many months back the history starts; the current month is included.
const historyMonths = 2

// historyFirstNumber numbers history tickets below the demo tickets (HD-24826 and up).
const historyFirstNumber = 24000

// historyQuiet keeps history this far from now, so its tickets are closed before the seed runs
// and never reach the board or the reopen window.
const historyQuiet = 24 * time.Hour

// Business hours the history is created in (the demo calendar opens 08:00 to 17:00).
const (
	historyOpenHour  = 8
	historyHourSpan  = 7  // created 08:00 to 14:59
	historyMinuteMix = 13 // spreads minutes past the hour
	historyHourMix   = 5  // spreads hours through the day
	historyDay       = 24 * time.Hour
)

// Lead times after creation, per priority. "On time" stays inside every demo project's targets;
// "late" misses them even across a weekend. Late replies are only given to tickets opened at
// 08:00, so the business clock has the whole day to run out.
var (
	replyOnTime = map[sla.Priority]time.Duration{
		sla.P1: 8 * time.Minute, sla.P2: 15 * time.Minute, sla.P3: 35 * time.Minute, sla.P4: 70 * time.Minute,
	}
	replyLate = map[sla.Priority]time.Duration{
		sla.P1: 45 * time.Minute, sla.P2: 90 * time.Minute, sla.P3: 5 * time.Hour, sla.P4: 9 * time.Hour,
	}
	resolveOnTime = map[sla.Priority]time.Duration{
		sla.P1: 80 * time.Minute, sla.P2: 3 * time.Hour, sla.P3: 6 * time.Hour, sla.P4: 24 * time.Hour,
	}
	resolveLate = map[sla.Priority]time.Duration{
		sla.P1: 5 * time.Hour, sla.P2: 3 * historyDay, sla.P3: 5 * historyDay, sla.P4: 10 * historyDay,
	}
)

// testLead is how long a client takes to approve a test; closeAfter is past the 3-working-day
// reopen window, when the lifecycle scan would have closed the ticket.
const (
	testLead   = 2 * time.Hour
	closeAfter = 5 * historyDay
)

// historyPriorities repeats over each client's month: mostly P3, some P2 and P4, a rare P1.
var historyPriorities = []sla.Priority{sla.P3, sla.P2, sla.P3, sla.P4, sla.P3, sla.P3, sla.P2, sla.P3, sla.P1, sla.P4}

// requestEvery makes every fourth ticket a service request.
const requestEvery = 4

// loggedChannels are how agent-logged tickets arrived, in turn.
var loggedChannels = []ticket.Channel{ticket.ChannelPhone, ticket.ChannelWhatsApp, ticket.ChannelEmail}

// historyClient is one client's history: tickets per full month, who reports them, and how many
// tickets miss a target in each month, oldest first (the current month last).
type historyClient struct {
	client    string
	perMonth  int
	requester string // email
	portal    bool   // the requester reports in the portal; otherwise agents log the calls
	late      [historyMonths + 1]int
}

// Riau falls below the 90% target in every month; Kominfo in none.
var historyClients = []historyClient{
	{client: RequesterClient, perMonth: 15, requester: emailHendra, portal: true, late: [3]int{1, 1, 0}},
	{client: clientBatam, perMonth: 12, requester: emailSri, portal: true, late: [3]int{1, 0, 1}},
	{client: clientRiau, perMonth: 8, requester: emailFajar, late: [3]int{1, 2, 1}},
	{client: clientBintan, perMonth: 6, requester: emailMaya},
	{client: clientPANRB, perMonth: 5, requester: emailArif},
}

// historySubjects are what each project's incidents were about, in turn.
var historySubjects = map[string][]string{
	projectEOffice: {"Disposition not showing in the head of division's inbox", "Letter numbering skips a number",
		"Cannot attach a scanned letter", "Digital signature fails on outgoing letters"},
	projectWebsite: {"News page shows an old article first", "Contact form not sending"},
	projectEmail:   {"Mailbox full warning", "Cannot sign in to webmail on a phone"},
	projectNetwork: {"Wi-Fi drops in the meeting room", "VPN disconnects every hour", "Slow internet on the 2nd floor"},
	projectFinanceApps: {"Budget report totals do not match", "Cannot export the cash book",
		"Approval stuck at the second level"},
	projectSIPD:      {"SIPD sync failed overnight", "Account codes missing after the update"},
	projectBintanWeb: {"Tourism page images not loading", "Announcement page returns an error"},
	projectGeneral:   {"Printer on floor 3 offline", "Laptop will not start", "Shared drive not visible"},
}

var historyRequests = []string{"New email account", "VPN / remote access", "Install licensed software"}

const (
	historyDescription = "Reported by the client; details are in the conversation."
	historyReply       = "Thank you, we are on it and will update you here."
	historyFixed       = "This is fixed now. Please let us know if anything still looks wrong."
)

// seedHistory adds the closed history tickets that don't exist yet.
func seedHistory(ctx context.Context, q *db.Queries, now time.Time, r refs) error {
	org, err := q.GetOrganization(ctx)
	if err != nil {
		return err
	}
	loc, err := sla.ParseLocation(org.Timezone)
	if err != nil {
		return err
	}
	current := report.MonthOf(now, loc)
	var all []tickets.SeedTicket
	for back := historyMonths; back >= 0; back-- {
		m := current
		for range back {
			m = m.Prev()
		}
		start, end := m.Bounds(loc)
		for _, hc := range historyClients {
			all = append(all, historyFor(hc, historyMonths-back, start, end, now, r)...)
		}
	}
	slices.SortStableFunc(all, func(a, b tickets.SeedTicket) int { return a.CreatedAt.Compare(b.CreatedAt) })
	for i, t := range all {
		t.Number, t.Prefix = historyFirstNumber+int64(i), org.TicketPrefix
		if _, err := tickets.InsertSeed(ctx, q, t); err != nil {
			return fmt.Errorf("history ticket %d: %w", t.Number, err)
		}
	}
	return nil
}

// historyFor builds one client's tickets for one month, spread over its weekdays. A month in
// progress gets the share of tickets for the days gone by.
func historyFor(hc historyClient, monthIndex int, start, end, now time.Time, r refs) []tickets.SeedTicket {
	last := now.Add(-historyQuiet)
	if end.Before(last) {
		last = end
	}
	if !last.After(start) {
		return nil
	}
	days := int(last.Sub(start) / historyDay)
	fullDays := int(end.Sub(start) / historyDay)
	count := hc.perMonth * days / fullDays
	// The late tickets are spread evenly through the month.
	lateAt := map[int]bool{}
	late := min(hc.late[monthIndex], count)
	for k := range late {
		lateAt[(k+1)*count/(late+1)] = true
	}
	agents := clientAgents(hc.client)
	requester := r.user(hc.requester)
	projects := clientProjects(hc.client)
	var out []tickets.SeedTicket
	for i := range count {
		isLate := lateAt[i]
		day := start.AddDate(0, 0, i*days/count)
		day = weekday(day, start, last)
		hour := historyOpenHour + (i*historyHourMix)%historyHourSpan
		if isLate {
			hour = historyOpenHour
		}
		created := time.Date(day.Year(), day.Month(), day.Day(), hour, (i*historyMinuteMix)%minutesPerHour, 0, 0, day.Location())
		p := historyPriorities[i%len(historyPriorities)]
		agent := r.user(agents[i%len(agents)])
		t := tickets.SeedTicket{
			Priority: p, ClientID: r.clients[hc.client], Requester: requester, CreatedBy: requester,
			Description: historyDescription, CreatedAt: created, Channel: ticket.ChannelPortal,
		}
		proj := projects[i%len(projects)]
		if (i+1)%requestEvery == 0 {
			item := historyRequests[i%len(historyRequests)]
			id := r.items[item]
			t.Type, t.Subject, t.CatalogItemID = ticket.TypeRequest, item, &id
			proj = projects[0]
		} else {
			subjects := historySubjects[proj.name]
			t.Type, t.Subject = ticket.TypeIncident, subjects[i%len(subjects)]
		}
		t.ProjectID = r.projects[hc.client][proj.name]

		// Late tickets alternate: a slow first reply (portal only), or a slow fix.
		lateReply := isLate && hc.portal && i%2 == 0
		replyAt := created.Add(replyOnTime[p])
		if lateReply {
			replyAt = created.Add(replyLate[p])
		}
		resolveAt := created.Add(resolveOnTime[p])
		if isLate && !lateReply {
			resolveAt = created.Add(resolveLate[p])
		}
		resolveAt = later(resolveAt, replyAt)
		if hc.portal {
			t.Messages = append(t.Messages, tickets.SeedMessage{
				At: replyAt, Author: agent, FromSupport: true, Visibility: ticket.VisibilityPublic, Body: historyReply,
			})
		} else {
			t.Channel, t.CreatedBy, replyAt = loggedChannels[i%len(loggedChannels)], agent, created
		}
		t.Steps = append(t.Steps, tickets.SeedStep{At: replyAt, Actor: agent, Assignee: &agent, Status: ticket.StatusInProgress})
		t.Messages = append(t.Messages, tickets.SeedMessage{
			At: resolveAt, Author: agent, FromSupport: true, Visibility: ticket.VisibilityPublic, Body: historyFixed,
		})
		if len(dod.ClientRequired(proj.dod)) > 0 {
			t.Steps = append(t.Steps,
				tickets.SeedStep{At: resolveAt, Actor: agent, Status: ticket.StatusInUAT},
				tickets.SeedStep{At: resolveAt.Add(testLead), Actor: requester, Status: ticket.StatusResolved, Cause: ticket.CauseUATApproved})
			resolveAt = resolveAt.Add(testLead)
		} else {
			t.Steps = append(t.Steps, tickets.SeedStep{At: resolveAt, Actor: agent, Status: ticket.StatusResolved})
		}
		closedAt := resolveAt.Add(closeAfter)
		if closedAt.After(now.Add(-historyQuiet)) {
			continue // still inside its reopen window: it would sit on the board
		}
		t.Steps = append(t.Steps, tickets.SeedStep{At: closedAt, Status: ticket.StatusClosed, Cause: ticket.CauseAutoClosed})
		out = append(out, t)
	}
	return out
}

// Days from a weekend day to the Monday after and to the Friday before.
var (
	toMonday = map[time.Weekday]int{time.Saturday: 2, time.Sunday: 1}
	toFriday = map[time.Weekday]int{time.Saturday: -1, time.Sunday: -2}
)

// weekday moves a weekend day to the Monday after, or to the Friday before when that Monday is
// past last.
func weekday(day, first, last time.Time) time.Time {
	ahead, weekend := toMonday[day.Weekday()]
	if !weekend {
		return day
	}
	if next := day.AddDate(0, 0, ahead); next.Before(last) {
		return next
	}
	if back := day.AddDate(0, 0, toFriday[day.Weekday()]); !back.Before(first) {
		return back
	}
	return day
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func clientAgents(name string) []string {
	for _, c := range clients {
		if c.name == name {
			return c.agents
		}
	}
	panic("seed: no client " + name)
}

func clientProjects(name string) []project {
	var out []project
	for _, p := range demoProjects {
		if p.client == name {
			out = append(out, p)
		}
	}
	return out
}
