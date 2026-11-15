package slaengine

import (
	"testing"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

var jakarta = mustZone("Asia/Jakarta")

func mustZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// at is a wall-clock time in Jakarta in November 2026 (13 = Friday, 16 = Monday).
func at(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, jakarta)
}

// schedules are Mon–Fri 08:00–17:00 WIB, with the given holidays.
func schedules(t *testing.T, holidays ...calendar.Date) sla.Schedules {
	t.Helper()
	weekdays := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	window := calendar.Window{Open: 8 * time.Hour, Close: 17 * time.Hour}
	b, err := calendar.NewBusinessHours(calendar.Config{
		Location: jakarta, Week: calendar.UniformWeek(weekdays, window), Holidays: holidays,
	})
	if err != nil {
		t.Fatal(err)
	}
	return sla.Schedules{Business: b, Always: calendar.Always{}, WorkingDay: window.Close - window.Open}
}

func dur(v int, u sla.Unit) sla.Duration { return sla.Duration{Value: v, Unit: u} }

// snapshot has the HANDOFF §6 defaults with P3's targets replaced.
func snapshot(first, resolve sla.Duration, cal sla.CalendarKind) ticket.SLASnapshot {
	s := ticket.SLASnapshot{Version: ticket.SnapshotVersion, ProjectSLA: sla.DefaultProjectSLA()}
	for i := range s.Targets {
		if s.Targets[i].Priority == sla.P3 {
			s.Targets[i].FirstResponse, s.Targets[i].Resolve, s.Targets[i].Calendar = first, resolve, cal
		}
	}
	return s
}

func created(t time.Time) Event                 { return Event{At: t, Kind: EventCreated, Status: ticket.StatusNew} }
func moved(t time.Time, to ticket.Status) Event { return Event{At: t, Kind: EventStatus, Status: to} }
func answered(t time.Time) Event                { return Event{At: t, Kind: EventFirstResponse} }

func ptr(t time.Time) *time.Time { return &t }

func TestTargetForUnits(t *testing.T) {
	s := schedules(t)
	tests := []struct {
		d    sla.Duration
		kind sla.CalendarKind
		want Target
	}{
		{dur(15, sla.UnitMinutes), sla.CalendarAlways, Target{15 * time.Minute, s.Always, sla.CalendarAlways}},
		{dur(4, sla.UnitHours), sla.CalendarBusinessHours, Target{4 * time.Hour, s.Business, sla.CalendarBusinessHours}},
		// A working day always counts the business calendar; a day always counts real time.
		{dur(2, sla.UnitWorkingDays), sla.CalendarAlways, Target{18 * time.Hour, s.Business, sla.CalendarBusinessHours}},
		{dur(2, sla.UnitDays), sla.CalendarBusinessHours, Target{48 * time.Hour, s.Always, sla.CalendarAlways}},
	}
	for _, tt := range tests {
		if got := TargetFor(tt.d, tt.kind, s); got != tt.want {
			t.Errorf("TargetFor(%v, %s) = %+v, want %+v", tt.d, tt.kind, got, tt.want)
		}
	}
}

func TestReplay(t *testing.T) {
	friday3pm := at(13, 15, 0)
	monday8am := at(16, 8, 0)
	business, always := sla.CalendarBusinessHours, sla.CalendarAlways
	pauseUATOff := func(s ticket.SLASnapshot) ticket.SLASnapshot { s.PauseOnUAT = false; return s }
	pauseNone := func(s ticket.SLASnapshot) ticket.SLASnapshot { s.PauseOnWaitingClient = false; return s }
	same := func(s ticket.SLASnapshot) ticket.SLASnapshot { return s }

	type want struct {
		frDue, resDue, resAtRisk *time.Time
		frMet, resMet            *time.Time
		frRunning, resRunning    bool
	}
	tests := []struct {
		name     string
		snap     ticket.SLASnapshot
		edit     func(ticket.SLASnapshot) ticket.SLASnapshot
		priority sla.Priority
		events   []Event
		facts    Facts
		holidays []calendar.Date
		want     want
	}{
		{
			name: "Q11: 2 d counts real days", snap: snapshot(dur(2, sla.UnitHours), dur(2, sla.UnitDays), business),
			events: []Event{created(friday3pm)}, facts: Facts{Status: ticket.StatusNew},
			want: want{frDue: ptr(at(13, 17, 0)), resDue: ptr(at(15, 15, 0)), resAtRisk: ptr(at(15, 3, 0)), frRunning: true, resRunning: true},
		},
		{
			name: "Q11: 2 wd counts business days", snap: snapshot(dur(2, sla.UnitHours), dur(2, sla.UnitWorkingDays), business),
			events: []Event{created(friday3pm)}, facts: Facts{Status: ticket.StatusNew},
			want: want{frDue: ptr(at(13, 17, 0)), resDue: ptr(at(17, 15, 0)), resAtRisk: ptr(at(17, 10, 30)), frRunning: true, resRunning: true},
		},
		{
			name: "a holiday doesn't count", snap: snapshot(dur(2, sla.UnitHours), dur(2, sla.UnitWorkingDays), business),
			events: []Event{created(friday3pm)}, facts: Facts{Status: ticket.StatusNew},
			holidays: []calendar.Date{calendar.NewDate(2026, time.November, 16)},
			want:     want{frDue: ptr(at(13, 17, 0)), resDue: ptr(at(18, 15, 0)), resAtRisk: ptr(at(18, 10, 30)), frRunning: true, resRunning: true},
		},
		{
			name: "due on closing time stays there", snap: snapshot(dur(1, sla.UnitHours), dur(9, sla.UnitHours), business),
			events: []Event{created(monday8am)}, facts: Facts{Status: ticket.StatusNew},
			want: want{frDue: ptr(at(16, 9, 0)), resDue: ptr(at(16, 17, 0)), resAtRisk: ptr(at(16, 14, 45)), frRunning: true, resRunning: true},
		},
		{
			name: "waiting on client pauses resolution, not first response",
			snap: snapshot(dur(4, sla.UnitHours), dur(4, sla.UnitHours), business),
			events: []Event{created(monday8am), moved(at(16, 9, 0), ticket.StatusInProgress),
				moved(at(16, 10, 0), ticket.StatusWaitingOnClient), moved(at(16, 13, 0), ticket.StatusInProgress)},
			facts: Facts{Status: ticket.StatusInProgress},
			want:  want{frDue: ptr(at(16, 12, 0)), resDue: ptr(at(16, 15, 0)), resAtRisk: ptr(at(16, 14, 0)), frRunning: true, resRunning: true},
		},
		{
			name: "no pause when the project says so", snap: snapshot(dur(4, sla.UnitHours), dur(4, sla.UnitHours), business), edit: pauseNone,
			events: []Event{created(monday8am), moved(at(16, 10, 0), ticket.StatusWaitingOnClient)},
			facts:  Facts{Status: ticket.StatusWaitingOnClient},
			want:   want{frDue: ptr(at(16, 12, 0)), resDue: ptr(at(16, 12, 0)), resAtRisk: ptr(at(16, 11, 0)), frRunning: true, resRunning: true},
		},
		{
			name: "paused now: no due time while paused", snap: snapshot(dur(4, sla.UnitHours), dur(4, sla.UnitHours), business),
			events: []Event{created(monday8am), moved(at(16, 10, 0), ticket.StatusWaitingOnClient)},
			facts:  Facts{Status: ticket.StatusWaitingOnClient},
			want:   want{frDue: ptr(at(16, 12, 0)), frRunning: true},
		},
		{
			name: "waiting then UAT with only the waiting pause on resumes",
			snap: snapshot(dur(4, sla.UnitHours), dur(4, sla.UnitHours), business), edit: pauseUATOff,
			events: []Event{created(monday8am), moved(at(16, 10, 0), ticket.StatusWaitingOnClient), moved(at(16, 11, 0), ticket.StatusInUAT)},
			facts:  Facts{Status: ticket.StatusInUAT},
			want:   want{frDue: ptr(at(16, 12, 0)), resDue: ptr(at(16, 13, 0)), resAtRisk: ptr(at(16, 12, 0)), frRunning: true, resRunning: true},
		},
		{
			name: "agent-logged: first response met at creation", snap: snapshot(dur(2, sla.UnitHours), dur(4, sla.UnitHours), business),
			events: []Event{created(monday8am), answered(monday8am)},
			facts:  Facts{Status: ticket.StatusNew, FirstResponseAt: ptr(monday8am)},
			want:   want{frMet: ptr(monday8am), resDue: ptr(at(16, 12, 0)), resAtRisk: ptr(at(16, 11, 0)), resRunning: true},
		},
		{
			name:   "resolved unanswered stops first response; resolved late keeps the due time",
			snap:   snapshot(dur(2, sla.UnitHours), dur(1, sla.UnitHours), always),
			events: []Event{created(monday8am), moved(at(16, 10, 0), ticket.StatusResolved)},
			facts:  Facts{Status: ticket.StatusResolved, ResolvedAt: ptr(at(16, 10, 0))},
			want:   want{frDue: ptr(at(16, 10, 0)), resDue: ptr(at(16, 9, 0)), resAtRisk: ptr(at(16, 8, 45)), resMet: ptr(at(16, 10, 0))},
		},
		{
			name: "reopening resumes without counting the resolved time",
			snap: snapshot(dur(15, sla.UnitMinutes), dur(4, sla.UnitHours), business),
			events: []Event{created(monday8am), answered(at(16, 8, 10)), moved(at(16, 9, 0), ticket.StatusResolved),
				moved(at(16, 12, 0), ticket.StatusInProgress)},
			facts: Facts{Status: ticket.StatusInProgress, FirstResponseAt: ptr(at(16, 8, 10))},
			want:  want{frMet: ptr(at(16, 8, 10)), resDue: ptr(at(16, 15, 0)), resAtRisk: ptr(at(16, 14, 0)), resRunning: true},
		},
		{
			name: "P3 business to P1 24/7 re-measures the history", priority: sla.P1,
			snap:   snapshot(dur(2, sla.UnitHours), dur(2, sla.UnitWorkingDays), business),
			events: []Event{created(friday3pm)}, facts: Facts{Status: ticket.StatusNew},
			want: want{frDue: ptr(at(13, 15, 15)), resDue: ptr(at(13, 19, 0)), resAtRisk: ptr(at(13, 18, 0)), frRunning: true, resRunning: true},
		},
		{
			name: "P1 24/7 to P2 business re-measures the other way", priority: sla.P2,
			snap:   snapshot(dur(2, sla.UnitHours), dur(2, sla.UnitWorkingDays), business),
			events: []Event{created(friday3pm)}, facts: Facts{Status: ticket.StatusNew},
			want: want{frDue: ptr(at(13, 15, 30)), resDue: ptr(at(16, 14, 0)), resAtRisk: ptr(at(16, 12, 0)), frRunning: true, resRunning: true},
		},
		{
			name: "out-of-order audit times follow the ticket's columns",
			snap: snapshot(dur(4, sla.UnitHours), dur(4, sla.UnitHours), business),
			events: []Event{created(monday8am), moved(at(16, 10, 0), ticket.StatusInProgress),
				moved(at(16, 10, 0), ticket.StatusWaitingOnClient)}, // really the other way round
			facts: Facts{Status: ticket.StatusInProgress},
			want:  want{frDue: ptr(at(16, 12, 0)), resDue: ptr(at(16, 12, 0)), resAtRisk: ptr(at(16, 11, 0)), frRunning: true, resRunning: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := tt.snap
			if tt.edit != nil {
				snap = tt.edit(snap)
			} else {
				snap = same(snap)
			}
			facts := tt.facts
			facts.Priority = tt.priority
			if facts.Priority == "" {
				facts.Priority = sla.P3
			}
			got, err := Replay(Input{
				Snapshot: snap, Type: ticket.TypeIncident, Events: tt.events, Facts: facts, Schedules: schedules(t, tt.holidays...),
			})
			if err != nil {
				t.Fatal(err)
			}
			check := func(name string, got, want *time.Time) {
				t.Helper()
				if (got == nil) != (want == nil) || (got != nil && !got.Equal(*want)) {
					t.Errorf("%s = %v, want %v", name, show(got), show(want))
				}
			}
			fr, res := got.FirstResponse, got.Resolution
			if fr.Running() {
				check("first response due", fr.DueAt, tt.want.frDue)
			} else if tt.want.frDue != nil {
				check("first response crossed", fr.DueAt, tt.want.frDue)
			}
			check("first response met", fr.MetAt, tt.want.frMet)
			if res.Running() || tt.want.resDue != nil {
				check("resolution due", res.DueAt, tt.want.resDue)
				check("resolution at risk", res.AtRiskAt, tt.want.resAtRisk)
			}
			check("resolution met", res.MetAt, tt.want.resMet)
			if fr.Running() != tt.want.frRunning || res.Running() != tt.want.resRunning {
				t.Errorf("running = %v/%v, want %v/%v", fr.Running(), res.Running(), tt.want.frRunning, tt.want.resRunning)
			}
		})
	}
}

func show(t *time.Time) string {
	if t == nil {
		return "nil"
	}
	return t.In(jakarta).Format("Mon 02 15:04")
}

func TestViewAt(t *testing.T) {
	s := schedules(t)
	since := at(16, 8, 0)
	now := at(16, 11, 0)
	base := Stored{Budget: 4 * time.Hour, Calendar: sla.CalendarBusinessHours, RunningSince: &since}
	tests := []struct {
		name      string
		edit      func(*Stored)
		open      bool
		phase     Phase
		remaining time.Duration
	}{
		{"running", func(*Stored) {}, true, PhaseRunning, time.Hour},
		{"at risk", func(c *Stored) { c.AtRisk = true }, true, PhaseAtRisk, time.Hour},
		{"breached beats at risk", func(c *Stored) { c.AtRisk, c.Breached = true, true }, true, PhaseBreached, time.Hour},
		{"paused", func(c *Stored) { c.RunningSince, c.Elapsed = nil, 2*time.Hour }, true, PhasePaused, 2 * time.Hour},
		{"stopped unanswered", func(c *Stored) { c.RunningSince = nil }, false, PhaseStopped, 4 * time.Hour},
		{"met", func(c *Stored) { c.RunningSince, c.MetAt, c.Elapsed = nil, &now, 3*time.Hour }, false, PhaseMet, time.Hour},
		{"missed", func(c *Stored) { c.RunningSince, c.MetAt, c.Breached, c.Elapsed = nil, &now, true, 5*time.Hour }, false, PhaseMissed, -time.Hour},
	}
	for _, tt := range tests {
		c := base
		tt.edit(&c)
		v := ViewAt(c, s, tt.open, now)
		if v.Phase != tt.phase || v.Remaining != tt.remaining || v.Day != 9*time.Hour {
			t.Errorf("%s: phase %s remaining %v day %v; want %s %v", tt.name, v.Phase, v.Remaining, v.Day, tt.phase, tt.remaining)
		}
	}
}
