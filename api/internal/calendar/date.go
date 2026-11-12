package calendar

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"time"
)

// DateLayout is the ISO 8601 calendar-date layout used in JSON, SQL and the API.
const DateLayout = time.DateOnly

const (
	secondsPerDay = int64(24 * time.Hour / time.Second)
	daysPerWeek   = 7
	// epochWeekday is the weekday of day number 0 (1970-01-01).
	epochWeekday = time.Thursday
)

// ErrInvalidDate is returned when text or a database value isn't a calendar date.
var ErrInvalidDate = errors.New("calendar: invalid date")

// Date is a civil calendar date with no time zone, e.g. a holiday or a contract start.
// The zero value is not a valid date; use NewDate, ParseDate or DateOf.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// NewDate returns the date, normalising out-of-range values like time.Date does.
func NewDate(year int, month time.Month, day int) Date {
	return DateOf(time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
}

// DateOf returns the civil date of t in t's location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// ParseDate parses "2006-01-02".
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("%w: %q", ErrInvalidDate, s)
	}
	return DateOf(t), nil
}

// String formats the date as "2006-01-02".
func (d Date) String() string { return d.midnightUTC().Format(DateLayout) }

// IsZero reports whether d is the zero value.
func (d Date) IsZero() bool { return d == Date{} }

// AddDays returns the date n days later (or earlier when n is negative).
func (d Date) AddDays(n int) Date { return NewDate(d.Year, d.Month, d.Day+n) }

// Weekday returns the day of the week.
func (d Date) Weekday() time.Weekday { return d.midnightUTC().Weekday() }

// Compare returns -1, 0 or +1 as d is before, equal to or after o.
func (d Date) Compare(o Date) int {
	switch a, b := d.dayNumber(), o.dayNumber(); {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// Before reports whether d is before o.
func (d Date) Before(o Date) bool { return d.Compare(o) < 0 }

// After reports whether d is after o.
func (d Date) After(o Date) bool { return d.Compare(o) > 0 }

// In returns midnight at the start of d in loc.
func (d Date) In(loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

// MarshalText implements encoding.TextMarshaler.
func (d Date) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Date) UnmarshalText(b []byte) error {
	parsed, err := ParseDate(string(b))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Scan implements sql.Scanner. Postgres dates arrive as time.Time at UTC midnight.
func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case time.Time:
		*d = DateOf(v)
		return nil
	case string:
		return d.UnmarshalText([]byte(v))
	case []byte:
		return d.UnmarshalText(v)
	default:
		return fmt.Errorf("%w: cannot scan %T", ErrInvalidDate, src)
	}
}

// Value implements driver.Valuer.
func (d Date) Value() (driver.Value, error) { return d.midnightUTC(), nil }

func (d Date) midnightUTC() time.Time { return d.In(time.UTC) }

// dayNumber counts days since 1970-01-01.
func (d Date) dayNumber() int64 { return floorDiv(d.midnightUTC().Unix(), secondsPerDay) }

// weekdayOf returns the weekday of a day number.
func weekdayOf(day int64) time.Weekday {
	return time.Weekday((day%daysPerWeek + daysPerWeek + int64(epochWeekday)) % daysPerWeek)
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}
