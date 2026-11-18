package tickets

import (
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
)

// ReportedRange keeps the tickets reported (created) from day From to day To, both included,
// read in the organization's time zone (owner, 18 Nov). A nil end leaves that side open.
type ReportedRange struct {
	From *calendar.Date
	To   *calendar.Date
}

// IsZero reports whether the range sets no limit at all.
func (r ReportedRange) IsZero() bool { return r.From == nil && r.To == nil }

// Validate refuses an end before the start.
func (r ReportedRange) Validate() error {
	if r.From != nil && r.To != nil && r.To.Before(*r.From) {
		return apperr.Invalid(apperr.CodeInvalidDateRange, "The end date is before the start date", "")
	}
	return nil
}

// Bounds returns local midnight at the start of From and local midnight after To in loc, the
// end excluded; nil for an open side. Counting calendar days keeps a day 23 or 25 hours long
// where the clocks change.
func (r ReportedRange) Bounds(loc *time.Location) (start, end *time.Time) {
	if r.From != nil {
		t := r.From.In(loc)
		start = &t
	}
	if r.To != nil {
		t := r.To.AddDays(1).In(loc)
		end = &t
	}
	return start, end
}

// CurrentMonth runs from the 1st to the last day of the month today falls in.
func CurrentMonth(today calendar.Date) ReportedRange {
	first := calendar.NewDate(today.Year, today.Month, 1)
	last := calendar.NewDate(today.Year, today.Month+1, 0) // day 0 is the day before the 1st
	return ReportedRange{From: &first, To: &last}
}

// orCurrentMonth is the range the board's resolved column covers: r, or the current month when
// r is empty. The column names both of its days, so half a range is refused.
func (r ReportedRange) orCurrentMonth(today calendar.Date) (from, to calendar.Date, err error) {
	switch {
	case r.IsZero():
		r = CurrentMonth(today)
	case r.From == nil || r.To == nil:
		return calendar.Date{}, calendar.Date{}, apperr.Invalid(apperr.CodeInvalidDateRange, "Choose both a start and an end date", "")
	}
	if err := r.Validate(); err != nil {
		return calendar.Date{}, calendar.Date{}, err
	}
	return *r.From, *r.To, nil
}
