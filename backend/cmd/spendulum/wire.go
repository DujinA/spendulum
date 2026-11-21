package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/entry"
	"github.com/DujinA/spendulum/backend/internal/fx"
	"github.com/DujinA/spendulum/backend/internal/httpapi"
	"github.com/DujinA/spendulum/backend/internal/platform/config"
	"github.com/DujinA/spendulum/backend/internal/platform/db"
	"github.com/DujinA/spendulum/backend/internal/settings"
	"github.com/DujinA/spendulum/backend/internal/webui"
)

func newAuthService(d *db.DB) *auth.Service {
	q := d.Queries()

	return auth.NewService(auth.Deps{
		Repo:    auth.NewPostgres(q),
		Tx:      d,
		Hasher:  auth.NewHasher(),
		Limiter: auth.NewLimiter(),
		Defaults: []auth.AccountDefaults{
			settings.NewService(settings.NewPostgres(q)),
			category.NewService(category.NewPostgres(q)),
		},
	})
}

func newAPIDeps(d *db.DB, cfg config.Config, logger *slog.Logger) httpapi.Deps {
	q := d.Queries()
	settingsSvc := settings.NewService(settings.NewPostgres(q))

	return httpapi.Deps{
		DB:         d,
		WebUI:      webui.Handler(webui.FS()),
		Logger:     logger,
		Auth:       auth.NewHandler(newAuthService(d), logger, cfg.CookieSecure, cfg.TrustedProxies),
		Settings:   settings.NewHandler(settingsSvc),
		Categories: category.NewHandler(category.NewService(category.NewPostgres(q))),
		Entries: entry.NewHandler(entry.NewService(entry.Deps{
			Repo: entry.NewPostgres(q), Tx: d, Settings: settingsSvc,
		}), logger),
	}
}

func newRateScheduler(d *db.DB, providerURL string, logger *slog.Logger) (*fx.Scheduler, error) {
	svc := fx.NewService(fx.Deps{
		Provider: fx.NewFrankfurter(providerURL, &http.Client{Timeout: time.Minute}),
		Repo:     fx.NewPostgres(d.Queries()),
		Entries:  noEntries{},
	})

	return fx.NewScheduler(svc, logger) //nolint:wrapcheck // the error names the fx package
}

// TODO(#29): replace with the entries adapter once the entries table exists.
type noEntries struct{}

func (noEntries) EarliestEntryDate(context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
