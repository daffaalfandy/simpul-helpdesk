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
	p, err := signedInOrg(ctx)
	if err != nil {
		return nil, err
	}
	h, err := s.sla.BusinessHours(ctx, p.OrganizationID)
	if err != nil {
		return nil, err
	}
	return gen.GetBusinessHours200JSONResponse(apiHours(h)), nil
}

func (s *server) UpdateBusinessHours(ctx context.Context, req gen.UpdateBusinessHoursRequestObject) (gen.UpdateBusinessHoursResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapSettingsManage)
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
	h, err := s.sla.UpdateBusinessHours(ctx, p.AuditActor(), p.OrganizationID, in)
	if err != nil {
		return nil, err
	}
	return gen.UpdateBusinessHours200JSONResponse(apiHours(h)), nil
}

func (s *server) ListHolidays(ctx context.Context, req gen.ListHolidaysRequestObject) (gen.ListHolidaysResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	list, err := s.sla.ListHolidays(ctx, p.OrganizationID, req.Params.Year)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListHolidays200JSONResponse, len(list))
	for i, h := range list {
		out[i] = apiHoliday(h)
	}
	return out, nil
}

func (s *server) CreateHoliday(ctx context.Context, req gen.CreateHolidayRequestObject) (gen.CreateHolidayResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	h, err := s.sla.CreateHoliday(ctx, p.AuditActor(), p.OrganizationID, domainDate(req.Body.Date), req.Body.Name)
	if err != nil {
		return nil, err
	}
	return gen.CreateHoliday201JSONResponse(apiHoliday(h)), nil
}

func (s *server) DeleteHoliday(ctx context.Context, req gen.DeleteHolidayRequestObject) (gen.DeleteHolidayResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	if err := s.sla.DeleteHoliday(ctx, p.AuditActor(), p.OrganizationID, req.HolidayId); err != nil {
		return nil, err
	}
	return gen.DeleteHoliday204Response{}, nil
}

func (s *server) ListHolidayPresets(ctx context.Context, _ gen.ListHolidayPresetsRequestObject) (gen.ListHolidayPresetsResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	presets, err := s.sla.HolidayPresets(ctx, p.OrganizationID)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListHolidayPresets200JSONResponse, len(presets))
	for i, pr := range presets {
		out[i] = gen.HolidayPreset{Year: pr.Year, Country: pr.Country, HolidayCount: pr.HolidayCount, MissingCount: pr.MissingCount}
	}
	return out, nil
}

func (s *server) LoadHolidayPreset(ctx context.Context, req gen.LoadHolidayPresetRequestObject) (gen.LoadHolidayPresetResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	r, err := s.sla.LoadHolidayPreset(ctx, p.AuditActor(), p.OrganizationID, req.Year)
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
