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
	CreateSession(ctx context.Context, s Session, tokenHash []byte) error
	SessionByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) (Session, error)
	RenewSession(ctx context.Context, userID, sessionID uuid.UUID, expiresAt time.Time) error
	DeleteSession(ctx context.Context, userID, sessionID uuid.UUID) error
	DeleteUserSessions(ctx context.Context, userID uuid.UUID) error
}

type PasswordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
	Verify(ctx context.Context, hash, password string) (bool, error)
}

// AccountDefaults creates the rows a new account starts with, such as settings and the Other categories.
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
	Deps
}

func NewService(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}

	return &Service{Deps: d}
}

type Login struct {
	Token   string
	Session Session
}

// CreateAccount is the create-account operation: the user, their default settings and the Other categories.
func (s *Service) CreateAccount(ctx context.Context, email, password string) (uuid.UUID, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return uuid.Nil, err
	}

	if err := CheckPassword(password); err != nil {
		return uuid.Nil, err
	}

	hash, err := s.Hasher.Hash(ctx, password)
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: hash password: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: new id: %w", err)
	}

	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.Repo.CreateUser(ctx, User{ID: id, Email: email, PasswordHash: hash}); err != nil {
			return err //nolint:wrapcheck // ErrEmailTaken must reach the caller as is
		}

		for _, d := range s.Defaults {
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

// ResetPassword sets a new password and revokes every session of the user.
func (s *Service) ResetPassword(ctx context.Context, email, password string) (uuid.UUID, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return uuid.Nil, ErrUserNotFound
	}

	if err := CheckPassword(password); err != nil {
		return uuid.Nil, err
	}

	user, err := s.Repo.UserByEmail(ctx, email)
	if err != nil {
		return uuid.Nil, err //nolint:wrapcheck // ErrUserNotFound must reach the caller as is
	}

	hash, err := s.Hasher.Hash(ctx, password)
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: hash password: %w", err)
	}

	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.Repo.SetPasswordHash(ctx, user.ID, hash); err != nil {
			return err //nolint:wrapcheck // wrapped below
		}

		return s.Repo.DeleteUserSessions(ctx, user.ID)
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: reset password: %w", err)
	}

	return user.ID, nil
}

// Login answers ErrInvalidCredentials for a wrong password and an unknown email alike,
// and hashes once in both cases, so neither the answer nor its timing tells them apart.
func (s *Service) Login(ctx context.Context, email, password, ip string) (Login, error) {
	now := s.Now()
	normalized, emailErr := NormalizeEmail(email)

	key := normalized + "\x00" + ip
	if emailErr != nil {
		key = email + "\x00" + ip
	}

	if wait := s.Limiter.RetryAfter(key, now); wait > 0 {
		return Login{}, &TooManyAttemptsError{RetryAfter: wait}
	}

	user, err := s.findUser(ctx, normalized, emailErr == nil)
	if err != nil {
		return Login{}, err
	}

	ok, err := s.Hasher.Verify(ctx, user.PasswordHash, password)
	if err != nil {
		return Login{}, fmt.Errorf("auth: verify password: %w", err)
	}

	if !ok || user.ID == uuid.Nil {
		s.Limiter.Fail(key, now)

		return Login{}, ErrInvalidCredentials
	}

	s.Limiter.Reset(key)

	id, err := uuid.NewV7()
	if err != nil {
		return Login{}, fmt.Errorf("auth: new id: %w", err)
	}

	login := Login{Token: newToken(), Session: Session{ID: id, UserID: user.ID, ExpiresAt: now.Add(SessionLifetime)}}
	if err := s.Repo.CreateSession(ctx, login.Session, HashToken(login.Token)); err != nil {
		return Login{}, fmt.Errorf("auth: create session: %w", err)
	}

	return login, nil
}

// An unknown user comes back with the dummy hash, so Login still runs one argon2id hash.
func (s *Service) findUser(ctx context.Context, email string, valid bool) (User, error) {
	if !valid {
		return User{PasswordHash: dummyHash}, nil
	}

	user, err := s.Repo.UserByEmail(ctx, email)

	switch {
	case errors.Is(err, ErrUserNotFound):
		return User{PasswordHash: dummyHash}, nil
	case err != nil:
		return User{}, fmt.Errorf("auth: find user: %w", err)
	}

	return user, nil
}

// Authenticate finds the session of a token and renews it once it is older than 15 days.
func (s *Service) Authenticate(ctx context.Context, token string) (Session, bool, error) {
	if token == "" {
		return Session{}, false, ErrNoSession
	}

	now := s.Now()

	session, err := s.Repo.SessionByTokenHash(ctx, HashToken(token), now)
	if err != nil {
		return Session{}, false, err //nolint:wrapcheck // ErrNoSession must reach the caller as is
	}

	if !session.dueForRenewal(now) {
		return session, false, nil
	}

	session.ExpiresAt = now.Add(SessionLifetime)
	if err := s.Repo.RenewSession(ctx, session.UserID, session.ID, session.ExpiresAt); err != nil {
		return Session{}, false, fmt.Errorf("auth: renew session: %w", err)
	}

	return session, true, nil
}

func (s *Service) Logout(ctx context.Context, session Session) error {
	if err := s.Repo.DeleteSession(ctx, session.UserID, session.ID); err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}

	return nil
}

func (s *Service) Email(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.Repo.UserEmail(ctx, userID) //nolint:wrapcheck // the repository already says what failed
}
