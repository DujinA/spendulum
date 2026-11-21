package entry

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
	"github.com/DujinA/spendulum/backend/internal/httpapi/problem"
)

const (
	defaultPageSize = 50
	maxPageSize     = 100
)

type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) Handler {
	return Handler{svc: svc, logger: logger}
}

func (h Handler) CreateEntry(ctx context.Context, req gen.CreateEntryRequestObject) (gen.CreateEntryResponseObject, error) {
	session, _ := auth.SessionFrom(ctx)

	e, err := h.svc.Create(ctx, session.UserID, req.Params.IdempotencyKey, NewEntry{
		Kind:       category.Kind(req.Body.Kind),
		Amount:     req.Body.Amount,
		CategoryID: req.Body.CategoryId,
		VendorName: valueOf(req.Body.VendorName),
		EntryDate:  req.Body.EntryDate.Time,
		Note:       valueOf(req.Body.Note),
	})

	if rule, ok := errors.AsType[RuleError](err); ok {
		h.logger.InfoContext(ctx, "entry refused", slog.String("user_id", session.UserID.String()), slog.String("reason", rule.Error()))

		return gen.CreateEntry422ApplicationProblemPlusJSONResponse{
			UnprocessableContentApplicationProblemPlusJSONResponse: gen.UnprocessableContentApplicationProblemPlusJSONResponse(
				problem.New(http.StatusUnprocessableEntity, rule.Error()),
			),
		}, nil
	}

	if err != nil {
		return nil, err
	}

	h.logger.InfoContext(ctx, "entry created", slog.String("user_id", session.UserID.String()), slog.String("entry_id", e.ID.String()))

	return gen.CreateEntry201JSONResponse(toJSON(e)), nil
}

func (h Handler) ListEntries(ctx context.Context, req gen.ListEntriesRequestObject) (gen.ListEntriesResponseObject, error) {
	session, _ := auth.SessionFrom(ctx)

	limit := defaultPageSize
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}

	if limit < 1 || limit > maxPageSize {
		return listBadRequest("The limit must be between 1 and 100."), nil
	}

	var after *Cursor

	if req.Params.Cursor != nil {
		c, ok := decodeCursor(*req.Params.Cursor)
		if !ok {
			return listBadRequest("The cursor is not one this API gave out."), nil
		}

		after = &c
	}

	page, err := h.svc.List(ctx, session.UserID, after, limit)
	if err != nil {
		return nil, err
	}

	out := gen.ListEntries200JSONResponse{Entries: make([]gen.Entry, len(page.Entries))}
	for i, e := range page.Entries {
		out.Entries[i] = toJSON(e)
	}

	if page.Next != nil {
		next := encodeCursor(*page.Next)
		out.NextCursor = &next
	}

	return out, nil
}

func (h Handler) GetEntry(ctx context.Context, req gen.GetEntryRequestObject) (gen.GetEntryResponseObject, error) {
	session, _ := auth.SessionFrom(ctx)

	e, err := h.svc.Get(ctx, session.UserID, req.Id)

	switch {
	case errors.Is(err, ErrNotFound):
		return gen.GetEntry404ApplicationProblemPlusJSONResponse{
			NotFoundApplicationProblemPlusJSONResponse: gen.NotFoundApplicationProblemPlusJSONResponse(problem.New(http.StatusNotFound, "")),
		}, nil
	case err != nil:
		return nil, err
	}

	return gen.GetEntry200JSONResponse(toJSON(e)), nil
}

func listBadRequest(detail string) gen.ListEntries400ApplicationProblemPlusJSONResponse {
	return gen.ListEntries400ApplicationProblemPlusJSONResponse{
		BadRequestApplicationProblemPlusJSONResponse: gen.BadRequestApplicationProblemPlusJSONResponse(problem.New(http.StatusBadRequest, detail)),
	}
}

func toJSON(e Entry) gen.Entry {
	out := gen.Entry{
		Id: e.ID, Kind: gen.Kind(e.Kind), Amount: e.Amount, Currency: e.Currency,
		CategoryId: e.CategoryID, CategoryName: e.CategoryName, EntryDate: openapi_types.Date{Time: e.EntryDate},
		Note: e.Note, CreatedAt: e.CreatedAt,
	}

	if e.VendorID != uuid.Nil {
		out.VendorId, out.VendorName = &e.VendorID, &e.VendorName
	}

	return out
}

func valueOf(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func encodeCursor(c Cursor) string {
	raw := strings.Join([]string{c.EntryDate.Format(time.DateOnly), c.CreatedAt.UTC().Format(time.RFC3339Nano), c.ID.String()}, " ")

	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (Cursor, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, false
	}

	parts := strings.Split(string(raw), " ")
	if len(parts) != 3 {
		return Cursor{}, false
	}

	date, dateErr := time.Parse(time.DateOnly, parts[0])
	created, createdErr := time.Parse(time.RFC3339Nano, parts[1])
	id, idErr := uuid.Parse(parts[2])

	if dateErr != nil || createdErr != nil || idErr != nil {
		return Cursor{}, false
	}

	return Cursor{EntryDate: date, CreatedAt: created, ID: id}, true
}
