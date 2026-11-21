package category

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/platform/db/gen"
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
		Position: int32(c.Position), //nolint:gosec // positions are list indexes, far below 2^31
		Archived: c.Archived,
	})
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
		out[i] = fromRow(gen.UnarchivedCategoriesRow(r))
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
		out[i] = fromRow(r)
	}

	return out, nil
}

func fromRow(r gen.UnarchivedCategoriesRow) Category {
	return Category{ID: r.ID, Kind: Kind(r.Kind), Name: r.Name, Position: int(r.Position), Archived: r.Archived}
}
