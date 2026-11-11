package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/DujinA/spendulum/backend/internal/httpapi/problem"
)

func requestError(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		logger.InfoContext(r.Context(), "bad request", slog.Any("error", err))
		problem.Write(w, http.StatusBadRequest, err.Error())
	}
}

// The detail stays in the log, never in the response.
func responseError(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		logger.ErrorContext(r.Context(), "unhandled error", slog.Any("error", err))
		problem.Write(w, http.StatusInternalServerError, "")
	}
}
