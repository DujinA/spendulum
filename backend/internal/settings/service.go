package settings

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, userID uuid.UUID, s Settings) error
	Get(ctx context.Context, userID uuid.UUID) (Settings, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateDefaults(ctx context.Context, userID uuid.UUID) error {
	if err := s.repo.Create(ctx, userID, Defaults()); err != nil {
		return fmt.Errorf("settings: create defaults: %w", err)
	}

	return nil
}

func (s *Service) Get(ctx context.Context, userID uuid.UUID) (Settings, error) {
	return s.repo.Get(ctx, userID) //nolint:wrapcheck // the repository already says what failed
}
