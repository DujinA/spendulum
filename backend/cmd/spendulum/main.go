// Command spendulum is the one Spendulum binary.
//
//	spendulum serve     runs the HTTP server: the API and the web app
//	spendulum migrate   applies pending database migrations (ADR 0004)
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/DujinA/spendulum/backend/internal/platform/config"
	"github.com/DujinA/spendulum/backend/internal/platform/logging"
)

const usage = "usage: spendulum <serve|migrate>"

// command runs one subcommand until it finishes or ctx is cancelled.
type command func(ctx context.Context, cfg config.Config, logger *slog.Logger) error

func commands() map[string]command {
	return map[string]command{
		"serve":   serve,
		"migrate": migrate,
	}
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], environ(), os.Stdout))
}

// run executes one subcommand and returns the process exit code. SIGTERM and SIGINT
// cancel the context the subcommand runs under.
func run(ctx context.Context, args []string, env map[string]string, out io.Writer) int {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, os.Interrupt)
	defer stop()

	if len(args) == 0 {
		_, _ = fmt.Fprintln(out, usage)

		return 2
	}

	cmd, ok := commands()[args[0]]
	if !ok {
		_, _ = fmt.Fprintln(out, usage)

		return 2
	}

	cfg, err := config.Load(env)
	if err != nil {
		logging.New(out, "info").Error("invalid configuration", slog.Any("error", err))

		return 1
	}

	logger := logging.New(out, cfg.LogLevel)

	if err := cmd(ctx, cfg, logger); err != nil {
		logger.Error("command failed", slog.String("command", args[0]), slog.Any("error", err))

		return 1
	}

	return 0
}

func environ() map[string]string {
	env := make(map[string]string)

	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		env[k] = v
	}

	return env
}
