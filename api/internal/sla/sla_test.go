package sla

import (
	"errors"
	"testing"
	"time"
)

func standardTargets() []Target {
	return []Target{
		{Priority: P1, FirstResponseMinutes: 15, ResolveMinutes: 240, DeliverMinutes: 240, Calendar: CalendarAlways},
		{Priority: P2, FirstResponseMinutes: 30, ResolveMinutes: 480, DeliverMinutes: 540, Calendar: CalendarBusinessHours},
		{Priority: P3, FirstResponseMinutes: 120, ResolveMinutes: 1080, DeliverMinutes: 1080, DeliverFromCatalog: true, Calendar: CalendarBusinessHours},
		{Priority: P4, FirstResponseMinutes: 240, ResolveMinutes: 2700, DeliverMinutes: 2700, Calendar: CalendarBusinessHours},
	}
}

func TestValidateTargets(t *testing.T) {
	modify := func(i int, f func(*Target)) []Target {
		ts := standardTargets()
		f(&ts[i])
		return ts
	}
	tests := []struct {
		name    string
		targets []Target
		want    error
	}{
		{"standard plan", standardTargets(), nil},
		{"reordered", append(standardTargets()[2:], standardTargets()[:2]...), nil},
		{"missing a priority", standardTargets()[:3], ErrTargetsIncomplete},
		{"duplicate priority", modify(3, func(t *Target) { t.Priority = P1 }), ErrTargetsIncomplete},
		{"unknown priority", modify(3, func(t *Target) { t.Priority = "P5" }), ErrTargetsIncomplete},
		{"zero minutes", modify(0, func(t *Target) { t.FirstResponseMinutes = 0 }), ErrTargetRange},
		{"too long", modify(0, func(t *Target) { t.ResolveMinutes = MaxTargetMinutes + 1 }), ErrTargetRange},
		{"response after resolve", modify(1, func(t *Target) { t.FirstResponseMinutes = 481 }), ErrResponseTooLong},
		{"response after deliver", modify(1, func(t *Target) { t.DeliverMinutes = 20 }), ErrResponseTooLong},
		{"unknown calendar", modify(2, func(t *Target) { t.Calendar = "weekends" }), ErrUnknownCalendar},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateTargets(tt.targets)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if err == nil {
				for i, p := range Priorities {
					if got[i].Priority != p {
						t.Fatalf("not ordered: %+v", got)
					}
				}
			}
		})
	}
}

func TestHoursInputValidate(t *testing.T) {
	weekdays := []time.Weekday{time.Friday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday}
	tests := []struct {
		name string
		in   HoursInput
		want error
	}{
		{"office hours", HoursInput{Weekdays: weekdays, Opens: 8 * time.Hour, Closes: 17 * time.Hour}, nil},
		{"until midnight", HoursInput{Weekdays: weekdays, Opens: 0, Closes: 24 * time.Hour}, nil},
		{"no days", HoursInput{Opens: 8 * time.Hour, Closes: 17 * time.Hour}, ErrNoWeekdays},
		{"repeated day", HoursInput{Weekdays: []time.Weekday{time.Monday, time.Monday}, Opens: 8 * time.Hour, Closes: 17 * time.Hour}, ErrBadWeekday},
		{"unknown day", HoursInput{Weekdays: []time.Weekday{7}, Opens: 8 * time.Hour, Closes: 17 * time.Hour}, ErrBadWeekday},
		{"closes before opens", HoursInput{Weekdays: weekdays, Opens: 17 * time.Hour, Closes: 8 * time.Hour}, ErrBadHoursRange},
		{"past midnight", HoursInput{Weekdays: weekdays, Opens: 8 * time.Hour, Closes: 25 * time.Hour}, ErrBadHoursRange},
		{"seconds", HoursInput{Weekdays: weekdays, Opens: 8*time.Hour + time.Second, Closes: 17 * time.Hour}, ErrBadHoursRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days, err := tt.in.Validate()
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if err == nil && days[0] != time.Monday {
				t.Fatalf("not sorted: %v", days)
			}
		})
	}
}
