package category

import (
	"context"

	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) Handler {
	return Handler{svc: svc}
}

func (h Handler) ListCategories(ctx context.Context, _ gen.ListCategoriesRequestObject) (gen.ListCategoriesResponseObject, error) {
	session, _ := auth.SessionFrom(ctx)

	cats, err := h.svc.Unarchived(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	out := make(gen.ListCategories200JSONResponse, len(cats))
	for i, c := range cats {
		out[i] = gen.Category{Id: c.ID, Kind: gen.Kind(c.Kind), Name: c.Name}
	}

	return out, nil
}
