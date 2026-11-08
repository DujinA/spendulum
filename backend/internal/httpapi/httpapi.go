// Package httpapi is the HTTP edge: it serves the oapi-codegen strict server, maps
// errors to RFC 9457 problem+json, and falls back to the SPA outside /api.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
)

// Pinger reports whether the database answers.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps are the collaborators the HTTP edge needs.
type Deps struct {
	DB     Pinger
	WebUI  http.Handler
	Logger *slog.Logger
}

// server implements gen.StrictServerInterface. Feature handlers join it as they arrive.
type server struct {
	probes
}

var _ gen.StrictServerInterface = server{}

// NewHandler returns the handler for every request the binary serves.
func NewHandler(d Deps) http.Handler {
	mux := http.NewServeMux()

	strict := gen.NewStrictHandlerWithOptions(
		server{probes{db: d.DB, logger: d.Logger}},
		nil,
		gen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  requestError(d.Logger),
			ResponseErrorHandlerFunc: responseError(d.Logger),
		},
	)
	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{
		BaseRouter:       mux,
		ErrorHandlerFunc: requestError(d.Logger),
	})

	// Operations under /api register more specific patterns, so these only catch the rest.
	mux.HandleFunc("/api", notFound)
	mux.HandleFunc("/api/", notFound)
	mux.Handle("/", d.WebUI)

	return logRequests(d.Logger, noStoreOutsideSPA(mux))
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	writeProblem(w, http.StatusNotFound, "")
}
