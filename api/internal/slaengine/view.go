package slaengine

import (
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// Phase is a clock's state as people read it. Values are shown in the API.
type Phase string

const (
	PhaseRunning  Phase = "running"
	PhaseAtRisk   Phase = "at_risk"
	PhaseBreached Phase = "breached"
	PhasePaused   Phase = "paused"
	PhaseMet      Phase = "met"
	PhaseMissed   Phase = "missed"  // met, but after the deadline
	PhaseStopped  Phase = "stopped" // the ticket was resolved before this clock was met
)

var Phases = []Phase{PhaseRunning, PhaseAtRisk, PhaseBreached, PhasePaused, PhaseMet, PhaseMissed, PhaseStopped}

// Stored is a clock as the database keeps it.
type Stored struct {
	Budget       time.Duration
	Calendar     sla.CalendarKind
	Elapsed      time.Duration
	RunningSince *time.Time
	MetAt        *time.Time
	DueAt        *time.Time
	AtRisk       bool // flagged by the scan
	Breached     bool // flagged by the scan, or when the clock stopped late
}

// View is a clock at a moment, for the board and ticket pages.
type View struct {
	Phase     Phase
	Used      time.Duration
	Budget    time.Duration
	Remaining time.Duration // negative once over
	Day       time.Duration // what "1d" means on this clock's calendar
	DueAt     *time.Time
	MetAt     *time.Time
}

// ViewAt reads a stored clock at now. Flags decide at risk and breached, so the board agrees
// with the alerts; the numbers are live. open says the ticket is still being worked on, which
// tells a paused clock from a stopped one.
func ViewAt(c Stored, s sla.Schedules, open bool, now time.Time) View {
	used := c.Elapsed
	if c.RunningSince != nil {
		used += span(s.For(c.Calendar), *c.RunningSince, now)
	}
	v := View{
		Used: used, Budget: c.Budget, Remaining: c.Budget - used, Day: DayLength(c.Calendar, s),
		DueAt: c.DueAt, MetAt: c.MetAt,
	}
	switch {
	case c.MetAt != nil && c.Breached:
		v.Phase = PhaseMissed
	case c.MetAt != nil:
		v.Phase = PhaseMet
	case c.Breached:
		v.Phase = PhaseBreached
	case c.RunningSince == nil && open:
		v.Phase = PhasePaused
	case c.RunningSince == nil:
		v.Phase = PhaseStopped
	case c.AtRisk:
		v.Phase = PhaseAtRisk
	default:
		v.Phase = PhaseRunning
	}
	return v
}
