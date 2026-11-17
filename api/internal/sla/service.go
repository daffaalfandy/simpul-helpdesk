package sla

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

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

// JobInserter enqueues background jobs inside a transaction (a River client).
type JobInserter interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// RecomputeArgs re-runs every open ticket's SLA clocks on the current calendar (owner, 15 Nov:
// hours and holiday edits apply to open tickets). Each edit queues its own run: replaying is
// idempotent, and deduplicating could skip an edit made while a run was already reading.
type RecomputeArgs struct{}

// JobKindRecompute is RecomputeArgs' job kind; the worker lives in internal/tickets.
const JobKindRecompute = "sla_recompute"

func (RecomputeArgs) Kind() string { return JobKindRecompute }

// Service manages the organisation's business hours and holidays. SLA targets live on each
// project (internal/projects).
type Service struct {
	store *store.Store
	jobs  JobInserter
}

func NewService(st *store.Store, jobs JobInserter) *Service { return &Service{store: st, jobs: jobs} }

// Recompute queues the clock recompute in the edit's transaction: calendar edits here, and a new
// organisation time zone (internal/organization).
func (s *Service) Recompute(ctx context.Context, tx store.Tx) error {
	_, err := s.jobs.InsertTx(ctx, tx.PG, RecomputeArgs{}, nil)
	return err
}

// notFound maps a missing row to a 404 and passes other errors through.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound()
	}
	return err
}
