// Package testdb gives each integration test a fresh database, cloned from a template migrated
// once per package (one Postgres container per package, or TEST_PG_URL).
package testdb

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

const (
	// EnvURL points tests at an existing server instead of starting a container.
	// The user must be allowed to create databases.
	EnvURL = "TEST_PG_URL"

	image       = "postgres:18.6-alpine"
	adminDB     = "postgres"
	adminUser   = "postgres"
	adminPass   = "postgres"
	sslDisabled = "sslmode=disable"
	exitFailure = 1

	startAttempts   = 3
	startRetryDelay = 2 * time.Second
)

var (
	adminURL     string
	templateName = fmt.Sprintf("simdesk_tmpl_%d", os.Getpid())
	createMu     sync.Mutex // CREATE DATABASE … TEMPLATE fails if run concurrently
	counter      atomic.Int64
)

// Main runs the package's tests against a migrated template database.
func Main(m *testing.M) int {
	ctx := context.Background()
	var terminate func()
	adminURL = os.Getenv(EnvURL)
	if adminURL == "" {
		c, err := startContainer(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "testdb: start postgres (is Docker running? or set %s): %v\n", EnvURL, err)
			return exitFailure
		}
		terminate = func() { _ = testcontainers.TerminateContainer(c) }
		if adminURL, err = c.ConnectionString(ctx, sslDisabled); err != nil {
			terminate()
			fmt.Fprintln(os.Stderr, "testdb:", err)
			return exitFailure
		}
	}
	if err := buildTemplate(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "testdb: build template:", err)
		if terminate != nil {
			terminate()
		}
		return exitFailure
	}
	code := m.Run()
	_ = adminExec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", templateName))
	if terminate != nil {
		terminate()
	}
	return code
}

// startContainer starts Postgres, retrying because test packages run in
// parallel and can race while testcontainers starts its shared reaper.
func startContainer(ctx context.Context) (*postgres.PostgresContainer, error) {
	var lastErr error
	for attempt := range startAttempts {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * startRetryDelay)
		}
		c, err := postgres.Run(ctx, image,
			postgres.WithDatabase(adminDB), postgres.WithUsername(adminUser), postgres.WithPassword(adminPass),
			postgres.BasicWaitStrategies())
		if err == nil {
			return c, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func buildTemplate(ctx context.Context) error {
	if err := adminExec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", templateName)); err != nil {
		return err
	}
	if err := adminExec(ctx, fmt.Sprintf("CREATE DATABASE %s", templateName)); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, withDB(adminURL, templateName))
	if err != nil {
		return err
	}
	defer pool.Close()
	return store.MigrateUp(ctx, pool, io.Discard)
}

// NewEmpty returns a Store on a new database with no migrations applied, dropped when the test
// ends. Migration tests apply versions themselves.
func NewEmpty(tb testing.TB) *store.Store {
	tb.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("e_%d_%d", os.Getpid(), counter.Add(1))
	if err := adminExec(ctx, fmt.Sprintf("CREATE DATABASE %s", name)); err != nil {
		tb.Fatalf("testdb: create database: %v", err)
	}
	return open(tb, name)
}

// New returns a Store on a new database cloned from the migrated template.
// The database is dropped when the test ends.
func New(tb testing.TB) *store.Store {
	tb.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("t_%d_%d", os.Getpid(), counter.Add(1))
	createMu.Lock()
	err := adminExec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, templateName))
	createMu.Unlock()
	if err != nil {
		tb.Fatalf("testdb: create database: %v", err)
	}
	return open(tb, name)
}

// open connects to database name and drops it when the test ends.
func open(tb testing.TB, name string) *store.Store {
	tb.Helper()
	pool, err := pgxpool.New(context.Background(), withDB(adminURL, name))
	if err != nil {
		tb.Fatalf("testdb: connect: %v", err)
	}
	tb.Cleanup(func() {
		pool.Close()
		_ = adminExec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))
	})
	return store.New(pool)
}

func adminExec(ctx context.Context, sql string) error {
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, sql)
	return err
}

func withDB(raw, db string) string {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	u.Path = "/" + db
	return u.String()
}
