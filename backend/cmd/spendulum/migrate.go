package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/DujinA/spendulum/backend/internal/platform/config"
	"github.com/DujinA/spendulum/backend/internal/platform/db"
)

// migrate applies every pending migration. It fails when the database is unreachable.
func migrate(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	d, err := db.Open(ctx, cfg.DatabaseURL, cfg.DBTimeout)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer d.Close()

	ctx, cancel := context.WithTimeout(ctx, cfg.MigrateTimeout)
	defer cancel()

	applied, err := d.Migrate(ctx)
	if err != nil {
		return err //nolint:wrapcheck // db already says what failed
	}

	logger.InfoContext(ctx, "migrations applied", slog.Int("applied", applied))

	return nil
}
