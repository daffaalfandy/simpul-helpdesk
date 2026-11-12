package holidaypreset

import (
	"slices"
	"testing"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
)

func TestPresetsLoadAndValidate(t *testing.T) {
	presets, err := load()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := Years(), []int{2026, 2027}; !slices.Equal(got, want) {
		t.Fatalf("Years() = %v, want %v", got, want)
	}
	for year, p := range presets {
		if p.Year != year || len(p.Holidays) == 0 {
			t.Fatalf("%d: bad preset %+v", year, p)
		}
	}
}

func TestKnownDates(t *testing.T) {
	cases := map[string]string{
		"2026-03-21": "Idul Fitri 1447 Hijriah",
		"2026-08-17": "Proklamasi Kemerdekaan",
		"2027-03-10": "Idul Fitri 1448 Hijriah",
		"2027-05-20": "Hari Raya Waisak 2571 BE",
	}
	for day, name := range cases {
		d, err := calendar.ParseDate(day)
		if err != nil {
			t.Fatal(err)
		}
		p, ok := ForYear(d.Year)
		if !ok {
			t.Fatalf("no preset for %d", d.Year)
		}
		i := slices.IndexFunc(p.Holidays, func(h Holiday) bool { return h.Date == d })
		if i < 0 || p.Holidays[i].Name != name {
			t.Errorf("%s: want %q", day, name)
		}
	}
	if _, ok := ForYear(1999); ok {
		t.Fatal("unexpected preset for 1999")
	}
}

func TestValidateRejectsBadData(t *testing.T) {
	d := func(s string) calendar.Date {
		v, err := calendar.ParseDate(s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	base := Preset{Country: CountryIndonesia, Year: 2026, Source: "test"}
	bad := map[string][]Holiday{
		"wrong year": {{Date: d("2027-01-01"), Name: "x"}},
		"no name":    {{Date: d("2026-01-01")}},
		"unsorted":   {{Date: d("2026-02-01"), Name: "b"}, {Date: d("2026-01-01"), Name: "a"}},
		"duplicate":  {{Date: d("2026-01-01"), Name: "a"}, {Date: d("2026-01-01"), Name: "b"}},
		"empty":      nil,
	}
	for name, holidays := range bad {
		p := base
		p.Holidays = holidays
		if p.validate() == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
