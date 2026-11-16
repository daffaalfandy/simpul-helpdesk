package report

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// TargetPercent is the contract target: the share of a client's tickets within SLA (HANDOFF §10).
const TargetPercent = 90.0

const percentScale = 100

// Ticket is one ticket reported in the month, with its SLA outcome so far.
type Ticket struct {
	ClientID    uuid.UUID
	ProjectID   uuid.UUID
	ProjectName string
	Type        ticket.Type
	Channel     ticket.Channel
	// FirstResponse is the time on the clock until the first public staff reply; nil while
	// unanswered or before the SLA scan has replayed the ticket.
	FirstResponse         *time.Duration
	FirstResponseBreached bool
	ResolutionBreached    bool
	// PausesWhileWaiting is the ticket's SLA snapshot: its resolution clock stops while
	// waiting on the client.
	PausesWhileWaiting bool
}

// Breached reports whether the ticket missed a target. A target met late counts.
func (t Ticket) Breached() bool { return t.FirstResponseBreached || t.ResolutionBreached }

// Figures are the report's numbers for a set of tickets.
type Figures struct {
	Tickets   int
	Incidents int
	Requests  int
	// Breaches counts tickets, not clocks: a ticket that missed both targets is one breach.
	Breaches int
	// WithinPercent is the share of tickets with no breach; nil without tickets.
	WithinPercent *float64
	// AvgFirstResponse covers portal reports that have been answered. Tickets the team logs
	// are answered when logged, so they would only pull the average to zero. Nil when none.
	AvgFirstResponse *time.Duration
}

// BelowTarget reports whether tickets exist and too few were within SLA.
func (f Figures) BelowTarget() bool {
	return f.WithinPercent != nil && *f.WithinPercent < TargetPercent
}

// Summarize computes the figures for rows.
func Summarize(rows []Ticket) Figures {
	var f Figures
	var answered int
	var firstResponseTotal time.Duration
	for _, r := range rows {
		f.Tickets++
		switch r.Type {
		case ticket.TypeIncident:
			f.Incidents++
		case ticket.TypeRequest:
			f.Requests++
		}
		if r.Breached() {
			f.Breaches++
		}
		if r.Channel == ticket.ChannelPortal && r.FirstResponse != nil {
			answered++
			firstResponseTotal += *r.FirstResponse
		}
	}
	if f.Tickets > 0 {
		pct := float64(f.Tickets-f.Breaches) * percentScale / float64(f.Tickets)
		f.WithinPercent = &pct
	}
	if answered > 0 {
		avg := firstResponseTotal / time.Duration(answered)
		f.AvgFirstResponse = &avg
	}
	return f
}

// Ref names a client or project.
type Ref struct {
	ID   uuid.UUID
	Name string
}

// Group is one row of the report: a client or a project with its figures.
type Group struct {
	Ref
	Figures
}

// WaitingPause says how many of the month's tickets stop their resolution clock while waiting
// on the client, so the report's footnote only claims what the clocks did.
type WaitingPause string

const (
	WaitingPauseAll  WaitingPause = "all"
	WaitingPauseSome WaitingPause = "some"
	WaitingPauseNone WaitingPause = "none"
)

// WaitingPauseOf reports the pause rule across rows; ok is false without tickets.
func WaitingPauseOf(rows []Ticket) (WaitingPause, bool) {
	if len(rows) == 0 {
		return "", false
	}
	pausing := 0
	for _, r := range rows {
		if r.PausesWhileWaiting {
			pausing++
		}
	}
	switch pausing {
	case len(rows):
		return WaitingPauseAll, true
	case 0:
		return WaitingPauseNone, true
	default:
		return WaitingPauseSome, true
	}
}

// Report is the month for every client.
type Report struct {
	Totals Figures
	// Clients lists every client, busiest first; clients without tickets come last by name.
	Clients            []Group
	ClientsWithTickets int
	ClientsBelowTarget int
}

// Build groups rows by client. Every client in clients gets a row, with or without tickets;
// rows of clients not listed are left out.
func Build(clients []Ref, rows []Ticket) Report {
	byClient := make(map[uuid.UUID][]Ticket, len(clients))
	for _, r := range rows {
		byClient[r.ClientID] = append(byClient[r.ClientID], r)
	}
	rep := Report{Clients: make([]Group, 0, len(clients))}
	var counted []Ticket
	for _, c := range clients {
		own := byClient[c.ID]
		counted = append(counted, own...)
		g := Group{Ref: c, Figures: Summarize(own)}
		if g.Tickets > 0 {
			rep.ClientsWithTickets++
		}
		if g.BelowTarget() {
			rep.ClientsBelowTarget++
		}
		rep.Clients = append(rep.Clients, g)
	}
	rep.Totals = Summarize(counted)
	sortGroups(rep.Clients)
	return rep
}

// Projects groups rows by project, busiest first. Only projects with tickets appear.
func Projects(rows []Ticket) []Group {
	var groups []Group
	index := map[uuid.UUID]int{}
	byProject := map[uuid.UUID][]Ticket{}
	for _, r := range rows {
		if _, ok := index[r.ProjectID]; !ok {
			index[r.ProjectID] = len(groups)
			groups = append(groups, Group{Ref: Ref{ID: r.ProjectID, Name: r.ProjectName}})
		}
		byProject[r.ProjectID] = append(byProject[r.ProjectID], r)
	}
	for i := range groups {
		groups[i].Figures = Summarize(byProject[groups[i].ID])
	}
	sortGroups(groups)
	return groups
}

func sortGroups(gs []Group) {
	slices.SortStableFunc(gs, func(a, b Group) int {
		if c := cmp.Compare(b.Tickets, a.Tickets); c != 0 {
			return c
		}
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
}

// HoursMinutes splits d into whole hours and the minutes left, rounded down. Report times are
// on business hours, where a "day" has no single length, so they never use days.
func HoursMinutes(d time.Duration) (hours, minutes int) {
	d = max(d, 0)
	return int(d / time.Hour), int(d % time.Hour / time.Minute)
}
