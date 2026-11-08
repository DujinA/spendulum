package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/DujinA/spendulum/backend/migrations"
)

// Migrate applies every pending embedded migration and returns how many it applied.
// A session advisory lock keeps two concurrent runs from racing. The caller bounds
// the whole run with ctx.
func (d *DB) Migrate(ctx context.Context) (int, error) {
	p, sqlDB, err := d.provider()
	if err != nil {
		return 0, err
	}
	defer sqlDB.Close()

	results, err := p.Up(ctx)
	if err != nil {
		return len(results), fmt.Errorf("db: migrate: %w", err)
	}

	return len(results), nil
}

// HasPending reports whether an embedded migration is not applied yet. A database
// newer than the binary has nothing pending (ADR 0008).
func (d *DB) HasPending(ctx context.Context) (bool, error) {
	p, sqlDB, err := d.provider()
	if err != nil {
		return false, err
	}
	defer sqlDB.Close()

	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	pending, err := p.HasPending(ctx)
	if err != nil {
		return false, fmt.Errorf("db: check pending migrations: %w", err)
	}

	return pending, nil
}

func (d *DB) provider() (*goose.Provider, *sql.DB, error) {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, nil, fmt.Errorf("db: migration lock: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(d.pool)

	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithSessionLocker(locker))
	if err != nil {
		_ = sqlDB.Close()

		return nil, nil, fmt.Errorf("db: migrations: %w", err)
	}

	return p, sqlDB, nil
}
