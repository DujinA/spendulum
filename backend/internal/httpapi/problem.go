package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/DujinA/spendulum/backend/internal/httpapi/problem"
)

func requestError(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		detail := err.Error()
		if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
			detail = wrongType(typeErr)
		}

		logger.InfoContext(r.Context(), "bad request", slog.String("error", detail))
		problem.Write(w, http.StatusBadRequest, detail)
	}
}

// A JSON type error quotes the rejected number, which can be an amount (ADR 0009, docs/agents/go.md).
func wrongType(err *json.UnmarshalTypeError) string {
	if err.Field == "" {
		return "The request body has the wrong type."
	}

	return err.Field + " has the wrong type."
}

// The detail stays in the log, never in the response.
func responseError(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		logger.ErrorContext(r.Context(), "unhandled error", slog.Any("error", err))
		problem.Write(w, http.StatusInternalServerError, "")
	}
}
