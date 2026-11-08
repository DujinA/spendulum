package config_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DujinA/spendulum/backend/internal/platform/config"
)

func validEnv() map[string]string {
	return map[string]string{"SPENDULUM_DATABASE_URL": "postgres://app:secret@localhost:5432/spendulum"}
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := config.Load(validEnv())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := config.Config{
		DatabaseURL:     "postgres://app:secret@localhost:5432/spendulum",
		HTTPAddr:        ":8080",
		DBTimeout:       5 * time.Second,
		MigrateTimeout:  5 * time.Minute,
		ShutdownTimeout: 20 * time.Second,
		LogLevel:        "info",
	}
	if cfg != want {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoadNamesTheBadVariable(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantVar string
	}{
		{
			name:    "missing database URL",
			env:     map[string]string{},
			wantVar: "SPENDULUM_DATABASE_URL",
		},
		{
			name:    "empty database URL",
			env:     map[string]string{"SPENDULUM_DATABASE_URL": ""},
			wantVar: "SPENDULUM_DATABASE_URL",
		},
		{
			name:    "database URL that is not postgres",
			env:     map[string]string{"SPENDULUM_DATABASE_URL": "mysql://localhost/x"},
			wantVar: "SPENDULUM_DATABASE_URL",
		},
		{
			name:    "database URL with an invalid option",
			env:     map[string]string{"SPENDULUM_DATABASE_URL": "postgres://app:secret@localhost:5432/spendulum?sslmode=sometimes"},
			wantVar: "SPENDULUM_DATABASE_URL",
		},
		{
			name:    "database URL with an invalid pool option",
			env:     map[string]string{"SPENDULUM_DATABASE_URL": "postgres://app:secret@localhost:5432/spendulum?pool_max_conns=nope"},
			wantVar: "SPENDULUM_DATABASE_URL",
		},
		{
			name:    "HTTP address without a port",
			env:     with(validEnv(), "SPENDULUM_HTTP_ADDR", "not-an-address"),
			wantVar: "SPENDULUM_HTTP_ADDR",
		},
		{
			name:    "unparsable duration",
			env:     with(validEnv(), "SPENDULUM_DB_TIMEOUT", "soon"),
			wantVar: "SPENDULUM_DB_TIMEOUT",
		},
		{
			name:    "zero shutdown timeout",
			env:     with(validEnv(), "SPENDULUM_SHUTDOWN_TIMEOUT", "0s"),
			wantVar: "SPENDULUM_SHUTDOWN_TIMEOUT",
		},
		{
			name:    "unknown log level",
			env:     with(validEnv(), "SPENDULUM_LOG_LEVEL", "loud"),
			wantVar: "SPENDULUM_LOG_LEVEL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.Load(tt.env)
			if err == nil {
				t.Fatal("Load() succeeded, want an error")
			}

			var varErr *config.VarError
			if !errors.As(err, &varErr) || varErr.Var != tt.wantVar {
				t.Errorf("Load() error = %v, want a VarError for %s", err, tt.wantVar)
			}

			if !strings.Contains(err.Error(), tt.wantVar) {
				t.Errorf("error message %q does not name %s", err, tt.wantVar)
			}
		})
	}
}

func with(env map[string]string, key, value string) map[string]string {
	env[key] = value

	return env
}
