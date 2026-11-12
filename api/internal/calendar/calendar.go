// Package calendar does working-time arithmetic: when is a target of N working minutes
// due, and how much working time lies between two instants. It is pure (no I/O) and is
// shared by the SLA engine, UAT auto-approve and auto-close windows.
//
// Rules:
//   - Working time is counted in the schedule's location, by wall clock.
//   - A window is open at its start and closed at its end: [Open, Close).
//   - A due time that lands exactly on a window's close stays there (Mon 08:00 + 9h on a
//     08:00–17:00 day is Mon 17:00). Callers decide breach with now.After(due).
//   - Holidays are civil dates in the schedule's location; a holiday has no working time.
//   - Offset changes (DST) are exact: a skipped hour has no working time and a repeated
//     hour counts twice, because time is measured on the wall clock between transitions.
package calendar

import (
	"errors"
	"slices"
	"time"
)

// DayLength is the span of one wall-clock day.
const DayLength = 24 * time.Hour

// Construction errors.
var (
	ErrNoLocation         = errors.New("calendar: location is required")
	ErrInvalidWindow      = errors.New("calendar: window must satisfy 0 <= open < close <= 24:00")
	ErrOverlappingWindows = errors.New("calendar: windows on the same day overlap")
	ErrNoWorkingTime      = errors.New("calendar: the week has no working time")
)

// Schedule measures working time.
type Schedule interface {
	// Add returns the earliest instant t >= start with Between(start, t) == d.
	// d <= 0 returns start unchanged.
	Add(start time.Time, d time.Duration) time.Time
	// Between returns the working time in [a, b). It is negative when b is before a.
	Between(a, b time.Time) time.Duration
	// IsOpen reports whether t is inside working time.
	IsOpen(t time.Time) bool
}

// Always is a 24/7 schedule.
type Always struct{}

var _ Schedule = Always{}

// Add implements Schedule.
func (Always) Add(start time.Time, d time.Duration) time.Time {
	if d <= 0 {
		return start
	}
	return start.Add(d)
}

// Between implements Schedule.
func (Always) Between(a, b time.Time) time.Duration { return b.Sub(a) }

// IsOpen implements Schedule.
func (Always) IsOpen(time.Time) bool { return true }

// Window is a span of a day, as offsets from local midnight.
type Window struct {
	Open  time.Duration
	Close time.Duration
}

// Length is the window's duration.
func (w Window) Length() time.Duration { return w.Close - w.Open }

func (w Window) valid() bool { return w.Open >= 0 && w.Open < w.Close && w.Close <= DayLength }

// Week holds each weekday's windows, indexed by time.Weekday. An empty day is closed.
type Week [daysPerWeek][]Window

// UniformWeek opens the same window on each of the given days.
func UniformWeek(days []time.Weekday, w Window) Week {
	var week Week
	for _, d := range days {
		week[d] = []Window{w}
	}
	return week
}

// WorkingDay is the longest single day of working time; it defines "one working day".
func (wk Week) WorkingDay() time.Duration {
	var longest time.Duration
	for _, windows := range wk {
		longest = max(longest, total(windows))
	}
	return longest
}

func total(windows []Window) time.Duration {
	var sum time.Duration
	for _, w := range windows {
		sum += w.Length()
	}
	return sum
}

// Config describes a business-hours schedule.
type Config struct {
	Location *time.Location
	Week     Week
	Holidays []Date
}

// BusinessHours is a Schedule with weekly windows and holidays.
type BusinessHours struct {
	loc       *time.Location
	week      Week
	dayTotal  [daysPerWeek]time.Duration
	weekTotal time.Duration
	holidays  []int64 // sorted, unique day numbers
}

var _ Schedule = (*BusinessHours)(nil)

// NewBusinessHours validates cfg and builds the schedule.
func NewBusinessHours(cfg Config) (*BusinessHours, error) {
	if cfg.Location == nil {
		return nil, ErrNoLocation
	}
	b := &BusinessHours{loc: cfg.Location}
	for day, windows := range cfg.Week {
		sorted := slices.Clone(windows)
		slices.SortFunc(sorted, func(x, y Window) int { return int(x.Open - y.Open) })
		for i, w := range sorted {
			if !w.valid() {
				return nil, ErrInvalidWindow
			}
			if i > 0 && w.Open < sorted[i-1].Close {
				return nil, ErrOverlappingWindows
			}
		}
		b.week[day] = sorted
		b.dayTotal[day] = total(sorted)
		b.weekTotal += b.dayTotal[day]
	}
	if b.weekTotal == 0 {
		return nil, ErrNoWorkingTime
	}
	b.holidays = make([]int64, 0, len(cfg.Holidays))
	for _, h := range cfg.Holidays {
		b.holidays = append(b.holidays, h.dayNumber())
	}
	slices.Sort(b.holidays)
	b.holidays = slices.Compact(b.holidays)
	return b, nil
}

// Location returns the schedule's time zone.
func (b *BusinessHours) Location() *time.Location { return b.loc }

// IsOpen implements Schedule.
func (b *BusinessHours) IsOpen(t time.Time) bool {
	day, tod := split(b.wall(t))
	if b.isHoliday(day) {
		return false
	}
	for _, w := range b.week[weekdayOf(day)] {
		if tod >= w.Open && tod < w.Close {
			return true
		}
	}
	return false
}

// Between implements Schedule.
func (b *BusinessHours) Between(a, c time.Time) time.Duration {
	switch {
	case a.Equal(c):
		return 0
	case c.Before(a):
		return -b.Between(c, a)
	}
	var sum time.Duration
	for cur := a; cur.Before(c); {
		end := b.stretchEnd(cur)
		if end.IsZero() || end.After(c) {
			end = c
		}
		ws := b.wall(cur)
		sum += b.wallBetween(ws, ws.Add(end.Sub(cur)))
		cur = end
	}
	return sum
}

// Add implements Schedule.
func (b *BusinessHours) Add(start time.Time, d time.Duration) time.Time {
	if d <= 0 {
		return start
	}
	remaining := d
	for cur := start; ; {
		ws := b.wall(cur)
		end := b.stretchEnd(cur)
		if !end.IsZero() {
			if capacity := b.wallBetween(ws, ws.Add(end.Sub(cur))); capacity < remaining {
				remaining -= capacity
				cur = end
				continue
			}
		}
		return cur.Add(b.wallAdd(ws, remaining).Sub(ws))
	}
}

// stretchEnd is the next instant the location's UTC offset changes, or zero if never.
func (b *BusinessHours) stretchEnd(t time.Time) time.Time {
	_, end := t.In(b.loc).ZoneBounds()
	return end
}

// wall maps an instant to its wall-clock reading in the schedule's location, expressed
// as a UTC time so that wall arithmetic is plain addition within one offset stretch.
func (b *BusinessHours) wall(t time.Time) time.Time {
	_, offset := t.In(b.loc).Zone()
	return t.UTC().Add(time.Duration(offset) * time.Second)
}

// wallBetween counts working time between two wall times (ws <= we) in one offset stretch.
func (b *BusinessHours) wallBetween(ws, we time.Time) time.Duration {
	d0, t0 := split(ws)
	d1, t1 := split(we)
	if d0 == d1 {
		return b.dayOverlap(d0, t0, t1)
	}
	return b.dayOverlap(d0, t0, DayLength) + b.fullDays(d0+1, d1) + b.dayOverlap(d1, 0, t1)
}

// wallAdd returns the earliest wall time w >= ws with wallBetween(ws, w) == remaining (> 0).
func (b *BusinessHours) wallAdd(ws time.Time, remaining time.Duration) time.Time {
	day, tod := split(ws)
	if !b.isHoliday(day) {
		for _, w := range b.week[weekdayOf(day)] {
			if w.Close <= tod {
				continue
			}
			from := max(w.Open, tod)
			if avail := w.Close - from; remaining <= avail {
				return dayStart(day).Add(from + remaining)
			} else {
				remaining -= avail
			}
		}
	}
	day++
	// Skip whole weeks while the time left is longer than a week. Holidays only remove
	// working time, so k weeks never hold more than k*weekTotal and this can't overshoot.
	for {
		weeks := int64((remaining - 1) / b.weekTotal)
		if weeks == 0 {
			break
		}
		next := day + weeks*daysPerWeek
		remaining -= b.fullDays(day, next)
		day = next
	}
	for ; ; day++ {
		if b.isHoliday(day) {
			continue
		}
		for _, w := range b.week[weekdayOf(day)] {
			if remaining <= w.Length() {
				return dayStart(day).Add(w.Open + remaining)
			}
			remaining -= w.Length()
		}
	}
}

// dayOverlap is the working time of day number `day` within [from, to) of that day.
func (b *BusinessHours) dayOverlap(day int64, from, to time.Duration) time.Duration {
	if b.isHoliday(day) {
		return 0
	}
	var sum time.Duration
	for _, w := range b.week[weekdayOf(day)] {
		if lo, hi := max(w.Open, from), min(w.Close, to); hi > lo {
			sum += hi - lo
		}
	}
	return sum
}

// fullDays is the working time of whole days in [from, to).
func (b *BusinessHours) fullDays(from, to int64) time.Duration {
	if to <= from {
		return 0
	}
	weeks := (to - from) / daysPerWeek
	sum := time.Duration(weeks) * b.weekTotal
	for day := from + weeks*daysPerWeek; day < to; day++ {
		sum += b.dayTotal[weekdayOf(day)]
	}
	i, _ := slices.BinarySearch(b.holidays, from)
	for ; i < len(b.holidays) && b.holidays[i] < to; i++ {
		sum -= b.dayTotal[weekdayOf(b.holidays[i])]
	}
	return sum
}

func (b *BusinessHours) isHoliday(day int64) bool {
	_, found := slices.BinarySearch(b.holidays, day)
	return found
}

// split breaks a wall time (as UTC) into its day number and time of day.
func split(w time.Time) (int64, time.Duration) {
	day := floorDiv(w.Unix(), secondsPerDay)
	return day, w.Sub(dayStart(day))
}

func dayStart(day int64) time.Time { return time.Unix(day*secondsPerDay, 0).UTC() }
