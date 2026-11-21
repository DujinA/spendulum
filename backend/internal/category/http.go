package category

import (
	"context"
	"errors"
	"net/http"

	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
	"github.com/DujinA/spendulum/backend/internal/httpapi/problem"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) Handler {
	return Handler{svc: svc}
}

func (h Handler) ListCategories(ctx context.Context, req gen.ListCategoriesRequestObject) (gen.ListCategoriesResponseObject, error) {
	session, _ := auth.SessionFrom(ctx)

	list := h.svc.Unarchived
	if req.Params.IncludeArchived != nil && *req.Params.IncludeArchived {
		list = h.svc.All
	}

	cats, err := list(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	out := make(gen.ListCategories200JSONResponse, len(cats))
	for i, c := range cats {
		out[i] = toJSON(c)
	}

	return out, nil
}

func (h Handler) CreateCategory(ctx context.Context, req gen.CreateCategoryRequestObject) (gen.CreateCategoryResponseObject, error) {
	session, _ := auth.SessionFrom(ctx)

	var note string
	if req.Body.Note != nil {
		note = *req.Body.Note
	}

	c, err := h.svc.Create(ctx, session.UserID, req.Params.IdempotencyKey, NewCategory{
		Kind: Kind(req.Body.Kind), Name: req.Body.Name, Note: note,
	})

	if rule, ok := errors.AsType[RuleError](err); ok {
		return gen.CreateCategory422ApplicationProblemPlusJSONResponse{
			UnprocessableContentApplicationProblemPlusJSONResponse: unprocessable(rule),
		}, nil
	}

	if err != nil {
		return nil, err
	}

	return gen.CreateCategory201JSONResponse(toJSON(c)), nil
}

func unprocessable(rule RuleError) gen.UnprocessableContentApplicationProblemPlusJSONResponse {
	return gen.UnprocessableContentApplicationProblemPlusJSONResponse(problem.New(http.StatusUnprocessableEntity, rule.Error()))
}

func (h Handler) UpdateCategory(ctx context.Context, req gen.UpdateCategoryRequestObject) (gen.UpdateCategoryResponseObject, error) {
	session, _ := auth.SessionFrom(ctx)

	c, err := h.svc.Update(ctx, session.UserID, req.Id, Change{Name: req.Body.Name, Note: req.Body.Note, Archived: req.Body.Archived})

	if rule, ok := errors.AsType[RuleError](err); ok {
		return gen.UpdateCategory422ApplicationProblemPlusJSONResponse{UnprocessableContentApplicationProblemPlusJSONResponse: unprocessable(rule)}, nil
	}

	switch {
	case errors.Is(err, ErrNotFound):
		return gen.UpdateCategory404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: notFound()}, nil
	case err != nil:
		return nil, err
	}

	return gen.UpdateCategory200JSONResponse(toJSON(c)), nil
}

func notFound() gen.NotFoundApplicationProblemPlusJSONResponse {
	return gen.NotFoundApplicationProblemPlusJSONResponse(problem.New(http.StatusNotFound, ""))
}

func (h Handler) DeleteCategory(ctx context.Context, req gen.DeleteCategoryRequestObject) (gen.DeleteCategoryResponseObject, error) {
	session, _ := auth.SessionFrom(ctx)

	err := h.svc.Delete(ctx, session.UserID, req.Id)

	switch {
	case errors.Is(err, ErrNotFound):
		return gen.DeleteCategory404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: notFound()}, nil
	case errors.Is(err, ErrInUse):
		return gen.DeleteCategory409ApplicationProblemPlusJSONResponse{
			ConflictApplicationProblemPlusJSONResponse: gen.ConflictApplicationProblemPlusJSONResponse(problem.New(
				http.StatusConflict, "This category has entries, so it cannot be deleted. Archive it instead.",
			)),
		}, nil
	case err != nil:
		return nil, err
	}

	return gen.DeleteCategory204Response{}, nil
}

func (h Handler) ReorderCategories(ctx context.Context, req gen.ReorderCategoriesRequestObject) (gen.ReorderCategoriesResponseObject, error) {
	session, _ := auth.SessionFrom(ctx)

	err := h.svc.Reorder(ctx, session.UserID, Kind(req.Body.Kind), req.Body.Ids)

	if rule, ok := errors.AsType[RuleError](err); ok {
		return gen.ReorderCategories422ApplicationProblemPlusJSONResponse{UnprocessableContentApplicationProblemPlusJSONResponse: unprocessable(rule)}, nil
	}

	switch {
	case errors.Is(err, ErrNotFound):
		return gen.ReorderCategories404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: notFound()}, nil
	case err != nil:
		return nil, err
	}

	return gen.ReorderCategories204Response{}, nil
}

func toJSON(c Category) gen.Category {
	return gen.Category{Id: c.ID, Kind: gen.Kind(c.Kind), Name: c.Name, Note: c.Note, Archived: c.Archived}
}
