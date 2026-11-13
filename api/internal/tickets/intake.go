package tickets

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/projects"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// IntakeProject is a "Which system?" choice with the targets its tickets get. Requesters see
// it, so it carries targets only, never alert settings.
type IntakeProject struct {
	ID          uuid.UUID
	Name        string
	Description *string
	Targets     []sla.ProjectTarget
}

// IntakeItem is a "What do you need?" choice.
type IntakeItem struct {
	ID          uuid.UUID
	Name        string
	Description *string
}

// Intake is what a report form needs for one client: its active projects with their targets,
// the project service requests go to, the working hours, and the visible catalogue items.
type Intake struct {
	ClientID         uuid.UUID
	Hours            sla.Hours
	Projects         []IntakeProject
	DefaultProjectID *uuid.UUID // nil while the client has no project
	CatalogItems     []IntakeItem
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
	if _, err := q.GetClient(ctx, db.GetClientParams{ID: id, AgentID: a.scopeAgentID}); err != nil {
		return Intake{}, notFound(err)
	}
	out := Intake{ClientID: id}
	if out.Hours, err = s.sla.BusinessHours(ctx); err != nil {
		return Intake{}, err
	}
	rows, err := q.ListActiveClientProjects(ctx, id)
	if err != nil {
		return Intake{}, err
	}
	targets, err := projects.TargetsByProject(ctx, q, nil)
	if err != nil {
		return Intake{}, err
	}
	out.Projects = make([]IntakeProject, 0, len(rows))
	for _, r := range rows {
		out.Projects = append(out.Projects, IntakeProject{ID: r.ID, Name: r.Name, Description: r.Description, Targets: targets[r.ID]})
	}
	def, err := q.GetClientDefaultProject(ctx, id)
	switch {
	case err == nil:
		out.DefaultProjectID = &def.ID
	case !errors.Is(err, pgx.ErrNoRows):
		return Intake{}, err
	}
	active := true
	items, err := q.ListCatalogItems(ctx, &active)
	if err != nil {
		return Intake{}, err
	}
	out.CatalogItems = make([]IntakeItem, 0, len(items))
	for _, r := range items {
		out.CatalogItems = append(out.CatalogItems, IntakeItem{ID: r.ID, Name: r.Name, Description: r.Description})
	}
	return out, nil
}
