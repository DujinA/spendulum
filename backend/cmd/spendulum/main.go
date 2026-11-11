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

const usage = "usage: spendulum <serve|migrate|user>"

type invocation struct {
	cfg    config.Config
	logger *slog.Logger
	args   []string
	in     io.Reader
	out    io.Writer
}

type command func(ctx context.Context, inv invocation) error

func commands() map[string]command {
	return map[string]command{
		"serve":   serve,
		"migrate": migrate,
		"user":    user,
	}
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], environ(), os.Stdin, os.Stdout))
}

func run(ctx context.Context, args []string, env map[string]string, in io.Reader, out io.Writer) int {
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

	if err := cmd(ctx, invocation{cfg: cfg, logger: logger, args: args[1:], in: in, out: out}); err != nil {
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
