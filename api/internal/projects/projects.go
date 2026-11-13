// Package projects manages the projects each client has. A project carries the SLA its tickets
// follow (follow-up #1) and, in later milestones, its repository and Definition of Done.
// Projects are scoped like clients: agents see the projects of clients they are linked to.
package projects

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// Audit actions and target type written by this package.
const (
	ActionCreated        = "project.created"
	ActionUpdated        = "project.updated"
	ActionDeleted        = "project.deleted"
	ActionDefaultChanged = "project.default_changed"
	ActionSLAUpdated     = "project.sla_updated"

	TargetProject = "project"
)

const (
	dataClient                   = "client_id"
	dataName                     = "name"
	dataDescription              = "description"
	dataActive                   = "active"
	dataPrevious                 = "previous_default_id"
	dataPauseOnWaitingClient     = "pause_on_waiting_client"
	dataPauseOnUAT               = "pause_on_uat"
	dataAtRiskPercent            = "at_risk_percent"
	dataNotifySupervisorOnBreach = "notify_supervisor_on_breach"
	dataNotifyPicOnP1Breach      = "notify_pic_on_p1_breach"
	dataFirstResponse            = "first_response"
	dataResolve                  = "resolve"
	dataDeliver                  = "deliver"
	dataCalendar                 = "calendar"
	// dataTargetField names one priority's field in audit data, e.g. "P1.resolve".
	dataTargetField = "%s.%s"
)

// Field limits, matching the database CHECK constraints.
const (
	MaxNameLen        = 100
	MaxDescriptionLen = 300
)

const (
	constraintName          = "projects_name_key"
	constraintClientFK      = "projects_client_id_fkey"
	constraintDefaultExists = "projects_default_exists"
	constraintDefaultActive = "projects_default_active"
)

type Project struct {
	ID          uuid.UUID
	Client      clients.Ref
	Name        string
	Description *string
	IsDefault   bool
	Active      bool
	SLA         sla.ProjectSLA
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateInput adds a project. A client's first project becomes its default.
type CreateInput struct {
	ClientID    uuid.UUID
	Name        string
	Description *string
	IsDefault   bool
	Active      bool
}

// UpdateInput changes a project's details. The client never changes.
type UpdateInput struct {
	Name        string
	Description *string
	IsDefault   bool
	Active      bool
}

type Service struct {
	store *store.Store
	sla   *sla.Service
}

func NewService(st *store.Store, slaSvc *sla.Service) *Service {
	return &Service{store: st, sla: slaSvc}
}

// List returns the projects p may see, optionally for one client (404 if p can't see it).
func (s *Service) List(ctx context.Context, p *auth.Principal, clientID *uuid.UUID) ([]Project, error) {
	q := s.store.Q()
	agentID := clients.AgentScope(p)
	if clientID != nil {
		if _, err := q.GetClient(ctx, db.GetClientParams{ID: *clientID, AgentID: agentID}); err != nil {
			return nil, notFound(err)
		}
	}
	rows, err := q.ListProjects(ctx, db.ListProjectsParams{ClientID: clientID, AgentID: agentID})
	if err != nil {
		return nil, err
	}
	targets, err := targetsByProject(ctx, q, nil)
	if err != nil {
		return nil, err
	}
	out := make([]Project, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(db.GetProjectRow(r), targets[r.ID]))
	}
	return out, nil
}

// Get returns one project, or 404 if p may not see it.
func (s *Service) Get(ctx context.Context, p *auth.Principal, id uuid.UUID) (Project, error) {
	return get(ctx, s.store.Q(), id, clients.AgentScope(p))
}

func (s *Service) Create(ctx context.Context, actor audit.Actor, in CreateInput) (Project, error) {
	name, desc, err := normalize(in.Name, in.Description)
	if err != nil {
		return Project{}, err
	}
	if in.IsDefault && !in.Active {
		return Project{}, errDefaultHidden()
	}
	defaults := sla.DefaultProjectSLA()
	var id uuid.UUID
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		if _, err := tx.Q.GetClientForUpdate(ctx, in.ClientID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.Invalid(apperr.CodeClientNotFound, "Choose a client", "")
			}
			return err
		}
		n, err := tx.Q.CountClientProjects(ctx, in.ClientID)
		if err != nil {
			return err
		}
		// A client's first project receives its service requests.
		isDefault := in.IsDefault || n == 0
		active := in.Active || isDefault
		if id, err = tx.Q.CreateProject(ctx, db.CreateProjectParams{
			ClientID: in.ClientID, Name: name, Description: desc, Active: active,
		}); err != nil {
			return err
		}
		if err := WriteSLA(ctx, tx.Q, id, defaults); err != nil {
			return err
		}
		if isDefault {
			if err := tx.Q.ClearClientDefaultProject(ctx, db.ClearClientDefaultProjectParams{ClientID: in.ClientID, KeepID: id}); err != nil {
				return err
			}
			if err := tx.Q.MarkProjectDefault(ctx, id); err != nil {
				return err
			}
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionCreated, TargetProject, &id, map[string]any{
			dataClient: in.ClientID, dataName: name, dataActive: active,
		}))
	})
	if err != nil {
		return Project{}, projectError(err)
	}
	return get(ctx, s.store.Q(), id, nil)
}

func (s *Service) Update(ctx context.Context, actor audit.Actor, id uuid.UUID, in UpdateInput) (Project, error) {
	name, desc, err := normalize(in.Name, in.Description)
	if err != nil {
		return Project{}, err
	}
	if in.IsDefault && !in.Active {
		return Project{}, errDefaultHidden()
	}
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		old, err := tx.Q.GetProjectForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		// Default switches are serialised per client.
		if _, err := tx.Q.GetClientForUpdate(ctx, old.ClientID); err != nil {
			return err
		}
		if old.IsDefault && !in.IsDefault {
			return apperr.Conflict(apperr.CodeProjectIsDefault, "This is the client's default project",
				"Make another project the default instead.")
		}
		if err := tx.Q.UpdateProject(ctx, db.UpdateProjectParams{ID: id, Name: name, Description: desc, Active: in.Active}); err != nil {
			return err
		}
		changes := audit.Changes{}
		audit.Track(changes, dataName, old.Name, name)
		audit.TrackPtr(changes, dataDescription, old.Description, desc)
		audit.Track(changes, dataActive, old.Active, in.Active)
		if len(changes) > 0 {
			if err := audit.Write(ctx, tx.Q, actor.Entry(ActionUpdated, TargetProject, &id, changes)); err != nil {
				return err
			}
		}
		if in.IsDefault && !old.IsDefault {
			return makeDefault(ctx, tx.Q, actor, id, old.ClientID)
		}
		return nil
	})
	if err != nil {
		return Project{}, projectError(err)
	}
	return get(ctx, s.store.Q(), id, nil)
}

// Delete removes a project. The default can't go while the client has other active projects.
func (s *Service) Delete(ctx context.Context, actor audit.Actor, id uuid.UUID) error {
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		old, err := tx.Q.GetProjectForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if _, err := tx.Q.GetClientForUpdate(ctx, old.ClientID); err != nil {
			return err
		}
		if old.IsDefault {
			others, err := tx.Q.CountOtherActiveProjects(ctx, db.CountOtherActiveProjectsParams{ClientID: old.ClientID, ID: id})
			if err != nil {
				return err
			}
			if others > 0 {
				return apperr.Conflict(apperr.CodeProjectIsDefault, "This is the client's default project",
					"Make another project the default first.")
			}
		}
		if err := tx.Q.DeleteProject(ctx, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionDeleted, TargetProject, &id, map[string]any{
			dataClient: old.ClientID, dataName: old.Name,
		}))
	})
	return projectError(err)
}

// UpdateSLA replaces a project's targets and settings. Open tickets keep the snapshot they were
// created with; new tickets use these.
func (s *Service) UpdateSLA(ctx context.Context, actor audit.Actor, id uuid.UUID, in sla.ProjectSLA) (Project, error) {
	hours, err := s.sla.BusinessHours(ctx)
	if err != nil {
		return Project{}, err
	}
	workingDay := hours.WorkingDay()
	if workingDay <= 0 {
		workingDay = sla.DefaultWorkingDay
	}
	next, err := sla.ValidateProjectSLA(in, workingDay)
	if err != nil {
		return Project{}, apperr.Invalid(apperr.CodeInvalidSlaTargets, "Check the SLA targets", err.Error())
	}
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		old, err := tx.Q.GetProjectForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		oldTargets, err := targetsByProject(ctx, tx.Q, &id)
		if err != nil {
			return err
		}
		prev := sla.ProjectSLA{Targets: oldTargets[id], ProjectSettings: settingsFromRow(old)}
		if err := WriteSLA(ctx, tx.Q, id, next); err != nil {
			return err
		}
		changes := slaChanges(prev, next)
		if len(changes) == 0 {
			return nil
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionSLAUpdated, TargetProject, &id, changes))
	})
	if err != nil {
		return Project{}, projectError(err)
	}
	return get(ctx, s.store.Q(), id, nil)
}

func makeDefault(ctx context.Context, q *db.Queries, actor audit.Actor, id, clientID uuid.UUID) error {
	previous, err := q.ListProjects(ctx, db.ListProjectsParams{ClientID: &clientID})
	if err != nil {
		return err
	}
	var previousID *uuid.UUID
	for _, r := range previous {
		if r.IsDefault && r.ID != id {
			previousID = &r.ID
		}
	}
	if err := q.ClearClientDefaultProject(ctx, db.ClearClientDefaultProjectParams{ClientID: clientID, KeepID: id}); err != nil {
		return err
	}
	if err := q.MarkProjectDefault(ctx, id); err != nil {
		return err
	}
	return audit.Write(ctx, q, actor.Entry(ActionDefaultChanged, TargetProject, &id, map[string]any{
		dataClient: clientID, dataPrevious: previousID,
	}))
}

// WriteSLA stores a project's targets and settings on q, inside the caller's transaction.
// Callers validate first; the seed uses it directly.
func WriteSLA(ctx context.Context, q *db.Queries, id uuid.UUID, s sla.ProjectSLA) error {
	for _, t := range s.Targets {
		if err := q.UpsertProjectSlaTarget(ctx, db.UpsertProjectSlaTargetParams{
			ProjectID:          id,
			Priority:           string(t.Priority),
			FirstResponseValue: int32(t.FirstResponse.Value),
			FirstResponseUnit:  string(t.FirstResponse.Unit),
			ResolveValue:       int32(t.Resolve.Value),
			ResolveUnit:        string(t.Resolve.Unit),
			DeliverValue:       int32(t.Deliver.Value),
			DeliverUnit:        string(t.Deliver.Unit),
			Calendar:           string(t.Calendar),
		}); err != nil {
			return err
		}
	}
	return q.UpdateProjectSlaSettings(ctx, db.UpdateProjectSlaSettingsParams{
		ID:                       id,
		PauseOnWaitingClient:     s.PauseOnWaitingClient,
		PauseOnUat:               s.PauseOnUAT,
		AtRiskPercent:            int16(s.AtRiskPercent),
		NotifySupervisorOnBreach: s.NotifySupervisorOnBreach,
		NotifyPicOnP1Breach:      s.NotifyPicOnP1Breach,
	})
}

// slaChanges lists every target field and setting that changed, keyed like "P1.resolve".
func slaChanges(prev, next sla.ProjectSLA) audit.Changes {
	c := audit.Changes{}
	for _, t := range next.Targets {
		old, _ := prev.Target(t.Priority)
		key := func(field string) string { return fmt.Sprintf(dataTargetField, t.Priority, field) }
		audit.Track(c, key(dataFirstResponse), old.FirstResponse, t.FirstResponse)
		audit.Track(c, key(dataResolve), old.Resolve, t.Resolve)
		audit.Track(c, key(dataDeliver), old.Deliver, t.Deliver)
		audit.Track(c, key(dataCalendar), old.Calendar, t.Calendar)
	}
	audit.Track(c, dataPauseOnWaitingClient, prev.PauseOnWaitingClient, next.PauseOnWaitingClient)
	audit.Track(c, dataPauseOnUAT, prev.PauseOnUAT, next.PauseOnUAT)
	audit.Track(c, dataAtRiskPercent, prev.AtRiskPercent, next.AtRiskPercent)
	audit.Track(c, dataNotifySupervisorOnBreach, prev.NotifySupervisorOnBreach, next.NotifySupervisorOnBreach)
	audit.Track(c, dataNotifyPicOnP1Breach, prev.NotifyPicOnP1Breach, next.NotifyPicOnP1Breach)
	return c
}

func get(ctx context.Context, q *db.Queries, id uuid.UUID, agentID *uuid.UUID) (Project, error) {
	r, err := q.GetProject(ctx, db.GetProjectParams{ID: id, AgentID: agentID})
	if err != nil {
		return Project{}, notFound(err)
	}
	targets, err := targetsByProject(ctx, q, &id)
	if err != nil {
		return Project{}, err
	}
	return fromRow(r, targets[id]), nil
}

// targetsByProject reads targets for one project, or for all when id is nil, ordered P1–P4.
func targetsByProject(ctx context.Context, q *db.Queries, id *uuid.UUID) (map[uuid.UUID][]sla.ProjectTarget, error) {
	rows, err := q.ListProjectSlaTargets(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]sla.ProjectTarget)
	for _, r := range rows {
		out[r.ProjectID] = append(out[r.ProjectID], targetFromRow(r))
	}
	return out, nil
}

func targetFromRow(r db.ListProjectSlaTargetsRow) sla.ProjectTarget {
	d := func(v int32, u string) sla.Duration { return sla.Duration{Value: int(v), Unit: sla.Unit(u)} }
	return sla.ProjectTarget{
		Priority:      sla.Priority(r.Priority),
		FirstResponse: d(r.FirstResponseValue, r.FirstResponseUnit),
		Resolve:       d(r.ResolveValue, r.ResolveUnit),
		Deliver:       d(r.DeliverValue, r.DeliverUnit),
		Calendar:      sla.CalendarKind(r.Calendar),
	}
}

func settingsFromRow(r db.GetProjectForUpdateRow) sla.ProjectSettings {
	return sla.ProjectSettings{
		PauseOnWaitingClient:     r.SlaPauseOnWaitingClient,
		PauseOnUAT:               r.SlaPauseOnUat,
		AtRiskPercent:            sla.AtRiskPercent(r.SlaAtRiskPercent),
		NotifySupervisorOnBreach: r.SlaNotifySupervisorOnBreach,
		NotifyPicOnP1Breach:      r.SlaNotifyPicOnP1Breach,
	}
}

func fromRow(r db.GetProjectRow, targets []sla.ProjectTarget) Project {
	if targets == nil {
		targets = []sla.ProjectTarget{}
	}
	return Project{
		ID: r.ID, Client: clients.Ref{ID: r.ClientID, Name: r.ClientName}, Name: r.Name, Description: r.Description,
		IsDefault: r.IsDefault, Active: r.Active,
		SLA: sla.ProjectSLA{Targets: targets, ProjectSettings: sla.ProjectSettings{
			PauseOnWaitingClient:     r.SlaPauseOnWaitingClient,
			PauseOnUAT:               r.SlaPauseOnUat,
			AtRiskPercent:            sla.AtRiskPercent(r.SlaAtRiskPercent),
			NotifySupervisorOnBreach: r.SlaNotifySupervisorOnBreach,
			NotifyPicOnP1Breach:      r.SlaNotifyPicOnP1Breach,
		}},
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// normalize trims the name and description and checks their length; a blank description is unset.
func normalize(name string, description *string) (string, *string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > MaxNameLen {
		return "", nil, apperr.Invalid(apperr.CodeInvalidName, "Enter the project's name", "")
	}
	if description == nil {
		return name, nil, nil
	}
	d := strings.TrimSpace(*description)
	if d == "" {
		return name, nil, nil
	}
	if len([]rune(d)) > MaxDescriptionLen {
		return "", nil, apperr.Invalid(apperr.CodeInvalidRequest, "The description is too long", "")
	}
	return name, &d, nil
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound()
	}
	return err
}

func errDefaultHidden() error {
	return apperr.Invalid(apperr.CodeProjectIsDefault, "The default project can't be hidden",
		"Service requests go to it, so it stays on the report form.")
}

func projectError(err error) error {
	switch {
	case err == nil:
		return nil
	case store.IsViolation(err, store.SQLStateUniqueViolation, constraintName):
		return apperr.Conflict(apperr.CodeProjectNameTaken, "This client already has a project with this name", "")
	case store.IsViolation(err, store.SQLStateForeignKeyViolation, constraintClientFK):
		return apperr.Invalid(apperr.CodeClientNotFound, "Choose a client", "")
	case store.IsViolation(err, store.SQLStateCheckViolation, constraintDefaultExists),
		store.IsViolation(err, store.SQLStateCheckViolation, constraintDefaultActive):
		return errDefaultHidden()
	}
	return err
}
