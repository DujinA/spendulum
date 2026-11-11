package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/platform/db/dbtest"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

func TestCreateUserWithATakenEmailIsErrEmailTaken(t *testing.T) {
	repo := auth.NewPostgres(dbtest.Open(t).Queries())

	if err := repo.CreateUser(t.Context(), newUser("ana@example.test")); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := repo.CreateUser(t.Context(), newUser("ana@example.test")); !errors.Is(err, auth.ErrEmailTaken) {
		t.Errorf("second CreateUser = %v, want ErrEmailTaken", err)
	}
}

func TestUserByEmailReadsTheUserBackOrIsErrUserNotFound(t *testing.T) {
	repo := auth.NewPostgres(dbtest.Open(t).Queries())
	u := newUser("ana@example.test")

	if err := repo.CreateUser(t.Context(), u); err != nil {
		t.Fatal(err)
	}

	if got, err := repo.UserByEmail(t.Context(), "ana@example.test"); err != nil || got != u {
		t.Errorf("UserByEmail = %+v, %v; want %+v", got, err, u)
	}

	if _, err := repo.UserByEmail(t.Context(), "bob@example.test"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("unknown email: %v, want ErrUserNotFound", err)
	}
}

func TestSessionIsFoundByTokenHashUntilItExpires(t *testing.T) {
	repo := auth.NewPostgres(dbtest.Open(t).Queries())
	u := newUser("ana@example.test")
	_ = repo.CreateUser(t.Context(), u)

	s := auth.Session{ID: uuid.Must(uuid.NewV7()), UserID: u.ID, ExpiresAt: t0().Add(time.Hour)}
	if err := repo.CreateSession(t.Context(), u.ID, s, []byte("hash"), u.PasswordHash); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := repo.SessionByTokenHash(t.Context(), []byte("hash"), t0())
	if err != nil || got.ID != s.ID || got.UserID != s.UserID || !got.ExpiresAt.Equal(s.ExpiresAt) {
		t.Errorf("SessionByTokenHash = %+v, %v; want %+v", got, err, s)
	}

	if _, err := repo.SessionByTokenHash(t.Context(), []byte("hash"), t0().Add(time.Hour)); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("at expiry: %v, want ErrNoSession", err)
	}
}

func TestOneUserNeverRenewsOrDeletesAnotherUsersSessions(t *testing.T) {
	repo := auth.NewPostgres(dbtest.Open(t).Queries())
	a, b := newUser("ana@example.test"), newUser("bob@example.test")
	_ = repo.CreateUser(t.Context(), a)
	_ = repo.CreateUser(t.Context(), b)

	aSession := auth.Session{ID: uuid.Must(uuid.NewV7()), UserID: a.ID, ExpiresAt: t0().Add(time.Hour)}
	if err := repo.CreateSession(t.Context(), a.ID, aSession, []byte("a"), a.PasswordHash); err != nil {
		t.Fatal(err)
	}

	_ = repo.RenewSession(t.Context(), b.ID, aSession.ID, t0().Add(48*time.Hour))
	_ = repo.DeleteSession(t.Context(), b.ID, aSession.ID)
	_ = repo.DeleteUserSessions(t.Context(), b.ID)

	got, err := repo.SessionByTokenHash(t.Context(), []byte("a"), t0())
	if err != nil {
		t.Fatalf("A's session is gone after B's deletes: %v", err)
	}

	if !got.ExpiresAt.Equal(aSession.ExpiresAt) {
		t.Errorf("A's expiry = %s after B's renew, want %s", got.ExpiresAt, aSession.ExpiresAt)
	}

	if err := repo.SetPasswordHash(t.Context(), b.ID, "b's hash"); err != nil {
		t.Fatal(err)
	}

	if got, _ := repo.UserByEmail(t.Context(), a.Email); got.PasswordHash != a.PasswordHash {
		t.Error("setting B's password changed A's")
	}

	if email, _ := repo.UserEmail(t.Context(), b.ID); email != b.Email {
		t.Errorf("B's email = %q, want %q", email, b.Email)
	}
}

func TestCreateSessionAfterThePasswordChangedStartsNoSession(t *testing.T) {
	repo := auth.NewPostgres(dbtest.Open(t).Queries())
	u := newUser("ana@example.test")
	_ = repo.CreateUser(t.Context(), u)

	if err := repo.SetPasswordHash(t.Context(), u.ID, "new hash"); err != nil {
		t.Fatal(err)
	}

	s := auth.Session{ID: uuid.Must(uuid.NewV7()), UserID: u.ID, ExpiresAt: t0().Add(time.Hour)}
	if err := repo.CreateSession(t.Context(), u.ID, s, []byte("hash"), u.PasswordHash); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("CreateSession with the old hash = %v, want ErrInvalidCredentials", err)
	}

	if _, err := repo.SessionByTokenHash(t.Context(), []byte("hash"), t0()); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("a session exists after the password changed: %v", err)
	}
}

// The login verified the old password before the reset; its session must not outlive the reset.
func TestALoginRacingAPasswordResetStartsNoSession(t *testing.T) {
	d := dbtest.Open(t)
	repo := auth.NewPostgres(d.Queries())
	u := newUser("ana@example.test")
	_ = repo.CreateUser(t.Context(), u)

	locked, resetDone := make(chan struct{}), make(chan error, 1)
	release := make(chan struct{})
	releaseReset := sync.OnceFunc(func() { close(release) })
	// Registered after dbtest.Open, so it runs before the pool closes: an open transaction would block Close.
	t.Cleanup(releaseReset)

	go func() {
		resetDone <- d.InTx(t.Context(), func(ctx context.Context) error {
			if err := repo.SetPasswordHash(ctx, u.ID, "new hash"); err != nil {
				return err
			}

			if err := repo.DeleteUserSessions(ctx, u.ID); err != nil {
				return err
			}

			close(locked)
			<-release

			return nil
		})
	}()

	<-locked

	s := auth.Session{ID: uuid.Must(uuid.NewV7()), UserID: u.ID, ExpiresAt: t0().Add(time.Hour)}
	loginDone := make(chan error, 1)

	go func() { loginDone <- repo.CreateSession(t.Context(), u.ID, s, []byte("hash"), u.PasswordHash) }()

	select {
	case err := <-loginDone:
		t.Fatalf("CreateSession finished while the reset was in flight: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	releaseReset()

	if err := <-resetDone; err != nil {
		t.Fatalf("reset: %v", err)
	}

	if err := <-loginDone; !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("CreateSession after the reset committed = %v, want ErrInvalidCredentials", err)
	}

	if _, err := repo.SessionByTokenHash(t.Context(), []byte("hash"), t0()); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("a session survived the reset: %v", err)
	}
}

func newUser(email string) auth.User {
	return auth.User{ID: uuid.Must(uuid.NewV7()), Email: email, PasswordHash: "hash of " + email}
}
