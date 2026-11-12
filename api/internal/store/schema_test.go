package store_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

func pgCode(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

const (
	codeInsufficientPrivilege = "42501"
	codeCheckViolation        = "23514"
	codeUniqueViolation       = "23505"
)

func TestAuditLogIsAppendOnly(t *testing.T) {
	ctx := context.Background()
	st := testdb.New(t)
	pool := st.Pool()
	if _, err := pool.Exec(ctx, `INSERT INTO audit_log (actor_kind, action) VALUES ('system', 'test.event')`); err != nil {
		t.Fatal(err)
	}
	for name, sql := range map[string]string{
		"update":   `UPDATE audit_log SET action = 'changed'`,
		"delete":   `DELETE FROM audit_log`,
		"truncate": `TRUNCATE audit_log`,
	} {
		if _, err := pool.Exec(ctx, sql); pgCode(err) != codeInsufficientPrivilege {
			t.Errorf("%s: err = %v, want SQLSTATE %s", name, err, codeInsufficientPrivilege)
		}
	}
}

func TestRequesterMustHaveClient(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t).Pool()
	_, err := pool.Exec(ctx, `INSERT INTO users (email, name, role) VALUES ('a@b.test', 'A', 'requester')`)
	if pgCode(err) != codeCheckViolation {
		t.Fatalf("requester without client: err = %v, want check violation", err)
	}
	var clientID string
	if err := pool.QueryRow(ctx, `INSERT INTO clients (name) VALUES ('C') RETURNING id`).Scan(&clientID); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO users (email, name, role, client_id) VALUES ('b@b.test', 'B', 'agent', $1)`, clientID)
	if pgCode(err) != codeCheckViolation {
		t.Fatalf("agent with client: err = %v, want check violation", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (email, name, role, client_id) VALUES ('c@b.test', 'C', 'requester', $1)`, clientID); err != nil {
		t.Fatalf("requester with client: %v", err)
	}
}

func TestEmailMustBeLowercase(t *testing.T) {
	_, err := testdb.New(t).Pool().Exec(context.Background(),
		`INSERT INTO users (email, name, role) VALUES ('Rina@X.test', 'R', 'agent')`)
	if pgCode(err) != codeCheckViolation {
		t.Fatalf("err = %v, want check violation", err)
	}
}

func TestOrganizationIsSingleton(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t).Pool()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM organization`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("organization rows = %d (err %v), want 1", n, err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO organization (name) VALUES ('Second')`)
	if pgCode(err) != codeUniqueViolation {
		t.Fatalf("second organization: err = %v, want unique violation", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO organization (id, name) VALUES (false, 'Second')`)
	if pgCode(err) != codeCheckViolation {
		t.Fatalf("organization id=false: err = %v, want check violation", err)
	}
}
