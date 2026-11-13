package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

const (
	constraintProjectTargets       = "project_targets_complete"
	constraintProjectDefaultExists = "projects_default_exists"
	constraintProjectDefaultActive = "projects_default_active"
	constraintProjectSingleDefault = "projects_single_default_key"
	constraintProjectName          = "projects_name_key"
)

func newClient(t *testing.T, st *store.Store, name string) uuid.UUID {
	t.Helper()
	id, err := st.Q().UpsertClient(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestPrioritiesAreSeeded(t *testing.T) {
	st := testdb.New(t)
	rows, err := st.Q().ListPriorityLevels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"P1": "Critical", "P2": "High", "P3": "Standard", "P4": "Low"}
	if len(rows) != len(want) {
		t.Fatalf("rows = %+v", rows)
	}
	for _, r := range rows {
		if want[r.Code] != r.Name || r.Description == "" {
			t.Errorf("%s = %+v", r.Code, r)
		}
	}
}

func TestProjectNeedsEveryPriority(t *testing.T) {
	ctx := context.Background()
	st := testdb.New(t)
	pool := st.Pool()
	client := newClient(t, st, "Kominfo")

	_, err := pool.Exec(ctx, `INSERT INTO projects (client_id, name, is_default) VALUES ($1, 'Empty', true)`, client)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintProjectTargets) {
		t.Fatalf("project without targets: err = %v", err)
	}
	p := testdb.Project(t, st, client, "e-Office", true)
	_, err = pool.Exec(ctx, `DELETE FROM project_sla_targets WHERE project_id = $1 AND priority = 'P4'`, p)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintProjectTargets) {
		t.Fatalf("delete a target: err = %v", err)
	}
	_, err = pool.Exec(ctx, `UPDATE project_sla_targets SET resolve_unit = 'weeks' WHERE project_id = $1`, p)
	if !store.IsViolation(err, store.SQLStateCheckViolation, "project_sla_targets_resolve_unit_check") {
		t.Fatalf("unknown unit: err = %v", err)
	}
	// Deleting the project (or its client) takes the targets with it.
	if _, err := pool.Exec(ctx, `DELETE FROM clients WHERE id = $1`, client); err != nil {
		t.Fatalf("delete client: %v", err)
	}
}

func TestClientWithProjectsHasOneActiveDefault(t *testing.T) {
	ctx := context.Background()
	st := testdb.New(t)
	pool := st.Pool()
	client := newClient(t, st, "Bappeda")

	_ = testdb.Project(t, st, client, "e-Office", true)
	second := testdb.Project(t, st, client, "SIPD", false)

	_, err := pool.Exec(ctx, `UPDATE projects SET is_default = true WHERE id = $1`, second)
	if !store.IsViolation(err, store.SQLStateUniqueViolation, constraintProjectSingleDefault) {
		t.Fatalf("second default: err = %v", err)
	}
	_, err = pool.Exec(ctx, `UPDATE projects SET is_default = false WHERE client_id = $1`, client)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintProjectDefaultExists) {
		t.Fatalf("no default: err = %v", err)
	}
	_, err = pool.Exec(ctx, `UPDATE projects SET active = false WHERE is_default AND client_id = $1`, client)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintProjectDefaultActive) {
		t.Fatalf("hidden default: err = %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO projects (client_id, name) VALUES ($1, 'sipd')`, client)
	if !store.IsViolation(err, store.SQLStateUniqueViolation, constraintProjectName) {
		t.Fatalf("same name: err = %v", err)
	}
	// A client whose projects are all hidden needs no default.
	err = st.Tx(ctx, func(tx store.Tx) error {
		_, err := tx.PG.Exec(ctx, `UPDATE projects SET is_default = false, active = false WHERE client_id = $1`, client)
		return err
	})
	if err != nil {
		t.Fatalf("hide every project: %v", err)
	}
}
