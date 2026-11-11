package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/DujinA/spendulum/backend/internal/platform/db"
)

const userUsage = "usage: spendulum user create --email <email> | spendulum user reset-password --user <email>"

var (
	errUserUsage        = errors.New(userUsage)
	errPasswordsDiffer  = errors.New("the passwords do not match")
	errPasswordNotTyped = errors.New("read password: nothing was typed")
)

// The password is typed at the prompt, never passed as a flag, so it stays out of shell history.
func user(ctx context.Context, inv invocation) error {
	if len(inv.args) == 0 {
		return errUserUsage
	}

	sub, args := inv.args[0], inv.args[1:]

	var flagName string

	switch sub {
	case "create":
		flagName = "email"
	case "reset-password":
		flagName = "user"
	default:
		return errUserUsage
	}

	fs := flag.NewFlagSet("user "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	email := fs.String(flagName, "", "the user's email")

	if err := fs.Parse(args); err != nil || *email == "" || fs.NArg() != 0 {
		return errUserUsage
	}

	password, err := newPrompt(inv.in, inv.out).passwordTwice()
	if err != nil {
		return err
	}

	d, err := db.Open(ctx, inv.cfg.DatabaseURL, inv.cfg.DBTimeout)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer d.Close()

	svc := newAuthService(d)

	if sub == "create" {
		id, err := svc.CreateAccount(ctx, *email, password)
		if err != nil {
			return fmt.Errorf("user create: %w", err)
		}

		inv.logger.InfoContext(ctx, "account created", slog.String("user_id", id.String()))

		return nil
	}

	id, err := svc.ResetPassword(ctx, *email, password)
	if err != nil {
		return fmt.Errorf("user reset-password: %w", err)
	}

	inv.logger.InfoContext(ctx, "password reset; every session revoked", slog.String("user_id", id.String()))

	return nil
}

type prompt struct {
	terminal *os.File
	lines    *bufio.Reader
	out      io.Writer
}

func newPrompt(in io.Reader, out io.Writer) prompt {
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return prompt{terminal: f, out: out}
	}

	return prompt{lines: bufio.NewReader(in), out: out}
}

func (p prompt) passwordTwice() (string, error) {
	first, err := p.password("Password: ")
	if err != nil {
		return "", err
	}

	second, err := p.password("Repeat password: ")
	if err != nil {
		return "", err
	}

	if first != second {
		return "", errPasswordsDiffer
	}

	return first, nil
}

func (p prompt) password(label string) (string, error) {
	_, _ = fmt.Fprint(p.out, label)

	if p.terminal != nil {
		b, err := term.ReadPassword(int(p.terminal.Fd()))
		_, _ = fmt.Fprintln(p.out)

		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}

		return string(b), nil
	}

	line, err := p.lines.ReadString('\n')
	if err != nil && (line == "" || !errors.Is(err, io.EOF)) {
		return "", errPasswordNotTyped
	}

	return strings.TrimRight(line, "\r\n"), nil
}
