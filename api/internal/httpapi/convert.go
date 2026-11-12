package httpapi

import (
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

// weekdays maps API weekday names to Go weekdays.
var weekdays = map[gen.Weekday]time.Weekday{
	gen.WeekdaySun: time.Sunday,
	gen.WeekdayMon: time.Monday,
	gen.WeekdayTue: time.Tuesday,
	gen.WeekdayWed: time.Wednesday,
	gen.WeekdayThu: time.Thursday,
	gen.WeekdayFri: time.Friday,
	gen.WeekdaySat: time.Saturday,
}

func apiWeekday(d time.Weekday) gen.Weekday {
	for k, v := range weekdays {
		if v == d {
			return k
		}
	}
	return ""
}

func apiDate(d calendar.Date) openapi_types.Date { return openapi_types.Date{Time: d.In(time.UTC)} }

func apiDatePtr(d *calendar.Date) *openapi_types.Date {
	if d == nil {
		return nil
	}
	v := apiDate(*d)
	return &v
}

func domainDate(d openapi_types.Date) calendar.Date { return calendar.DateOf(d.Time) }

func domainDatePtr(d *openapi_types.Date) *calendar.Date {
	if d == nil {
		return nil
	}
	v := domainDate(*d)
	return &v
}
