package vendors

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/platform/namekey"
)

type Repository interface {
	// List answers every vendor of the user, archived or not, with its use for entries of the kind.
	List(ctx context.Context, userID uuid.UUID, kind category.Kind) ([]Vendor, error)
	// Use answers the id of the user's vendor with v's key, unarchived, and creates v when there is none.
	Use(ctx context.Context, userID uuid.UUID, v Vendor) (uuid.UUID, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Search(ctx context.Context, userID uuid.UUID, kind category.Kind, name string) (Matches, error) {
	all, err := s.repo.List(ctx, userID, kind)
	if err != nil {
		return Matches{}, fmt.Errorf("vendors: list: %w", err)
	}

	return Search(all, name), nil
}

// Use answers the vendor an entry names: the one with the name's key, which it unarchives, or a new one.
func (s *Service) Use(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error) {
	name = strings.TrimSpace(name)

	key := namekey.Of(name)
	if key == "" {
		return uuid.Nil, ErrBlankName
	}

	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("vendors: new id: %w", err)
	}

	return s.repo.Use(ctx, userID, Vendor{ID: id, Name: name, Key: key}) //nolint:wrapcheck // the repository already says what failed
}
