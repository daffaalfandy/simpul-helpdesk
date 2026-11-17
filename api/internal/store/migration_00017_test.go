package store_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

// versionBeforeOrganizations is the last migration of the one-organization install.
const versionBeforeOrganizations = 16

// An install with data keeps it all as its first organization, and no row looks edited.
func TestExistingInstallBecomesItsFirstOrganization(t *testing.T) {
	ctx := context.Background()
	st := testdb.NewEmpty(t)
	pool := st.Pool()
	if err := store.MigrateUpTo(ctx, pool, versionBeforeOrganizations); err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE organization SET name = 'Demo Vendor', ticket_prefix = 'OPS', timezone = 'Asia/Makassar', setup_completed_at = now()`)
	var client uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO clients (name) VALUES ('C') RETURNING id`).Scan(&client); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO holidays (date, name) VALUES ('2026-12-25', 'Natal')`)
	testdb.LegacyProject(t, st, client, "e-Office", true)
	var before time.Time
	if err := pool.QueryRow(ctx, `SELECT updated_at FROM clients WHERE id = $1`, client).Scan(&before); err != nil {
		t.Fatal(err)
	}

	if err := store.MigrateUp(ctx, pool, io.Discard); err != nil {
		t.Fatal(err)
	}
	var orgID uuid.UUID
	var name, prefix, tz string
	if err := pool.QueryRow(ctx, `SELECT id, name, ticket_prefix, timezone FROM organizations`).Scan(&orgID, &name, &prefix, &tz); err != nil {
		t.Fatal(err)
	}
	if name != "Demo Vendor" || prefix != "OPS" || tz != "Asia/Makassar" {
		t.Fatalf("organization = %s %s %s", name, prefix, tz)
	}
	for _, table := range []string{"clients", "projects", "holidays", "priorities", "business_hours"} {
		var mismatched, total int
		if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE organization_id <> $1), count(*) FROM `+table, orgID).
			Scan(&mismatched, &total); err != nil {
			t.Fatal(err)
		}
		if mismatched != 0 || total == 0 {
			t.Errorf("%s: %d of %d rows in another organization", table, mismatched, total)
		}
	}
	var updated time.Time
	if err := pool.QueryRow(ctx, `SELECT updated_at FROM clients WHERE id = $1`, client).Scan(&updated); err != nil || !updated.Equal(before) {
		t.Errorf("client updated_at = %s (err %v), want untouched %s", updated, err, before)
	}
	var setup *time.Time
	if err := pool.QueryRow(ctx, `SELECT setup_completed_at FROM install`).Scan(&setup); err != nil || setup == nil {
		t.Errorf("setup_completed_at = %v (err %v), want kept", setup, err)
	}
}

// A fresh install has no organization until first-run setup makes one, and no leftover
// defaults: creating an organization inserts its own.
func TestFreshInstallHasNoOrganization(t *testing.T) {
	ctx := context.Background()
	st := testdb.NewEmpty(t)
	if err := store.MigrateUp(ctx, st.Pool(), io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"organizations", "priorities", "business_hours"} {
		var n int
		if err := st.Pool().QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s rows = %d (err %v), want 0", table, n, err)
		}
	}
}
