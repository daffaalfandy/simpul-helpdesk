package httpapi

import (
	"context"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// GetBusinessHours returns the working hours to any signed-in user (the portal shows them).
func (s *server) GetBusinessHours(ctx context.Context, _ gen.GetBusinessHoursRequestObject) (gen.GetBusinessHoursResponseObject, error) {
	if _, ok := auth.PrincipalFrom(ctx); !ok {
		return nil, apperr.Unauthenticated()
	}
	h, err := s.sla.BusinessHours(ctx)
	if err != nil {
		return nil, err
	}
	return gen.GetBusinessHours200JSONResponse(apiHours(h)), nil
}

// UpdateBusinessHours replaces the working hours.
func (s *server) UpdateBusinessHours(ctx context.Context, req gen.UpdateBusinessHoursRequestObject) (gen.UpdateBusinessHoursResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	opens, errOpen := calendar.ParseClock(req.Body.Opens)
	closes, errClose := calendar.ParseClock(req.Body.Closes)
	if errOpen != nil || errClose != nil {
		return nil, apperr.Invalid(apperr.CodeInvalidBusinessHours, "Check the business hours", calendar.ErrInvalidClock.Error())
	}
	in := sla.HoursInput{Opens: opens, Closes: closes}
	for _, d := range req.Body.Weekdays {
		in.Weekdays = append(in.Weekdays, weekdays[d])
	}
	h, err := s.sla.UpdateBusinessHours(ctx, p.AuditActor(), in)
	if err != nil {
		return nil, err
	}
	return gen.UpdateBusinessHours200JSONResponse(apiHours(h)), nil
}

// ListHolidays lists holidays, optionally for one year.
func (s *server) ListHolidays(ctx context.Context, req gen.ListHolidaysRequestObject) (gen.ListHolidaysResponseObject, error) {
	if _, err := auth.Require(ctx, auth.CapAgentApp); err != nil {
		return nil, err
	}
	list, err := s.sla.ListHolidays(ctx, req.Params.Year)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListHolidays200JSONResponse, len(list))
	for i, h := range list {
		out[i] = apiHoliday(h)
	}
	return out, nil
}

// CreateHoliday adds a holiday.
func (s *server) CreateHoliday(ctx context.Context, req gen.CreateHolidayRequestObject) (gen.CreateHolidayResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	h, err := s.sla.CreateHoliday(ctx, p.AuditActor(), domainDate(req.Body.Date), req.Body.Name)
	if err != nil {
		return nil, err
	}
	return gen.CreateHoliday201JSONResponse(apiHoliday(h)), nil
}

// DeleteHoliday removes a holiday.
func (s *server) DeleteHoliday(ctx context.Context, req gen.DeleteHolidayRequestObject) (gen.DeleteHolidayResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	if err := s.sla.DeleteHoliday(ctx, p.AuditActor(), req.HolidayId); err != nil {
		return nil, err
	}
	return gen.DeleteHoliday204Response{}, nil
}

// ListHolidayPresets lists the bundled holiday lists.
func (s *server) ListHolidayPresets(ctx context.Context, _ gen.ListHolidayPresetsRequestObject) (gen.ListHolidayPresetsResponseObject, error) {
	if _, err := auth.Require(ctx, auth.CapSettingsManage); err != nil {
		return nil, err
	}
	presets, err := s.sla.HolidayPresets(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListHolidayPresets200JSONResponse, len(presets))
	for i, p := range presets {
		out[i] = gen.HolidayPreset{Year: p.Year, Country: p.Country, HolidayCount: p.HolidayCount, MissingCount: p.MissingCount}
	}
	return out, nil
}

// LoadHolidayPreset adds a bundled year's missing holidays.
func (s *server) LoadHolidayPreset(ctx context.Context, req gen.LoadHolidayPresetRequestObject) (gen.LoadHolidayPresetResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	r, err := s.sla.LoadHolidayPreset(ctx, p.AuditActor(), req.Year)
	if err != nil {
		return nil, err
	}
	return gen.LoadHolidayPreset200JSONResponse{Year: r.Year, AddedCount: r.Added, SkippedCount: r.Skipped}, nil
}

func apiHours(h sla.Hours) gen.BusinessHours {
	days := make([]gen.Weekday, len(h.Weekdays))
	for i, d := range h.Weekdays {
		days[i] = apiWeekday(d)
	}
	return gen.BusinessHours{
		Timezone: h.Timezone, Weekdays: days, Opens: calendar.FormatClock(h.Opens),
		Closes: calendar.FormatClock(h.Closes), WorkingDayMinutes: int(h.WorkingDay() / time.Minute),
	}
}

func apiHoliday(h sla.Holiday) gen.Holiday {
	return gen.Holiday{Id: h.ID, Date: apiDate(h.Date), Name: h.Name}
}
