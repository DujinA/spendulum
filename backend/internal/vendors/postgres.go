package vendors

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/platform/db/gen"
)

type Postgres struct {
	q *gen.Queries
}

func NewPostgres(q *gen.Queries) *Postgres {
	return &Postgres{q: q}
}

func (p *Postgres) Use(ctx context.Context, userID uuid.UUID, v Vendor) (uuid.UUID, error) {
	id, err := p.q.UseVendor(ctx, gen.UseVendorParams{ID: v.ID, UserID: userID, Name: v.Name, Key: v.Key})
	if err != nil {
		return uuid.Nil, fmt.Errorf("vendors: upsert: %w", err)
	}

	return id, nil
}

func (p *Postgres) Archive(ctx context.Context, userID, id uuid.UUID) error {
	n, err := p.q.ArchiveVendor(ctx, gen.ArchiveVendorParams{UserID: userID, ID: id})

	switch {
	case err != nil:
		return fmt.Errorf("vendors: archive: %w", err)
	case n == 0:
		return ErrNotFound
	}

	return nil
}

func (p *Postgres) List(ctx context.Context, userID uuid.UUID, kind category.Kind) ([]Vendor, error) {
	rows, err := p.q.Vendors(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("vendors: select: %w", err)
	}

	uses, err := p.q.VendorUses(ctx, gen.VendorUsesParams{UserID: userID, Kind: string(kind)})
	if err != nil {
		return nil, fmt.Errorf("vendors: select uses: %w", err)
	}

	byVendor := make(map[uuid.UUID]gen.VendorUsesRow, len(uses))
	for _, u := range uses {
		byVendor[u.VendorID] = u
	}

	out := make([]Vendor, len(rows))
	for i, r := range rows {
		out[i] = Vendor{ID: r.ID, Name: r.Name, Key: r.Key, Archived: r.Archived}

		if u, ok := byVendor[r.ID]; ok {
			out[i].UseCount = int(u.UseCount)
			out[i].Latest = &LatestUse{CategoryID: u.CategoryID, CategoryName: u.CategoryName, Currency: u.Currency}
		}
	}

	return out, nil
}
