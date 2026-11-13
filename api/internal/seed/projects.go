package seed

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/projects"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// Demo project names (Projects mock, HANDOFF §16, plus the systems the demo tickets use).
const (
	projectEOffice     = "e-Office"
	projectWebsite     = "Website"
	projectEmail       = "Email"
	projectNetwork     = "Network & infra support"
	projectFinanceApps = "Finance apps"
	projectSIPD        = "SIPD connector"
	projectBintanWeb   = "Public website"
)

// project is a demo project. The first one listed for a client is its default.
type project struct {
	client, name, description string
	sla                       sla.ProjectSLA
}

// fastSLA is the quicker set the Projects mock gives e-Office: P1 fixed in 2 hours, at risk at
// half time, and the client PIC alerted on a P1 breach. Placeholder numbers (HANDOFF §13 Q3).
func fastSLA() sla.ProjectSLA {
	d := func(v int, u sla.Unit) sla.Duration { return sla.Duration{Value: v, Unit: u} }
	s := sla.DefaultProjectSLA()
	s.Targets = []sla.ProjectTarget{
		{Priority: sla.P1, FirstResponse: d(15, sla.UnitMinutes), Resolve: d(2, sla.UnitHours), Deliver: d(2, sla.UnitHours), Calendar: sla.CalendarAlways},
		{Priority: sla.P2, FirstResponse: d(30, sla.UnitMinutes), Resolve: d(4, sla.UnitHours), Deliver: d(8, sla.UnitHours), Calendar: sla.CalendarBusinessHours},
		{Priority: sla.P3, FirstResponse: d(1, sla.UnitHours), Resolve: d(1, sla.UnitWorkingDays), Deliver: d(1, sla.UnitWorkingDays), Calendar: sla.CalendarBusinessHours},
		{Priority: sla.P4, FirstResponse: d(2, sla.UnitHours), Resolve: d(3, sla.UnitWorkingDays), Deliver: d(3, sla.UnitWorkingDays), Calendar: sla.CalendarBusinessHours},
	}
	s.AtRiskPercent = sla.AtRiskHalf
	s.NotifyPicOnP1Breach = true
	return s
}

var demoProjects = []project{
	{RequesterClient, projectEOffice, "Letters, dispositions, archives", fastSLA()},
	{RequesterClient, projectWebsite, "Public website", sla.DefaultProjectSLA()},
	{RequesterClient, projectEmail, "Mailboxes and accounts", sla.DefaultProjectSLA()},
	{clientBatam, projectNetwork, "Internet, Wi-Fi, remote access", sla.DefaultProjectSLA()},
	{clientBatam, projectFinanceApps, "Budgeting and finance systems", sla.DefaultProjectSLA()},
	{clientRiau, projectSIPD, "Regional budgeting and finance (SIPD)", sla.DefaultProjectSLA()},
	{clientBintan, projectBintanWeb, "Public website", sla.DefaultProjectSLA()},
}

// seedProjects upserts the demo projects and their SLAs and returns IDs by client and name.
// Each client's first project is made its default, so a re-run restores the demo defaults.
func seedProjects(ctx context.Context, q *db.Queries, clientIDs map[string]uuid.UUID) (map[string]map[string]uuid.UUID, error) {
	out := make(map[string]map[string]uuid.UUID)
	for _, p := range demoProjects {
		clientID := clientIDs[p.client]
		id, err := q.GetProjectByName(ctx, db.GetProjectByNameParams{ClientID: clientID, Name: p.name})
		desc := p.description
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if id, err = q.CreateProject(ctx, db.CreateProjectParams{ClientID: clientID, Name: p.name, Description: &desc, Active: true}); err != nil {
				return nil, fmt.Errorf("project %s: %w", p.name, err)
			}
		case err != nil:
			return nil, err
		default:
			if err := q.UpdateProject(ctx, db.UpdateProjectParams{ID: id, Name: p.name, Description: &desc, Active: true}); err != nil {
				return nil, fmt.Errorf("project %s: %w", p.name, err)
			}
		}
		if err := projects.WriteSLA(ctx, q, id, p.sla); err != nil {
			return nil, fmt.Errorf("project %s SLA: %w", p.name, err)
		}
		if out[p.client] == nil {
			out[p.client] = make(map[string]uuid.UUID)
			if err := q.ClearClientDefaultProject(ctx, db.ClearClientDefaultProjectParams{ClientID: clientID, KeepID: id}); err != nil {
				return nil, err
			}
			if err := q.MarkProjectDefault(ctx, id); err != nil {
				return nil, err
			}
		}
		out[p.client][p.name] = id
	}
	return out, nil
}
