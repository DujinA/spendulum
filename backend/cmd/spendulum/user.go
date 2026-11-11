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

	"github.com/google/uuid"
	"golang.org/x/term"

	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/platform/db"
)

const userUsage = "usage: spendulum user create --email <email> | spendulum user reset-password --user <email>"

var (
	errUserUsage        = errors.New(userUsage)
	errPasswordsDiffer  = errors.New("the passwords do not match")
	errPasswordNotTyped = errors.New("read password: nothing was typed")
)

type userCommand struct {
	flag string
	run  func(svc *auth.Service, ctx context.Context, email, password string) (uuid.UUID, error)
}

func userCommands() map[string]userCommand {
	return map[string]userCommand{
		"create":         {flag: "email", run: (*auth.Service).CreateAccount},
		"reset-password": {flag: "user", run: (*auth.Service).ResetPassword},
	}
}

// The password is typed at the prompt, never passed as a flag, so it stays out of shell history.
func user(ctx context.Context, inv invocation) error {
	if len(inv.args) == 0 {
		return errUserUsage
	}

	cmd, ok := userCommands()[inv.args[0]]
	if !ok {
		return errUserUsage
	}

	fs := flag.NewFlagSet("user "+inv.args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	email := fs.String(cmd.flag, "", "the user's email")

	if err := fs.Parse(inv.args[1:]); err != nil || *email == "" || fs.NArg() != 0 {
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

	id, err := cmd.run(newAuthService(d), ctx, *email, password)
	if err != nil {
		return fmt.Errorf("user %s: %w", inv.args[0], err)
	}

	inv.logger.InfoContext(ctx, "user command succeeded", slog.String("command", inv.args[0]), slog.String("user_id", id.String()))

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

	// Neither a terminal without echo nor piped input ends the prompt's line.
	defer func() { _, _ = fmt.Fprintln(p.out) }()

	if p.terminal != nil {
		b, err := term.ReadPassword(int(p.terminal.Fd()))
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
