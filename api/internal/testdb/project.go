package testdb

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// insertProject adds a project with the four targets its deferred rule needs, in one statement.
const insertProject = `
WITH p AS (
  INSERT INTO projects (client_id, name, is_default) VALUES ($1, $2, $3) RETURNING id
)
INSERT INTO project_sla_targets (project_id, priority, first_response_value, first_response_unit,
                                 resolve_value, resolve_unit, deliver_value, deliver_unit, calendar)
SELECT p.id, pr.code, 1, 'h', 4, 'h', 4, 'h', 'business_hours' FROM p, priorities pr
RETURNING project_id`

// Project inserts a project for clientID with simple targets (1 h response, 4 h resolve and
// deliver) and returns its id. Pass isDefault for a client's first project.
func Project(tb testing.TB, st *store.Store, clientID uuid.UUID, name string, isDefault bool) uuid.UUID {
	tb.Helper()
	rows, err := st.Pool().Query(context.Background(), insertProject, clientID, name, isDefault)
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
