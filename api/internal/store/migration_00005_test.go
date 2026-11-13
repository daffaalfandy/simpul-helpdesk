package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
	domain "github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// versionBeforeBackfill is the last migration with SLA plans and systems.
const versionBeforeBackfill = 4

// v1Snapshot is how M3 stored a ticket's SLA: its client's plan, in minutes.
const v1Snapshot = `{"version":1,"planId":"019a0000-0000-7000-8000-000000000001","planName":"Standard",
"targets":[
 {"priority":"P1","firstResponseMinutes":15,"resolveMinutes":240,"deliverMinutes":240,"deliverFromCatalog":false,"calendar":"always"},
 {"priority":"P2","firstResponseMinutes":30,"resolveMinutes":480,"deliverMinutes":540,"deliverFromCatalog":false,"calendar":"business_hours"},
 {"priority":"P3","firstResponseMinutes":120,"resolveMinutes":1080,"deliverMinutes":1080,"deliverFromCatalog":true,"calendar":"business_hours"},
 {"priority":"P4","firstResponseMinutes":240,"resolveMinutes":2700,"deliverMinutes":2700,"deliverFromCatalog":false,"calendar":"business_hours"}],
"catalogDeliverMinutes":2160}`

func TestBackfillMovesTicketsToProjects(t *testing.T) {
	ctx := context.Background()
	st := testdb.NewEmpty(t)
	pool := st.Pool()
	if err := store.MigrateUpTo(ctx, pool, versionBeforeBackfill); err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	scan := func(dst any, sql string, args ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(dst); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}

	// A 24/7 plan whose P2 deliver time is a whole day, to check the calendar-day unit.
	var gold uuid.UUID
	scan(&gold, `WITH p AS (INSERT INTO sla_plans (name) VALUES ('Gold') RETURNING id)
		INSERT INTO sla_targets (plan_id, priority, first_response_minutes, resolve_minutes, deliver_minutes, calendar)
		SELECT p.id, x.priority, x.fr, x.rs, x.dl, 'always'
		FROM p, (VALUES ('P1', 15, 240, 240), ('P2', 30, 90, 1440), ('P3', 60, 120, 120), ('P4', 60, 125, 125)) AS x(priority, fr, rs, dl)
		RETURNING plan_id`)

	var withTickets, onGold, onlyRequesters, empty, alreadyProject uuid.UUID
	scan(&withTickets, `INSERT INTO clients (name) VALUES ('With tickets') RETURNING id`)
	scan(&onGold, `INSERT INTO clients (name, sla_plan_id) VALUES ('On Gold', $1) RETURNING id`, gold)
	scan(&onlyRequesters, `INSERT INTO clients (name) VALUES ('Only requesters') RETURNING id`)
	scan(&empty, `INSERT INTO clients (name) VALUES ('Empty') RETURNING id`)
	scan(&alreadyProject, `INSERT INTO clients (name) VALUES ('Already has a project') RETURNING id`)
	existing := testdb.Project(t, st, alreadyProject, "e-Office", true)

	var requester, system, item uuid.UUID
	scan(&requester, `INSERT INTO users (email, name, role, client_id) VALUES ('r@c.test', 'R', 'requester', $1) RETURNING id`, withTickets)
	exec(`INSERT INTO users (email, name, role, client_id) VALUES ('r2@c.test', 'R2', 'requester', $1)`, onlyRequesters)
	scan(&system, `INSERT INTO systems (name) VALUES ('e-Office') RETURNING id`)
	scan(&item, `INSERT INTO catalog_items (name, delivery_minutes) VALUES ('VPN', 2160) RETURNING id`)

	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	insertTicket := func(number int, client uuid.UUID, typ string, systemID, itemID *uuid.UUID) {
		t.Helper()
		exec(`INSERT INTO tickets (number, key, type, status, priority, channel, client_id, requester_id, created_by,
			system_id, catalog_item_id, subject, description, sla_snapshot, created_at, updated_at)
			VALUES ($1, $2, $3, 'new', 'P3', 'portal', $4, $5, $5, $6, $7, 'S', 'D', $8, $9, $9)`,
			number, fmt.Sprintf("HD-%d", number), typ, client, requester, systemID, itemID, v1Snapshot, stamp)
	}
	insertTicket(1, withTickets, "incident", &system, nil)
	insertTicket(2, withTickets, "request", nil, &item)
	insertTicket(3, onGold, "incident", nil, nil)
	insertTicket(4, alreadyProject, "incident", nil, nil)

	if err := store.MigrateUp(ctx, pool, io.Discard); err != nil {
		t.Fatal(err)
	}

	projectOf := func(client uuid.UUID) (id uuid.UUID, name string) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT id, name FROM projects WHERE client_id = $1 AND is_default`, client).Scan(&id, &name); err != nil {
			t.Fatalf("default project of %s: %v", client, err)
		}
		return id, name
	}
	for _, c := range []uuid.UUID{withTickets, onGold, onlyRequesters} {
		if _, name := projectOf(c); name != "General support" {
			t.Errorf("client %s default = %q", c, name)
		}
	}
	if id, _ := projectOf(alreadyProject); id != existing {
		t.Errorf("a client's existing default was replaced")
	}
	var n int
	scan(&n, `SELECT count(*) FROM projects WHERE client_id = $1`, empty)
	if n != 0 {
		t.Errorf("a client with nothing got %d projects", n)
	}

	// Plan minutes become the units people write: business hours in working days or hours,
	// 24/7 in days or hours.
	type unitRow struct{ priority, field, want string }
	targets := func(project uuid.UUID) map[string]string {
		rows, err := pool.Query(ctx, `SELECT priority, first_response_value || ' ' || first_response_unit,
			resolve_value || ' ' || resolve_unit, deliver_value || ' ' || deliver_unit
			FROM project_sla_targets WHERE project_id = $1`, project)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]string{}
		for rows.Next() {
			var p, fr, rs, dl string
			if err := rows.Scan(&p, &fr, &rs, &dl); err != nil {
				t.Fatal(err)
			}
			out[p+".first"], out[p+".resolve"], out[p+".deliver"] = fr, rs, dl
		}
		return out
	}
	standardProject, _ := projectOf(withTickets)
	goldProject, _ := projectOf(onGold)
	for _, c := range []struct {
		project uuid.UUID
		rows    []unitRow
	}{
		{standardProject, []unitRow{
			{"P1", "first", "15 min"}, {"P1", "resolve", "4 h"}, {"P2", "resolve", "8 h"}, {"P2", "deliver", "1 wd"},
			{"P3", "first", "2 h"}, {"P3", "resolve", "2 wd"}, {"P4", "deliver", "5 wd"},
		}},
		{goldProject, []unitRow{{"P2", "resolve", "90 min"}, {"P2", "deliver", "1 d"}, {"P4", "resolve", "125 min"}}},
	} {
		got := targets(c.project)
		if len(got) != 3*len(sla.Priorities) {
			t.Fatalf("project %s targets = %v", c.project, got)
		}
		for _, r := range c.rows {
			if v := got[r.priority+"."+r.field]; v != r.want {
				t.Errorf("%s %s = %q, want %q", r.priority, r.field, v, r.want)
			}
		}
	}

	// Tickets point at their client's default, keep updated_at, and carry a v2 snapshot.
	rows, err := pool.Query(ctx, `SELECT key, project_id, updated_at, sla_snapshot FROM tickets ORDER BY number`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	wantProject := map[string]uuid.UUID{"HD-1": standardProject, "HD-2": standardProject, "HD-3": goldProject, "HD-4": existing}
	for rows.Next() {
		var key string
		var project uuid.UUID
		var updated time.Time
		var raw []byte
		if err := rows.Scan(&key, &project, &updated, &raw); err != nil {
			t.Fatal(err)
		}
		if project != wantProject[key] {
			t.Errorf("%s project = %s, want %s", key, project, wantProject[key])
		}
		if !updated.Equal(stamp) {
			t.Errorf("%s updated_at = %s, want %s", key, updated, stamp)
		}
		var snap domain.SLASnapshot
		if err := json.Unmarshal(raw, &snap); err != nil {
			t.Fatal(err)
		}
		if snap.Version != domain.SnapshotVersion || snap.ProjectID != project || len(snap.Targets) != len(sla.Priorities) {
			t.Errorf("%s snapshot = %+v", key, snap)
		}
		// The frozen plan's catalogue override (2160 working minutes) becomes P3's deliver time.
		eff, err := snap.Effective(sla.P3, domain.TypeRequest)
		if err != nil || eff.Resolution != (sla.Duration{Value: 4, Unit: sla.UnitWorkingDays}) || !eff.PauseOnUAT {
			t.Errorf("%s P3 request = %+v, err %v", key, eff, err)
		}
	}

	// The old tables are gone and requests are still the only tickets with a catalogue item.
	for _, table := range []string{"sla_plans", "sla_targets", "systems"} {
		var exists bool
		scan(&exists, `SELECT to_regclass($1) IS NOT NULL`, table)
		if exists {
			t.Errorf("table %s still exists", table)
		}
	}
	_, err = pool.Exec(ctx, `UPDATE tickets SET catalog_item_id = $1 WHERE key = 'HD-1'`, item)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintTicketTypeDetail) {
		t.Errorf("incident with a catalogue item: err = %v", err)
	}
}
