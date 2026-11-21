package category

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

const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
	nameKeyConstraint   = "categories_user_id_kind_name_key_key"
)

type Postgres struct {
	q *gen.Queries
}

func NewPostgres(q *gen.Queries) *Postgres {
	return &Postgres{q: q}
}

func (p *Postgres) Create(ctx context.Context, userID uuid.UUID, c Category) error {
	err := p.q.CreateCategory(ctx, gen.CreateCategoryParams{
		ID:       c.ID,
		UserID:   userID,
		Kind:     string(c.Kind),
		Name:     c.Name,
		NameKey:  c.NameKey(),
		Note:     c.Note,
		Position: int32(c.Position), //nolint:gosec // positions are list indexes, far below 2^31
		Archived: c.Archived,
	})
	if isNameTaken(err) {
		return errNameTaken
	}

	if err != nil {
		return fmt.Errorf("category: insert: %w", err)
	}

	return nil
}

func (p *Postgres) List(ctx context.Context, userID uuid.UUID) ([]Category, error) {
	rows, err := p.q.Categories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("category: select: %w", err)
	}

	out := make([]Category, len(rows))
	for i, r := range rows {
		out[i] = fromRow(gen.CategoryRow(r))
	}

	return out, nil
}

func (p *Postgres) ListUnarchived(ctx context.Context, userID uuid.UUID) ([]Category, error) {
	rows, err := p.q.UnarchivedCategories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("category: select unarchived: %w", err)
	}

	out := make([]Category, len(rows))
	for i, r := range rows {
		out[i] = fromRow(gen.CategoryRow(r))
	}

	return out, nil
}

func (p *Postgres) Get(ctx context.Context, userID, id uuid.UUID) (Category, error) {
	row, err := p.q.Category(ctx, gen.CategoryParams{UserID: userID, ID: id})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Category{}, ErrNotFound
	case err != nil:
		return Category{}, fmt.Errorf("category: select one: %w", err)
	}

	return fromRow(row), nil
}

func (p *Postgres) ByNameKey(ctx context.Context, userID uuid.UUID, kind Kind, key string) (Category, error) {
	row, err := p.q.CategoryByNameKey(ctx, gen.CategoryByNameKeyParams{UserID: userID, Kind: string(kind), NameKey: key})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Category{}, ErrNotFound
	case err != nil:
		return Category{}, fmt.Errorf("category: select by name key: %w", err)
	}

	return fromRow(gen.CategoryRow(row)), nil
}

func (p *Postgres) Update(ctx context.Context, userID uuid.UUID, c Category) error {
	n, err := p.q.UpdateCategory(ctx, gen.UpdateCategoryParams{
		UserID: userID, ID: c.ID, Name: c.Name, NameKey: c.NameKey(), Note: c.Note, Archived: c.Archived,
	})

	switch {
	case isNameTaken(err):
		return errNameTaken
	case err != nil:
		return fmt.Errorf("category: update: %w", err)
	case n == 0:
		return ErrNotFound
	}

	return nil
}

func (p *Postgres) SetOrder(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) error {
	if err := p.q.SetCategoryOrder(ctx, gen.SetCategoryOrderParams{UserID: userID, Ids: ids}); err != nil {
		return fmt.Errorf("category: set order: %w", err)
	}

	return nil
}

// Every entry row, a deleted entry's too, holds a foreign key to its category, so a category with entries cannot go.
func (p *Postgres) Delete(ctx context.Context, userID, id uuid.UUID) error {
	n, err := p.q.DeleteCategory(ctx, gen.DeleteCategoryParams{UserID: userID, ID: id})

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == foreignKeyViolation {
		return ErrInUse
	}

	switch {
	case err != nil:
		return fmt.Errorf("category: delete: %w", err)
	case n == 0:
		return ErrNotFound
	}

	return nil
}

func (p *Postgres) DeleteExpiredKeys(ctx context.Context, userID uuid.UUID, createdBefore time.Time) error {
	err := p.q.DeleteExpiredCategoryIdempotencyKeys(ctx, gen.DeleteExpiredCategoryIdempotencyKeysParams{UserID: userID, CreatedAt: createdBefore})
	if err != nil {
		return fmt.Errorf("category: delete expired idempotency keys: %w", err)
	}

	return nil
}

func (p *Postgres) ClaimKey(ctx context.Context, userID uuid.UUID, key string, requestHash []byte, categoryID uuid.UUID, at time.Time) (bool, error) {
	n, err := p.q.CreateCategoryIdempotencyKey(ctx, gen.CreateCategoryIdempotencyKeyParams{
		UserID: userID, Key: key, RequestHash: requestHash, CategoryID: categoryID, CreatedAt: at,
	})
	if err != nil {
		return false, fmt.Errorf("category: insert idempotency key: %w", err)
	}

	return n == 1, nil
}

func (p *Postgres) Key(ctx context.Context, userID uuid.UUID, key string) ([]byte, uuid.UUID, error) {
	row, err := p.q.CategoryIdempotencyKey(ctx, gen.CategoryIdempotencyKeyParams{UserID: userID, Key: key})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, uuid.Nil, errNoKey
	case err != nil:
		return nil, uuid.Nil, fmt.Errorf("category: select idempotency key: %w", err)
	}

	return row.RequestHash, row.CategoryID, nil
}

func isNameTaken(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)

	return ok && pgErr.Code == uniqueViolation && pgErr.ConstraintName == nameKeyConstraint
}

func fromRow(r gen.CategoryRow) Category {
	return Category{ID: r.ID, Kind: Kind(r.Kind), Name: r.Name, Note: r.Note, Position: int(r.Position), Archived: r.Archived}
}
