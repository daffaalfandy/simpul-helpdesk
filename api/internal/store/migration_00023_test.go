package store_test

import (
	"context"
	"io"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

// versionBeforeRequesterProjects is the last migration before requesters were linked to projects.
const versionBeforeRequesterProjects = 22

// insertProjectBefore23 is testdb's project insert without the requester links, which don't
// exist yet at that version.
const insertProjectBefore23 = `
WITH p AS (
  INSERT INTO projects (organization_id, client_id, name, is_default, active)
  SELECT organization_id, id, $2, $3, $4 FROM clients WHERE id = $1
  RETURNING id, organization_id
)
INSERT INTO project_sla_targets (project_id, priority, first_response_value, first_response_unit,
                                 resolve_value, resolve_unit, deliver_value, deliver_unit, calendar)
SELECT p.id, pr.code, 1, 'h', 4, 'h', 4, 'h', 'business_hours' FROM p JOIN priorities pr ON pr.organization_id = p.organization_id
RETURNING project_id`

// Every requester keeps access: the backfill links them to every project of their client, hidden
// ones too, and to nothing of another client. The links then refuse another client's project.
func TestRequestersLinkedToTheirClientsProjects(t *testing.T) {
	ctx := context.Background()
	st := testdb.NewEmpty(t)
	pool := st.Pool()
	if err := store.MigrateUpTo(ctx, pool, versionBeforeRequesterProjects); err != nil {
		t.Fatal(err)
	}
	row := func(sql string, args ...any) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := pool.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	org := row(`INSERT INTO organizations (name, ticket_prefix) VALUES ('Vendor', 'HD') RETURNING id`)
	if _, err := pool.Exec(ctx, `INSERT INTO priorities (organization_id, code, name, description)
		VALUES ($1, 'P1', 'Critical', 'd'), ($1, 'P2', 'High', 'd'), ($1, 'P3', 'Standard', 'd'), ($1, 'P4', 'Low', 'd')`, org); err != nil {
		t.Fatal(err)
	}
	clientA := row(`INSERT INTO clients (organization_id, name) VALUES ($1, 'A') RETURNING id`, org)
	clientB := row(`INSERT INTO clients (organization_id, name) VALUES ($1, 'B') RETURNING id`, org)
	eOffice := row(insertProjectBefore23, clientA, "e-Office", true, true)
	hidden := row(insertProjectBefore23, clientA, "Old intranet", false, false)
	network := row(insertProjectBefore23, clientB, "Network", true, true)
	hendra := row(`INSERT INTO users (email, name, role, client_id) VALUES ('h@a.test', 'H', 'requester', $1) RETURNING id`, clientA)
	dian := row(`INSERT INTO users (email, name, role, client_id, status) VALUES ('d@a.test', 'D', 'requester', $1, 'suspended') RETURNING id`, clientA)
	sri := row(`INSERT INTO users (email, name, role, client_id) VALUES ('s@b.test', 'S', 'requester', $1) RETURNING id`, clientB)
	staff := row(`INSERT INTO users (email, name, role) VALUES ('r@v.test', 'R', 'staff') RETURNING id`)

	if err := store.MigrateUp(ctx, pool, io.Discard); err != nil {
		t.Fatal(err)
	}
	type link struct{ user, project uuid.UUID }
	rows, err := pool.Query(ctx, `SELECT user_id, project_id FROM requester_projects`)
	if err != nil {
		t.Fatal(err)
	}
	got := map[link]bool{}
	for rows.Next() {
		var l link
		if err := rows.Scan(&l.user, &l.project); err != nil {
			t.Fatal(err)
		}
		got[l] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []link{{hendra, eOffice}, {hendra, hidden}, {dian, eOffice}, {dian, hidden}, {sri, network}}
	for _, l := range want {
		if !got[l] {
			t.Errorf("missing link %v", l)
		}
	}
	if len(got) != len(want) {
		t.Errorf("links = %v, want exactly %v", got, want)
	}

	insert := func(user, project, client uuid.UUID) error {
		_, err := pool.Exec(ctx, `INSERT INTO requester_projects (user_id, project_id, client_id, organization_id)
			VALUES ($1, $2, $3, $4)`, user, project, client, org)
		return err
	}
	for name, err := range map[string]error{
		"another client's project": insert(hendra, network, clientA),
		"as the other client":      insert(hendra, network, clientB),
		"staff":                    insert(staff, eOffice, clientA),
	} {
		if code, _, _ := store.Violation(err); code != store.SQLStateForeignKeyViolation {
			t.Errorf("%s: err = %v, want a foreign key violation", name, err)
		}
	}
	// A deleted project leaves every requester.
	if _, err := pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, hidden); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM requester_projects WHERE project_id = $1`, hidden).Scan(&n); err != nil || n != 0 {
		t.Errorf("links to a deleted project = %d (err %v)", n, err)
	}

	if err := store.MigrateDown(ctx, pool, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateUp(ctx, pool, io.Discard); err != nil {
		t.Fatal(err)
	}
}
