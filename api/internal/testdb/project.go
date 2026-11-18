package testdb

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// insertProject adds a project with the four targets its deferred rule needs, in one statement.
// The client's requesters report on it, as migration 00023 linked them to every project, so
// tests keep whole-client access unless they narrow it.
const insertProject = `
WITH p AS (
  INSERT INTO projects (organization_id, client_id, name, is_default)
  SELECT organization_id, id, $2, $3 FROM clients WHERE id = $1
  RETURNING id, client_id, organization_id
), linked AS (
  INSERT INTO requester_projects (user_id, project_id, client_id, organization_id)
  SELECT u.id, p.id, p.client_id, p.organization_id FROM p JOIN users u ON u.client_id = p.client_id
)
INSERT INTO project_sla_targets (project_id, priority, first_response_value, first_response_unit,
                                 resolve_value, resolve_unit, deliver_value, deliver_unit, calendar)
SELECT p.id, pr.code, 1, 'h', 4, 'h', 4, 'h', 'business_hours' FROM p JOIN priorities pr ON pr.organization_id = p.organization_id
RETURNING project_id`

// insertLegacyProject is insertProject before organizations (migration 00017), for migration
// tests that build data on an older schema.
const insertLegacyProject = `
WITH p AS (
  INSERT INTO projects (client_id, name, is_default) VALUES ($1, $2, $3) RETURNING id
)
INSERT INTO project_sla_targets (project_id, priority, first_response_value, first_response_unit,
                                 resolve_value, resolve_unit, deliver_value, deliver_unit, calendar)
SELECT p.id, pr.code, 1, 'h', 4, 'h', 4, 'h', 'business_hours' FROM p, priorities pr
RETURNING project_id`

// Project inserts a project for clientID with simple targets (1 h response, 4 h resolve and
// deliver) and no Definition of Done conditions, links the client's requesters to it, and
// returns its id. Pass isDefault for a client's first project.
func Project(tb testing.TB, st *store.Store, clientID uuid.UUID, name string, isDefault bool) uuid.UUID {
	tb.Helper()
	return project(tb, st, insertProject, clientID, name, isDefault)
}

// LegacyProject is Project on a schema from before organizations (migration tests).
func LegacyProject(tb testing.TB, st *store.Store, clientID uuid.UUID, name string, isDefault bool) uuid.UUID {
	tb.Helper()
	return project(tb, st, insertLegacyProject, clientID, name, isDefault)
}

func project(tb testing.TB, st *store.Store, sql string, clientID uuid.UUID, name string, isDefault bool) uuid.UUID {
	tb.Helper()
	rows, err := st.Pool().Query(context.Background(), sql, clientID, name, isDefault)
	if err != nil {
		tb.Fatalf("testdb: insert project: %v", err)
	}
	defer rows.Close()
	var id uuid.UUID
	for rows.Next() {
		if err := rows.Scan(&id); err != nil {
			tb.Fatalf("testdb: insert project: %v", err)
		}
	}
	if err := rows.Err(); err != nil {
		tb.Fatalf("testdb: insert project: %v", err)
	}
	return id
}

// SetProjectDoD replaces a project's Definition of Done.
func SetProjectDoD(tb testing.TB, st *store.Store, id uuid.UUID, c dod.Config) {
	tb.Helper()
	if _, err := st.Pool().Exec(context.Background(),
		`UPDATE projects SET dod_config = $2 WHERE id = $1`, id, c.JSON()); err != nil {
		tb.Fatalf("testdb: set project DoD: %v", err)
	}
}
