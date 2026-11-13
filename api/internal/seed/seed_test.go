package seed_test

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/config"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/holidaypreset"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/seed"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

var cheapParams = auth.Params{Memory: 64, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}

func TestSeedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	st := testdb.New(t)
	h := auth.NewHasher(cheapParams, 1)
	for range 2 {
		if err := seed.Run(ctx, config.EnvDev, st, h, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	count := func(sql string) int {
		var n int
		if err := st.Pool().QueryRow(ctx, sql).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	presetDays := 0
	for _, year := range holidaypreset.Years() {
		p, _ := holidaypreset.ForYear(year)
		presetDays += len(p.Holidays)
	}
	got := map[string]int{
		"users":            count(`SELECT count(*) FROM users`),
		"clients":          count(`SELECT count(*) FROM clients`),
		"plans":            count(`SELECT count(*) FROM sla_plans`),
		"gold targets":     count(`SELECT count(*) FROM sla_targets t JOIN sla_plans p ON p.id = t.plan_id WHERE p.name = 'Gold'`),
		"gold clients":     count(`SELECT count(*) FROM clients c JOIN sla_plans p ON p.id = c.sla_plan_id WHERE p.name = 'Gold'`),
		"agent links":      count(`SELECT count(*) FROM client_agents`),
		"holidays":         count(`SELECT count(*) FROM holidays`),
		"standard default": count(`SELECT count(*) FROM sla_plans WHERE is_default AND name = 'Standard'`),
		"invited":          count(`SELECT count(*) FROM users WHERE status = 'invited' AND password_hash IS NULL`),
		"systems":          count(`SELECT count(*) FROM systems`),
		"catalog items":    count(`SELECT count(*) FROM catalog_items`),
		"projects":         count(`SELECT count(*) FROM projects`),
		"project targets":  count(`SELECT count(*) FROM project_sla_targets`),
		"default projects": count(`SELECT count(*) FROM projects WHERE is_default`),
		"e-Office P1 fix":  count(`SELECT resolve_value FROM project_sla_targets t JOIN projects p ON p.id = t.project_id WHERE p.name = 'e-Office' AND t.priority = 'P1'`),
		"tickets":          count(`SELECT count(*) FROM tickets`),
		"messages":         count(`SELECT count(*) FROM ticket_messages`),
		"internal notes":   count(`SELECT count(*) FROM ticket_messages WHERE visibility = 'internal'`),
		"ticket history":   count(`SELECT count(DISTINCT target_id) FROM audit_log WHERE action = 'ticket.created'`),
		"next number":      count(`SELECT nextval('ticket_number_seq')::int`),
	}
	want := map[string]int{
		"users": 8, "clients": 8, "plans": 2, "gold targets": 4, "gold clients": 2, "agent links": 7,
		"holidays": presetDays, "standard default": 1, "invited": 1, "systems": 7, "catalog items": 5,
		"projects": 7, "project targets": 28, "default projects": 4, "e-Office P1 fix": 2,
		"tickets": 6, "messages": 7, "internal notes": 1, "ticket history": 6, "next number": 24872,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %d, want %d", k, got[k], w)
		}
	}
	org, err := st.Q().GetOrganization(ctx)
	if err != nil || org.Name != seed.OrgName {
		t.Fatalf("org = %+v err %v", org, err)
	}
}

func TestSeedRefusesOutsideDev(t *testing.T) {
	err := seed.Run(context.Background(), config.EnvProd, testdb.New(t), auth.NewHasher(cheapParams, 1), io.Discard)
	if !errors.Is(err, seed.ErrNotDev) {
		t.Fatalf("err = %v, want ErrNotDev", err)
	}
}
