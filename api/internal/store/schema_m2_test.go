package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

const (
	constraintDefaultExists   = "sla_plans_default_exists"
	constraintTargetsComplete = "sla_plan_targets_complete"
	constraintSingleDefault   = "sla_plans_single_default_key"
	constraintClientPlanFK    = "clients_sla_plan_id_fkey"
)

var priorities = []string{"P1", "P2", "P3", "P4"}

// createPlan inserts a complete, non-default plan.
func createPlan(t *testing.T, st *store.Store, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := st.Tx(context.Background(), func(tx store.Tx) error {
		var err error
		if id, err = tx.Q.CreateSlaPlan(context.Background(), name); err != nil {
			return err
		}
		for _, p := range priorities {
			if err := tx.Q.UpsertSlaTarget(context.Background(), db.UpsertSlaTargetParams{
				PlanID: id, Priority: p, FirstResponseMinutes: 10, ResolveMinutes: 60, DeliverMinutes: 60,
				Calendar: "business_hours",
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestExactlyOneDefaultSlaPlan(t *testing.T) {
	ctx := context.Background()
	st := testdb.New(t)
	pool := st.Pool()
	gold := createPlan(t, st, "Gold")

	_, err := pool.Exec(ctx, `UPDATE sla_plans SET is_default = true WHERE id = $1`, gold)
	if !store.IsViolation(err, store.SQLStateUniqueViolation, constraintSingleDefault) {
		t.Fatalf("second default: err = %v", err)
	}
	_, err = pool.Exec(ctx, `UPDATE sla_plans SET is_default = false`)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintDefaultExists) {
		t.Fatalf("no default: err = %v", err)
	}
	_, err = pool.Exec(ctx, `DELETE FROM sla_plans WHERE is_default`)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintDefaultExists) {
		t.Fatalf("delete default: err = %v", err)
	}
	// Switching inside one transaction is fine.
	err = st.Tx(ctx, func(tx store.Tx) error {
		if _, err := tx.PG.Exec(ctx, `UPDATE sla_plans SET is_default = false WHERE is_default`); err != nil {
			return err
		}
		_, err := tx.PG.Exec(ctx, `UPDATE sla_plans SET is_default = true WHERE id = $1`, gold)
		return err
	})
	if err != nil {
		t.Fatalf("switch default: %v", err)
	}
}

func TestSlaPlanNeedsEveryPriority(t *testing.T) {
	ctx := context.Background()
	st := testdb.New(t)
	pool := st.Pool()

	_, err := pool.Exec(ctx, `INSERT INTO sla_plans (name) VALUES ('Empty')`)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintTargetsComplete) {
		t.Fatalf("plan without targets: err = %v", err)
	}
	_, err = pool.Exec(ctx, `DELETE FROM sla_targets WHERE priority = 'P4'`)
	if !store.IsViolation(err, store.SQLStateCheckViolation, constraintTargetsComplete) {
		t.Fatalf("delete a target: err = %v", err)
	}
	// Deleting a whole plan takes its targets with it.
	gold := createPlan(t, st, "Gold")
	if _, err := pool.Exec(ctx, `DELETE FROM sla_plans WHERE id = $1`, gold); err != nil {
		t.Fatalf("delete plan: %v", err)
	}
}

func TestStandardPlanMatchesHandoff(t *testing.T) {
	ctx := context.Background()
	q := testdb.New(t).Q()
	plans, err := q.ListSlaPlans(ctx)
	if err != nil || len(plans) != 1 || plans[0].Name != "Standard" || !plans[0].IsDefault {
		t.Fatalf("plans = %+v, err %v", plans, err)
	}
	targets, err := q.ListSlaTargets(ctx, &plans[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []db.ListSlaTargetsRow{
		{PlanID: plans[0].ID, Priority: "P1", FirstResponseMinutes: 15, ResolveMinutes: 240, DeliverMinutes: 240, Calendar: "always"},
		{PlanID: plans[0].ID, Priority: "P2", FirstResponseMinutes: 30, ResolveMinutes: 480, DeliverMinutes: 540, Calendar: "business_hours"},
		{PlanID: plans[0].ID, Priority: "P3", FirstResponseMinutes: 120, ResolveMinutes: 1080, DeliverMinutes: 1080, DeliverFromCatalog: true, Calendar: "business_hours"},
		{PlanID: plans[0].ID, Priority: "P4", FirstResponseMinutes: 240, ResolveMinutes: 2700, DeliverMinutes: 2700, Calendar: "business_hours"},
	}
	if len(targets) != len(want) {
		t.Fatalf("targets = %+v", targets)
	}
	for i := range want {
		if targets[i] != want[i] {
			t.Errorf("target %d = %+v, want %+v", i, targets[i], want[i])
		}
	}
}

func TestClientPlanAndContract(t *testing.T) {
	ctx := context.Background()
	st := testdb.New(t)
	pool := st.Pool()

	var clientID, planID, defaultID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO clients (name) VALUES ('C') RETURNING id, sla_plan_id`).Scan(&clientID, &planID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM sla_plans WHERE is_default`).Scan(&defaultID); err != nil || planID != defaultID {
		t.Fatalf("client plan = %s, default = %s (err %v)", planID, defaultID, err)
	}

	gold := createPlan(t, st, "Gold")
	if _, err := pool.Exec(ctx, `UPDATE clients SET sla_plan_id = $1 WHERE id = $2`, gold, clientID); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `DELETE FROM sla_plans WHERE id = $1`, gold)
	if !store.IsViolation(err, store.SQLStateRestrictViolation, constraintClientPlanFK) {
		t.Fatalf("delete plan in use: err = %v", err)
	}

	for name, sql := range map[string]string{
		"period backwards": `UPDATE clients SET contract_start = '2026-12-31', contract_end = '2026-01-01'`,
		"PIC email case":   `UPDATE clients SET pic_email = 'Hendra@X.test'`,
	} {
		if _, err := pool.Exec(ctx, sql); pgCode(err) != codeCheckViolation {
			t.Errorf("%s: err = %v, want check violation", name, err)
		}
	}
}

func TestDatesRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := testdb.New(t)
	q := st.Q()
	start, _ := calendar.ParseDate("2026-01-01")
	end, _ := calendar.ParseDate("2026-12-31")
	plans, err := q.ListSlaPlans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id, err := q.CreateClient(ctx, db.CreateClientParams{
		Name: "Dinas", ContractStart: &start, ContractEnd: &end, SlaPlanID: plans[0].ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := q.GetClient(ctx, db.GetClientParams{ID: id})
	if err != nil || c.ContractStart == nil || *c.ContractStart != start || *c.ContractEnd != end {
		t.Fatalf("client = %+v, err %v", c, err)
	}
	bare, err := q.CreateClient(ctx, db.CreateClientParams{Name: "Bare", SlaPlanID: plans[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if c, err := q.GetClient(ctx, db.GetClientParams{ID: bare}); err != nil || c.ContractStart != nil || c.ContractEnd != nil {
		t.Fatalf("bare client = %+v, err %v", c, err)
	}

	if _, err := q.CreateHoliday(ctx, db.CreateHolidayParams{Date: end, Name: "Test"}); err != nil {
		t.Fatal(err)
	}
	holidays, err := q.ListHolidays(ctx, db.ListHolidaysParams{FromDate: &start, ToDate: &end})
	if err != nil || len(holidays) != 1 || holidays[0].Date != end {
		t.Fatalf("holidays = %+v, err %v", holidays, err)
	}
	_, err = q.CreateHoliday(ctx, db.CreateHolidayParams{Date: end, Name: "Again"})
	if !store.IsViolation(err, store.SQLStateUniqueViolation, "holidays_date_key") {
		t.Fatalf("duplicate holiday: err = %v", err)
	}
}

func TestBusinessHoursChecks(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t).Pool()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM business_hours WHERE open_minute = 480 AND close_minute = 1020`).Scan(&n); err != nil || n != 5 {
		t.Fatalf("default hours rows = %d (err %v), want Mon–Fri", n, err)
	}
	for name, sql := range map[string]string{
		"open after close": `INSERT INTO business_hours VALUES (0, 600, 540)`,
		"past midnight":    `INSERT INTO business_hours VALUES (0, 0, 1441)`,
		"bad weekday":      `INSERT INTO business_hours VALUES (7, 480, 1020)`,
	} {
		if _, err := pool.Exec(ctx, sql); pgCode(err) != codeCheckViolation {
			t.Errorf("%s: err = %v, want check violation", name, err)
		}
	}
}
