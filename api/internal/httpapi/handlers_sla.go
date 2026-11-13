package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// ListSlaPlans lists SLA plans, the default first.
func (s *server) ListSlaPlans(ctx context.Context, _ gen.ListSlaPlansRequestObject) (gen.ListSlaPlansResponseObject, error) {
	if _, err := auth.Require(ctx, auth.CapAgentApp); err != nil {
		return nil, err
	}
	plans, err := s.sla.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListSlaPlans200JSONResponse, len(plans))
	for i, p := range plans {
		out[i] = apiPlan(p)
	}
	return out, nil
}

func (s *server) CreateSlaPlan(ctx context.Context, req gen.CreateSlaPlanRequestObject) (gen.CreateSlaPlanResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	plan, err := s.sla.CreatePlan(ctx, p.AuditActor(), planInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return gen.CreateSlaPlan201JSONResponse(apiPlan(plan)), nil
}

func (s *server) GetSlaPlan(ctx context.Context, req gen.GetSlaPlanRequestObject) (gen.GetSlaPlanResponseObject, error) {
	if _, err := auth.Require(ctx, auth.CapAgentApp); err != nil {
		return nil, err
	}
	plan, err := s.sla.GetPlan(ctx, req.PlanId)
	if err != nil {
		return nil, err
	}
	return gen.GetSlaPlan200JSONResponse(apiPlan(plan)), nil
}

func (s *server) UpdateSlaPlan(ctx context.Context, req gen.UpdateSlaPlanRequestObject) (gen.UpdateSlaPlanResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	plan, err := s.sla.UpdatePlan(ctx, p.AuditActor(), req.PlanId, planInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return gen.UpdateSlaPlan200JSONResponse(apiPlan(plan)), nil
}

// DeleteSlaPlan removes an unused, non-default plan.
func (s *server) DeleteSlaPlan(ctx context.Context, req gen.DeleteSlaPlanRequestObject) (gen.DeleteSlaPlanResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	if err := s.sla.DeletePlan(ctx, p.AuditActor(), req.PlanId); err != nil {
		return nil, err
	}
	return gen.DeleteSlaPlan204Response{}, nil
}

func (s *server) SetDefaultSlaPlan(ctx context.Context, req gen.SetDefaultSlaPlanRequestObject) (gen.SetDefaultSlaPlanResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	plan, err := s.sla.SetDefaultPlan(ctx, p.AuditActor(), req.PlanId)
	if err != nil {
		return nil, err
	}
	return gen.SetDefaultSlaPlan200JSONResponse(apiPlan(plan)), nil
}

func planInput(b gen.SlaPlanInput) sla.PlanInput {
	in := sla.PlanInput{Name: b.Name, Targets: make([]sla.Target, len(b.Targets))}
	for i, t := range b.Targets {
		in.Targets[i] = sla.Target{
			Priority: sla.Priority(t.Priority), FirstResponseMinutes: t.FirstResponseMinutes,
			ResolveMinutes: t.ResolveMinutes, DeliverMinutes: t.DeliverMinutes,
			DeliverFromCatalog: t.DeliverFromCatalog, Calendar: sla.CalendarKind(t.Calendar),
		}
	}
	return in
}

func apiPlan(p sla.Plan) gen.SlaPlan {
	return gen.SlaPlan{Id: p.ID, Name: p.Name, IsDefault: p.IsDefault, ClientCount: p.ClientCount, Targets: apiTargets(p.Targets)}
}

func apiTargets(ts []sla.Target) []gen.SlaTarget {
	out := make([]gen.SlaTarget, len(ts))
	for i, t := range ts {
		out[i] = gen.SlaTarget{
			Priority: gen.Priority(t.Priority), FirstResponseMinutes: t.FirstResponseMinutes,
			ResolveMinutes: t.ResolveMinutes, DeliverMinutes: t.DeliverMinutes,
			DeliverFromCatalog: t.DeliverFromCatalog, Calendar: gen.SlaCalendar(t.Calendar),
		}
	}
	return out
}
