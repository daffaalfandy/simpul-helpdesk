package sla

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// Unit is how a project target is written. Values match the project_sla_targets unit CHECKs.
type Unit string

const (
	UnitMinutes     Unit = "min"
	UnitHours       Unit = "h"
	UnitWorkingDays Unit = "wd" // one business-hours window
	UnitDays        Unit = "d"  // a calendar day, 24 hours
)

// Units lists every unit, smallest first.
var Units = []Unit{UnitMinutes, UnitHours, UnitWorkingDays, UnitDays}

func (u Unit) Valid() bool { return slices.Contains(Units, u) }

const (
	minutesPerHour = 60
	minutesPerDay  = 24 * minutesPerHour
)

// DefaultWorkingDay is the working-day length when no business hours are stored (Mon–Fri
// 08:00–17:00, HANDOFF §6).
const DefaultWorkingDay = 9 * time.Hour

// Duration is a target as people write it: "15 min", "2 working days".
type Duration struct {
	Value int  `json:"value"`
	Unit  Unit `json:"unit"`
}

// Minutes converts the duration for comparisons: a working day counts as one business-hours
// window and a day as 24 hours (follow-up #1). How the clock runs is the engine's business (M5b).
func (d Duration) Minutes(workingDay time.Duration) int {
	switch d.Unit {
	case UnitHours:
		return d.Value * minutesPerHour
	case UnitWorkingDays:
		return d.Value * int(workingDay/time.Minute)
	case UnitDays:
		return d.Value * minutesPerDay
	default:
		return d.Value
	}
}

// ProjectTarget is one priority's targets on a project.
type ProjectTarget struct {
	Priority      Priority     `json:"priority"`
	FirstResponse Duration     `json:"firstResponse"`
	Resolve       Duration     `json:"resolve"` // incidents
	Deliver       Duration     `json:"deliver"` // service requests
	Calendar      CalendarKind `json:"calendar"`
}

// AtRiskPercent is how much of a target may pass before a ticket is flagged at risk.
type AtRiskPercent int

// The at-risk thresholds a project can pick (follow-up #1). Values match the projects CHECK.
const (
	AtRiskHalf          AtRiskPercent = 50
	AtRiskThreeQuarters AtRiskPercent = 75
	AtRiskNinetyPercent AtRiskPercent = 90
)

var AtRiskPercents = []AtRiskPercent{AtRiskHalf, AtRiskThreeQuarters, AtRiskNinetyPercent}

func (p AtRiskPercent) Valid() bool { return slices.Contains(AtRiskPercents, p) }

// ProjectSettings are a project's pause and alert rules.
type ProjectSettings struct {
	PauseOnWaitingClient     bool          `json:"pauseOnWaitingClient"`
	PauseOnUAT               bool          `json:"pauseOnUat"`
	AtRiskPercent            AtRiskPercent `json:"atRiskPercent"`
	NotifySupervisorOnBreach bool          `json:"notifySupervisorOnBreach"`
	NotifyPicOnP1Breach      bool          `json:"notifyPicOnP1Breach"`
}

// ProjectSLA is everything a project says about service levels. Its JSON form is part of the
// ticket SLA snapshot, so the tags are a storage format.
type ProjectSLA struct {
	Targets []ProjectTarget `json:"targets"` // ordered P1–P4
	ProjectSettings
}

// DefaultProjectSLA is what a new project starts with (HANDOFF §6 defaults).
func DefaultProjectSLA() ProjectSLA {
	d := func(v int, u Unit) Duration { return Duration{Value: v, Unit: u} }
	return ProjectSLA{
		Targets: []ProjectTarget{
			{Priority: P1, FirstResponse: d(15, UnitMinutes), Resolve: d(4, UnitHours), Deliver: d(4, UnitHours), Calendar: CalendarAlways},
			{Priority: P2, FirstResponse: d(30, UnitMinutes), Resolve: d(8, UnitHours), Deliver: d(1, UnitWorkingDays), Calendar: CalendarBusinessHours},
			{Priority: P3, FirstResponse: d(2, UnitHours), Resolve: d(2, UnitWorkingDays), Deliver: d(2, UnitWorkingDays), Calendar: CalendarBusinessHours},
			{Priority: P4, FirstResponse: d(4, UnitHours), Resolve: d(5, UnitWorkingDays), Deliver: d(5, UnitWorkingDays), Calendar: CalendarBusinessHours},
		},
		ProjectSettings: ProjectSettings{
			PauseOnWaitingClient:     true,
			PauseOnUAT:               true,
			AtRiskPercent:            AtRiskThreeQuarters,
			NotifySupervisorOnBreach: true,
			NotifyPicOnP1Breach:      false,
		},
	}
}

// Target returns the targets for one priority.
func (s ProjectSLA) Target(p Priority) (ProjectTarget, bool) {
	i := slices.IndexFunc(s.Targets, func(t ProjectTarget) bool { return t.Priority == p })
	if i < 0 {
		return ProjectTarget{}, false
	}
	return s.Targets[i], true
}

var (
	ErrTargetsIncomplete  = errors.New("give exactly one target for each of P1, P2, P3 and P4")
	ErrUnknownCalendar    = errors.New("hours must be business_hours or always")
	ErrDurationRange      = fmt.Errorf("each target must be at least 1 and at most %d minutes long", MaxTargetMinutes)
	ErrUnknownUnit        = errors.New("units must be min, h, wd or d")
	ErrResponseNotShorter = errors.New("first response must be shorter than resolve and deliver")
	ErrUnknownAtRiskLimit = errors.New("at-risk threshold must be 50, 75 or 90 percent")
	errBadWorkingDay      = errors.New("the working day must be longer than zero")
)

// ValidateProjectSLA checks a full project SLA and returns it with targets ordered P1–P4.
// workingDay is the business-hours window used to compare working days with other units.
func ValidateProjectSLA(s ProjectSLA, workingDay time.Duration) (ProjectSLA, error) {
	if workingDay <= 0 {
		return ProjectSLA{}, errBadWorkingDay
	}
	if len(s.Targets) != len(Priorities) {
		return ProjectSLA{}, ErrTargetsIncomplete
	}
	byPriority := make(map[Priority]ProjectTarget, len(s.Targets))
	for _, t := range s.Targets {
		if !t.Priority.Valid() {
			return ProjectSLA{}, ErrTargetsIncomplete
		}
		if _, dup := byPriority[t.Priority]; dup {
			return ProjectSLA{}, ErrTargetsIncomplete
		}
		if err := validateTarget(t, workingDay); err != nil {
			return ProjectSLA{}, fmt.Errorf("%s: %w", t.Priority, err)
		}
		byPriority[t.Priority] = t
	}
	if !s.AtRiskPercent.Valid() {
		return ProjectSLA{}, ErrUnknownAtRiskLimit
	}
	out := ProjectSLA{Targets: make([]ProjectTarget, 0, len(Priorities)), ProjectSettings: s.ProjectSettings}
	for _, p := range Priorities {
		out.Targets = append(out.Targets, byPriority[p])
	}
	return out, nil
}

func validateTarget(t ProjectTarget, workingDay time.Duration) error {
	for _, d := range []Duration{t.FirstResponse, t.Resolve, t.Deliver} {
		if !d.Unit.Valid() {
			return ErrUnknownUnit
		}
		if d.Value < 1 || d.Minutes(workingDay) > MaxTargetMinutes {
			return ErrDurationRange
		}
	}
	if !t.Calendar.Valid() {
		return ErrUnknownCalendar
	}
	first := t.FirstResponse.Minutes(workingDay)
	if first >= t.Resolve.Minutes(workingDay) || first >= t.Deliver.Minutes(workingDay) {
		return ErrResponseNotShorter
	}
	return nil
}
