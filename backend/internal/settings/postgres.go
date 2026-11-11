package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/DujinA/spendulum/backend/internal/platform/db/gen"
)

type Postgres struct {
	q *gen.Queries
}

func NewPostgres(q *gen.Queries) *Postgres {
	return &Postgres{q: q}
}

func (p *Postgres) Create(ctx context.Context, userID uuid.UUID, s Settings) error {
	err := p.q.CreateSettings(ctx, gen.CreateSettingsParams{UserID: userID, PrimaryCurrency: s.PrimaryCurrency, TimeZone: s.TimeZone})
	if err != nil {
		return fmt.Errorf("settings: insert: %w", err)
	}

	return nil
}

func (p *Postgres) Get(ctx context.Context, userID uuid.UUID) (Settings, error) {
	row, err := p.q.Settings(ctx, userID)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Settings{}, ErrNotFound
	case err != nil:
		return Settings{}, fmt.Errorf("settings: select: %w", err)
	}

	return Settings{PrimaryCurrency: row.PrimaryCurrency, TimeZone: row.TimeZone}, nil
}
