package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

func apiDodConfig(c dod.Config) gen.DodConfig {
	return gen.DodConfig{
		Merged:             gen.DodMerged{Enabled: c.Merged.Enabled, Branch: gen.DodBranch(c.Merged.Branch)},
		CiPassed:           c.CIPassed,
		DeployedProduction: c.DeployedProduction,
		Uat:                gen.DodUat{Enabled: c.UAT.Enabled, AutoApproveWorkingDays: gen.AutoApproveDays(c.UAT.AutoApproveWorkingDays)},
		RequesterConfirms:  c.RequesterConfirms,
	}
}

func domainDodConfig(c gen.DodConfig) dod.Config {
	return dod.Config{
		Merged:             dod.Merged{Enabled: c.Merged.Enabled, Branch: dod.Branch(c.Merged.Branch)},
		CIPassed:           c.CiPassed,
		DeployedProduction: c.DeployedProduction,
		UAT:                dod.UAT{Enabled: c.Uat.Enabled, AutoApproveWorkingDays: int(c.Uat.AutoApproveWorkingDays)},
		RequesterConfirms:  c.RequesterConfirms,
	}
}

// SetTicketDod is staff only; the service checks the project's override switch.
func (s *server) SetTicketDod(ctx context.Context, req gen.SetTicketDodRequestObject) (gen.SetTicketDodResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	var cfg *dod.Config
	if req.Body.Dod != nil {
		c := domainDodConfig(*req.Body.Dod)
		cfg = &c
	}
	reason := ""
	if req.Body.Reason != nil {
		reason = *req.Body.Reason
	}
	t, err := s.tickets.SetDoD(ctx, p, req.TicketKey, cfg, reason)
	if err != nil {
		return nil, err
	}
	return gen.SetTicketDod200JSONResponse(apiTicket(t)), nil
}

func apiTicketDod(d tickets.TicketDoD) gen.TicketDod {
	out := gen.TicketDod{
		Source: gen.DodSource(d.Source), Config: apiDodConfig(d.Config), ProjectConfig: apiDodConfig(d.ProjectConfig),
		OverrideReason: d.OverrideReason, Items: make([]gen.TicketDodItem, len(d.Items)), Met: d.Met,
		Pending: apiDodItems(d.Pending), CanOverride: d.CanOverride,
	}
	for i, it := range d.Items {
		out.Items[i] = gen.TicketDodItem{Item: gen.DodItem(it.Item), DoneAt: it.DoneAt, DoneBy: apiRefPtr(it.DoneBy)}
	}
	return out
}

func apiDodItems(items []dod.Item) []gen.DodItem {
	out := make([]gen.DodItem, len(items))
	for i, it := range items {
		out[i] = gen.DodItem(it)
	}
	return out
}
