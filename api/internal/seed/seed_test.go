package seed_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/config"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/holidaypreset"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/secretbox"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/seed"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

var cheapParams = auth.Params{Memory: 64, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}

func testBox(t *testing.T) *secretbox.Box {
	t.Helper()
	b, err := secretbox.New(bytes.Repeat([]byte{1}, secretbox.KeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSeedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	st := testdb.New(t)
	h := auth.NewHasher(cheapParams, 1)
	for range 2 {
		if err := seed.Run(ctx, config.EnvDev, st, h, testBox(t), io.Discard); err != nil {
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
	// The demo tickets from the mock; the closed history below them is checked on its own.
	const demo = "t.number >= 24826"
	presetDays := 0
	for _, year := range holidaypreset.Years() {
		p, _ := holidaypreset.ForYear(year)
		presetDays += len(p.Holidays)
	}
	got := map[string]int{
		"users":               count(`SELECT count(*) FROM users`),
		"clients":             count(`SELECT count(*) FROM clients`),
		"agent links":         count(`SELECT count(*) FROM client_agents`),
		"holidays":            count(`SELECT count(*) FROM holidays`),
		"invited":             count(`SELECT count(*) FROM users WHERE status = 'invited' AND password_hash IS NULL`),
		"catalog items":       count(`SELECT count(*) FROM catalog_items`),
		"tickets on projects": count(`SELECT count(*) FROM tickets t JOIN projects p ON p.id = t.project_id AND p.client_id = t.client_id WHERE ` + demo),
		"requests on default": count(`SELECT count(*) FROM tickets t JOIN projects p ON p.id = t.project_id WHERE t.type = 'request' AND p.is_default AND ` + demo),
		"projects":            count(`SELECT count(*) FROM projects`),
		"project targets":     count(`SELECT count(*) FROM project_sla_targets`),
		"default projects":    count(`SELECT count(*) FROM projects WHERE is_default`),
		"e-Office P1 fix":     count(`SELECT resolve_value FROM project_sla_targets t JOIN projects p ON p.id = t.project_id WHERE p.name = 'e-Office' AND t.priority = 'P1'`),
		"tickets":             count(`SELECT count(*) FROM tickets t WHERE ` + demo),
		"messages":            count(`SELECT count(*) FROM ticket_messages m JOIN tickets t ON t.id = m.ticket_id WHERE ` + demo),
		"internal notes":      count(`SELECT count(*) FROM ticket_messages WHERE visibility = 'internal'`),
		"ticket history":      count(`SELECT count(DISTINCT a.target_id) FROM audit_log a JOIN tickets t ON t.id = a.target_id WHERE a.action = 'ticket.created' AND ` + demo),
		"next number":         count(`SELECT nextval('ticket_number_seq')::int`),
		// The board's resolved column starts at local midnight.
		"resolved today": count(`SELECT count(*) FROM tickets, organization o
			WHERE status = 'resolved' AND resolved_at >= date_trunc('day', now() AT TIME ZONE o.timezone) AT TIME ZONE o.timezone`),
		"assigned, not started": count(`SELECT count(*) FROM tickets WHERE status = 'assigned'`),
		"watchers":              count(`SELECT count(*) FROM ticket_watchers`),
		"notifications":         count(`SELECT count(*) FROM notifications`),
		"unread notifications":  count(`SELECT count(*) FROM notifications WHERE read_at IS NULL`),
		"emails pending":        count(`SELECT count(*) FROM notifications WHERE email AND emailed_at IS NULL`),
		"internal notes to client staff": count(`SELECT count(*) FROM notifications n JOIN users u ON u.id = n.user_id
			WHERE n.event_type = 'internal_note' AND u.role = 'requester'`),
		"projects asking for UAT":      count(`SELECT count(*) FROM projects WHERE (dod_config->'uat'->>'enabled')::boolean`),
		"projects asking to confirm":   count(`SELECT count(*) FROM projects WHERE (dod_config->>'requester_confirms')::boolean`),
		"in UAT":                       count(`SELECT count(*) FROM tickets WHERE status = 'in_uat'`),
		"items met by the requester":   count(`SELECT count(*) FROM ticket_dod_items i JOIN tickets t ON t.id = i.ticket_id WHERE i.done_by = t.requester_id AND ` + demo),
		"UAT requests to client staff": count(`SELECT count(*) FROM notifications WHERE event_type = 'uat_requested'`),
		"repositories":                 count(`SELECT count(*) FROM repositories WHERE last_delivery_at IS NOT NULL`),
		"projects reading code":        count(`SELECT count(*) FROM projects WHERE repository_id IS NOT NULL`),
		"code events":                  count(`SELECT count(*) FROM code_events`),
		"code links":                   count(`SELECT count(*) FROM code_event_links`),
		"unknown keys":                 count(`SELECT count(*) FROM code_event_links WHERE outcome = 'unknown_ticket'`),
		"code notifications":           count(`SELECT count(*) FROM notifications WHERE event_type = 'code'`),
	}
	want := map[string]int{
		"users": 11, "clients": 8, "agent links": 7,
		"holidays": presetDays, "invited": 4, "catalog items": 5,
		"projects": 8, "tickets on projects": 12, "requests on default": 4, "project targets": 32, "default projects": 5, "e-Office P1 fix": 2,
		"tickets": 12, "messages": 12, "internal notes": 1, "ticket history": 12, "next number": 24891,
		"resolved today": 2, "assigned, not started": 1,
		"watchers": 1, "notifications": 51, "emails pending": 0, "internal notes to client staff": 0,
		"projects asking for UAT": 2, "projects asking to confirm": 2, "in UAT": 2, "items met by the requester": 1,
		"UAT requests to client staff": 3,
		"repositories":                 3, "projects reading code": 4, "code events": 8, "code links": 8, "unknown keys": 1,
		"code notifications": 2,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %d, want %d", k, got[k], w)
		}
	}
	// Only the latest day's demo changes are unread; how many depends on the time of day.
	if u := got["unread notifications"]; u == 0 || u >= got["notifications"] {
		t.Errorf("unread notifications = %d of %d", u, got["notifications"])
	}
	org, err := st.Q().GetOrganization(ctx)
	if err != nil || org.Name != seed.OrgName {
		t.Fatalf("org = %+v err %v", org, err)
	}

	// History: closed tickets over more than one month, closed after they were resolved.
	history := count(`SELECT count(*) FROM tickets t WHERE NOT ` + demo)
	if history == 0 {
		t.Error("no history tickets")
	}
	if n := count(`SELECT count(*) FROM tickets t WHERE NOT ` + demo + ` AND (status <> 'closed' OR closed_at < resolved_at)`); n != 0 {
		t.Errorf("%d history tickets are not closed after being resolved", n)
	}
	if n := count(`SELECT count(DISTINCT date_trunc('month', created_at AT TIME ZONE 'Asia/Jakarta')) FROM tickets t WHERE NOT ` + demo); n < 2 {
		t.Errorf("history spans %d months, want at least 2", n)
	}

	// The SLA scan replays every history ticket (silently: no notifications) and finds the
	// breaches the history was built with.
	var logs bytes.Buffer
	notificationsBefore := count(`SELECT count(*) FROM notifications`)
	slaScan := tickets.NewSLAScanner(st, sla.NewService(st, nil), nil, slog.New(slog.NewTextHandler(&logs, nil)))
	if err := slaScan.Work(ctx, &river.Job[tickets.SLAScanArgs]{}); err != nil {
		t.Fatal(err)
	}
	if logs.Len() > 0 {
		t.Errorf("SLA scan: %s", logs.String())
	}
	if n := count(`SELECT count(*) FROM tickets t LEFT JOIN ticket_sla s ON s.ticket_id = t.id WHERE s.ticket_id IS NULL`); n != 0 {
		t.Errorf("%d tickets not replayed", n)
	}
	if n := count(`SELECT count(*) FROM ticket_sla s JOIN tickets t ON t.id = s.ticket_id WHERE NOT ` + demo +
		` AND (s.first_response_breached_at IS NOT NULL OR s.resolution_breached_at IS NOT NULL)`); n == 0 {
		t.Error("no history ticket breached")
	}
	if n := count(`SELECT count(*) FROM notifications`) - notificationsBefore; n != 0 {
		t.Errorf("the backfill notified %d times", n)
	}

	// No client deadline has passed yet: the demo's tests and reopen windows are all open.
	lifecycle := `SELECT count(*) FROM audit_log WHERE actor_name = '` + tickets.ActorNameLifecycle + `'`
	before := count(lifecycle)
	scan := tickets.NewLifecycleScanner(tickets.NewService(st, nil, sla.NewService(st, nil), nil), slog.New(slog.DiscardHandler))
	if err := scan.Work(ctx, &river.Job[tickets.LifecycleScanArgs]{}); err != nil {
		t.Fatal(err)
	}
	if n := count(lifecycle) - before; n != 0 {
		t.Errorf("the lifecycle scan moved %d demo tickets", n)
	}
}

func TestSeedRefusesOutsideDev(t *testing.T) {
	err := seed.Run(context.Background(), config.EnvProd, testdb.New(t), auth.NewHasher(cheapParams, 1), testBox(t), io.Discard)
	if !errors.Is(err, seed.ErrNotDev) {
		t.Fatalf("err = %v, want ErrNotDev", err)
	}
}
