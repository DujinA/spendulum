package entry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/settings"
)

type Repository interface {
	DeleteExpiredKeys(ctx context.Context, userID uuid.UUID, createdBefore time.Time) error
	// Create answers ErrUnknownCategory when the user has no category with that id and kind.
	Create(ctx context.Context, userID uuid.UUID, e Entry) error
	// ClaimKey answers false when the user already holds the key.
	ClaimKey(ctx context.Context, userID uuid.UUID, key string, requestHash []byte, entryID uuid.UUID, at time.Time) (bool, error)
	Key(ctx context.Context, userID uuid.UUID, key string) (requestHash []byte, entryID uuid.UUID, err error)
	Get(ctx context.Context, userID, id uuid.UUID) (Entry, error)
	List(ctx context.Context, userID uuid.UUID, after *Cursor, limit int) ([]Entry, error)
}

type Vendors interface {
	Use(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error)
}

type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type Settings interface {
	Get(ctx context.Context, userID uuid.UUID) (settings.Settings, error)
}

type Deps struct {
	Repo     Repository
	Vendors  Vendors
	Tx       Transactor
	Settings Settings
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

var errKeyHeld = errors.New("entry: idempotency key already held")

// Create records the entry once per key: a retry with the same key and request answers the entry the first one created.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, key string, n NewEntry) (Entry, error) {
	n, err := n.normalized()
	if err != nil {
		return Entry{}, err
	}

	st, err := s.d.Settings.Get(ctx, userID)
	if err != nil {
		return Entry{}, fmt.Errorf("entry: read settings: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return Entry{}, fmt.Errorf("entry: new id: %w", err)
	}

	now := s.d.Now()
	hash := n.fingerprint()
	e := Entry{
		ID: id, Kind: n.Kind, Amount: n.Amount, Currency: st.PrimaryCurrency,
		CategoryID: n.CategoryID, EntryDate: n.EntryDate, Note: n.Note, CreatedAt: now,
	}

	// Inserting before claiming the key lets a concurrent retry wait on the key, then roll back its own insert.
	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Repo.DeleteExpiredKeys(ctx, userID, now.Add(-KeyLifetime)); err != nil {
			return err //nolint:wrapcheck // wrapped below
		}

		if n.VendorName != "" {
			vendorID, err := s.d.Vendors.Use(ctx, userID, n.VendorName)
			if err != nil {
				return err //nolint:wrapcheck // wrapped below
			}

			e.VendorID = vendorID
		}

		if err := s.d.Repo.Create(ctx, userID, e); err != nil {
			return err //nolint:wrapcheck // ErrUnknownCategory must reach the caller as is
		}

		claimed, err := s.d.Repo.ClaimKey(ctx, userID, key, hash, id, now)

		switch {
		case err != nil:
			return err //nolint:wrapcheck // wrapped below
		case !claimed:
			return errKeyHeld
		}

		return nil
	})

	switch {
	case errors.Is(err, errKeyHeld):
		return s.replay(ctx, userID, key, hash)
	case err != nil:
		return Entry{}, fmt.Errorf("entry: create: %w", err)
	}

	return s.Get(ctx, userID, id)
}

func (s *Service) replay(ctx context.Context, userID uuid.UUID, key string, hash []byte) (Entry, error) {
	held, entryID, err := s.d.Repo.Key(ctx, userID, key)
	if err != nil {
		return Entry{}, fmt.Errorf("entry: read idempotency key: %w", err)
	}

	if !bytes.Equal(held, hash) {
		return Entry{}, ErrKeyReused
	}

	return s.Get(ctx, userID, entryID)
}

func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Entry, error) {
	return s.d.Repo.Get(ctx, userID, id) //nolint:wrapcheck // ErrNotFound must reach the caller as is
}

// List answers one page in list order: newest entry date first, then newest created first.
func (s *Service) List(ctx context.Context, userID uuid.UUID, after *Cursor, limit int) (Page, error) {
	entries, err := s.d.Repo.List(ctx, userID, after, limit+1)
	if err != nil {
		return Page{}, fmt.Errorf("entry: list: %w", err)
	}

	if len(entries) <= limit {
		return Page{Entries: entries}, nil
	}

	last := entries[limit-1]

	return Page{
		Entries: entries[:limit],
		Next:    &Cursor{EntryDate: last.EntryDate, CreatedAt: last.CreatedAt, ID: last.ID},
	}, nil
}
