package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/DujinA/spendulum/backend/internal/httpapi"
	"github.com/DujinA/spendulum/backend/internal/platform/config"
	"github.com/DujinA/spendulum/backend/internal/platform/db"
	"github.com/DujinA/spendulum/backend/internal/webui"
)

var errPendingMigrations = errors.New("the database has pending migrations: run spendulum migrate first")

// A drain past the deadline closes the remaining connections and still exits 0.
func serve(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	d, err := db.Open(ctx, cfg.DatabaseURL, cfg.DBTimeout)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer d.Close()

	if err := checkMigrations(ctx, d, logger); err != nil {
		return err
	}

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen on SPENDULUM_HTTP_ADDR %q: %w", cfg.HTTPAddr, err)
	}

	srv := &http.Server{
		Handler: httpapi.NewHandler(httpapi.Deps{
			DB:     d,
			WebUI:  webui.Handler(webui.FS()),
			Logger: logger,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return context.WithoutCancel(ctx) },
	}

	serveErr := make(chan error, 1)

	go func() { serveErr <- srv.Serve(ln) }()

	logger.InfoContext(ctx, "serving", slog.String("addr", ln.Addr().String()))

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down", slog.Duration("deadline", cfg.ShutdownTimeout))

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("shutdown deadline passed; closing the remaining connections", slog.Any("error", err))
		_ = srv.Close()
	}

	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}

	logger.Info("stopped")

	return nil
}

// Pending migrations stop serve (ADR 0004). An unreachable database does not: /readyz reports it.
func checkMigrations(ctx context.Context, d *db.DB, logger *slog.Logger) error {
	pending, err := d.HasPending(ctx)

	switch {
	case err != nil:
		logger.WarnContext(ctx, "could not check for pending migrations; starting anyway", slog.Any("error", err))

		return nil
	case pending:
		return errPendingMigrations
	default:
		return nil
	}
}
