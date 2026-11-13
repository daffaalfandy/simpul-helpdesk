package sla_test

import (
	"context"
	"os"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

var actor = audit.SystemActor(audit.ActorNameCLI)

func TestSchedulesUseStoredHoursAndHolidays(t *testing.T) {
	ctx := context.Background()
	svc := sla.NewService(testdb.New(t))
	wed, _ := calendar.ParseDate("2026-11-18")
	if _, err := svc.CreateHoliday(ctx, actor, wed, "Test holiday"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateBusinessHours(ctx, actor, sla.HoursInput{
		Weekdays: []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
		Opens:    7*time.Hour + 30*time.Minute, Closes: 16 * time.Hour,
	}); err != nil {
		t.Fatal(err)
	}
	s, err := svc.Schedules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	start := time.Date(2026, 11, 17, 15, 0, 0, 0, jkt) // Tue
	got := s.For(sla.CalendarBusinessHours).Add(start, 2*time.Hour)
	if want := time.Date(2026, 11, 19, 8, 30, 0, 0, jkt); !got.Equal(want) {
		t.Fatalf("due = %s, want %s (Wednesday is a holiday, hours 07:30–16:00)", got, want)
	}
	if got := s.For(sla.CalendarAlways).Add(start, 2*time.Hour); !got.Equal(start.Add(2 * time.Hour)) {
		t.Fatalf("24/7 due = %s", got)
	}
}

func TestPresetLoadIsIdempotentAndKeepsEdits(t *testing.T) {
	ctx := context.Background()
	svc := sla.NewService(testdb.New(t))
	idulFitri, _ := calendar.ParseDate("2027-03-10")
	if _, err := svc.CreateHoliday(ctx, actor, idulFitri, "Lebaran (kantor)"); err != nil {
		t.Fatal(err)
	}
	first, err := svc.LoadHolidayPreset(ctx, actor, 2027)
	if err != nil || first.Added == 0 || first.Skipped != 1 {
		t.Fatalf("first load = %+v, err %v", first, err)
	}
	second, err := svc.LoadHolidayPreset(ctx, actor, 2027)
	if err != nil || second.Added != 0 || second.Skipped != first.Added+first.Skipped {
		t.Fatalf("second load = %+v, err %v", second, err)
	}
	year := 2027
	holidays, err := svc.ListHolidays(ctx, &year)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range holidays {
		if h.Date == idulFitri && h.Name != "Lebaran (kantor)" {
			t.Fatalf("admin's name was overwritten: %q", h.Name)
		}
	}
	presets, err := svc.HolidayPresets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range presets {
		if p.Year == year && p.MissingCount != 0 {
			t.Fatalf("2027 missing = %d after load", p.MissingCount)
		}
	}
}
