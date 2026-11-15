// Package slaengine runs a ticket's two SLA clocks, first response and resolution (HANDOFF §6).
// It is pure: it replays the ticket's history against its SLA snapshot and the working-time
// calendar, so the same function serves every change, a calendar edit and a backfill.
package slaengine

import (
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// Clock names one of a ticket's SLA clocks. Values are stored and shown in the API.
type Clock string

const (
	ClockFirstResponse Clock = "first_response"
	ClockResolution    Clock = "resolution" // resolve for incidents, deliver for service requests
)

var Clocks = []Clock{ClockFirstResponse, ClockResolution}

// percent is the at-risk threshold's denominator.
const percent = 100

// Target is one SLA target as an amount of time on the schedule that measures it.
type Target struct {
	Budget   time.Duration
	Schedule calendar.Schedule
	Calendar sla.CalendarKind // which schedule, for printing days ("1d" is one working day or 24 h)
}

// TargetFor measures a target per HANDOFF §13 Q11: minutes and hours follow the project's clock;
// a working day is one business-hours window on the business calendar; a day is 24 real hours.
func TargetFor(d sla.Duration, kind sla.CalendarKind, s sla.Schedules) Target {
	value := time.Duration(d.Value)
	switch d.Unit {
	case sla.UnitHours:
		return Target{Budget: value * time.Hour, Schedule: s.For(kind), Calendar: kind}
	case sla.UnitWorkingDays:
		return Target{Budget: value * s.WorkingDay, Schedule: s.Business, Calendar: sla.CalendarBusinessHours}
	case sla.UnitDays:
		return Target{Budget: value * hoursPerDay * time.Hour, Schedule: s.Always, Calendar: sla.CalendarAlways}
	}
	return Target{Budget: value * time.Minute, Schedule: s.For(kind), Calendar: kind}
}

const hoursPerDay = 24

// DayLength is what "1d" means on a calendar: one working day on business hours, else 24 h.
func DayLength(kind sla.CalendarKind, s sla.Schedules) time.Duration {
	if kind == sla.CalendarAlways {
		return hoursPerDay * time.Hour
	}
	return s.WorkingDay
}

// EventKind is a change that moves a clock.
type EventKind string

const (
	EventCreated       EventKind = "created"
	EventStatus        EventKind = "status"
	EventFirstResponse EventKind = "first_response"
)

// Event is one change in a ticket's history. Priority changes aren't events: the current
// priority picks the targets and the whole history is measured against them.
type Event struct {
	At     time.Time
	Kind   EventKind
	Status ticket.Status // created: the first status; status: the new one
}

// Facts are the ticket's current columns. Audit times are transaction starts, so two changes
// can land out of order; the replay trusts these over its own result.
type Facts struct {
	Status          ticket.Status
	Priority        sla.Priority
	FirstResponseAt *time.Time
	ResolvedAt      *time.Time
}

// Input is everything a replay needs.
type Input struct {
	Snapshot  ticket.SLASnapshot
	Type      ticket.Type
	Events    []Event // oldest first
	Facts     Facts
	Schedules sla.Schedules
}

// ClockState is one clock after the replay.
type ClockState struct {
	Target       Target
	Elapsed      time.Duration // working time used in finished intervals, on Target's schedule
	RunningSince *time.Time    // nil while paused or stopped
	MetAt        *time.Time    // first response sent, or resolved
	DueAt        *time.Time    // when the budget runs out (or ran out); nil if it never will
	AtRiskAt     *time.Time    // when the at-risk share of the budget is used
}

// Running reports whether the clock is counting now.
func (c ClockState) Running() bool { return c.RunningSince != nil }

// State is a ticket's two clocks.
type State struct {
	FirstResponse ClockState
	Resolution    ClockState
}

// interval is a stretch of time a clock ran; end is nil while it still runs.
type interval struct {
	start time.Time
	end   *time.Time
}

// track is one clock's running intervals.
type track struct{ intervals []interval }

func (t *track) running() bool {
	return len(t.intervals) > 0 && t.intervals[len(t.intervals)-1].end == nil
}

func (t *track) start(at time.Time) {
	if !t.running() {
		t.intervals = append(t.intervals, interval{start: at})
	}
}

func (t *track) stop(at time.Time) {
	if t.running() {
		end := at
		t.intervals[len(t.intervals)-1].end = &end
	}
}

// Replay runs both clocks through the ticket's history. First response never pauses: it stops at
// the first public staff reply (at creation for agent-logged tickets), or when the ticket is
// resolved unanswered, and runs again if it is reopened. Resolution pauses in Waiting on client
// and In UAT when the snapshot says so, stops at Resolved or Closed, and resumes on reopening.
// A priority change swaps the targets and measures the time already used on the new schedule.
func Replay(in Input) (State, error) {
	var fr, res track
	var frMet, resMet *time.Time
	for _, e := range in.Events {
		switch e.Kind {
		case EventCreated:
			fr.start(e.At)
			if runs(e.Status, in.Snapshot) {
				res.start(e.At)
			}
		case EventFirstResponse:
			if frMet == nil {
				at := e.At
				frMet = &at
				fr.stop(e.At)
			}
		case EventStatus:
			switch {
			case done(e.Status):
				fr.stop(e.At)
				res.stop(e.At)
				if resMet == nil {
					at := e.At
					resMet = &at
				}
			case runs(e.Status, in.Snapshot):
				resMet = nil
				res.start(e.At)
				if frMet == nil {
					fr.start(e.At)
				}
			default: // a pausing status
				resMet = nil
				res.stop(e.At)
				if frMet == nil {
					fr.start(e.At)
				}
			}
		}
	}
	reconcile(&fr, &res, &frMet, &resMet, in)

	eff, err := in.Snapshot.Effective(in.Facts.Priority, in.Type)
	if err != nil {
		return State{}, err
	}
	pct := int64(in.Snapshot.AtRiskPercent)
	return State{
		FirstResponse: clockState(fr, frMet, TargetFor(eff.FirstResponse, eff.Calendar, in.Schedules), pct),
		Resolution:    clockState(res, resMet, TargetFor(eff.Resolution, eff.Calendar, in.Schedules), pct),
	}, nil
}

// reconcile makes the replay agree with the ticket's columns, which are always right.
func reconcile(fr, res *track, frMet, resMet **time.Time, in Input) {
	last := lastEventAt(in.Events)
	if at := in.Facts.FirstResponseAt; at != nil {
		*frMet = at
		fr.stop(*at)
	}
	switch {
	case done(in.Facts.Status):
		at := last
		if in.Facts.ResolvedAt != nil {
			at = *in.Facts.ResolvedAt
		}
		res.stop(at)
		fr.stop(at)
		if *resMet == nil {
			*resMet = &at
		}
	case runs(in.Facts.Status, in.Snapshot):
		*resMet = nil
		res.start(last)
	default:
		*resMet = nil
		res.stop(last)
	}
}

func lastEventAt(events []Event) time.Time {
	if len(events) == 0 {
		return time.Time{}
	}
	return events[len(events)-1].At
}

func clockState(t track, met *time.Time, target Target, atRiskPercent int64) ClockState {
	c := ClockState{Target: target, MetAt: met}
	for _, iv := range t.intervals {
		if iv.end == nil {
			since := iv.start
			c.RunningSince = &since
			continue
		}
		c.Elapsed += span(target.Schedule, iv.start, *iv.end)
	}
	c.DueAt = crossing(t, target.Schedule, target.Budget)
	c.AtRiskAt = crossing(t, target.Schedule, target.Budget*time.Duration(atRiskPercent)/percent)
	return c
}

// crossing is when the clock has used amount of working time: inside a finished interval, or
// in the running one. Nil if the clock stopped before using it.
func crossing(t track, s calendar.Schedule, amount time.Duration) *time.Time {
	var used time.Duration
	for _, iv := range t.intervals {
		left := amount - used
		if iv.end == nil {
			at := s.Add(iv.start, max(left, 0))
			return &at
		}
		d := span(s, iv.start, *iv.end)
		if d >= left {
			at := s.Add(iv.start, max(left, 0))
			return &at
		}
		used += d
	}
	return nil
}

// span is the working time from a to b; out-of-order audit times give zero, never negative.
func span(s calendar.Schedule, a, b time.Time) time.Duration {
	return max(s.Between(a, b), 0)
}

func done(st ticket.Status) bool { return st == ticket.StatusResolved || st == ticket.StatusClosed }

// runs reports whether the resolution clock counts in this status.
func runs(st ticket.Status, snap ticket.SLASnapshot) bool {
	switch st {
	case ticket.StatusWaitingOnClient:
		return !snap.PauseOnWaitingClient
	case ticket.StatusInUAT:
		return !snap.PauseOnUAT
	}
	return !done(st)
}
