package calendar

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ErrInvalidClock is returned for text that isn't "HH:MM" between 00:00 and 24:00.
var ErrInvalidClock = errors.New("calendar: time of day must be HH:MM between 00:00 and 24:00")

const (
	clockLen       = len("15:04")
	clockSeparator = ':'
	maxClockHour   = 24
)

// ParseClock parses "HH:MM" into an offset from midnight. "24:00" is the end of the day.
func ParseClock(s string) (time.Duration, error) {
	if len(s) != clockLen || s[2] != clockSeparator || !digits(s[:2]) || !digits(s[3:]) {
		return 0, ErrInvalidClock
	}
	h, _ := strconv.Atoi(s[:2])
	m, _ := strconv.Atoi(s[3:])
	if m >= int(time.Hour/time.Minute) || h > maxClockHour || (h == maxClockHour && m != 0) {
		return 0, ErrInvalidClock
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, nil
}

func digits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// FormatClock formats an offset from midnight as "HH:MM" (whole minutes; "24:00" allowed).
func FormatClock(d time.Duration) string {
	d = d.Truncate(time.Minute)
	return fmt.Sprintf("%02d:%02d", int(d/time.Hour), int(d%time.Hour/time.Minute))
}
