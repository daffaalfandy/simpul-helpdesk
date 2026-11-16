package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

const (
	constraintDoDItem          = "ticket_dod_items_item"
	constraintDoDItemsKey      = "ticket_dod_items_pkey"
	constraintOverrideReason   = "ticket_dod_overrides_reason_length"
	constraintOverrideConfig   = "ticket_dod_overrides_config_object"
	constraintProjectDoDObject = "projects_dod_config_object"
	sqlInsertDoDItem           = `INSERT INTO ticket_dod_items (ticket_id, item) VALUES ($1, $2)`
	sqlInsertDoDOverride       = `INSERT INTO ticket_dod_overrides (ticket_id, config, reason, set_by) VALUES ($1, $2, $3, $4)`
	overrideReasonRune         = "x"
)

// Projects that existed before the Definition of Done resolve by hand: the column default has no
// conditions, and it is the Go None.
func TestProjectDoDDefault(t *testing.T) {
	ctx := context.Background()
	f := newTicketFixture(t)
	var raw []byte
	if err := f.pool.QueryRow(ctx, `SELECT dod_config FROM projects WHERE id = $1`, f.project).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if got, err := dod.Parse(raw); err != nil || got != dod.None() {
		t.Fatalf("default = %+v, %v", got, err)
	}
	_, err := f.pool.Exec(ctx, `UPDATE projects SET dod_config = '[]' WHERE id = $1`, f.project)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintProjectDoDObject) {
		t.Errorf("array config: err = %v", err)
	}
}

// Every item the code knows is one the database accepts, once per ticket.
func TestDoDItemsMatchSchema(t *testing.T) {
	ctx := context.Background()
	f := newTicketFixture(t)
	tk := f.mustInsert(t, ticket{})
	for _, it := range dod.Items {
		if _, err := f.pool.Exec(ctx, sqlInsertDoDItem, tk, string(it)); err != nil {
			t.Errorf("item %s: %v", it, err)
		}
	}
	_, err := f.pool.Exec(ctx, sqlInsertDoDItem, tk, string(dod.ItemMerged))
	if !store.IsViolation(err, store.SQLStateUniqueViolation, constraintDoDItemsKey) {
		t.Errorf("same item twice: err = %v", err)
	}
	_, err = f.pool.Exec(ctx, sqlInsertDoDItem, tk, "reviewed")
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintDoDItem) {
		t.Errorf("unknown item: err = %v", err)
	}
}

func TestDoDOverrideConstraints(t *testing.T) {
	ctx := context.Background()
	f := newTicketFixture(t)
	tk := f.mustInsert(t, ticket{})
	cfg := dod.None().JSON()
	cases := []struct {
		name       string
		config     []byte
		reason     string
		constraint string
	}{
		{"empty reason", cfg, "", constraintOverrideReason},
		{"reason too long", cfg, strings.Repeat(overrideReasonRune, tickets.MaxReasonLen+1), constraintOverrideReason},
		{"array config", []byte(`[]`), "Hotfix", constraintOverrideConfig},
	}
	for _, c := range cases {
		_, err := f.pool.Exec(ctx, sqlInsertDoDOverride, tk, c.config, c.reason, f.agent)
		if !store.IsViolation(err, store.SQLStateCheckViolation, c.constraint) {
			t.Errorf("%s: err = %v", c.name, err)
		}
	}
	if _, err := f.pool.Exec(ctx, sqlInsertDoDOverride, tk, cfg, "Hotfix approved by phone", f.agent); err != nil {
		t.Fatalf("valid override: %v", err)
	}
}
