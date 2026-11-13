package store

import (
	"context"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/daffaalfandy/simpul-helpdesk/api/migrations"
)

// MigrateUp applies all app migrations, then River's job-queue migrations.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	provider, closeDB, err := newProvider(pool)
	if err != nil {
		return err
	}
	defer closeDB()

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("app migrations: %w", err)
	}
	for _, r := range results {
		fmt.Fprintf(out, "applied %s (%s)\n", r.Source.Path, r.Duration.Round(1e6))
	}

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("job queue migrator: %w", err)
	}
	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("job queue migrations: %w", err)
	}
	for _, v := range res.Versions {
		fmt.Fprintf(out, "applied job queue migration %03d\n", v.Version)
	}
	if len(results) == 0 && len(res.Versions) == 0 {
		fmt.Fprintln(out, "database is up to date")
	}
	return nil
}

// MigrateUpTo applies app migrations up to and including version, and nothing else. Tests use
// it to load data in an older schema before a data migration runs.
func MigrateUpTo(ctx context.Context, pool *pgxpool.Pool, version int64) error {
	provider, closeDB, err := newProvider(pool)
	if err != nil {
		return err
	}
	defer closeDB()
	if _, err := provider.UpTo(ctx, version); err != nil {
		return fmt.Errorf("app migrations to %d: %w", version, err)
	}
	return nil
}

// MigrateDown rolls back the most recent app migration.
func MigrateDown(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	provider, closeDB, err := newProvider(pool)
	if err != nil {
		return err
	}
	defer closeDB()
	r, err := provider.Down(ctx)
	if err != nil {
		return fmt.Errorf("rollback: %w", err)
	}
	fmt.Fprintf(out, "rolled back %s\n", r.Source.Path)
	return nil
}

// MigrateStatus prints the state of each app migration.
func MigrateStatus(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	provider, closeDB, err := newProvider(pool)
	if err != nil {
		return err
	}
	defer closeDB()
	statuses, err := provider.Status(ctx)
	if err != nil {
		return err
	}
	for _, s := range statuses {
		fmt.Fprintf(out, "%-10s %s\n", s.State, s.Source.Path)
	}
	return nil
}

func newProvider(pool *pgxpool.Pool) (*goose.Provider, func(), error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("load migrations: %w", err)
	}
	return provider, func() { _ = sqlDB.Close() }, nil
}
