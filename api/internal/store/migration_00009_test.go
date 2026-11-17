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

// versionBeforeLoggedFirstResponse is the last migration before agent-logged tickets count as
// answered when logged.
const versionBeforeLoggedFirstResponse = 8

func TestLoggedTicketsGetTheirFirstResponse(t *testing.T) {
	ctx := context.Background()
	st := testdb.NewEmpty(t)
	pool := st.Pool()
	if err := store.MigrateUpTo(ctx, pool, versionBeforeLoggedFirstResponse); err != nil {
		t.Fatal(err)
	}
	var client, requester, agent uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO clients (name) VALUES ('C') RETURNING id`).Scan(&client); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, name, role, client_id) VALUES ('r@c.test', 'R', 'requester', $1) RETURNING id`,
		client).Scan(&requester); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, name, role) VALUES ('a@v.test', 'A', 'agent') RETURNING id`).Scan(&agent); err != nil {
		t.Fatal(err)
	}
	project := testdb.LegacyProject(t, st, client, "e-Office", true)
	created := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	insert := func(number int, channel string, createdBy uuid.UUID) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		if err := pool.QueryRow(ctx, `
			INSERT INTO tickets (number, key, type, status, priority, channel, client_id, requester_id, created_by,
			                     project_id, subject, description, sla_snapshot, created_at, updated_at)
			VALUES ($1, $2, 'incident', 'new', 'P3', $3, $4, $5, $6, $7, 'S', 'D', '{}', $8, $8)
			RETURNING id`, number, fmt.Sprintf("HD-%d", number), channel, client, requester, createdBy, project, created).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	logged, reported := insert(1, "phone", agent), insert(2, "portal", requester)
	// The old rule already replayed the logged ticket as unanswered.
	if _, err := pool.Exec(ctx, `INSERT INTO ticket_sla (ticket_id, first_response_budget_ms, first_response_calendar,
		resolution_budget_ms, resolution_calendar, first_response_breached_at)
		VALUES ($1, 3600000, 'business_hours', 14400000, 'business_hours', $2)`, logged, created.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	if err := store.MigrateUp(ctx, pool, io.Discard); err != nil {
		t.Fatal(err)
	}

	var firstResponse *time.Time
	var updated time.Time
	if err := pool.QueryRow(ctx, `SELECT first_response_at, updated_at FROM tickets WHERE id = $1`, logged).Scan(&firstResponse, &updated); err != nil {
		t.Fatal(err)
	}
	if firstResponse == nil || !firstResponse.Equal(created) || !updated.Equal(created) {
		t.Fatalf("logged ticket: first response %v, updated %v; want both %v", firstResponse, updated, created)
	}
	if err := pool.QueryRow(ctx, `SELECT first_response_at FROM tickets WHERE id = $1`, reported).Scan(&firstResponse); err != nil {
		t.Fatal(err)
	}
	if firstResponse != nil {
		t.Fatalf("a portal report must still wait for a reply, got %v", firstResponse)
	}
	var entries, clocks int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action = 'ticket.first_responded' AND target_id = $1`, logged).Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ticket_sla WHERE ticket_id = $1`, logged).Scan(&clocks); err != nil {
		t.Fatal(err)
	}
	if entries != 1 || clocks != 0 {
		t.Fatalf("audit entries %d (want 1), stale clocks %d (want 0, so the scan replays them)", entries, clocks)
	}
}
