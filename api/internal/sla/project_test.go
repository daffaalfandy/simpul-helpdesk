package sla

import (
	"errors"
	"testing"
	"time"
)

const testWorkingDay = DefaultWorkingDay

func TestDurationMinutes(t *testing.T) {
	eightHourDay := 8 * time.Hour
	tests := []struct {
		name       string
		d          Duration
		workingDay time.Duration
		want       int
	}{
		{"minutes", Duration{15, UnitMinutes}, testWorkingDay, 15},
		{"hours", Duration{4, UnitHours}, testWorkingDay, 240},
		{"working days on a 9-hour day", Duration{2, UnitWorkingDays}, testWorkingDay, 1080},
		{"working days follow the window", Duration{2, UnitWorkingDays}, eightHourDay, 960},
		{"calendar days are 24 hours", Duration{2, UnitDays}, testWorkingDay, 2880},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.d.Minutes(tt.workingDay); got != tt.want {
				t.Fatalf("Minutes = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestValidateProjectSLA(t *testing.T) {
	modify := func(i int, f func(*ProjectTarget)) ProjectSLA {
		s := DefaultProjectSLA()
		f(&s.Targets[i])
		return s
	}
	reordered := DefaultProjectSLA()
	reordered.Targets = append(reordered.Targets[2:], reordered.Targets[:2]...)
	badRisk := DefaultProjectSLA()
	badRisk.AtRiskPercent = 60
	missing := DefaultProjectSLA()
	missing.Targets = missing.Targets[:3]

	tests := []struct {
		name string
		sla  ProjectSLA
		want error
	}{
		{"defaults", DefaultProjectSLA(), nil},
		{"reordered", reordered, nil},
		{"missing a priority", missing, ErrTargetsIncomplete},
		{"duplicate priority", modify(3, func(t *ProjectTarget) { t.Priority = P1 }), ErrTargetsIncomplete},
		{"unknown priority", modify(3, func(t *ProjectTarget) { t.Priority = "P5" }), ErrTargetsIncomplete},
		{"zero value", modify(0, func(t *ProjectTarget) { t.FirstResponse.Value = 0 }), ErrDurationRange},
		{"longer than a year", modify(3, func(t *ProjectTarget) { t.Resolve = Duration{366, UnitDays} }), ErrDurationRange},
		{"unknown unit", modify(1, func(t *ProjectTarget) { t.Resolve.Unit = "weeks" }), ErrUnknownUnit},
		{"unknown calendar", modify(2, func(t *ProjectTarget) { t.Calendar = "weekends" }), ErrUnknownCalendar},
		{"response equal to resolve", modify(0, func(t *ProjectTarget) { t.FirstResponse = Duration{4, UnitHours} }), ErrResponseNotShorter},
		{"response after resolve", modify(1, func(t *ProjectTarget) { t.FirstResponse = Duration{9, UnitHours} }), ErrResponseNotShorter},
		{"response after deliver", modify(1, func(t *ProjectTarget) { t.Deliver = Duration{20, UnitMinutes} }), ErrResponseNotShorter},
		{"a working day equals 9 hours", modify(1, func(t *ProjectTarget) {
			t.FirstResponse = Duration{9, UnitHours}
			t.Resolve = Duration{10, UnitHours} // deliver stays 1 working day
		}), ErrResponseNotShorter},
		{"a day is longer than a working day", modify(1, func(t *ProjectTarget) {
			t.FirstResponse = Duration{1, UnitWorkingDays}
			t.Resolve = Duration{1, UnitDays}
			t.Deliver = Duration{1, UnitDays}
		}), nil},
		{"unknown at-risk threshold", badRisk, ErrUnknownAtRiskLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateProjectSLA(tt.sla, testWorkingDay)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if err != nil {
				return
			}
			for i, p := range Priorities {
				if got.Targets[i].Priority != p {
					t.Fatalf("not ordered: %+v", got.Targets)
				}
			}
			if got.ProjectSettings != tt.sla.ProjectSettings {
				t.Fatalf("settings changed: %+v", got.ProjectSettings)
			}
		})
	}
}

func TestValidateProjectSLARejectsAnEmptyWorkingDay(t *testing.T) {
	if _, err := ValidateProjectSLA(DefaultProjectSLA(), 0); err == nil {
		t.Fatal("want an error for a zero working day")
	}
}

func TestProjectSLATarget(t *testing.T) {
	s := DefaultProjectSLA()
	got, ok := s.Target(P3)
	if !ok || got.Resolve != (Duration{2, UnitWorkingDays}) {
		t.Fatalf("Target(P3) = %+v, %v", got, ok)
	}
	if _, ok := s.Target("P9"); ok {
		t.Fatal("Target found an unknown priority")
	}
}
