package sla

import (
	"testing"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
)

// nov is a wall-clock time in Jakarta in November 2026 (13 = Friday, 16 = Monday).
func nov(t *testing.T, day, hour int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	return time.Date(2026, time.November, day, hour, 0, 0, 0, loc)
}

// officeSchedules are Mon–Fri 08:00–17:00 in Jakarta, with the given holidays.
func officeSchedules(t *testing.T, holidays ...calendar.Date) Schedules {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	weekdays := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	window := calendar.Window{Open: 8 * time.Hour, Close: 17 * time.Hour}
	b, err := calendar.NewBusinessHours(calendar.Config{
		Location: loc, Week: calendar.UniformWeek(weekdays, window), Holidays: holidays,
	})
	if err != nil {
		t.Fatal(err)
	}
	return Schedules{Business: b, Always: calendar.Always{}, WorkingDay: window.Length()}
}

func TestAddWorkingDays(t *testing.T) {
	holiday := calendar.NewDate(2026, time.November, 18) // a Wednesday
	tests := []struct {
		name     string
		from     time.Time
		n        int
		holidays []calendar.Date
		want     time.Time
	}{
		{"within the week", nov(t, 16, 15), 3, nil, nov(t, 19, 15)},
		{"over a weekend", nov(t, 13, 16), 3, nil, nov(t, 18, 16)},
		{"from a Saturday", nov(t, 14, 10), 3, nil, nov(t, 18, 17)},
		{"after hours", nov(t, 16, 20), 1, nil, nov(t, 17, 17)},
		{"skips a holiday", nov(t, 16, 15), 3, []calendar.Date{holiday}, nov(t, 20, 15)},
		{"zero days", nov(t, 16, 15), 0, nil, nov(t, 16, 15)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := officeSchedules(t, tt.holidays...)
			if got := s.AddWorkingDays(tt.from, tt.n); !got.Equal(tt.want) {
				t.Fatalf("AddWorkingDays = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkingDaysBetween(t *testing.T) {
	s := officeSchedules(t)
	tests := []struct {
		name string
		a, b time.Time
		want int
	}{
		{"same day", nov(t, 16, 9), nov(t, 16, 16), 0},
		{"one full day", nov(t, 16, 9), nov(t, 17, 9), 1},
		{"over a weekend", nov(t, 13, 9), nov(t, 17, 10), 2},
		{"backwards", nov(t, 17, 9), nov(t, 16, 9), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.WorkingDaysBetween(tt.a, tt.b); got != tt.want {
				t.Fatalf("WorkingDaysBetween = %d, want %d", got, tt.want)
			}
		})
	}
}
