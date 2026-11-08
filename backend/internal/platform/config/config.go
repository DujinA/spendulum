// Package config loads the process configuration from environment variables and
// validates it, so an invalid setting stops the process at startup.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/jackc/pgx/v5/pgconn"
)

// Config is every setting the binary reads. Each field names its environment variable.
type Config struct {
	// DatabaseURL is the PostgreSQL connection URL, for example
	// postgres://user:password@localhost:5432/spendulum.
	DatabaseURL string `env:"SPENDULUM_DATABASE_URL,required,notEmpty"`
	// HTTPAddr is the address `serve` listens on.
	HTTPAddr string `env:"SPENDULUM_HTTP_ADDR" envDefault:":8080"`
	// DBTimeout bounds every database call and every connection attempt.
	DBTimeout time.Duration `env:"SPENDULUM_DB_TIMEOUT" envDefault:"5s"`
	// MigrateTimeout bounds one whole `spendulum migrate` run.
	MigrateTimeout time.Duration `env:"SPENDULUM_MIGRATE_TIMEOUT" envDefault:"5m"`
	// ShutdownTimeout is how long `serve` waits for in-flight requests after SIGTERM.
	ShutdownTimeout time.Duration `env:"SPENDULUM_SHUTDOWN_TIMEOUT" envDefault:"20s"`
	// LogLevel is one of debug, info, warn or error.
	LogLevel string `env:"SPENDULUM_LOG_LEVEL" envDefault:"info"`
}

// VarError reports an invalid or missing environment variable by name.
type VarError struct {
	Var    string
	Reason string
}

func (e *VarError) Error() string {
	return fmt.Sprintf("config: %s: %s", e.Var, e.Reason)
}

// Load reads the configuration from environ, a map of environment variables.
// The returned error is a *VarError, or an errors.Join of them, naming each bad variable.
func Load(environ map[string]string) (Config, error) {
	cfg, err := env.ParseAsWithOptions[Config](env.Options{Environment: environ})
	if err != nil {
		return Config{}, fromEnvError(err)
	}

	return cfg, cfg.validate()
}

func (c Config) validate() error {
	var errs []error

	u, err := url.Parse(c.DatabaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		errs = append(errs, &VarError{Var: "SPENDULUM_DATABASE_URL", Reason: "must be a postgres:// URL with a host"})
	} else if _, err := pgconn.ParseConfig(c.DatabaseURL); err != nil {
		errs = append(errs, &VarError{Var: "SPENDULUM_DATABASE_URL", Reason: "is not a valid connection URL"})
	}

	if _, _, err := net.SplitHostPort(c.HTTPAddr); err != nil {
		errs = append(errs, &VarError{Var: "SPENDULUM_HTTP_ADDR", Reason: "must be host:port, for example :8080"})
	}

	for name, d := range map[string]time.Duration{
		"SPENDULUM_DB_TIMEOUT":       c.DBTimeout,
		"SPENDULUM_MIGRATE_TIMEOUT":  c.MigrateTimeout,
		"SPENDULUM_SHUTDOWN_TIMEOUT": c.ShutdownTimeout,
	} {
		if d <= 0 {
			errs = append(errs, &VarError{Var: name, Reason: "must be a positive duration"})
		}
	}

	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, &VarError{Var: "SPENDULUM_LOG_LEVEL", Reason: "must be one of debug, info, warn, error"})
	}

	return errors.Join(errs...)
}

// fromEnvError turns the env library's errors into VarErrors, which always name the
// variable; the library's own parse errors name only the Go field.
func fromEnvError(err error) error {
	var agg env.AggregateError
	if !errors.As(err, &agg) {
		return err
	}

	errs := make([]error, 0, len(agg.Errors))

	for _, e := range agg.Errors {
		var (
			notSet env.VarIsNotSetError
			empty  env.EmptyVarError
			parse  env.ParseError
		)

		switch {
		case errors.As(e, &notSet):
			errs = append(errs, &VarError{Var: notSet.Key, Reason: "is required but not set"})
		case errors.As(e, &empty):
			errs = append(errs, &VarError{Var: empty.Key, Reason: "is required but empty"})
		case errors.As(e, &parse):
			errs = append(errs, &VarError{Var: varName(parse.Name), Reason: "has an invalid value: " + parse.Err.Error()})
		default:
			errs = append(errs, e)
		}
	}

	return errors.Join(errs...)
}

// varName returns the environment variable name declared on the Config field.
func varName(field string) string {
	f, ok := reflect.TypeFor[Config]().FieldByName(field)
	if !ok {
		return field
	}

	name, _, _ := strings.Cut(f.Tag.Get("env"), ",")

	return name
}
