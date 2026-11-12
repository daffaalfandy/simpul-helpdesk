package calendar_test

import (
	"errors"
	"math/rand/v2"
	"testing"
	"time"
	_ "time/tzdata" // tests must not depend on the host's zoneinfo

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
)

const (
	minuteLayout = "2006-01-02 15:04"
	nanoLayout   = "2006-01-02 15:04:05.999999999"

	zoneJakarta  = "Asia/Jakarta"
	zoneNewYork  = "America/New_York"
	zoneLondon   = "Europe/London"
	zoneLordHowe = "Australia/Lord_Howe"
	zoneSantiago = "America/Santiago"
)

var weekdays = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}

func loc(tb testing.TB, name string) *time.Location {
	tb.Helper()
	l, err := time.LoadLocation(name)
	if err != nil {
		tb.Fatal(err)
	}
	return l
}

// at parses "2026-11-16 09:00" (or with seconds and nanoseconds) in l.
func at(tb testing.TB, l *time.Location, s string) time.Time {
	tb.Helper()
	for _, layout := range []string{minuteLayout, nanoLayout} {
		if t, err := time.ParseInLocation(layout, s, l); err == nil {
			return t
		}
	}
	tb.Fatalf("bad time %q", s)
	return time.Time{}
}

func date(tb testing.TB, s string) calendar.Date {
	tb.Helper()
	d, err := calendar.ParseDate(s)
	if err != nil {
		tb.Fatal(err)
	}
	return d
}

func clock(tb testing.TB, s string) time.Duration {
	tb.Helper()
	d, err := calendar.ParseClock(s)
	if err != nil {
		tb.Fatal(err)
	}
	return d
}

func window(tb testing.TB, open, close string) calendar.Window {
	return calendar.Window{Open: clock(tb, open), Close: clock(tb, close)}
}

func mustHours(tb testing.TB, cfg calendar.Config) *calendar.BusinessHours {
	tb.Helper()
	b, err := calendar.NewBusinessHours(cfg)
	if err != nil {
		tb.Fatal(err)
	}
	return b
}

func dateRange(tb testing.TB, from, to string) []calendar.Date {
	var out []calendar.Date
	for d, end := date(tb, from), date(tb, to); !d.After(end); d = d.AddDays(1) {
		out = append(out, d)
	}
	return out
}

// officeHours is Jakarta, Mon–Fri 08:00–17:00. Holiday Wed 2026-11-18, and a holiday run
// the length of Idul Fitri: Mon 2026-11-30 to Mon 2026-12-07. 2026-11-16 is a Monday.
func officeHours(tb testing.TB) *calendar.BusinessHours {
	holidays := append([]calendar.Date{date(tb, "2026-11-18")}, dateRange(tb, "2026-11-30", "2026-12-07")...)
	return mustHours(tb, calendar.Config{
		Location: loc(tb, zoneJakarta),
		Week:     calendar.UniformWeek(weekdays, window(tb, "08:00", "17:00")),
		Holidays: holidays,
	})
}

func plainOfficeHours(tb testing.TB, zone string) *calendar.BusinessHours {
	return mustHours(tb, calendar.Config{
		Location: loc(tb, zone),
		Week:     calendar.UniformWeek(weekdays, window(tb, "08:00", "17:00")),
	})
}

func allDay(tb testing.TB, zone string) *calendar.BusinessHours {
	all := []time.Weekday{time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday}
	return mustHours(tb, calendar.Config{
		Location: loc(tb, zone),
		Week:     calendar.UniformWeek(all, window(tb, "00:00", "24:00")),
	})
}

func TestAdd(t *testing.T) {
	jkt := loc(t, zoneJakarta)
	office := officeHours(t)

	splitDay := mustHours(t, calendar.Config{Location: jkt, Week: calendar.Week{
		time.Monday: {window(t, "13:00", "17:00"), window(t, "08:00", "12:00")}, // unsorted on purpose
	}})
	lateShift := mustHours(t, calendar.Config{Location: jkt, Week: calendar.Week{
		time.Monday:  {window(t, "20:00", "24:00")},
		time.Tuesday: {window(t, "00:00", "02:00")},
	}})

	tests := []struct {
		name  string
		sched calendar.Schedule
		start time.Time
		d     time.Duration
		want  time.Time
	}{
		{"inside a window", office, at(t, jkt, "2026-11-16 09:00"), 2 * time.Hour, at(t, jkt, "2026-11-16 11:00")},
		{"before open", office, at(t, jkt, "2026-11-16 06:00"), 30 * time.Minute, at(t, jkt, "2026-11-16 08:30")},
		{"after close", office, at(t, jkt, "2026-11-16 18:00"), 30 * time.Minute, at(t, jkt, "2026-11-17 08:30")},
		{"start exactly at close", office, at(t, jkt, "2026-11-16 17:00"), 30 * time.Minute, at(t, jkt, "2026-11-17 08:30")},
		{"lands exactly on close", office, at(t, jkt, "2026-11-16 08:00"), 9 * time.Hour, at(t, jkt, "2026-11-16 17:00")},
		{"two working days", office, at(t, jkt, "2026-11-16 08:00"), 18 * time.Hour, at(t, jkt, "2026-11-17 17:00")},
		{"carries to next day", office, at(t, jkt, "2026-11-16 16:00"), 2 * time.Hour, at(t, jkt, "2026-11-17 09:00")},
		{"skips a holiday", office, at(t, jkt, "2026-11-17 16:00"), 2 * time.Hour, at(t, jkt, "2026-11-19 09:00")},
		{"starts on a holiday", office, at(t, jkt, "2026-11-18 10:00"), time.Hour, at(t, jkt, "2026-11-19 09:00")},
		{"over a weekend", office, at(t, jkt, "2026-11-20 16:30"), time.Hour, at(t, jkt, "2026-11-23 08:30")},
		{"starts on a weekend", office, at(t, jkt, "2026-11-21 10:00"), 15 * time.Minute, at(t, jkt, "2026-11-23 08:15")},
		{"one nanosecond after Sunday", office, at(t, jkt, "2026-11-22 23:59:59.999999999"), time.Nanosecond, at(t, jkt, "2026-11-23 08:00:00.000000001")},
		{"nanoseconds across close", office, at(t, jkt, "2026-11-16 16:59:59.999999999"), 2 * time.Nanosecond, at(t, jkt, "2026-11-17 08:00:00.000000001")},
		{"over a holiday run", office, at(t, jkt, "2026-11-27 16:00"), 2 * time.Hour, at(t, jkt, "2026-12-08 09:00")},
		{"five working days", office, at(t, jkt, "2026-11-20 13:00"), 45 * time.Hour, at(t, jkt, "2026-11-27 13:00")},
		{"52 weeks skip whole weeks", plainOfficeHours(t, zoneJakarta), at(t, jkt, "2026-01-05 08:00"), 52 * 45 * time.Hour, at(t, jkt, "2027-01-01 17:00")},
		{"zero inside a window", office, at(t, jkt, "2026-11-16 09:00"), 0, at(t, jkt, "2026-11-16 09:00")},
		{"zero outside hours", office, at(t, jkt, "2026-11-21 10:00"), 0, at(t, jkt, "2026-11-21 10:00")},
		{"negative returns start", office, at(t, jkt, "2026-11-16 09:00"), -time.Hour, at(t, jkt, "2026-11-16 09:00")},
		{"input in UTC", office, time.Date(2026, 11, 17, 1, 0, 0, 0, time.UTC), time.Hour, at(t, jkt, "2026-11-17 09:00")},
		{"holiday judged in the schedule's zone", office, time.Date(2026, 11, 17, 17, 30, 0, 0, time.UTC), time.Hour, at(t, jkt, "2026-11-19 09:00")},
		{"window closing at 24:00", lateShift, at(t, jkt, "2026-11-16 23:00"), 2 * time.Hour, at(t, jkt, "2026-11-17 01:00")},
		{"split day, before lunch", splitDay, at(t, jkt, "2026-11-16 11:30"), time.Hour, at(t, jkt, "2026-11-16 13:30")},
		{"split day, during lunch", splitDay, at(t, jkt, "2026-11-16 12:30"), time.Hour, at(t, jkt, "2026-11-16 14:00")},
		{"24/7", calendar.Always{}, at(t, jkt, "2026-11-21 10:00"), 4 * time.Hour, at(t, jkt, "2026-11-21 14:00")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sched.Add(tt.start, tt.d)
			if !got.Equal(tt.want) {
				t.Fatalf("Add(%s, %s) = %s, want %s", tt.start, tt.d, got.In(jkt), tt.want)
			}
		})
	}
}

func TestBetween(t *testing.T) {
	jkt := loc(t, zoneJakarta)
	office := officeHours(t)
	tests := []struct {
		name  string
		sched calendar.Schedule
		a, b  time.Time
		want  time.Duration
	}{
		{"inside a window", office, at(t, jkt, "2026-11-16 09:00"), at(t, jkt, "2026-11-16 11:30"), 150 * time.Minute},
		{"from before open", office, at(t, jkt, "2026-11-16 06:00"), at(t, jkt, "2026-11-16 09:00"), time.Hour},
		{"overnight closed", office, at(t, jkt, "2026-11-16 18:00"), at(t, jkt, "2026-11-17 07:00"), 0},
		{"over a weekend", office, at(t, jkt, "2026-11-20 16:00"), at(t, jkt, "2026-11-23 09:00"), 2 * time.Hour},
		{"over a holiday", office, at(t, jkt, "2026-11-17 16:00"), at(t, jkt, "2026-11-19 09:00"), 2 * time.Hour},
		{"week with a holiday", office, at(t, jkt, "2026-11-16 00:00"), at(t, jkt, "2026-11-23 00:00"), 36 * time.Hour},
		{"week without holidays", plainOfficeHours(t, zoneJakarta), at(t, jkt, "2026-11-16 00:00"), at(t, jkt, "2026-11-23 00:00"), 45 * time.Hour},
		{"empty range", office, at(t, jkt, "2026-11-16 09:00"), at(t, jkt, "2026-11-16 09:00"), 0},
		{"reversed range is negative", office, at(t, jkt, "2026-11-16 11:00"), at(t, jkt, "2026-11-16 09:00"), -2 * time.Hour},
		{"last nanosecond of a window", office, at(t, jkt, "2026-11-16 16:59:59.999999999"), at(t, jkt, "2026-11-16 17:00"), time.Nanosecond},
		{"close is not working time", office, at(t, jkt, "2026-11-16 17:00"), at(t, jkt, "2026-11-16 17:00:00.000000001"), 0},
		{"whole year of weekdays", plainOfficeHours(t, zoneJakarta), at(t, jkt, "2026-01-01 00:00"), at(t, jkt, "2027-01-01 00:00"), 261 * 9 * time.Hour},
		{"24/7", calendar.Always{}, at(t, jkt, "2026-11-21 10:00"), at(t, jkt, "2026-11-22 10:00"), 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sched.Between(tt.a, tt.b); got != tt.want {
				t.Fatalf("Between = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestOffsetChanges(t *testing.T) {
	ny := loc(t, zoneNewYork)
	london := loc(t, zoneLondon)
	nyAllDay := allDay(t, zoneNewYork)
	everyDay := []time.Weekday{time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday}
	repeatedHour := mustHours(t, calendar.Config{Location: ny, Week: calendar.UniformWeek(everyDay, window(t, "01:00", "02:00"))})
	skippedHour := mustHours(t, calendar.Config{Location: ny, Week: calendar.UniformWeek(everyDay, window(t, "02:00", "03:00"))})
	sundaySkippedHour := mustHours(t, calendar.Config{Location: ny, Week: calendar.Week{time.Sunday: {window(t, "02:00", "03:00")}}})

	t.Run("weekend spring-forward", func(t *testing.T) {
		got := plainOfficeHours(t, zoneNewYork).Add(at(t, ny, "2026-03-06 16:00"), 2*time.Hour)
		if want := at(t, ny, "2026-03-09 09:00"); !got.Equal(want) {
			t.Fatalf("got %s, want %s", got, want)
		}
	})
	betweens := []struct {
		name  string
		sched *calendar.BusinessHours
		a, b  time.Time
		want  time.Duration
	}{
		{"spring-forward weekend is 47h", nyAllDay, at(t, ny, "2026-03-07 00:00"), at(t, ny, "2026-03-09 00:00"), 47 * time.Hour},
		{"fall-back day is 25h", nyAllDay, at(t, ny, "2026-11-01 00:00"), at(t, ny, "2026-11-02 00:00"), 25 * time.Hour},
		{"repeated hour counts twice", repeatedHour, at(t, ny, "2026-11-01 00:00"), at(t, ny, "2026-11-02 00:00"), 2 * time.Hour},
		{"skipped hour has no time", skippedHour, at(t, ny, "2026-03-08 00:00"), at(t, ny, "2026-03-09 00:00"), 0},
		{"London fall-back day is 25h", allDay(t, zoneLondon), at(t, london, "2026-10-25 00:00"), at(t, london, "2026-10-26 00:00"), 25 * time.Hour},
	}
	for _, tt := range betweens {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sched.Between(tt.a, tt.b); got != tt.want {
				t.Fatalf("Between = %s, want %s", got, tt.want)
			}
		})
	}
	t.Run("24h across spring-forward", func(t *testing.T) {
		got := nyAllDay.Add(at(t, ny, "2026-03-08 00:00"), 24*time.Hour)
		if want := at(t, ny, "2026-03-09 01:00"); !got.Equal(want) {
			t.Fatalf("got %s, want %s", got, want)
		}
	})
	t.Run("only window is skipped", func(t *testing.T) {
		got := sundaySkippedHour.Add(at(t, ny, "2026-03-07 12:00"), 30*time.Minute)
		if want := at(t, ny, "2026-03-15 02:30"); !got.Equal(want) {
			t.Fatalf("got %s, want %s", got, want)
		}
	})
}

func TestNewBusinessHoursErrors(t *testing.T) {
	jkt := loc(t, zoneJakarta)
	tests := []struct {
		name string
		cfg  calendar.Config
		want error
	}{
		{"no location", calendar.Config{Week: calendar.UniformWeek(weekdays, window(t, "08:00", "17:00"))}, calendar.ErrNoLocation},
		{"all days closed", calendar.Config{Location: jkt}, calendar.ErrNoWorkingTime},
		{"open after close", calendar.Config{Location: jkt, Week: calendar.Week{time.Monday: {{Open: 17 * time.Hour, Close: 8 * time.Hour}}}}, calendar.ErrInvalidWindow},
		{"empty window", calendar.Config{Location: jkt, Week: calendar.Week{time.Monday: {{Open: 8 * time.Hour, Close: 8 * time.Hour}}}}, calendar.ErrInvalidWindow},
		{"past midnight", calendar.Config{Location: jkt, Week: calendar.Week{time.Monday: {{Open: 8 * time.Hour, Close: 25 * time.Hour}}}}, calendar.ErrInvalidWindow},
		{"negative open", calendar.Config{Location: jkt, Week: calendar.Week{time.Monday: {{Open: -time.Hour, Close: 8 * time.Hour}}}}, calendar.ErrInvalidWindow},
		{"overlapping", calendar.Config{Location: jkt, Week: calendar.Week{time.Monday: {window(t, "08:00", "12:00"), window(t, "11:00", "17:00")}}}, calendar.ErrOverlappingWindows},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := calendar.NewBusinessHours(tt.cfg); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
	t.Run("duplicate holidays and touching windows are fine", func(t *testing.T) {
		b := mustHours(t, calendar.Config{
			Location: jkt,
			Week:     calendar.Week{time.Monday: {window(t, "08:00", "12:00"), window(t, "12:00", "17:00")}},
			Holidays: []calendar.Date{date(t, "2026-11-16"), date(t, "2026-11-16")},
		})
		if got := b.Between(at(t, jkt, "2026-11-16 00:00"), at(t, jkt, "2026-11-24 00:00")); got != 9*time.Hour {
			t.Fatalf("got %s, want 9h", got)
		}
	})
}

func TestWorkingDay(t *testing.T) {
	if got := calendar.UniformWeek(weekdays, window(t, "08:00", "17:00")).WorkingDay(); got != 9*time.Hour {
		t.Fatalf("uniform: got %s", got)
	}
	mixed := calendar.Week{
		time.Monday: {window(t, "08:00", "12:00"), window(t, "13:00", "17:00")},
		time.Friday: {window(t, "07:30", "16:30")},
	}
	if got := mixed.WorkingDay(); got != 9*time.Hour {
		t.Fatalf("mixed: got %s", got)
	}
}

func TestHolidaysOnClosedDaysChangeNothing(t *testing.T) {
	jkt := loc(t, zoneJakarta)
	weekend := mustHours(t, calendar.Config{
		Location: jkt,
		Week:     calendar.UniformWeek(weekdays, window(t, "08:00", "17:00")),
		Holidays: []calendar.Date{date(t, "2026-11-21"), date(t, "2026-11-22")},
	})
	a, b := at(t, jkt, "2026-11-16 00:00"), at(t, jkt, "2026-11-30 00:00")
	if got, want := weekend.Between(a, b), plainOfficeHours(t, zoneJakarta).Between(a, b); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

// Properties over random weeks, holidays and zones.

var propertyZones = []string{zoneJakarta, zoneNewYork, zoneLondon, zoneLordHowe, zoneSantiago}

const (
	propertyCases   = 300
	oracleCases     = 30
	slotMinutes     = 15
	maxWindowsDay   = 2
	maxHolidays     = 12
	holidaySpanDays = 120
	maxSpanDays     = 40
	oracleSpanDays  = 4
)

var propertyEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func randomSchedule(tb testing.TB, r *rand.Rand, zone string) *calendar.BusinessHours {
	slotsPerDay := int(calendar.DayLength / (slotMinutes * time.Minute))
	slot := slotMinutes * time.Minute
	for {
		var week calendar.Week
		for day := range week {
			cursor := 0
			for range r.IntN(maxWindowsDay + 1) {
				if cursor >= slotsPerDay {
					break
				}
				open := cursor + r.IntN(slotsPerDay-cursor)
				closeAt := open + 1 + r.IntN(slotsPerDay-open)
				week[day] = append(week[day], calendar.Window{Open: time.Duration(open) * slot, Close: time.Duration(closeAt) * slot})
				cursor = closeAt
			}
		}
		var holidays []calendar.Date
		for range r.IntN(maxHolidays) {
			holidays = append(holidays, calendar.DateOf(propertyEpoch).AddDays(r.IntN(holidaySpanDays)))
		}
		b, err := calendar.NewBusinessHours(calendar.Config{Location: loc(tb, zone), Week: week, Holidays: holidays})
		if errors.Is(err, calendar.ErrNoWorkingTime) {
			continue
		}
		if err != nil {
			tb.Fatal(err)
		}
		return b
	}
}

func randomInstant(r *rand.Rand) time.Time {
	return propertyEpoch.Add(time.Duration(r.Int64N(int64(holidaySpanDays * calendar.DayLength))))
}

func TestProperties(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for _, zone := range propertyZones {
		t.Run(zone, func(t *testing.T) {
			for i := range propertyCases {
				b := randomSchedule(t, r, zone)
				s := randomInstant(r)
				d := time.Duration(r.Int64N(int64(maxSpanDays * calendar.DayLength / 4)))
				due := b.Add(s, d)
				if got := b.Between(s, due); d > 0 && got != d {
					t.Fatalf("case %d: Between(s, Add(s, %s)) = %s", i, d, got)
				}
				if due.After(s) && b.Between(s, due.Add(-time.Nanosecond)) >= d {
					t.Fatalf("case %d: Add(s, %s) = %s is not the earliest", i, d, due)
				}
				a, c := s, s.Add(time.Duration(r.Int64N(int64(maxSpanDays*calendar.DayLength))))
				m := a.Add(time.Duration(r.Int64N(int64(c.Sub(a)) + 1)))
				if b.Between(a, m)+b.Between(m, c) != b.Between(a, c) {
					t.Fatalf("case %d: Between is not additive", i)
				}
				if b.Between(c, a) != -b.Between(a, c) {
					t.Fatalf("case %d: Between is not antisymmetric", i)
				}
				if later := b.Add(m, d); later.Before(due) && !m.Before(s) {
					t.Fatalf("case %d: Add is not monotonic", i)
				}
				if open := b.IsOpen(m); open != (b.Between(m, m.Add(time.Nanosecond)) == time.Nanosecond) {
					t.Fatalf("case %d: IsOpen(%s) = %v disagrees with Between", i, m, open)
				}
			}
		})
	}
}

// TestOracle counts open minutes one by one with IsOpen and compares with Between.
func TestOracle(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for _, zone := range propertyZones {
		t.Run(zone, func(t *testing.T) {
			for i := range oracleCases {
				b := randomSchedule(t, r, zone)
				a := randomInstant(r).Truncate(time.Minute)
				minutes := r.IntN(oracleSpanDays * int(calendar.DayLength/time.Minute))
				var open int
				for m := range minutes {
					if b.IsOpen(a.Add(time.Duration(m) * time.Minute)) {
						open++
					}
				}
				if got := b.Between(a, a.Add(time.Duration(minutes)*time.Minute)); got != time.Duration(open)*time.Minute {
					t.Fatalf("case %d: Between = %s, oracle %dm", i, got, open)
				}
			}
		})
	}
}

func TestAllDayEqualsAlways(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	for _, zone := range propertyZones {
		b := allDay(t, zone)
		for range propertyCases {
			a := randomInstant(r)
			c := a.Add(time.Duration(r.Int64N(int64(maxSpanDays * calendar.DayLength))))
			if got, want := b.Between(a, c), (calendar.Always{}).Between(a, c); got != want {
				t.Fatalf("%s: Between = %s, want %s", zone, got, want)
			}
			if got, want := b.Add(a, c.Sub(a)), (calendar.Always{}).Add(a, c.Sub(a)); !got.Equal(want) {
				t.Fatalf("%s: Add = %s, want %s", zone, got, want)
			}
		}
	}
}

func BenchmarkAddFiveWorkingDays(b *testing.B) {
	s := officeHours(b)
	start := at(b, loc(b, zoneJakarta), "2026-11-20 13:00")
	for b.Loop() {
		s.Add(start, 45*time.Hour)
	}
}

func BenchmarkBetweenOneYearNewYork(b *testing.B) {
	s := plainOfficeHours(b, zoneNewYork)
	ny := loc(b, zoneNewYork)
	a, c := at(b, ny, "2026-01-01 00:00"), at(b, ny, "2027-01-01 00:00")
	for b.Loop() {
		s.Between(a, c)
	}
}

func BenchmarkBetweenCenturyJakarta(b *testing.B) {
	s := officeHours(b)
	jkt := loc(b, zoneJakarta)
	a, c := at(b, jkt, "2000-01-01 00:00"), at(b, jkt, "2100-01-01 00:00")
	for b.Loop() {
		s.Between(a, c)
	}
}
