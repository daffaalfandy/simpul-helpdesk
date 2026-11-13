package tickets

import (
	"context"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// IntakeSystem is a "Which system?" choice.
type IntakeSystem struct {
	ID          uuid.UUID
	Name        string
	Description *string
}

// IntakeItem is a "What do you need?" choice.
type IntakeItem struct {
	ID              uuid.UUID
	Name            string
	Description     *string
	DeliveryMinutes int
}

// Intake is what a report form needs for one client: its SLA targets (for the priority
// cards), the working hours, and the visible systems and catalogue items.
type Intake struct {
	ClientID     uuid.UUID
	Targets      []sla.Target
	Hours        sla.Hours
	Systems      []IntakeSystem
	CatalogItems []IntakeItem
}

// Intake returns the report form's choices. Requesters always get their own client; staff
// name a client they may see.
func (s *Service) Intake(ctx context.Context, p *auth.Principal, clientID *uuid.UUID) (Intake, error) {
	a, err := accessFor(p)
	if err != nil {
		return Intake{}, err
	}
	var id uuid.UUID
	switch {
	case !a.staff:
		id = *a.scopeClientID
	case clientID == nil:
		return Intake{}, apperr.Invalid(apperr.CodeClientRequired, "Choose a client", "")
	default:
		id = *clientID
	}
	q := s.store.Q()
	c, err := q.GetClient(ctx, db.GetClientParams{ID: id, AgentID: a.scopeAgentID})
	if err != nil {
		return Intake{}, notFound(err)
	}
	out := Intake{ClientID: id}
	if out.Targets, err = sla.Targets(ctx, q, c.SlaPlanID); err != nil {
		return Intake{}, err
	}
	if out.Hours, err = s.sla.BusinessHours(ctx); err != nil {
		return Intake{}, err
	}
	active := true
	systems, err := q.ListSystems(ctx, &active)
	if err != nil {
		return Intake{}, err
	}
	out.Systems = make([]IntakeSystem, 0, len(systems))
	for _, r := range systems {
		out.Systems = append(out.Systems, IntakeSystem{ID: r.ID, Name: r.Name, Description: r.Description})
	}
	items, err := q.ListCatalogItems(ctx, &active)
	if err != nil {
		return Intake{}, err
	}
	out.CatalogItems = make([]IntakeItem, 0, len(items))
	for _, r := range items {
		out.CatalogItems = append(out.CatalogItems, IntakeItem{
			ID: r.ID, Name: r.Name, Description: r.Description, DeliveryMinutes: int(r.DeliveryMinutes),
		})
	}
	return out, nil
}
