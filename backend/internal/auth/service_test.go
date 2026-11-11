package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/auth"
)

func TestCreateAccountStoresTheEmailLowercasedAndCreatesTheDefaults(t *testing.T) {
	f := newFixture()

	id, err := f.svc.CreateAccount(t.Context(), "  Ana@Example.test ", "correct horse battery")
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	if u, ok := f.repo.users["ana@example.test"]; !ok || u.ID != id {
		t.Errorf("users = %+v, want ana@example.test with id %s", f.repo.users, id)
	}

	if id.Version() != 7 {
		t.Errorf("user id version = %d, want UUIDv7", id.Version())
	}

	if len(f.defaults.created) != 1 || f.defaults.created[0] != id {
		t.Errorf("defaults created for %v, want [%s]", f.defaults.created, id)
	}
}

func TestCreateAccountRejectsBadInputAndCreatesNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		email, password string
		want            error
	}{
		"password of 11 characters": {"ana@example.test", "elevenchars", auth.ErrPasswordTooShort},
		"password of 11 letters č":  {"ana@example.test", "čččččččččćć", auth.ErrPasswordTooShort},
		"not an email":              {"ana", "correct horse battery", auth.ErrInvalidEmail},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture()

			if _, err := f.svc.CreateAccount(t.Context(), tc.email, tc.password); !errors.Is(err, tc.want) {
				t.Errorf("CreateAccount = %v, want %v", err, tc.want)
			}

			if len(f.repo.users) != 0 || len(f.defaults.created) != 0 {
				t.Errorf("something was created: users %v, defaults %v", f.repo.users, f.defaults.created)
			}
		})
	}
}

func TestCreateAccountWithAnEmailThatDiffersOnlyInCaseIsTaken(t *testing.T) {
	f := newFixture()
	if _, err := f.svc.CreateAccount(t.Context(), "ana@example.test", "correct horse battery"); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.CreateAccount(t.Context(), "ANA@example.TEST", "correct horse battery"); !errors.Is(err, auth.ErrEmailTaken) {
		t.Errorf("CreateAccount = %v, want ErrEmailTaken", err)
	}
}

func TestLoginWithAnUnknownEmailStillRunsOneHash(t *testing.T) {
	f := newFixture()

	_, err := f.svc.Login(t.Context(), "nobody@example.test", "whatever password", "192.0.2.1")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("Login = %v, want ErrInvalidCredentials", err)
	}

	if f.hasher.verifies != 1 {
		t.Errorf("hashes run = %d, want 1", f.hasher.verifies)
	}
}

func TestLoginWithAWrongPasswordIsInvalidCredentials(t *testing.T) {
	f := newFixture()
	f.account(t)

	if _, err := f.svc.Login(t.Context(), "ana@example.test", "wrong password!", "192.0.2.1"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("Login = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginStartsA30DaySessionForAnyLetterCaseOfTheEmail(t *testing.T) {
	f := newFixture()
	id := f.account(t)

	login, err := f.svc.Login(t.Context(), "ANA@example.test", "correct horse battery", "192.0.2.1")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if login.Session.UserID != id || !login.Session.ExpiresAt.Equal(f.now.Add(30*24*time.Hour)) {
		t.Errorf("session = %+v, want user %s expiring 30 days from now", login.Session, id)
	}

	if login.Session.ID.Version() != 7 {
		t.Errorf("session id version = %d, want UUIDv7", login.Session.ID.Version())
	}

	got, renewed, err := f.svc.Authenticate(t.Context(), login.Token)
	if err != nil || renewed || got != login.Session {
		t.Errorf("Authenticate = %+v, %v, %v; want the new session, not renewed", got, renewed, err)
	}
}

func TestSixthLoginAfterFiveFailuresIsRefusedEvenWithTheRightPassword(t *testing.T) {
	f := newFixture()
	f.account(t)

	for range 5 {
		_, _ = f.svc.Login(t.Context(), "ana@example.test", "wrong password!", "192.0.2.1")
	}

	f.now = f.now.Add(5 * time.Minute)
	f.hasher.verifies = 0

	_, err := f.svc.Login(t.Context(), "Ana@example.test", "correct horse battery", "192.0.2.1")

	var tooMany *auth.TooManyAttemptsError
	if !errors.As(err, &tooMany) || tooMany.RetryAfter != 10*time.Minute {
		t.Fatalf("Login = %v, want TooManyAttemptsError with RetryAfter 10m", err)
	}

	if f.hasher.verifies != 0 {
		t.Errorf("a refused attempt ran %d hashes, want 0", f.hasher.verifies)
	}

	if _, err := f.svc.Login(t.Context(), "ana@example.test", "correct horse battery", "198.51.100.7"); err != nil {
		t.Errorf("login from another IP = %v, want success", err)
	}
}

func TestASuccessfulLoginDoesNotForgetEarlierFailures(t *testing.T) {
	f := newFixture()
	f.account(t)

	for range 4 {
		_, _ = f.svc.Login(t.Context(), "ana@example.test", "wrong password!", "192.0.2.1")
	}

	if _, err := f.svc.Login(t.Context(), "ana@example.test", "correct horse battery", "192.0.2.1"); err != nil {
		t.Fatalf("login after 4 failures = %v, want success", err)
	}

	_, _ = f.svc.Login(t.Context(), "ana@example.test", "wrong password!", "192.0.2.1")

	var tooMany *auth.TooManyAttemptsError
	if _, err := f.svc.Login(t.Context(), "ana@example.test", "correct horse battery", "192.0.2.1"); !errors.As(err, &tooMany) {
		t.Errorf("login after 5 failures within 15 minutes = %v, want TooManyAttemptsError", err)
	}
}

func TestSessionIsRenewedTo30DaysFromNowOnlyOnceOlderThan15Days(t *testing.T) {
	f := newFixture()
	f.account(t)

	login, err := f.svc.Login(t.Context(), "ana@example.test", "correct horse battery", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}

	f.now = f.now.Add(15 * 24 * time.Hour)
	if _, renewed, _ := f.svc.Authenticate(t.Context(), login.Token); renewed {
		t.Error("a session exactly 15 days old was renewed")
	}

	f.now = f.now.Add(time.Hour)

	s, renewed, err := f.svc.Authenticate(t.Context(), login.Token)
	if err != nil || !renewed || !s.ExpiresAt.Equal(f.now.Add(30*24*time.Hour)) {
		t.Fatalf("Authenticate = %+v, %v, %v; want renewed to 30 days from now", s, renewed, err)
	}

	for _, stored := range f.repo.sessions {
		if stored.ID == s.ID && !stored.ExpiresAt.Equal(s.ExpiresAt) {
			t.Errorf("stored expiry = %s, want the renewed %s", stored.ExpiresAt, s.ExpiresAt)
		}
	}
}

func TestExpiredOrUnknownTokenHasNoSession(t *testing.T) {
	f := newFixture()
	f.account(t)

	login, _ := f.svc.Login(t.Context(), "ana@example.test", "correct horse battery", "192.0.2.1")
	f.now = f.now.Add(30 * 24 * time.Hour)

	for name, token := range map[string]string{"expired": login.Token, "unknown": "nope", "empty": ""} {
		if _, _, err := f.svc.Authenticate(t.Context(), token); !errors.Is(err, auth.ErrNoSession) {
			t.Errorf("%s token: Authenticate = %v, want ErrNoSession", name, err)
		}
	}
}

func TestResetPasswordChangesThePasswordAndRevokesEverySession(t *testing.T) {
	f := newFixture()
	f.account(t)

	old, _ := f.svc.Login(t.Context(), "ana@example.test", "correct horse battery", "192.0.2.1")

	if _, err := f.svc.ResetPassword(t.Context(), "ANA@example.test", "a brand new password"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	if _, _, err := f.svc.Authenticate(t.Context(), old.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("old session after reset: %v, want ErrNoSession", err)
	}

	if _, err := f.svc.Login(t.Context(), "ana@example.test", "correct horse battery", "192.0.2.1"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("login with the old password = %v, want ErrInvalidCredentials", err)
	}

	if _, err := f.svc.Login(t.Context(), "ana@example.test", "a brand new password", "192.0.2.1"); err != nil {
		t.Errorf("login with the new password = %v", err)
	}
}

func TestResetPasswordOfAnUnknownUserOrWithAShortPasswordFails(t *testing.T) {
	f := newFixture()
	f.account(t)

	if _, err := f.svc.ResetPassword(t.Context(), "bob@example.test", "a brand new password"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("unknown user: %v, want ErrUserNotFound", err)
	}

	if _, err := f.svc.ResetPassword(t.Context(), "ana@example.test", "short"); !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Errorf("short password: %v, want ErrPasswordTooShort", err)
	}
}

func TestLogoutDeletesOnlyThatSession(t *testing.T) {
	f := newFixture()
	f.account(t)

	phone, _ := f.svc.Login(t.Context(), "ana@example.test", "correct horse battery", "192.0.2.1")
	desktop, _ := f.svc.Login(t.Context(), "ana@example.test", "correct horse battery", "192.0.2.1")

	if err := f.svc.Logout(t.Context(), phone.Session); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if _, _, err := f.svc.Authenticate(t.Context(), phone.Token); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("logged-out session: %v, want ErrNoSession", err)
	}

	if _, _, err := f.svc.Authenticate(t.Context(), desktop.Token); err != nil {
		t.Errorf("other session: %v, want it still valid", err)
	}
}

type fixture struct {
	svc      *auth.Service
	repo     *fakeRepo
	hasher   *fakeHasher
	defaults *fakeDefaults
	now      time.Time
}

func newFixture() *fixture {
	f := &fixture{repo: newFakeRepo(), hasher: &fakeHasher{}, defaults: &fakeDefaults{}, now: t0()}
	f.svc = auth.NewService(auth.Deps{
		Repo:     f.repo,
		Tx:       fakeTx{},
		Hasher:   f.hasher,
		Limiter:  auth.NewLimiter(),
		Defaults: []auth.AccountDefaults{f.defaults},
		Now:      func() time.Time { return f.now },
	})

	return f
}

func (f *fixture) account(t *testing.T) uuid.UUID {
	t.Helper()

	id, err := f.svc.CreateAccount(t.Context(), "ana@example.test", "correct horse battery")
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	return id
}

type fakeTx struct{}

func (fakeTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// fakeHasher "hashes" by prefixing, and counts verifications.
type fakeHasher struct{ verifies int }

func (*fakeHasher) Hash(_ context.Context, pw string) (string, error) { return "hash:" + pw, nil }

func (h *fakeHasher) Verify(_ context.Context, hash, pw string) (bool, error) {
	h.verifies++

	return hash == "hash:"+pw, nil
}

type fakeDefaults struct{ created []uuid.UUID }

func (d *fakeDefaults) CreateDefaults(_ context.Context, userID uuid.UUID) error {
	d.created = append(d.created, userID)

	return nil
}

type fakeRepo struct {
	mu       sync.Mutex
	users    map[string]auth.User
	sessions map[string]auth.Session
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{users: map[string]auth.User{}, sessions: map[string]auth.Session{}}
}

func (r *fakeRepo) CreateUser(_ context.Context, u auth.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.users[u.Email]; ok {
		return auth.ErrEmailTaken
	}

	r.users[u.Email] = u

	return nil
}

func (r *fakeRepo) UserByEmail(_ context.Context, email string) (auth.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.users[email]
	if !ok {
		return auth.User{}, auth.ErrUserNotFound
	}

	return u, nil
}

func (r *fakeRepo) UserEmail(_ context.Context, userID uuid.UUID) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, u := range r.users {
		if u.ID == userID {
			return u.Email, nil
		}
	}

	return "", auth.ErrUserNotFound
}

func (r *fakeRepo) SetPasswordHash(_ context.Context, userID uuid.UUID, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for email, u := range r.users {
		if u.ID == userID {
			u.PasswordHash = hash
			r.users[email] = u

			return nil
		}
	}

	return auth.ErrUserNotFound
}

func (r *fakeRepo) CreateSession(_ context.Context, userID uuid.UUID, s auth.Session, tokenHash []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s.UserID = userID
	r.sessions[string(tokenHash)] = s

	return nil
}

func (r *fakeRepo) SessionByTokenHash(_ context.Context, tokenHash []byte, now time.Time) (auth.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[string(tokenHash)]
	if !ok || !s.ExpiresAt.After(now) {
		return auth.Session{}, auth.ErrNoSession
	}

	return s, nil
}

func (r *fakeRepo) RenewSession(_ context.Context, userID, id uuid.UUID, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for k, s := range r.sessions {
		if s.UserID == userID && s.ID == id {
			s.ExpiresAt = expiresAt
			r.sessions[k] = s
		}
	}

	return nil
}

func (r *fakeRepo) DeleteSession(_ context.Context, userID, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for k, s := range r.sessions {
		if s.UserID == userID && s.ID == id {
			delete(r.sessions, k)
		}
	}

	return nil
}

func (r *fakeRepo) DeleteUserSessions(_ context.Context, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for k, s := range r.sessions {
		if s.UserID == userID {
			delete(r.sessions, k)
		}
	}

	return nil
}
