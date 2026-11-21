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
	"github.com/DujinA/spendulum/backend/internal/platform/db"
)

var errPendingMigrations = errors.New("the database has pending migrations: run spendulum migrate first")

// A drain past the deadline closes the remaining connections and still exits 0.
func serve(ctx context.Context, inv invocation) error {
	cfg, logger := inv.cfg, inv.logger

	d, err := db.Open(ctx, cfg.DatabaseURL, cfg.DBTimeout)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer d.Close()

	if err := checkMigrations(ctx, d, logger); err != nil {
		return err
	}

	rates, err := newRateScheduler(d, cfg.RateProviderURL, logger)
	if err != nil {
		return err
	}

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen on SPENDULUM_HTTP_ADDR %q: %w", cfg.HTTPAddr, err)
	}

	defer background(ctx, rates.Run)()

	srv := &http.Server{
		Handler:           httpapi.NewHandler(newAPIDeps(d, cfg, logger)),
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

// background runs fn until the returned stop cancels it and waits for it to return.
func background(ctx context.Context, fn func(context.Context)) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		fn(ctx)
		close(done)
	}()

	return func() {
		cancel()
		<-done
	}
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
