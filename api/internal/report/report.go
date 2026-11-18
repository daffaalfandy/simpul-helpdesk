package report

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// TargetPercent is the contract target: the share of a project's tickets within SLA (HANDOFF §10).
const TargetPercent = 90.0

const percentScale = 100

// Ticket is one ticket reported in the month, with its SLA outcome so far.
type Ticket struct {
	ProjectID uuid.UUID
	Priority  sla.Priority
	Type      ticket.Type
	Channel   ticket.Channel
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

// Ref names a client.
type Ref struct {
	ID   uuid.UUID
	Name string
}

// Project is one project of the organization, with its client.
type Project struct {
	ID     uuid.UUID
	Name   string
	Client Ref
	// Active is false for a hidden project: off the report form, its tickets kept.
	Active bool
}

// ProjectGroup is one row of the report: a project with its figures.
type ProjectGroup struct {
	Project
	Figures
}

// PriorityGroup is one priority's figures.
type PriorityGroup struct {
	Priority sla.Priority
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

// Report is the month for every project.
type Report struct {
	Totals Figures
	// Projects lists the projects busiest first, then by client and project name.
	Projects            []ProjectGroup
	ProjectsWithTickets int
	ProjectsBelowTarget int
}

// Build groups rows by project. Every active project gets a row, with or without tickets; a
// hidden one only when it has tickets in the month. Rows of projects not listed are left out.
func Build(projects []Project, rows []Ticket) Report {
	byProject := make(map[uuid.UUID][]Ticket, len(projects))
	for _, r := range rows {
		byProject[r.ProjectID] = append(byProject[r.ProjectID], r)
	}
	rep := Report{Projects: make([]ProjectGroup, 0, len(projects))}
	var counted []Ticket
	for _, p := range projects {
		own := byProject[p.ID]
		if !p.Active && len(own) == 0 {
			continue
		}
		counted = append(counted, own...)
		g := ProjectGroup{Project: p, Figures: Summarize(own)}
		if g.Tickets > 0 {
			rep.ProjectsWithTickets++
		}
		if g.BelowTarget() {
			rep.ProjectsBelowTarget++
		}
		rep.Projects = append(rep.Projects, g)
	}
	rep.Totals = Summarize(counted)
	slices.SortStableFunc(rep.Projects, func(a, b ProjectGroup) int {
		if c := cmp.Compare(b.Tickets, a.Tickets); c != 0 {
			return c
		}
		if c := cmp.Compare(strings.ToLower(a.Client.Name), strings.ToLower(b.Client.Name)); c != 0 {
			return c
		}
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return rep
}

// ByPriority groups rows by priority, P1 first. Only priorities with tickets appear.
func ByPriority(rows []Ticket) []PriorityGroup {
	byPriority := make(map[sla.Priority][]Ticket, len(sla.Priorities))
	for _, r := range rows {
		byPriority[r.Priority] = append(byPriority[r.Priority], r)
	}
	var out []PriorityGroup
	for _, p := range sla.Priorities {
		if own := byPriority[p]; len(own) > 0 {
			out = append(out, PriorityGroup{Priority: p, Figures: Summarize(own)})
		}
	}
	return out
}

// HoursMinutes splits d into whole hours and the minutes left, rounded down. Report times are
// on business hours, where a "day" has no single length, so they never use days.
func HoursMinutes(d time.Duration) (hours, minutes int) {
	d = max(d, 0)
	return int(d / time.Hour), int(d % time.Hour / time.Minute)
}

// Outcome is where one SLA clock stands, for the report's ticket list.
type Outcome string

const (
	OutcomeMet      Outcome = "met"
	OutcomeBreached Outcome = "breached"
	OutcomeRunning  Outcome = "running"
)

// ClockOutcome reads a clock: breached when it missed its target (met late included), met when it
// was met or the ticket is done without a breach (a ticket resolved before any reply), else running.
func ClockOutcome(met, breached, done bool) Outcome {
	switch {
	case breached:
		return OutcomeBreached
	case met || done:
		return OutcomeMet
	default:
		return OutcomeRunning
	}
}
