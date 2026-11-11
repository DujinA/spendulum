package category

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, userID uuid.UUID, c Category) error
	List(ctx context.Context, userID uuid.UUID) ([]Category, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateDefaults(ctx context.Context, userID uuid.UUID) error {
	for _, kind := range []Kind{Expense, Income} {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("category: new id: %w", err)
		}

		if err := s.repo.Create(ctx, userID, Category{ID: id, Kind: kind, Name: OtherName}); err != nil {
			return fmt.Errorf("category: create %s Other: %w", kind, err)
		}
	}

	return nil
}
