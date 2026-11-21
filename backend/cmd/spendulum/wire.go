package main

import (
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
			newCategoryService(d),
		},
	})
}

func newCategoryService(d *db.DB) *category.Service {
	repo := category.NewPostgres(d.Queries())

	return category.NewService(category.Deps{Repo: repo, Keys: repo, Tx: d})
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
		Categories: category.NewHandler(newCategoryService(d)),
		Entries: entry.NewHandler(entry.NewService(entry.Deps{
			Repo: entry.NewPostgres(q), Tx: d, Settings: settingsSvc,
		}), logger),
		Currencies: fx.NewHandler(fx.NewService(fx.Deps{Repo: fx.NewPostgres(q)})),
	}
}

func newRateScheduler(d *db.DB, providerURL string, logger *slog.Logger) (*fx.Scheduler, error) {
	q := d.Queries()
	svc := fx.NewService(fx.Deps{
		Provider: fx.NewFrankfurter(providerURL, &http.Client{Timeout: time.Minute}),
		Repo:     fx.NewPostgres(q),
		Entries:  entry.NewService(entry.Deps{Repo: entry.NewPostgres(q)}),
	})

	return fx.NewScheduler(svc, logger) //nolint:wrapcheck // the error names the fx package
}
