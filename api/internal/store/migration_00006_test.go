package store_test

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

// versionBeforeBoard is the last migration without tickets.status_changed_at.
const versionBeforeBoard = 5

func TestStatusChangedAtBackfillAndTrigger(t *testing.T) {
	ctx := context.Background()
	st := testdb.NewEmpty(t)
	pool := st.Pool()
	if err := store.MigrateUpTo(ctx, pool, versionBeforeBoard); err != nil {
		t.Fatal(err)
	}
	var client, requester uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO clients (name) VALUES ('C') RETURNING id`).Scan(&client); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, name, role, client_id) VALUES ('r@c.test', 'R', 'requester', $1) RETURNING id`,
		client).Scan(&requester); err != nil {
		t.Fatal(err)
	}
	project := testdb.LegacyProject(t, st, client, "e-Office", true)

	created := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	moved := created.Add(3 * time.Hour)
	insert := func(number int) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := pool.QueryRow(ctx, `
			INSERT INTO tickets (number, key, type, status, priority, channel, client_id, requester_id, created_by,
			                     project_id, subject, description, sla_snapshot, created_at, updated_at)
			VALUES ($1, $2, 'incident', 'in_progress', 'P3', 'portal', $3, $4, $4, $5, 'S', 'D', '{}', $6, $6)
			RETURNING id`, number, fmt.Sprintf("HD-%d", number), client, requester, project, created).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	withHistory, withoutHistory := insert(1), insert(2)
	for _, at := range []time.Time{created.Add(time.Hour), moved} {
		if _, err := pool.Exec(ctx, `INSERT INTO audit_log (at, actor_kind, action, target_type, target_id)
			VALUES ($1, 'system', 'ticket.status_changed', 'ticket', $2)`, at, withHistory); err != nil {
			t.Fatal(err)
		}
	}
	// Other actions on the ticket don't count as moves.
	if _, err := pool.Exec(ctx, `INSERT INTO audit_log (at, actor_kind, action, target_type, target_id)
		VALUES ($1, 'system', 'ticket.priority_changed', 'ticket', $2)`, moved.Add(time.Hour), withHistory); err != nil {
		t.Fatal(err)
	}

	if err := store.MigrateUp(ctx, pool, io.Discard); err != nil {
		t.Fatal(err)
	}

	times := func(id uuid.UUID) (changed, updated time.Time) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT status_changed_at, updated_at FROM tickets WHERE id = $1`, id).
			Scan(&changed, &updated); err != nil {
			t.Fatal(err)
		}
		return changed, updated
	}
	for _, c := range []struct {
		id   uuid.UUID
		want time.Time
	}{{withHistory, moved}, {withoutHistory, created}} {
		changed, updated := times(c.id)
		if !changed.Equal(c.want) {
			t.Errorf("status_changed_at = %s, want %s", changed, c.want)
		}
		if !updated.Equal(created) {
			t.Errorf("backfill moved updated_at to %s", updated)
		}
	}

	// From now on the trigger keeps it: a priority change leaves it, a status change moves it.
	if _, err := pool.Exec(ctx, `UPDATE tickets SET priority = 'P1' WHERE id = $1`, withoutHistory); err != nil {
		t.Fatal(err)
	}
	if changed, _ := times(withoutHistory); !changed.Equal(created) {
		t.Errorf("priority change moved status_changed_at to %s", changed)
	}
	if _, err := pool.Exec(ctx, `UPDATE tickets SET status = 'waiting_on_client' WHERE id = $1`, withoutHistory); err != nil {
		t.Fatal(err)
	}
	if changed, _ := times(withoutHistory); !changed.After(created) {
		t.Errorf("status change kept status_changed_at at %s", changed)
	}
}
