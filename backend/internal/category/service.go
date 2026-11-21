package category

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	// Create answers errNameTaken when the kind already has a category with that name key.
	Create(ctx context.Context, userID uuid.UUID, c Category) error
	List(ctx context.Context, userID uuid.UUID) ([]Category, error)
	ListUnarchived(ctx context.Context, userID uuid.UUID) ([]Category, error)
	Get(ctx context.Context, userID, id uuid.UUID) (Category, error)
	ByNameKey(ctx context.Context, userID uuid.UUID, kind Kind, key string) (Category, error)
	// Update writes name, note and archived; it answers ErrNotFound or errNameTaken.
	Update(ctx context.Context, userID uuid.UUID, c Category) error
	// Delete answers ErrInUse while any entry is in the category.
	Delete(ctx context.Context, userID, id uuid.UUID) error
	// SetOrder gives each category its index in ids as its position.
	SetOrder(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) error
}

type Keys interface {
	DeleteExpiredKeys(ctx context.Context, userID uuid.UUID, createdBefore time.Time) error
	// ClaimKey answers false when the user already holds the key.
	ClaimKey(ctx context.Context, userID uuid.UUID, key string, requestHash []byte, categoryID uuid.UUID, at time.Time) (bool, error)
	// Key answers errNoKey when the user does not hold the key.
	Key(ctx context.Context, userID uuid.UUID, key string) (requestHash []byte, categoryID uuid.UUID, err error)
}

type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type Deps struct {
	Repo Repository
	Keys Keys
	Tx   Transactor
	Now  func() time.Time
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

var (
	errNameTaken = errors.New("category: name taken")
	errKeyHeld   = errors.New("category: idempotency key already held")
	errNoKey     = errors.New("category: idempotency key not held")
)

// Unarchived lists what a picker offers: expense categories first, each kind in manual order.
func (s *Service) Unarchived(ctx context.Context, userID uuid.UUID) ([]Category, error) {
	return s.d.Repo.ListUnarchived(ctx, userID) //nolint:wrapcheck // the repository already says what failed
}

// All lists archived categories too, in the same order.
func (s *Service) All(ctx context.Context, userID uuid.UUID) ([]Category, error) {
	return s.d.Repo.List(ctx, userID) //nolint:wrapcheck // the repository already says what failed
}

func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Category, error) {
	return s.d.Repo.Get(ctx, userID, id) //nolint:wrapcheck // ErrNotFound must reach the caller as is
}

// Create adds the category at the end of its kind, once per key.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, key string, n NewCategory) (Category, error) {
	n, err := n.normalized()
	if err != nil {
		return Category{}, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return Category{}, fmt.Errorf("category: new id: %w", err)
	}

	now := s.d.Now()
	hash := n.fingerprint()
	c := Category{ID: id, Kind: n.Kind, Name: n.Name, Note: n.Note}

	err = s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Keys.DeleteExpiredKeys(ctx, userID, now.Add(-KeyLifetime)); err != nil {
			return err //nolint:wrapcheck // wrapped below
		}

		all, err := s.d.Repo.List(ctx, userID)
		if err != nil {
			return err //nolint:wrapcheck // wrapped below
		}

		c.Position = nextPosition(all, c.Kind)

		if err := s.d.Repo.Create(ctx, userID, c); err != nil {
			return err //nolint:wrapcheck // errNameTaken must reach the switch below as is
		}

		claimed, err := s.d.Keys.ClaimKey(ctx, userID, key, hash, id, now)

		switch {
		case err != nil:
			return err //nolint:wrapcheck // wrapped below
		case !claimed:
			return errKeyHeld
		}

		return nil
	})

	// A retry of a create that went through finds its own name taken, or its key held.
	switch {
	case errors.Is(err, errKeyHeld), errors.Is(err, errNameTaken):
		return s.replayOr(ctx, userID, key, hash, c)
	case err != nil:
		return Category{}, fmt.Errorf("category: create: %w", err)
	}

	return c, nil
}

func (s *Service) replayOr(ctx context.Context, userID uuid.UUID, key string, hash []byte, c Category) (Category, error) {
	held, categoryID, err := s.d.Keys.Key(ctx, userID, key)

	switch {
	case errors.Is(err, errNoKey):
		return Category{}, s.nameTakenBy(ctx, userID, c)
	case err != nil:
		return Category{}, fmt.Errorf("category: read idempotency key: %w", err)
	case !bytes.Equal(held, hash):
		return Category{}, ErrKeyReused
	}

	return s.Get(ctx, userID, categoryID)
}

func (s *Service) nameTakenBy(ctx context.Context, userID uuid.UUID, c Category) error {
	existing, err := s.d.Repo.ByNameKey(ctx, userID, c.Kind, c.NameKey())
	if err != nil {
		return fmt.Errorf("category: read the category with the same name: %w", err)
	}

	return nameTaken(existing)
}

func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, ch Change) (Category, error) {
	c, err := s.d.Repo.Get(ctx, userID, id)
	if err != nil {
		return Category{}, err //nolint:wrapcheck // ErrNotFound must reach the caller as is
	}

	c, err = c.with(ch)
	if err != nil {
		return Category{}, err
	}

	err = s.d.Repo.Update(ctx, userID, c)

	switch {
	case errors.Is(err, errNameTaken):
		return Category{}, s.nameTakenBy(ctx, userID, c)
	case err != nil:
		return Category{}, err //nolint:wrapcheck // ErrNotFound must reach the caller as is
	}

	return c, nil
}

// Reorder sets the manual order of one kind; ids lists all of its categories, archived ones too.
func (s *Service) Reorder(ctx context.Context, userID uuid.UUID, kind Kind, ids []uuid.UUID) error {
	all, err := s.d.Repo.List(ctx, userID)
	if err != nil {
		return fmt.Errorf("category: reorder: %w", err)
	}

	if err := checkOrder(all, kind, ids); err != nil {
		return err
	}

	return s.d.Repo.SetOrder(ctx, userID, ids) //nolint:wrapcheck // the repository already says what failed
}

func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.d.Repo.Delete(ctx, userID, id) //nolint:wrapcheck // ErrNotFound and ErrInUse must reach the caller as is
}

func nextPosition(all []Category, kind Kind) int {
	next := 0

	for _, c := range all {
		if c.Kind == kind {
			next = max(next, c.Position+1)
		}
	}

	return next
}

func (s *Service) CreateDefaults(ctx context.Context, userID uuid.UUID) error {
	for _, kind := range []Kind{Expense, Income} {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("category: new id: %w", err)
		}

		if err := s.d.Repo.Create(ctx, userID, Category{ID: id, Kind: kind, Name: OtherName}); err != nil {
			return fmt.Errorf("category: create %s Other: %w", kind, err)
		}
	}

	return nil
}
