package sla

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/holidaypreset"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// BusinessHours returns the working window. M2 stores the same window on every working day.
func (s *Service) BusinessHours(ctx context.Context) (Hours, error) {
	return readHours(ctx, s.store.Q())
}

// UpdateBusinessHours replaces the working window.
func (s *Service) UpdateBusinessHours(ctx context.Context, actor audit.Actor, in HoursInput) (Hours, error) {
	days, err := in.Validate()
	if err != nil {
		return Hours{}, apperr.Invalid(apperr.CodeInvalidBusinessHours, "Check the business hours", err.Error())
	}
	var updated Hours
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		if err := tx.Q.LockOrganization(ctx); err != nil {
			return err
		}
		old, err := readHours(ctx, tx.Q)
		if err != nil {
			return err
		}
		if err := tx.Q.DeleteBusinessHours(ctx); err != nil {
			return err
		}
		for _, d := range days {
			if err := tx.Q.InsertBusinessHours(ctx, db.InsertBusinessHoursParams{
				Weekday: int16(d), OpenMinute: toMinutes(in.Opens), CloseMinute: toMinutes(in.Closes),
			}); err != nil {
				return err
			}
		}
		updated = Hours{Timezone: old.Timezone, Weekdays: days, Opens: in.Opens, Closes: in.Closes}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionHoursUpdated, TargetBusinessHours, nil, map[string]any{
			dataFrom: describeHours(old), dataTo: describeHours(updated),
		}))
	})
	if err != nil {
		return Hours{}, err
	}
	return updated, nil
}

func readHours(ctx context.Context, q *db.Queries) (Hours, error) {
	tz, err := q.GetOrganizationTimezone(ctx)
	if err != nil {
		return Hours{}, err
	}
	rows, err := q.ListBusinessHours(ctx)
	if err != nil {
		return Hours{}, err
	}
	h := Hours{Timezone: tz}
	for _, r := range rows {
		h.Weekdays = append(h.Weekdays, time.Weekday(r.Weekday))
		h.Opens, h.Closes = fromMinutes(r.OpenMinute), fromMinutes(r.CloseMinute)
	}
	return h, nil
}

// describeHours is the audit form, e.g. "Monday,Tuesday 08:00-17:00".
func describeHours(h Hours) string {
	names := make([]string, 0, len(h.Weekdays))
	for _, d := range h.Weekdays {
		names = append(names, d.String())
	}
	return fmt.Sprintf("%s %s-%s", strings.Join(names, ","), calendar.FormatClock(h.Opens), calendar.FormatClock(h.Closes))
}

func toMinutes(d time.Duration) int16   { return int16(d / time.Minute) }
func fromMinutes(m int16) time.Duration { return time.Duration(m) * time.Minute }

// ListHolidays returns holidays in date order, optionally for one year.
func (s *Service) ListHolidays(ctx context.Context, year *int) ([]Holiday, error) {
	params := db.ListHolidaysParams{}
	if year != nil {
		from, to := yearBounds(*year)
		params.FromDate, params.ToDate = &from, &to
	}
	rows, err := s.store.Q().ListHolidays(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]Holiday, 0, len(rows))
	for _, r := range rows {
		out = append(out, Holiday{ID: r.ID, Date: r.Date, Name: r.Name})
	}
	return out, nil
}

// CreateHoliday adds one holiday.
func (s *Service) CreateHoliday(ctx context.Context, actor audit.Actor, date calendar.Date, name string) (Holiday, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > MaxHolidayNameLen {
		return Holiday{}, apperr.Invalid(apperr.CodeInvalidName, "Enter a holiday name", "")
	}
	var id uuid.UUID
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		var err error
		if id, err = tx.Q.CreateHoliday(ctx, db.CreateHolidayParams{Date: date, Name: name}); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionHolidayCreated, TargetHoliday, &id, map[string]any{
			dataDate: date, dataName: name,
		}))
	})
	if store.IsViolation(err, store.SQLStateUniqueViolation, constraintHolidayDate) {
		return Holiday{}, apperr.Conflict(apperr.CodeHolidayDateTaken, "That date is already a holiday", date.String())
	}
	if err != nil {
		return Holiday{}, err
	}
	return Holiday{ID: id, Date: date, Name: name}, nil
}

// DeleteHoliday removes one holiday.
func (s *Service) DeleteHoliday(ctx context.Context, actor audit.Actor, id uuid.UUID) error {
	return s.store.Tx(ctx, func(tx store.Tx) error {
		row, err := tx.Q.DeleteHoliday(ctx, id)
		if err != nil {
			return notFound(err)
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionHolidayDeleted, TargetHoliday, &id, map[string]any{
			dataDate: row.Date, dataName: row.Name,
		}))
	})
}

// HolidayPresets lists the bundled holiday lists with how many of their dates are missing.
func (s *Service) HolidayPresets(ctx context.Context) ([]PresetSummary, error) {
	years := holidaypreset.Years()
	out := make([]PresetSummary, 0, len(years))
	for _, year := range years {
		p, _ := holidaypreset.ForYear(year)
		have, err := s.ListHolidays(ctx, &year)
		if err != nil {
			return nil, err
		}
		dates := make(map[calendar.Date]bool, len(have))
		for _, h := range have {
			dates[h.Date] = true
		}
		missing := 0
		for _, h := range p.Holidays {
			if !dates[h.Date] {
				missing++
			}
		}
		out = append(out, PresetSummary{Year: year, Country: p.Country, HolidayCount: len(p.Holidays), MissingCount: missing})
	}
	return out, nil
}

// LoadHolidayPreset adds a bundled year's holidays that are missing. Dates that already
// have a holiday are left alone, so admin edits survive.
func (s *Service) LoadHolidayPreset(ctx context.Context, actor audit.Actor, year int) (PresetLoad, error) {
	p, ok := holidaypreset.ForYear(year)
	if !ok {
		return PresetLoad{}, apperr.NotFound()
	}
	result := PresetLoad{Year: year}
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		for _, h := range p.Holidays {
			n, err := tx.Q.InsertHolidayIfMissing(ctx, db.InsertHolidayIfMissingParams{Date: h.Date, Name: h.Name})
			if err != nil {
				return err
			}
			if n > 0 {
				result.Added++
			} else {
				result.Skipped++
			}
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionPresetLoaded, TargetHoliday, nil, map[string]any{
			dataYear: year, dataAdded: result.Added, dataSkipped: result.Skipped,
		}))
	})
	if err != nil {
		return PresetLoad{}, err
	}
	return result, nil
}

// Schedules builds the working-time clocks from the organisation's time zone, business
// hours and holidays.
func (s *Service) Schedules(ctx context.Context) (Schedules, error) {
	q := s.store.Q()
	hours, err := readHours(ctx, q)
	if err != nil {
		return Schedules{}, err
	}
	loc, err := time.LoadLocation(hours.Timezone)
	if err != nil {
		return Schedules{}, fmt.Errorf("organisation time zone: %w", err)
	}
	rows, err := q.ListHolidays(ctx, db.ListHolidaysParams{})
	if err != nil {
		return Schedules{}, err
	}
	holidays := make([]calendar.Date, 0, len(rows))
	for _, r := range rows {
		holidays = append(holidays, r.Date)
	}
	business, err := calendar.NewBusinessHours(calendar.Config{
		Location: loc,
		Week:     calendar.UniformWeek(hours.Weekdays, calendar.Window{Open: hours.Opens, Close: hours.Closes}),
		Holidays: holidays,
	})
	if err != nil {
		return Schedules{}, fmt.Errorf("business hours: %w", err)
	}
	return Schedules{Business: business, Always: calendar.Always{}}, nil
}

func yearBounds(year int) (calendar.Date, calendar.Date) {
	return calendar.NewDate(year, time.January, 1), calendar.NewDate(year, time.December, 31)
}
