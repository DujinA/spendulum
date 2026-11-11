package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/DujinA/spendulum/backend/internal/platform/db/gen"
)

const uniqueViolation = "23505"

type Postgres struct {
	q *gen.Queries
}

func NewPostgres(q *gen.Queries) *Postgres {
	return &Postgres{q: q}
}

func (p *Postgres) CreateUser(ctx context.Context, u User) error {
	err := p.q.CreateUser(ctx, gen.CreateUserParams{ID: u.ID, Email: u.Email, PasswordHash: u.PasswordHash})
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == uniqueViolation {
		return ErrEmailTaken
	}

	if err != nil {
		return fmt.Errorf("auth: insert user: %w", err)
	}

	return nil
}

func (p *Postgres) UserByEmail(ctx context.Context, email string) (User, error) {
	row, err := p.q.UserByEmail(ctx, email)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return User{}, ErrUserNotFound
	case err != nil:
		return User{}, fmt.Errorf("auth: select user: %w", err)
	}

	return User{ID: row.ID, Email: row.Email, PasswordHash: row.PasswordHash}, nil
}

func (p *Postgres) UserEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	email, err := p.q.UserEmail(ctx, userID)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", ErrUserNotFound
	case err != nil:
		return "", fmt.Errorf("auth: select user email: %w", err)
	}

	return email, nil
}

func (p *Postgres) SetPasswordHash(ctx context.Context, userID uuid.UUID, hash string) error {
	n, err := p.q.SetPasswordHash(ctx, gen.SetPasswordHashParams{ID: userID, PasswordHash: hash})

	switch {
	case err != nil:
		return fmt.Errorf("auth: update password hash: %w", err)
	case n == 0:
		return ErrUserNotFound
	}

	return nil
}

func (p *Postgres) CreateSession(ctx context.Context, userID uuid.UUID, s Session, tokenHash []byte) error {
	err := p.q.CreateSession(ctx, gen.CreateSessionParams{ID: s.ID, UserID: userID, TokenHash: tokenHash, ExpiresAt: s.ExpiresAt})
	if err != nil {
		return fmt.Errorf("auth: insert session: %w", err)
	}

	return nil
}

func (p *Postgres) SessionByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) (Session, error) {
	row, err := p.q.SessionByTokenHash(ctx, gen.SessionByTokenHashParams{TokenHash: tokenHash, ExpiresAt: now})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Session{}, ErrNoSession
	case err != nil:
		return Session{}, fmt.Errorf("auth: select session: %w", err)
	}

	return Session{ID: row.ID, UserID: row.UserID, ExpiresAt: row.ExpiresAt}, nil
}

func (p *Postgres) RenewSession(ctx context.Context, userID, sessionID uuid.UUID, expiresAt time.Time) error {
	if err := p.q.RenewSession(ctx, gen.RenewSessionParams{UserID: userID, ID: sessionID, ExpiresAt: expiresAt}); err != nil {
		return fmt.Errorf("auth: renew session: %w", err)
	}

	return nil
}

func (p *Postgres) DeleteSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	if err := p.q.DeleteSession(ctx, gen.DeleteSessionParams{UserID: userID, ID: sessionID}); err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}

	return nil
}

func (p *Postgres) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	if err := p.q.DeleteUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("auth: delete user sessions: %w", err)
	}

	return nil
}
