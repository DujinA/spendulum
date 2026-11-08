// Package db owns the PostgreSQL connection pool, the generated sqlc queries and the
// Goose migrations.
//
// Every database call runs under a timeout: the queries returned by Queries apply
// the configured timeout to each call, and connection attempts share it, so a stuck
// database never hangs a request.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/DujinA/spendulum/backend/internal/platform/db/gen"
)

// DB is the connection pool together with the per-call timeout.
type DB struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// Open creates the pool without connecting: connections open on first use, so the
// server starts even when the database is down.
func Open(ctx context.Context, url string, timeout time.Duration) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("db: parse URL: %w", err)
	}

	cfg.ConnConfig.ConnectTimeout = timeout

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	return &DB{pool: pool, timeout: timeout}, nil
}

// Close waits for acquired connections to be released and closes the pool.
func (d *DB) Close() {
	d.pool.Close()
}

// Queries returns the generated queries, each bounded by the timeout.
func (d *DB) Queries() *gen.Queries {
	return gen.New(d.dbtx())
}

// Ping runs a trivial query, which proves the database answers.
func (d *DB) Ping(ctx context.Context) error {
	if _, err := d.Queries().Ping(ctx); err != nil {
		return fmt.Errorf("db: ping: %w", err)
	}

	return nil
}

func (d *DB) dbtx() gen.DBTX {
	return timeoutDBTX{db: d.pool, timeout: d.timeout}
}
