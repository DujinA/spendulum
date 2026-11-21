package vendors

import (
	"context"
	"net/http"

	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
	"github.com/DujinA/spendulum/backend/internal/httpapi/problem"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) Handler {
	return Handler{svc: svc}
}

func (h Handler) ListVendors(ctx context.Context, req gen.ListVendorsRequestObject) (gen.ListVendorsResponseObject, error) {
	session, _ := auth.SessionFrom(ctx)

	kind := category.Kind(req.Params.Kind)
	if kind != category.Expense && kind != category.Income {
		return gen.ListVendors400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: gen.BadRequestApplicationProblemPlusJSONResponse(
				problem.New(http.StatusBadRequest, "The kind must be expense or income."),
			),
		}, nil
	}

	var name string
	if req.Params.Name != nil {
		name = *req.Params.Name
	}

	m, err := h.svc.Search(ctx, session.UserID, kind, name)
	if err != nil {
		return nil, err
	}

	out := gen.ListVendors200JSONResponse{Vendors: toJSONs(m.Vendors), NearMatches: toJSONs(m.NearMatches)}

	if m.Match != nil {
		match := toJSON(*m.Match)
		out.Match = &match
	}

	return out, nil
}

func toJSONs(vs []Vendor) []gen.Vendor {
	out := make([]gen.Vendor, len(vs))
	for i, v := range vs {
		out[i] = toJSON(v)
	}

	return out
}

func toJSON(v Vendor) gen.Vendor {
	out := gen.Vendor{Id: v.ID, Name: v.Name, Archived: v.Archived, UseCount: v.UseCount}

	if v.Latest != nil {
		out.Latest = &gen.LatestUse{CategoryId: v.Latest.CategoryID, CategoryName: v.Latest.CategoryName, Currency: v.Latest.Currency}
	}

	return out
}
