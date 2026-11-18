package tickets

import (
	"testing"
	"time"

	_ "time/tzdata"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
)

func date(y int, m time.Month, d int) *calendar.Date {
	v := calendar.NewDate(y, m, d)
	return &v
}

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestReportedRangeBounds(t *testing.T) {
	jakarta := mustLoad(t, "Asia/Jakarta")
	// Berlin moves its clocks forward on 29 Mar 2026, so that day has 23 hours.
	berlin := mustLoad(t, "Europe/Berlin")
	cases := []struct {
		name       string
		r          ReportedRange
		loc        *time.Location
		start, end string // RFC 3339 in UTC; empty for an open side
		zero       bool
	}{
		{name: "no limit", r: ReportedRange{}, loc: jakarta, zero: true},
		{name: "from only", r: ReportedRange{From: date(2026, time.November, 1)}, loc: jakarta,
			start: "2026-10-31T17:00:00Z"},
		{name: "to only", r: ReportedRange{To: date(2026, time.November, 30)}, loc: jakarta,
			end: "2026-11-30T17:00:00Z"},
		{name: "one day", r: ReportedRange{From: date(2026, time.November, 18), To: date(2026, time.November, 18)}, loc: jakarta,
			start: "2026-11-17T17:00:00Z", end: "2026-11-18T17:00:00Z"},
		{name: "end of year", r: ReportedRange{From: date(2026, time.December, 1), To: date(2026, time.December, 31)}, loc: jakarta,
			start: "2026-11-30T17:00:00Z", end: "2026-12-31T17:00:00Z"},
		{name: "clocks go forward", r: ReportedRange{From: date(2026, time.March, 29), To: date(2026, time.March, 29)}, loc: berlin,
			start: "2026-03-28T23:00:00Z", end: "2026-03-29T22:00:00Z"},
		{name: "clocks go back", r: ReportedRange{From: date(2026, time.October, 25), To: date(2026, time.October, 25)}, loc: berlin,
			start: "2026-10-24T22:00:00Z", end: "2026-10-25T23:00:00Z"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.r.IsZero(); got != c.zero {
				t.Errorf("IsZero = %v, want %v", got, c.zero)
			}
			start, end := c.r.Bounds(c.loc)
			check := func(side string, got *time.Time, want string) {
				switch {
				case want == "" && got != nil:
					t.Errorf("%s = %s, want open", side, got.UTC().Format(time.RFC3339))
				case want != "" && (got == nil || got.UTC().Format(time.RFC3339) != want):
					t.Errorf("%s = %v, want %s", side, got, want)
				}
			}
			check("start", start, c.start)
			check("end", end, c.end)
		})
	}
}

func TestReportedRangeValidate(t *testing.T) {
	cases := []struct {
		name string
		r    ReportedRange
		ok   bool
	}{
		{"no limit", ReportedRange{}, true},
		{"from only", ReportedRange{From: date(2026, time.November, 1)}, true},
		{"to only", ReportedRange{To: date(2026, time.November, 1)}, true},
		{"same day", ReportedRange{From: date(2026, time.November, 1), To: date(2026, time.November, 1)}, true},
		{"reversed", ReportedRange{From: date(2026, time.November, 2), To: date(2026, time.November, 1)}, false},
	}
	for _, c := range cases {
		err := c.r.Validate()
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if !c.ok && !apperr.IsCode(err, apperr.CodeInvalidDateRange) {
			t.Errorf("%s: err = %v, want %s", c.name, err, apperr.CodeInvalidDateRange)
		}
	}
}

func TestCurrentMonth(t *testing.T) {
	cases := []struct {
		today, from, to calendar.Date
	}{
		{*date(2026, time.November, 18), *date(2026, time.November, 1), *date(2026, time.November, 30)},
		{*date(2026, time.December, 31), *date(2026, time.December, 1), *date(2026, time.December, 31)},
		{*date(2028, time.February, 1), *date(2028, time.February, 1), *date(2028, time.February, 29)},
		{*date(2026, time.February, 28), *date(2026, time.February, 1), *date(2026, time.February, 28)},
	}
	for _, c := range cases {
		r := CurrentMonth(c.today)
		if *r.From != c.from || *r.To != c.to {
			t.Errorf("CurrentMonth(%s) = %s..%s, want %s..%s", c.today, r.From, r.To, c.from, c.to)
		}
	}
}

// The board's resolved column always names both of its days.
func TestOrCurrentMonth(t *testing.T) {
	today := *date(2026, time.November, 18)
	cases := []struct {
		name     string
		r        ReportedRange
		from, to calendar.Date
		ok       bool
	}{
		{"default", ReportedRange{}, *date(2026, time.November, 1), *date(2026, time.November, 30), true},
		{"asked", ReportedRange{From: date(2026, time.October, 1), To: date(2026, time.October, 31)},
			*date(2026, time.October, 1), *date(2026, time.October, 31), true},
		{"from only", ReportedRange{From: date(2026, time.October, 1)}, calendar.Date{}, calendar.Date{}, false},
		{"to only", ReportedRange{To: date(2026, time.October, 31)}, calendar.Date{}, calendar.Date{}, false},
		{"reversed", ReportedRange{From: date(2026, time.October, 31), To: date(2026, time.October, 1)}, calendar.Date{}, calendar.Date{}, false},
	}
	for _, c := range cases {
		from, to, err := c.r.orCurrentMonth(today)
		if c.ok && (err != nil || from != c.from || to != c.to) {
			t.Errorf("%s: %s..%s, %v; want %s..%s", c.name, from, to, err, c.from, c.to)
		}
		if !c.ok && !apperr.IsCode(err, apperr.CodeInvalidDateRange) {
			t.Errorf("%s: err = %v, want %s", c.name, err, apperr.CodeInvalidDateRange)
		}
	}
}
