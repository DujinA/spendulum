package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/DujinA/spendulum/backend/internal/platform/db/gen"
)

type DB struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// The pool connects on first use, so serve starts while the database is down.
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

func (d *DB) Close() {
	d.pool.Close()
}

func (d *DB) Queries() *gen.Queries {
	return gen.New(d.dbtx())
}

func (d *DB) Ping(ctx context.Context) error {
	if _, err := d.Queries().Ping(ctx); err != nil {
		return fmt.Errorf("db: ping: %w", err)
	}

	return nil
}

func (d *DB) dbtx() gen.DBTX {
	return timeoutDBTX{db: d.pool, timeout: d.timeout}
}
