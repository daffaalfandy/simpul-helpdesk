// Package report computes the SLA report (HANDOFF §10) over a range of months from ticket rows.
// It has no database access, so the report page, the Excel file and the PDF share one set of rules.
package report

import (
	"errors"
	"fmt"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
)

// MonthLayout is how a month is written in URLs and the API: "2026-11".
const MonthLayout = "2006-01"

// MonthsBack is how far the report reaches: the current month and this many before it (owner,
// 18 Nov). Older months are refused.
const MonthsBack = 12

// ErrInvalidMonth is returned when text isn't a month in MonthLayout.
var ErrInvalidMonth = errors.New("report: invalid month")

// Month is a calendar month with no time zone. A report month starts at local midnight on the
// 1st in the organisation's time zone.
type Month struct {
	Year  int
	Month time.Month
}

// ParseMonth parses "2026-11".
func ParseMonth(s string) (Month, error) {
	t, err := time.Parse(MonthLayout, s)
	if err != nil || t.Format(MonthLayout) != s {
		return Month{}, fmt.Errorf("%w: %q", ErrInvalidMonth, s)
	}
	return Month{Year: t.Year(), Month: t.Month()}, nil
}

// MonthOf returns the month t falls in, read in loc.
func MonthOf(t time.Time, loc *time.Location) Month {
	d := calendar.DateOf(t.In(loc))
	return Month{Year: d.Year, Month: d.Month}
}

// String formats the month as "2026-11".
func (m Month) String() string { return m.first().In(time.UTC).Format(MonthLayout) }

// Bounds returns local midnight on the 1st of m and of the next month, in loc.
func (m Month) Bounds(loc *time.Location) (start, end time.Time) {
	return m.first().In(loc), m.Next().first().In(loc)
}

// Oldest returns the earliest month the report shows when current is this month.
func Oldest(current Month) Month { return current.add(-MonthsBack) }

// Next returns the month after m.
func (m Month) Next() Month { return m.add(1) }

// Prev returns the month before m.
func (m Month) Prev() Month { return m.add(-1) }

// Before reports whether m comes before o.
func (m Month) Before(o Month) bool {
	return m.Year < o.Year || (m.Year == o.Year && m.Month < o.Month)
}

// After reports whether m comes after o.
func (m Month) After(o Month) bool { return o.Before(m) }

// Errors MonthRange.Validate returns.
var (
	ErrMonthInFuture  = errors.New("report: month hasn't started")
	ErrMonthTooOld    = errors.New("report: month before the oldest one the report shows")
	ErrMonthsReversed = errors.New("report: end month before start month")
)

// MonthRange is the whole months a report covers, From to To, both included (owner, 18 Nov).
type MonthRange struct {
	From, To Month
}

// SingleMonth is the range of one month.
func SingleMonth(m Month) MonthRange { return MonthRange{From: m, To: m} }

// Single reports whether r is one month.
func (r MonthRange) Single() bool { return r.From == r.To }

// Validate keeps r inside the report's window when current is this month: both ends from
// Oldest(current) to current, and To not before From.
func (r MonthRange) Validate(current Month) error {
	oldest := Oldest(current)
	switch {
	case r.From.After(current) || r.To.After(current):
		return ErrMonthInFuture
	case r.From.Before(oldest) || r.To.Before(oldest):
		return ErrMonthTooOld
	case r.To.Before(r.From):
		return ErrMonthsReversed
	}
	return nil
}

// Bounds returns local midnight on the 1st of From and on the 1st of the month after To, in loc.
func (r MonthRange) Bounds(loc *time.Location) (start, end time.Time) {
	start, _ = r.From.Bounds(loc)
	_, end = r.To.Bounds(loc)
	return start, end
}

func (m Month) add(n int) Month {
	d := calendar.NewDate(m.Year, m.Month+time.Month(n), 1)
	return Month{Year: d.Year, Month: d.Month}
}

func (m Month) first() calendar.Date { return calendar.NewDate(m.Year, m.Month, 1) }
