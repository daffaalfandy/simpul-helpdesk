package sla

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// Audit actions and target types written by this package.
const (
	ActionPlanCreated        = "sla_plan.created"
	ActionPlanUpdated        = "sla_plan.updated"
	ActionPlanDeleted        = "sla_plan.deleted"
	ActionPlanDefaultChanged = "sla_plan.default_changed"
	ActionHoursUpdated       = "business_hours.updated"
	ActionHolidayCreated     = "holiday.created"
	ActionHolidayDeleted     = "holiday.deleted"
	ActionPresetLoaded       = "holiday.preset_loaded"

	TargetPlan          = "sla_plan"
	TargetBusinessHours = "business_hours"
	TargetHoliday       = "holiday"
)

// Audit data keys.
const (
	dataName    = "name"
	dataTargets = "targets"
	dataFrom    = "from"
	dataTo      = "to"
	dataDate    = "date"
	dataYear    = "year"
	dataAdded   = "added"
	dataSkipped = "skipped"
)

// Constraint names the service maps to API errors.
const (
	constraintPlanName      = "sla_plans_name_key"
	constraintClientPlanFK  = "clients_sla_plan_id_fkey"
	constraintHolidayDate   = "holidays_date_key"
	constraintDefaultExists = "sla_plans_default_exists"
)

// Service manages SLA plans, business hours and holidays.
type Service struct {
	store *store.Store
}

// NewService returns a Service.
func NewService(st *store.Store) *Service { return &Service{store: st} }

// ListPlans returns every plan, the default first.
func (s *Service) ListPlans(ctx context.Context) ([]Plan, error) {
	q := s.store.Q()
	rows, err := q.ListSlaPlans(ctx)
	if err != nil {
		return nil, err
	}
	targets, err := q.ListSlaTargets(ctx, nil)
	if err != nil {
		return nil, err
	}
	byPlan := make(map[uuid.UUID][]Target, len(rows))
	for _, t := range targets {
		byPlan[t.PlanID] = append(byPlan[t.PlanID], targetFromRow(t))
	}
	plans := make([]Plan, 0, len(rows))
	for _, r := range rows {
		plans = append(plans, Plan{
			ID: r.ID, Name: r.Name, IsDefault: r.IsDefault, ClientCount: int(r.ClientCount),
			Targets: byPlan[r.ID],
		})
	}
	return plans, nil
}

// GetPlan returns one plan.
func (s *Service) GetPlan(ctx context.Context, id uuid.UUID) (Plan, error) {
	return getPlan(ctx, s.store.Q(), id)
}

// CreatePlan adds a non-default plan.
func (s *Service) CreatePlan(ctx context.Context, actor audit.Actor, in PlanInput) (Plan, error) {
	name, targets, err := validatePlan(in)
	if err != nil {
		return Plan{}, err
	}
	var id uuid.UUID
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		if id, err = tx.Q.CreateSlaPlan(ctx, name); err != nil {
			return err
		}
		if err := upsertTargets(ctx, tx.Q, id, targets); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionPlanCreated, TargetPlan, &id, map[string]any{
			dataName: name, dataTargets: targets,
		}))
	})
	if err != nil {
		return Plan{}, planError(err)
	}
	return s.GetPlan(ctx, id)
}

// UpdatePlan replaces a plan's name and all its targets.
func (s *Service) UpdatePlan(ctx context.Context, actor audit.Actor, id uuid.UUID, in PlanInput) (Plan, error) {
	name, targets, err := validatePlan(in)
	if err != nil {
		return Plan{}, err
	}
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		old, err := tx.Q.GetSlaPlanForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		oldTargets, err := tx.Q.ListSlaTargets(ctx, &id)
		if err != nil {
			return err
		}
		if old.Name != name {
			if err := tx.Q.RenameSlaPlan(ctx, db.RenameSlaPlanParams{ID: id, Name: name}); err != nil {
				return err
			}
		}
		if err := upsertTargets(ctx, tx.Q, id, targets); err != nil {
			return err
		}
		changes := audit.Changes{}
		audit.Track(changes, dataName, old.Name, name)
		for i, t := range oldTargets {
			audit.Track(changes, string(t.Priority), targetFromRow(t), targets[i])
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionPlanUpdated, TargetPlan, &id, changes))
	})
	if err != nil {
		return Plan{}, planError(err)
	}
	return s.GetPlan(ctx, id)
}

// DeletePlan removes a plan no client uses. The default plan can't be deleted.
func (s *Service) DeletePlan(ctx context.Context, actor audit.Actor, id uuid.UUID) error {
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		if err := tx.Q.LockOrganization(ctx); err != nil {
			return err
		}
		plan, err := tx.Q.GetSlaPlanForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if plan.IsDefault {
			return errPlanIsDefault()
		}
		if n, err := tx.Q.CountClientsOnSlaPlan(ctx, id); err != nil {
			return err
		} else if n > 0 {
			return errPlanInUse()
		}
		if err := tx.Q.DeleteSlaPlan(ctx, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionPlanDeleted, TargetPlan, &id, map[string]any{dataName: plan.Name}))
	})
	return planError(err)
}

// SetDefaultPlan makes a plan the default for new clients.
func (s *Service) SetDefaultPlan(ctx context.Context, actor audit.Actor, id uuid.UUID) (Plan, error) {
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		// The organisation lock serialises default switches; reads below see the latest commit.
		if err := tx.Q.LockOrganization(ctx); err != nil {
			return err
		}
		plan, err := tx.Q.GetSlaPlanForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if plan.IsDefault {
			return nil
		}
		old, err := tx.Q.GetDefaultSlaPlanForUpdate(ctx)
		if err != nil {
			return err
		}
		// Unset first: the partial unique index allows only one default at a time.
		if err := tx.Q.SetSlaPlanDefault(ctx, db.SetSlaPlanDefaultParams{ID: old.ID, IsDefault: false}); err != nil {
			return err
		}
		if err := tx.Q.SetSlaPlanDefault(ctx, db.SetSlaPlanDefaultParams{ID: id, IsDefault: true}); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionPlanDefaultChanged, TargetPlan, &id, map[string]any{
			dataFrom: old.Name, dataTo: plan.Name,
		}))
	})
	if err != nil {
		return Plan{}, planError(err)
	}
	return s.GetPlan(ctx, id)
}

func validatePlan(in PlanInput) (string, []Target, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > MaxPlanNameLen {
		return "", nil, apperr.Invalid(apperr.CodeInvalidName, "Enter a plan name", "")
	}
	targets, err := ValidateTargets(in.Targets)
	if err != nil {
		return "", nil, apperr.Invalid(apperr.CodeInvalidSlaTargets, "Check the SLA targets", err.Error())
	}
	return name, targets, nil
}

func upsertTargets(ctx context.Context, q *db.Queries, planID uuid.UUID, targets []Target) error {
	for _, t := range targets {
		if err := q.UpsertSlaTarget(ctx, db.UpsertSlaTargetParams{
			PlanID:               planID,
			Priority:             string(t.Priority),
			FirstResponseMinutes: int32(t.FirstResponseMinutes),
			ResolveMinutes:       int32(t.ResolveMinutes),
			DeliverMinutes:       int32(t.DeliverMinutes),
			DeliverFromCatalog:   t.DeliverFromCatalog,
			Calendar:             string(t.Calendar),
		}); err != nil {
			return err
		}
	}
	return nil
}

func getPlan(ctx context.Context, q *db.Queries, id uuid.UUID) (Plan, error) {
	r, err := q.GetSlaPlan(ctx, id)
	if err != nil {
		return Plan{}, notFound(err)
	}
	rows, err := q.ListSlaTargets(ctx, &id)
	if err != nil {
		return Plan{}, err
	}
	p := Plan{ID: r.ID, Name: r.Name, IsDefault: r.IsDefault, ClientCount: int(r.ClientCount)}
	for _, t := range rows {
		p.Targets = append(p.Targets, targetFromRow(t))
	}
	return p, nil
}

func targetFromRow(r db.ListSlaTargetsRow) Target {
	return Target{
		Priority:             Priority(r.Priority),
		FirstResponseMinutes: int(r.FirstResponseMinutes),
		ResolveMinutes:       int(r.ResolveMinutes),
		DeliverMinutes:       int(r.DeliverMinutes),
		DeliverFromCatalog:   r.DeliverFromCatalog,
		Calendar:             CalendarKind(r.Calendar),
	}
}

// planError maps database constraint errors to API errors.
func planError(err error) error {
	switch {
	case err == nil:
		return nil
	case store.IsViolation(err, store.SQLStateUniqueViolation, constraintPlanName):
		return apperr.Conflict(apperr.CodeSlaPlanNameTaken, "A plan with this name already exists", "")
	case store.IsViolation(err, store.SQLStateRestrictViolation, constraintClientPlanFK):
		return errPlanInUse()
	case store.IsViolation(err, store.SQLStateCheckViolation, constraintDefaultExists):
		return errPlanIsDefault()
	}
	return err
}

func errPlanInUse() error {
	return apperr.Conflict(apperr.CodeSlaPlanInUse, "Clients still use this plan", "Move them to another plan first.")
}

func errPlanIsDefault() error {
	return apperr.Conflict(apperr.CodeSlaPlanIsDefault, "The default plan can't be deleted", "Make another plan the default first.")
}

// notFound maps a missing row to a 404 and passes other errors through.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound()
	}
	return err
}
