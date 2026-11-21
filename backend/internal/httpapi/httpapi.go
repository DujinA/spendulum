package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/entry"
	"github.com/DujinA/spendulum/backend/internal/fx"
	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
	"github.com/DujinA/spendulum/backend/internal/httpapi/problem"
	"github.com/DujinA/spendulum/backend/internal/settings"
	"github.com/DujinA/spendulum/backend/internal/vendors"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	DB         Pinger
	WebUI      http.Handler
	Logger     *slog.Logger
	Auth       auth.Handler
	Settings   settings.Handler
	Categories category.Handler
	Vendors    vendors.Handler
	Entries    entry.Handler
	Currencies fx.Handler
}

// Embedded fields take their alias names, so each feature's Handler can be embedded.
type (
	settingsHandler = settings.Handler
	categoryHandler = category.Handler
	vendorsHandler  = vendors.Handler
	entryHandler    = entry.Handler
	currencyHandler = fx.Handler
)

type server struct {
	probes
	auth.Handler
	settingsHandler
	categoryHandler
	vendorsHandler
	entryHandler
	currencyHandler
}

var _ gen.StrictServerInterface = server{}

func NewHandler(d Deps) http.Handler {
	mux := http.NewServeMux()

	strict := gen.NewStrictHandlerWithOptions(
		server{probes{db: d.DB, logger: d.Logger}, d.Auth, d.Settings, d.Categories, d.Vendors, d.Entries, d.Currencies},
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
