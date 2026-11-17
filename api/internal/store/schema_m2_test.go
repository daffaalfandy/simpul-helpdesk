package store_test

import (
	"context"
	"testing"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

func TestClientContract(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t).Pool()
	if _, err := pool.Exec(ctx, `INSERT INTO clients (organization_id, name) VALUES ((SELECT id FROM organizations), 'C')`); err != nil {
		t.Fatal(err)
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
	org := testdb.Organization(t, st)
	start, _ := calendar.ParseDate("2026-01-01")
	end, _ := calendar.ParseDate("2026-12-31")
	id, err := q.CreateClient(ctx, db.CreateClientParams{OrganizationID: org, Name: "Dinas", ContractStart: &start, ContractEnd: &end})
	if err != nil {
		t.Fatal(err)
	}
	c, err := q.GetClient(ctx, db.GetClientParams{ID: id, OrganizationID: org})
	if err != nil || c.ContractStart == nil || *c.ContractStart != start || *c.ContractEnd != end {
		t.Fatalf("client = %+v, err %v", c, err)
	}
	bare, err := q.CreateClient(ctx, db.CreateClientParams{OrganizationID: org, Name: "Bare"})
	if err != nil {
		t.Fatal(err)
	}
	if c, err := q.GetClient(ctx, db.GetClientParams{ID: bare, OrganizationID: org}); err != nil || c.ContractStart != nil || c.ContractEnd != nil {
		t.Fatalf("bare client = %+v, err %v", c, err)
	}

	if _, err := q.CreateHoliday(ctx, db.CreateHolidayParams{OrganizationID: org, Date: end, Name: "Test"}); err != nil {
		t.Fatal(err)
	}
	holidays, err := q.ListHolidays(ctx, db.ListHolidaysParams{OrganizationID: org, FromDate: &start, ToDate: &end})
	if err != nil || len(holidays) != 1 || holidays[0].Date != end {
		t.Fatalf("holidays = %+v, err %v", holidays, err)
	}
	_, err = q.CreateHoliday(ctx, db.CreateHolidayParams{OrganizationID: org, Date: end, Name: "Again"})
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
		"open after close": `INSERT INTO business_hours VALUES (0, 600, 540, (SELECT id FROM organizations))`,
		"past midnight":    `INSERT INTO business_hours VALUES (0, 0, 1441, (SELECT id FROM organizations))`,
		"bad weekday":      `INSERT INTO business_hours VALUES (7, 480, 1020, (SELECT id FROM organizations))`,
	} {
		if _, err := pool.Exec(ctx, sql); pgCode(err) != codeCheckViolation {
			t.Errorf("%s: err = %v, want check violation", name, err)
		}
	}
}
