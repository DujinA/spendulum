package settings_test

import (
	"errors"
	"testing"

	"github.com/DujinA/spendulum/backend/internal/platform/db/dbtest"
	"github.com/DujinA/spendulum/backend/internal/settings"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

func TestSettingsAreReadBackForTheirUser(t *testing.T) {
	d := dbtest.Open(t)
	repo := settings.NewPostgres(d.Queries())
	user := dbtest.NewUser(t, d.Queries())
	want := settings.Settings{PrimaryCurrency: "EUR", TimeZone: "Europe/Vienna"}

	if err := repo.Create(t.Context(), user, want); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(t.Context(), user)
	if err != nil || got != want {
		t.Errorf("Get = %+v, %v; want %+v", got, err, want)
	}
}

func TestOneUserNeverReadsAnotherUsersSettings(t *testing.T) {
	d := dbtest.Open(t)
	repo := settings.NewPostgres(d.Queries())
	a, b := dbtest.NewUser(t, d.Queries()), dbtest.NewUser(t, d.Queries())

	if err := repo.Create(t.Context(), a, settings.Defaults()); err != nil {
		t.Fatalf("Create for A: %v", err)
	}

	if _, err := repo.Get(t.Context(), b); !errors.Is(err, settings.ErrNotFound) {
		t.Errorf("B's Get with only A's settings stored = %v, want ErrNotFound", err)
	}

	bs := settings.Settings{PrimaryCurrency: "USD", TimeZone: "America/New_York"}
	if err := repo.Create(t.Context(), b, bs); err != nil {
		t.Fatalf("Create for B: %v", err)
	}

	if got, _ := repo.Get(t.Context(), b); got != bs {
		t.Errorf("B reads %+v, want B's own %+v", got, bs)
	}

	if got, _ := repo.Get(t.Context(), a); got != settings.Defaults() {
		t.Errorf("A reads %+v, want A's own %+v", got, settings.Defaults())
	}
}
