// Package sla holds the SLA vocabulary (priorities, clocks, project targets), the organisation's
// business hours and holiday calendar, and builds the working-time schedules the SLA engine
// measures with.
package sla

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
)

// Priority is a ticket priority. Values match the priorities.code CHECK constraint.
type Priority string

// Priorities, most urgent first (HANDOFF §6).
const (
	P1 Priority = "P1"
	P2 Priority = "P2"
	P3 Priority = "P3"
	P4 Priority = "P4"
)

// Priorities lists every priority in order.
var Priorities = []Priority{P1, P2, P3, P4}

func (p Priority) Valid() bool { return slices.Contains(Priorities, p) }

// CalendarKind says which clock a target runs on. Values match the project_sla_targets.calendar CHECK.
type CalendarKind string

const (
	CalendarBusinessHours CalendarKind = "business_hours"
	CalendarAlways        CalendarKind = "always" // 24/7
)

func (k CalendarKind) Valid() bool { return k == CalendarBusinessHours || k == CalendarAlways }

// MaxTargetMinutes caps any single target: one year of wall-clock minutes.
const MaxTargetMinutes = 365 * 24 * 60

// MaxHolidayNameLen matches the holidays.name CHECK constraint.
const MaxHolidayNameLen = 200

// Hours is the organisation's weekly working window.
type Hours struct {
	Timezone string
	Weekdays []time.Weekday // ascending, Sunday = 0
	Opens    time.Duration  // from local midnight
	Closes   time.Duration
}

func (h Hours) WorkingDay() time.Duration { return h.Closes - h.Opens }

// HoursInput replaces the working window. The time zone is an organisation setting.
type HoursInput struct {
	Weekdays []time.Weekday
	Opens    time.Duration
	Closes   time.Duration
}

var (
	ErrNoWeekdays    = errors.New("choose at least one working day")
	ErrBadWeekday    = errors.New("unknown or repeated weekday")
	ErrBadHoursRange = errors.New("opening time must be before closing time, within 00:00–24:00, in whole minutes")
)

// Validate checks the input and returns the weekdays sorted.
func (in HoursInput) Validate() ([]time.Weekday, error) {
	if len(in.Weekdays) == 0 {
		return nil, ErrNoWeekdays
	}
	days := slices.Clone(in.Weekdays)
	slices.Sort(days)
	for i, d := range days {
		if d < time.Sunday || d > time.Saturday || (i > 0 && days[i-1] == d) {
			return nil, ErrBadWeekday
		}
	}
	w := calendar.Window{Open: in.Opens, Close: in.Closes}
	if w.Open < 0 || w.Open >= w.Close || w.Close > calendar.DayLength ||
		w.Open%time.Minute != 0 || w.Close%time.Minute != 0 {
		return nil, ErrBadHoursRange
	}
	return days, nil
}

// Holiday is a day without working time.
type Holiday struct {
	ID   uuid.UUID
	Date calendar.Date
	Name string
}

// PresetSummary describes a bundled holiday list and how much of it is missing.
type PresetSummary struct {
	Year         int
	Country      string
	HolidayCount int
	MissingCount int
}

type PresetLoad struct {
	Year    int
	Added   int
	Skipped int // dates that already had a holiday
}

// Schedules are the two clocks SLA targets run on.
type Schedules struct {
	Business calendar.Schedule
	Always   calendar.Schedule
}

func (s Schedules) For(k CalendarKind) calendar.Schedule {
	if k == CalendarAlways {
		return s.Always
	}
	return s.Business
}
