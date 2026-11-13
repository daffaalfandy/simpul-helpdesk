// Package holidaypreset embeds official holiday lists (Indonesia: each year's SKB 3 Menteri) for
// admins to load. Loading never overwrites admin edits; a moved lunar date needs a release.
package holidaypreset

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"sync"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
)

// CountryIndonesia is the ISO 3166-1 code of the bundled presets.
const CountryIndonesia = "ID"

const dataGlob = "data/id/*.json"

//go:embed data/id/*.json
var files embed.FS

type Holiday struct {
	Date calendar.Date `json:"date"`
	Name string        `json:"name"`
}

// Preset is one year's official list.
type Preset struct {
	Country  string    `json:"country"`
	Year     int       `json:"year"`
	Source   string    `json:"source"`
	Holidays []Holiday `json:"holidays"`
}

var errInvalidPreset = errors.New("holidaypreset: invalid preset")

var load = sync.OnceValues(func() (map[int]Preset, error) {
	names, err := fs.Glob(files, dataGlob)
	if err != nil {
		return nil, err
	}
	presets := make(map[int]Preset, len(names))
	for _, name := range names {
		b, err := files.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var p Preset
		if err := json.Unmarshal(b, &p); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if err := p.validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if _, dup := presets[p.Year]; dup {
			return nil, fmt.Errorf("%s: %w: duplicate year %d", name, errInvalidPreset, p.Year)
		}
		presets[p.Year] = p
	}
	return presets, nil
})

func (p Preset) validate() error {
	if p.Country != CountryIndonesia || p.Source == "" || len(p.Holidays) == 0 {
		return fmt.Errorf("%w: country, source and holidays are required", errInvalidPreset)
	}
	for i, h := range p.Holidays {
		switch {
		case h.Date.Year != p.Year:
			return fmt.Errorf("%w: %s is outside %d", errInvalidPreset, h.Date, p.Year)
		case h.Name == "":
			return fmt.Errorf("%w: %s has no name", errInvalidPreset, h.Date)
		case i > 0 && !p.Holidays[i-1].Date.Before(h.Date):
			return fmt.Errorf("%w: %s is not after %s", errInvalidPreset, h.Date, p.Holidays[i-1].Date)
		}
	}
	return nil
}

// all returns every preset. The data is embedded and checked by tests, so a parse
// failure is a build defect.
func all() map[int]Preset {
	presets, err := load()
	if err != nil {
		panic(err)
	}
	return presets
}

// Years lists the years that have a preset, ascending.
func Years() []int { return slices.Sorted(maps.Keys(all())) }

func ForYear(year int) (Preset, bool) {
	p, ok := all()[year]
	return p, ok
}
