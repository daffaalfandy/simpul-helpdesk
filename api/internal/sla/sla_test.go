package sla

import (
	"errors"
	"testing"
	"time"
)

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
