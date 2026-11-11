package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	CreateUser(ctx context.Context, u User) error
	UserByEmail(ctx context.Context, email string) (User, error)
	UserEmail(ctx context.Context, userID uuid.UUID) (string, error)
	SetPasswordHash(ctx context.Context, userID uuid.UUID, hash string) error
	// CreateSession answers ErrInvalidCredentials when the password hash is no longer verifiedHash.
	CreateSession(ctx context.Context, userID uuid.UUID, s Session, tokenHash []byte, verifiedHash string) error
	SessionByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) (Session, error)
	RenewSession(ctx context.Context, userID, sessionID uuid.UUID, expiresAt time.Time) error
	DeleteSession(ctx context.Context, userID, sessionID uuid.UUID) error
	DeleteUserSessions(ctx context.Context, userID uuid.UUID) error
}

type PasswordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
	Verify(ctx context.Context, hash, password string) (bool, error)
}

type AccountDefaults interface {
	CreateDefaults(ctx context.Context, userID uuid.UUID) error
}

type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type Deps struct {
	Repo     Repository
	Tx       Transactor
	Hasher   PasswordHasher
	Limiter  *Limiter
	Defaults []AccountDefaults
	Now      func() time.Time
}

type Service struct {
	d Deps
}

func NewService(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}

	return &Service{d: d}
}

type StartedSession struct {
	Token   string
	Session Session
}

// CreateAccount creates the user, their default settings and the Other categories in one transaction.
func (s *Service) CreateAccount(ctx context.Context, email, password string) (uuid.UUID, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return uuid.Nil, err
	}

	if err := checkPassword(password); err != nil {
		return uuid.Nil, err
	}

	hash, err := s.d.Hasher.Hash(ctx, password)
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: hash password: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: new id: %w", err)
	}

	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Repo.CreateUser(ctx, User{ID: id, Email: email, PasswordHash: hash}); err != nil {
			return err //nolint:wrapcheck // ErrEmailTaken must reach the caller as is
		}

		for _, d := range s.d.Defaults {
			if err := d.CreateDefaults(ctx, id); err != nil {
				return err //nolint:wrapcheck // each feature names itself in its error
			}
		}

		return nil
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: create account: %w", err)
	}

	return id, nil
}

func (s *Service) ResetPassword(ctx context.Context, email, password string) (uuid.UUID, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return uuid.Nil, ErrUserNotFound
	}

	if err := checkPassword(password); err != nil {
		return uuid.Nil, err
	}

	user, err := s.d.Repo.UserByEmail(ctx, email)
	if err != nil {
		return uuid.Nil, err //nolint:wrapcheck // ErrUserNotFound must reach the caller as is
	}

	hash, err := s.d.Hasher.Hash(ctx, password)
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: hash password: %w", err)
	}

	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Repo.SetPasswordHash(ctx, user.ID, hash); err != nil {
			return err //nolint:wrapcheck // wrapped below
		}

		return s.d.Repo.DeleteUserSessions(ctx, user.ID)
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: reset password: %w", err)
	}

	return user.ID, nil
}

// A wrong password and an unknown email both hash once and answer ErrInvalidCredentials, so neither tells them apart.
func (s *Service) Login(ctx context.Context, email, password, ip string) (StartedSession, error) {
	now := s.d.Now()
	user := User{PasswordHash: dummyHash}

	normalized, emailErr := normalizeEmail(email)
	if emailErr != nil {
		normalized = email
	}

	key := string(hashToken(normalized + "\x00" + ip))
	if wait := s.d.Limiter.Attempt(key, now); wait > 0 {
		return StartedSession{}, &TooManyAttemptsError{RetryAfter: wait}
	}

	if emailErr == nil {
		found, err := s.d.Repo.UserByEmail(ctx, normalized)

		switch {
		case err == nil:
			user = found
		case !errors.Is(err, ErrUserNotFound):
			return StartedSession{}, fmt.Errorf("auth: find user: %w", err)
		}
	}

	ok, err := s.d.Hasher.Verify(ctx, user.PasswordHash, password)
	if err != nil {
		return StartedSession{}, fmt.Errorf("auth: verify password: %w", err)
	}

	if !ok || user.ID == uuid.Nil {
		return StartedSession{}, ErrInvalidCredentials
	}

	id, err := uuid.NewV7()
	if err != nil {
		return StartedSession{}, fmt.Errorf("auth: new id: %w", err)
	}

	started := StartedSession{Token: newToken(), Session: Session{ID: id, UserID: user.ID, ExpiresAt: now.Add(SessionLifetime)}}
	if err := s.d.Repo.CreateSession(ctx, user.ID, started.Session, hashToken(started.Token), user.PasswordHash); err != nil {
		return StartedSession{}, fmt.Errorf("auth: create session: %w", err)
	}

	s.d.Limiter.Release(key, now)

	return started, nil
}

// Authenticate also renews a session once it is older than 15 days, and reports whether it did.
func (s *Service) Authenticate(ctx context.Context, token string) (Session, bool, error) {
	if token == "" {
		return Session{}, false, ErrNoSession
	}

	now := s.d.Now()

	session, err := s.d.Repo.SessionByTokenHash(ctx, hashToken(token), now)
	if err != nil {
		return Session{}, false, err //nolint:wrapcheck // ErrNoSession must reach the caller as is
	}

	if !session.dueForRenewal(now) {
		return session, false, nil
	}

	session.ExpiresAt = now.Add(SessionLifetime)
	if err := s.d.Repo.RenewSession(ctx, session.UserID, session.ID, session.ExpiresAt); err != nil {
		return Session{}, false, fmt.Errorf("auth: renew session: %w", err)
	}

	return session, true, nil
}

func (s *Service) Logout(ctx context.Context, session Session) error {
	if err := s.d.Repo.DeleteSession(ctx, session.UserID, session.ID); err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}

	return nil
}

func (s *Service) Email(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.d.Repo.UserEmail(ctx, userID) //nolint:wrapcheck // the repository already says what failed
}
