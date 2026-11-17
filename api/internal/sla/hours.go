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

// BusinessHours returns the organization's working window. M2 stores the same window on every
// working day.
func (s *Service) BusinessHours(ctx context.Context, orgID uuid.UUID) (Hours, error) {
	return readHours(ctx, s.store.Q(), orgID)
}

func (s *Service) UpdateBusinessHours(ctx context.Context, actor audit.Actor, orgID uuid.UUID, in HoursInput) (Hours, error) {
	days, err := in.Validate()
	if err != nil {
		return Hours{}, apperr.Invalid(apperr.CodeInvalidBusinessHours, "Check the business hours", err.Error())
	}
	var updated Hours
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		if err := tx.Q.LockOrganization(ctx, orgID); err != nil {
			return err
		}
		old, err := readHours(ctx, tx.Q, orgID)
		if err != nil {
			return err
		}
		if err := tx.Q.DeleteBusinessHours(ctx, orgID); err != nil {
			return err
		}
		for _, d := range days {
			if err := tx.Q.InsertBusinessHours(ctx, db.InsertBusinessHoursParams{
				OrganizationID: orgID, Weekday: int16(d), OpenMinute: toMinutes(in.Opens), CloseMinute: toMinutes(in.Closes),
			}); err != nil {
				return err
			}
		}
		updated = Hours{Timezone: old.Timezone, Weekdays: days, Opens: in.Opens, Closes: in.Closes}
		if err := audit.Write(ctx, tx.Q, actor.Entry(ActionHoursUpdated, TargetBusinessHours, nil, map[string]any{
			dataFrom: describeHours(old), dataTo: describeHours(updated),
		})); err != nil {
			return err
		}
		return s.Recompute(ctx, tx, orgID)
	})
	if err != nil {
		return Hours{}, err
	}
	return updated, nil
}

func readHours(ctx context.Context, q *db.Queries, orgID uuid.UUID) (Hours, error) {
	tz, err := q.GetOrganizationTimezone(ctx, orgID)
	if err != nil {
		return Hours{}, err
	}
	rows, err := q.ListBusinessHours(ctx, orgID)
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

// ListHolidays returns the organization's holidays in date order, optionally for one year.
func (s *Service) ListHolidays(ctx context.Context, orgID uuid.UUID, year *int) ([]Holiday, error) {
	params := db.ListHolidaysParams{OrganizationID: orgID}
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

func (s *Service) CreateHoliday(ctx context.Context, actor audit.Actor, orgID uuid.UUID, date calendar.Date, name string) (Holiday, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > MaxHolidayNameLen {
		return Holiday{}, apperr.Invalid(apperr.CodeInvalidName, "Enter a holiday name", "")
	}
	var id uuid.UUID
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		var err error
		if id, err = tx.Q.CreateHoliday(ctx, db.CreateHolidayParams{OrganizationID: orgID, Date: date, Name: name}); err != nil {
			return err
		}
		if err := audit.Write(ctx, tx.Q, actor.Entry(ActionHolidayCreated, TargetHoliday, &id, map[string]any{
			dataDate: date, dataName: name,
		})); err != nil {
			return err
		}
		return s.Recompute(ctx, tx, orgID)
	})
	if store.IsViolation(err, store.SQLStateUniqueViolation, constraintHolidayDate) {
		return Holiday{}, apperr.Conflict(apperr.CodeHolidayDateTaken, "That date is already a holiday", date.String())
	}
	if err != nil {
		return Holiday{}, err
	}
	return Holiday{ID: id, Date: date, Name: name}, nil
}

func (s *Service) DeleteHoliday(ctx context.Context, actor audit.Actor, orgID, id uuid.UUID) error {
	return s.store.Tx(ctx, func(tx store.Tx) error {
		row, err := tx.Q.DeleteHoliday(ctx, db.DeleteHolidayParams{ID: id, OrganizationID: orgID})
		if err != nil {
			return notFound(err)
		}
		if err := audit.Write(ctx, tx.Q, actor.Entry(ActionHolidayDeleted, TargetHoliday, &id, map[string]any{
			dataDate: row.Date, dataName: row.Name,
		})); err != nil {
			return err
		}
		return s.Recompute(ctx, tx, orgID)
	})
}

// HolidayPresets lists the bundled holiday lists with how many of their dates are missing.
func (s *Service) HolidayPresets(ctx context.Context, orgID uuid.UUID) ([]PresetSummary, error) {
	years := holidaypreset.Years()
	out := make([]PresetSummary, 0, len(years))
	for _, year := range years {
		p, _ := holidaypreset.ForYear(year)
		have, err := s.ListHolidays(ctx, orgID, &year)
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
func (s *Service) LoadHolidayPreset(ctx context.Context, actor audit.Actor, orgID uuid.UUID, year int) (PresetLoad, error) {
	p, ok := holidaypreset.ForYear(year)
	if !ok {
		return PresetLoad{}, apperr.NotFound()
	}
	result := PresetLoad{Year: year}
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		for _, h := range p.Holidays {
			n, err := tx.Q.InsertHolidayIfMissing(ctx, db.InsertHolidayIfMissingParams{OrganizationID: orgID, Date: h.Date, Name: h.Name})
			if err != nil {
				return err
			}
			if n > 0 {
				result.Added++
			} else {
				result.Skipped++
			}
		}
		if err := audit.Write(ctx, tx.Q, actor.Entry(ActionPresetLoaded, TargetHoliday, nil, map[string]any{
			dataYear: year, dataAdded: result.Added, dataSkipped: result.Skipped,
		})); err != nil {
			return err
		}
		return s.Recompute(ctx, tx, orgID)
	})
	if err != nil {
		return PresetLoad{}, err
	}
	return result, nil
}

// Schedules builds an organization's working-time clocks from its time zone, business hours and
// holidays.
func (s *Service) Schedules(ctx context.Context, orgID uuid.UUID) (Schedules, error) {
	return LoadSchedules(ctx, s.store.Q(), orgID)
}

// LoadSchedules is Schedules on the caller's connection: inside a transaction, pass its queries
// so the read doesn't take a second connection from the pool while the first is held.
func LoadSchedules(ctx context.Context, q *db.Queries, orgID uuid.UUID) (Schedules, error) {
	hours, err := readHours(ctx, q, orgID)
	if err != nil {
		return Schedules{}, err
	}
	loc, err := ParseLocation(hours.Timezone)
	if err != nil {
		return Schedules{}, err
	}
	rows, err := q.ListHolidays(ctx, db.ListHolidaysParams{OrganizationID: orgID})
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
	workingDay := hours.WorkingDay()
	if workingDay <= 0 {
		workingDay = DefaultWorkingDay
	}
	return Schedules{Business: business, Always: calendar.Always{}, WorkingDay: workingDay}, nil
}

func yearBounds(year int) (calendar.Date, calendar.Date) {
	return calendar.NewDate(year, time.January, 1), calendar.NewDate(year, time.December, 31)
}

// Location is an organization's time zone, which decides its business hours, "today" and when
// its summaries go out.
func Location(ctx context.Context, q *db.Queries, orgID uuid.UUID) (*time.Location, error) {
	tz, err := q.GetOrganizationTimezone(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return ParseLocation(tz)
}

// ParseLocation loads an organisation time zone name.
func ParseLocation(tz string) (*time.Location, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("organisation time zone: %w", err)
	}
	return loc, nil
}

// Calendars loads each organization's schedules once, for jobs that go through tickets of
// several organizations in one run. It is not safe for concurrent use, and it is meant to live
// for one run only: the API and the worker can't share a cache that calendar edits would have
// to invalidate in both.
type Calendars struct {
	loaded map[uuid.UUID]Schedules
}

func NewCalendars() *Calendars { return &Calendars{loaded: map[uuid.UUID]Schedules{}} }

// For returns the organization's schedules, loading them on q the first time.
func (c *Calendars) For(ctx context.Context, q *db.Queries, orgID uuid.UUID) (Schedules, error) {
	if s, ok := c.loaded[orgID]; ok {
		return s, nil
	}
	s, err := LoadSchedules(ctx, q, orgID)
	if err != nil {
		return Schedules{}, err
	}
	c.loaded[orgID] = s
	return s, nil
}
