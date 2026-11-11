package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	SessionLifetime   = 30 * 24 * time.Hour
	sessionRenewAfter = 15 * 24 * time.Hour
	MinPasswordLength = 12
)

var (
	ErrInvalidEmail       = errors.New("the email is not a valid address")
	ErrPasswordTooShort   = fmt.Errorf("the password must be at least %d characters", MinPasswordLength)
	ErrEmailTaken         = errors.New("a user with this email already exists")
	ErrUserNotFound       = errors.New("no user has this email")
	ErrInvalidCredentials = errors.New("email or password is incorrect")
	ErrNoSession          = errors.New("no valid session")
)

type TooManyAttemptsError struct {
	RetryAfter time.Duration
}

func (e *TooManyAttemptsError) Error() string {
	return fmt.Sprintf("too many failed logins; retry after %s", e.RetryAfter)
}

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
}

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
}

// A session issued or renewed more than 15 days ago has less than 15 days left.
func (s Session) dueForRenewal(now time.Time) bool {
	return s.ExpiresAt.Sub(now) < SessionLifetime-sessionRenewAfter
}

// Emails are unique after lowercasing (ADR 0009).
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))

	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidEmail
	}

	return email, nil
}

func CheckPassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}

	return nil
}

// The cookie carries the token; the database keeps only its hash.
func newToken() string {
	return rand.Text()
}

func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}

type sessionKey struct{}

func WithSession(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

func SessionFrom(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(Session)

	return s, ok
}
