package sla

import (
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// Audit actions and target types written by this package.
const (
	ActionHoursUpdated   = "business_hours.updated"
	ActionHolidayCreated = "holiday.created"
	ActionHolidayDeleted = "holiday.deleted"
	ActionPresetLoaded   = "holiday.preset_loaded"

	TargetBusinessHours = "business_hours"
	TargetHoliday       = "holiday"
)

const (
	dataName    = "name"
	dataFrom    = "from"
	dataTo      = "to"
	dataDate    = "date"
	dataYear    = "year"
	dataAdded   = "added"
	dataSkipped = "skipped"
)

// Constraint names the service maps to API errors.
const constraintHolidayDate = "holidays_date_key"

// Service manages the organisation's business hours and holidays. SLA targets live on each
// project (internal/projects).
type Service struct {
	store *store.Store
}

func NewService(st *store.Store) *Service { return &Service{store: st} }

// notFound maps a missing row to a 404 and passes other errors through.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound()
	}
	return err
}
