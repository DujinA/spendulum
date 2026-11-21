package settings

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

func (h Handler) GetSettings(ctx context.Context, _ gen.GetSettingsRequestObject) (gen.GetSettingsResponseObject, error) {
	session, _ := auth.SessionFrom(ctx)

	s, err := h.svc.Get(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	return gen.GetSettings200JSONResponse{PrimaryCurrency: s.PrimaryCurrency, TimeZone: s.TimeZone}, nil
}
