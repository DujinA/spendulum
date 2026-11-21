package fx

import (
	"context"

	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) Handler {
	return Handler{svc: svc}
}

func (h Handler) ListCurrencies(ctx context.Context, _ gen.ListCurrenciesRequestObject) (gen.ListCurrenciesResponseObject, error) {
	cs, err := h.svc.Currencies(ctx)
	if err != nil {
		return nil, err
	}

	out := make(gen.ListCurrencies200JSONResponse, len(cs))
	for i, c := range cs {
		out[i] = gen.Currency{Code: c.Code, MinorUnits: c.MinorUnits}
	}

	return out, nil
}
