package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
)

const problemContentType = "application/problem+json"

func problem(status int, detail string) gen.Problem {
	p := gen.Problem{Title: http.StatusText(status), Status: status}
	if detail != "" {
		p.Detail = &detail
	}

	return p
}

func writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", problemContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem(status, detail)) //nolint:errchkjson // the status is sent; a failed write means the client left
}

func requestError(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		logger.InfoContext(r.Context(), "bad request", slog.Any("error", err))
		writeProblem(w, http.StatusBadRequest, err.Error())
	}
}

// The detail stays in the log, never in the response.
func responseError(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		logger.ErrorContext(r.Context(), "unhandled error", slog.Any("error", err))
		writeProblem(w, http.StatusInternalServerError, "")
	}
}
