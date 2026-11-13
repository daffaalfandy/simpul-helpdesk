package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/projects"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// ListProjects is staff only; the service scopes agents to their clients' projects.
func (s *server) ListProjects(ctx context.Context, req gen.ListProjectsRequestObject) (gen.ListProjectsResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	list, err := s.projects.List(ctx, p, req.Params.ClientId)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListProjects200JSONResponse, len(list))
	for i, pr := range list {
		out[i] = apiProject(pr)
	}
	return out, nil
}

func (s *server) GetProject(ctx context.Context, req gen.GetProjectRequestObject) (gen.GetProjectResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	pr, err := s.projects.Get(ctx, p, req.ProjectId)
	if err != nil {
		return nil, err
	}
	return gen.GetProject200JSONResponse(apiProject(pr)), nil
}

func (s *server) CreateProject(ctx context.Context, req gen.CreateProjectRequestObject) (gen.CreateProjectResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapProjectsManage)
	if err != nil {
		return nil, err
	}
	b := req.Body
	pr, err := s.projects.Create(ctx, p.AuditActor(), projects.CreateInput{
		ClientID: b.ClientId, Name: b.Name, Description: b.Description, IsDefault: b.IsDefault, Active: b.Active,
	})
	if err != nil {
		return nil, err
	}
	return gen.CreateProject201JSONResponse(apiProject(pr)), nil
}

func (s *server) UpdateProject(ctx context.Context, req gen.UpdateProjectRequestObject) (gen.UpdateProjectResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapProjectsManage)
	if err != nil {
		return nil, err
	}
	b := req.Body
	pr, err := s.projects.Update(ctx, p.AuditActor(), req.ProjectId, projects.UpdateInput{
		Name: b.Name, Description: b.Description, IsDefault: b.IsDefault, Active: b.Active,
	})
	if err != nil {
		return nil, err
	}
	return gen.UpdateProject200JSONResponse(apiProject(pr)), nil
}

func (s *server) DeleteProject(ctx context.Context, req gen.DeleteProjectRequestObject) (gen.DeleteProjectResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapProjectsManage)
	if err != nil {
		return nil, err
	}
	if err := s.projects.Delete(ctx, p.AuditActor(), req.ProjectId); err != nil {
		return nil, err
	}
	return gen.DeleteProject204Response{}, nil
}

func (s *server) UpdateProjectSla(ctx context.Context, req gen.UpdateProjectSlaRequestObject) (gen.UpdateProjectSlaResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapProjectsManage)
	if err != nil {
		return nil, err
	}
	pr, err := s.projects.UpdateSLA(ctx, p.AuditActor(), req.ProjectId, domainProjectSLA(*req.Body))
	if err != nil {
		return nil, err
	}
	return gen.UpdateProjectSla200JSONResponse(apiProject(pr)), nil
}

func apiProject(p projects.Project) gen.Project {
	return gen.Project{
		Id: p.ID, Client: gen.Ref{Id: p.Client.ID, Name: p.Client.Name}, Name: p.Name, Description: p.Description,
		IsDefault: p.IsDefault, Active: p.Active, Sla: apiProjectSLA(p.SLA), CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func apiProjectSLA(s sla.ProjectSLA) gen.ProjectSla {
	return gen.ProjectSla{
		Targets:                  apiProjectTargets(s.Targets),
		PauseOnWaitingClient:     s.PauseOnWaitingClient,
		PauseOnUat:               s.PauseOnUAT,
		AtRiskPercent:            gen.AtRiskPercent(s.AtRiskPercent),
		NotifySupervisorOnBreach: s.NotifySupervisorOnBreach,
		NotifyPicOnP1Breach:      s.NotifyPicOnP1Breach,
	}
}

func apiProjectTargets(ts []sla.ProjectTarget) []gen.ProjectTarget {
	out := make([]gen.ProjectTarget, len(ts))
	for i, t := range ts {
		out[i] = gen.ProjectTarget{
			Priority:      gen.Priority(t.Priority),
			FirstResponse: apiDuration(t.FirstResponse),
			Resolve:       apiDuration(t.Resolve),
			Deliver:       apiDuration(t.Deliver),
			Calendar:      gen.SlaCalendar(t.Calendar),
		}
	}
	return out
}

func apiDuration(d sla.Duration) gen.Duration {
	return gen.Duration{Value: d.Value, Unit: gen.DurationUnit(d.Unit)}
}

func domainDuration(d gen.Duration) sla.Duration {
	return sla.Duration{Value: d.Value, Unit: sla.Unit(d.Unit)}
}

func domainProjectSLA(b gen.ProjectSla) sla.ProjectSLA {
	targets := make([]sla.ProjectTarget, len(b.Targets))
	for i, t := range b.Targets {
		targets[i] = sla.ProjectTarget{
			Priority:      sla.Priority(t.Priority),
			FirstResponse: domainDuration(t.FirstResponse),
			Resolve:       domainDuration(t.Resolve),
			Deliver:       domainDuration(t.Deliver),
			Calendar:      sla.CalendarKind(t.Calendar),
		}
	}
	return sla.ProjectSLA{Targets: targets, ProjectSettings: sla.ProjectSettings{
		PauseOnWaitingClient:     b.PauseOnWaitingClient,
		PauseOnUAT:               b.PauseOnUat,
		AtRiskPercent:            sla.AtRiskPercent(b.AtRiskPercent),
		NotifySupervisorOnBreach: b.NotifySupervisorOnBreach,
		NotifyPicOnP1Breach:      b.NotifyPicOnP1Breach,
	}}
}
