package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
	"github.com/DujinA/spendulum/backend/internal/httpapi/problem"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	DB     Pinger
	WebUI  http.Handler
	Logger *slog.Logger
	Auth   auth.Handler
}

type server struct {
	probes
	auth.Handler
}

var _ gen.StrictServerInterface = server{}

func NewHandler(d Deps) http.Handler {
	mux := http.NewServeMux()

	strict := gen.NewStrictHandlerWithOptions(
		server{probes{db: d.DB, logger: d.Logger}, d.Auth},
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

	return logRequests(d.Logger, noStoreOutsideSPA(rejectCrossOrigin(requireJSON(onAPI(d.Auth.RequireSession, mux)))))
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	problem.Write(w, http.StatusNotFound, "")
}
