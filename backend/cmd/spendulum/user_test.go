package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/platform/db"
	"github.com/DujinA/spendulum/backend/internal/platform/db/dbtest"
	"github.com/DujinA/spendulum/backend/internal/settings"
)

const goodPassword = "correct horse battery"

func TestUserCreateCreatesTheUserWithDefaultSettingsAndOtherCategories(t *testing.T) {
	url := dbtest.NewDatabase(t)

	out, code := runUser(t, url, typed(goodPassword, goodPassword), "create", "--email", "Ana@Example.test")
	if code != 0 {
		t.Fatalf("user create exited %d:\n%s", code, out)
	}

	d := openDB(t, url)

	user, err := auth.NewPostgres(d.Queries()).UserByEmail(t.Context(), "ana@example.test")
	if err != nil {
		t.Fatalf("no user ana@example.test: %v", err)
	}

	if user.ID.Version() != 7 {
		t.Errorf("user id version = %d, want UUIDv7", user.ID.Version())
	}

	if !strings.Contains(out, user.ID.String()) || strings.Contains(strings.ToLower(out), "ana@example.test") {
		t.Errorf("output should name the user_id and never the email:\n%s", out)
	}

	got, err := settings.NewPostgres(d.Queries()).Get(t.Context(), user.ID)
	if err != nil || got != (settings.Settings{PrimaryCurrency: "RSD", TimeZone: "Europe/Belgrade"}) {
		t.Errorf("settings = %+v, %v; want RSD and Europe/Belgrade", got, err)
	}

	cats, err := category.NewPostgres(d.Queries()).List(t.Context(), user.ID)
	if err != nil || len(cats) != 2 {
		t.Fatalf("categories = %+v, %v; want one Other per kind", cats, err)
	}

	for i, kind := range []category.Kind{category.Expense, category.Income} {
		if cats[i].Kind != kind || cats[i].Name != "Other" || cats[i].ID.Version() != 7 {
			t.Errorf("category %d = %+v, want %s Other with a UUIDv7", i, cats[i], kind)
		}
	}
}

func TestUserCreateWithABadPasswordFailsAndCreatesNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		stdin, want string
	}{
		"11 characters":     {typed("elevenchars", "elevenchars"), "at least 12 characters"},
		"entries differ":    {typed(goodPassword, goodPassword+"!"), "do not match"},
		"no second entry":   {goodPassword + "\n", "read password"},
		"nothing was typed": {"", "read password"},
	} {
		t.Run(name, func(t *testing.T) {
			url := dbtest.NewDatabase(t)

			out, code := runUser(t, url, tc.stdin, "create", "--email", "ana@example.test")
			if code == 0 {
				t.Fatalf("user create exited 0:\n%s", out)
			}

			if !strings.Contains(out, tc.want) {
				t.Errorf("output does not say %q:\n%s", tc.want, out)
			}

			if _, err := auth.NewPostgres(openDB(t, url).Queries()).UserByEmail(t.Context(), "ana@example.test"); !errors.Is(err, auth.ErrUserNotFound) {
				t.Errorf("a user was created: %v", err)
			}
		})
	}
}

func TestUserCreateWithAnEmailThatMatchesInAnyCaseFails(t *testing.T) {
	url := dbtest.NewDatabase(t)

	if out, code := runUser(t, url, typed(goodPassword, goodPassword), "create", "--email", "ana@example.test"); code != 0 {
		t.Fatalf("first user create exited %d:\n%s", code, out)
	}

	out, code := runUser(t, url, typed(goodPassword, goodPassword), "create", "--email", "ANA@example.Test")
	if code == 0 {
		t.Fatalf("second user create exited 0:\n%s", out)
	}

	if !strings.Contains(out, "a user with this email already exists") {
		t.Errorf("output does not say the email is taken:\n%s", out)
	}
}

func TestUserCreateTakesThePasswordOnlyAtThePrompt(t *testing.T) {
	url := dbtest.NewDatabase(t)

	for _, args := range [][]string{
		{"create"},
		{"create", "--email", "ana@example.test", "--password", goodPassword},
		{"create", "--email", "ana@example.test", "extra"},
		{"rename"},
		{},
	} {
		if out, code := runUser(t, url, typed(goodPassword, goodPassword), args...); code == 0 {
			t.Errorf("user %v exited 0:\n%s", args, out)
		}
	}
}

func TestUserResetPasswordChangesThePasswordAndRevokesEverySession(t *testing.T) {
	url := dbtest.NewDatabase(t)

	if out, code := runUser(t, url, typed(goodPassword, goodPassword), "create", "--email", "ana@example.test"); code != 0 {
		t.Fatalf("user create exited %d:\n%s", code, out)
	}

	svc := newAuthService(openDB(t, url))

	phone, err := svc.Login(t.Context(), "ana@example.test", goodPassword, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}

	desktop, _ := svc.Login(t.Context(), "ana@example.test", goodPassword, "192.0.2.2")

	newPassword := "a brand new password"
	if out, code := runUser(t, url, typed(newPassword, newPassword), "reset-password", "--user", "Ana@example.test"); code != 0 {
		t.Fatalf("user reset-password exited %d:\n%s", code, out)
	}

	for _, l := range []auth.Login{phone, desktop} {
		if _, _, err := svc.Authenticate(t.Context(), l.Token); !errors.Is(err, auth.ErrNoSession) {
			t.Errorf("session after reset: %v, want ErrNoSession", err)
		}
	}

	if _, err := svc.Login(t.Context(), "ana@example.test", goodPassword, "192.0.2.3"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("old password: %v, want ErrInvalidCredentials", err)
	}

	if _, err := svc.Login(t.Context(), "ana@example.test", newPassword, "192.0.2.3"); err != nil {
		t.Errorf("new password: %v", err)
	}
}

func TestUserResetPasswordOfAnUnknownUserOrWithDifferentEntriesFails(t *testing.T) {
	url := dbtest.NewDatabase(t)

	if out, code := runUser(t, url, typed(goodPassword, goodPassword), "create", "--email", "ana@example.test"); code != 0 {
		t.Fatalf("user create exited %d:\n%s", code, out)
	}

	for name, tc := range map[string]struct {
		user, stdin, want string
	}{
		"unknown user":   {"bob@example.test", typed(goodPassword, goodPassword), "no user has this email"},
		"entries differ": {"ana@example.test", typed("a brand new password", "a brand new passwort"), "do not match"},
	} {
		t.Run(name, func(t *testing.T) {
			out, code := runUser(t, url, tc.stdin, "reset-password", "--user", tc.user)
			if code == 0 || !strings.Contains(out, tc.want) {
				t.Errorf("exit %d, output without %q:\n%s", code, tc.want, out)
			}
		})
	}
}

func runUser(t *testing.T, url, stdin string, args ...string) (string, int) {
	t.Helper()

	var out syncBuffer

	code := run(t.Context(), append([]string{"user"}, args...), map[string]string{"SPENDULUM_DATABASE_URL": url}, strings.NewReader(stdin), &out)

	return out.String(), code
}

func typed(first, second string) string {
	return first + "\n" + second + "\n"
}

func openDB(t *testing.T, url string) *db.DB {
	t.Helper()

	d, err := db.Open(t.Context(), url, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(d.Close)

	return d
}
