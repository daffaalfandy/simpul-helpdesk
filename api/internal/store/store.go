// Package store owns the database pool and transactions.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"simdesk/internal/db"
)

// Store wraps the connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// Tx is a running transaction with typed queries.
type Tx struct {
	Q  *db.Queries
	PG pgx.Tx
}

// Open connects to Postgres and verifies the connection.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{pool: pool}, nil
}

// New wraps an existing pool (used by tests).
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the pool for wiring River and migrations only.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Close closes the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping checks the database is reachable.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Q runs single statements outside a transaction.
func (s *Store) Q() *db.Queries { return db.New(s.pool) }

// Tx runs fn in a transaction. It commits if fn returns nil and rolls back otherwise.
func (s *Store) Tx(ctx context.Context, fn func(Tx) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(Tx{Q: db.New(tx), PG: tx})
	})
}
