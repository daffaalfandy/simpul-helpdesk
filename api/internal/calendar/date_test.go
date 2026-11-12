package calendar_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
)

func TestDate(t *testing.T) {
	d := date(t, "2028-02-28")
	if got := d.AddDays(1).String(); got != "2028-02-29" {
		t.Fatalf("leap day: got %s", got)
	}
	if got := d.AddDays(2).String(); got != "2028-03-01" {
		t.Fatalf("after leap day: got %s", got)
	}
	if got := date(t, "2026-11-16").Weekday(); got != time.Monday {
		t.Fatalf("weekday: got %s", got)
	}
	if !date(t, "2026-01-01").Before(date(t, "2026-01-02")) || date(t, "2026-01-01").After(date(t, "2026-01-02")) {
		t.Fatal("ordering is wrong")
	}
	if date(t, "1969-12-31").Compare(date(t, "1970-01-01")) != -1 {
		t.Fatal("dates before 1970 must order correctly")
	}
	jkt := loc(t, zoneJakarta)
	if got := calendar.DateOf(time.Date(2026, 11, 17, 17, 30, 0, 0, time.UTC).In(jkt)); got != date(t, "2026-11-18") {
		t.Fatalf("DateOf uses the time's zone: got %s", got)
	}
	for _, bad := range []string{"", "2026-13-01", "2026-02-30", "26-01-01", "2026/01/01"} {
		if _, err := calendar.ParseDate(bad); !errors.Is(err, calendar.ErrInvalidDate) {
			t.Errorf("ParseDate(%q) err = %v", bad, err)
		}
	}
}

func TestDateEncoding(t *testing.T) {
	type holder struct {
		Date calendar.Date `json:"date"`
	}
	b, err := json.Marshal(holder{Date: date(t, "2027-03-10")})
	if err != nil || string(b) != `{"date":"2027-03-10"}` {
		t.Fatalf("marshal: %s %v", b, err)
	}
	var h holder
	if err := json.Unmarshal(b, &h); err != nil || h.Date != date(t, "2027-03-10") {
		t.Fatalf("unmarshal: %+v %v", h, err)
	}

	var scanned calendar.Date
	for _, src := range []any{time.Date(2027, 3, 10, 0, 0, 0, 0, time.UTC), "2027-03-10", []byte("2027-03-10")} {
		if err := scanned.Scan(src); err != nil || scanned != date(t, "2027-03-10") {
			t.Fatalf("Scan(%T) = %s, %v", src, scanned, err)
		}
	}
	if err := scanned.Scan(nil); !errors.Is(err, calendar.ErrInvalidDate) {
		t.Fatalf("Scan(nil) err = %v", err)
	}
	v, err := date(t, "2027-03-10").Value()
	if err != nil || !v.(time.Time).Equal(time.Date(2027, 3, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("Value = %v, %v", v, err)
	}
}

func TestClock(t *testing.T) {
	valid := map[string]time.Duration{
		"00:00": 0,
		"08:00": 8 * time.Hour,
		"07:30": 7*time.Hour + 30*time.Minute,
		"23:59": 23*time.Hour + 59*time.Minute,
		"24:00": 24 * time.Hour,
	}
	for s, want := range valid {
		got, err := calendar.ParseClock(s)
		if err != nil || got != want {
			t.Errorf("ParseClock(%q) = %s, %v", s, got, err)
		}
		if back := calendar.FormatClock(got); back != s {
			t.Errorf("FormatClock(%s) = %q, want %q", got, back, s)
		}
	}
	for _, bad := range []string{"", "8:00", "08:60", "24:01", "25:00", "+1:00", "-1:00", "08-00", "08:000"} {
		if _, err := calendar.ParseClock(bad); !errors.Is(err, calendar.ErrInvalidClock) {
			t.Errorf("ParseClock(%q) err = %v", bad, err)
		}
	}
}
