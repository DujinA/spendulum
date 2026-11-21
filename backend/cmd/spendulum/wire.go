package main

import (
	"log/slog"

	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/entry"
	"github.com/DujinA/spendulum/backend/internal/httpapi"
	"github.com/DujinA/spendulum/backend/internal/platform/config"
	"github.com/DujinA/spendulum/backend/internal/platform/db"
	"github.com/DujinA/spendulum/backend/internal/settings"
	"github.com/DujinA/spendulum/backend/internal/vendors"
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
	vendorSvc := vendors.NewService(vendors.NewPostgres(q))

	return httpapi.Deps{
		DB:         d,
		WebUI:      webui.Handler(webui.FS()),
		Logger:     logger,
		Auth:       auth.NewHandler(newAuthService(d), logger, cfg.CookieSecure, cfg.TrustedProxies),
		Settings:   settings.NewHandler(settingsSvc),
		Categories: category.NewHandler(category.NewService(category.NewPostgres(q))),
		Vendors:    vendors.NewHandler(vendorSvc),
		Entries: entry.NewHandler(entry.NewService(entry.Deps{
			Repo: entry.NewPostgres(q), Vendors: vendorSvc, Tx: d, Settings: settingsSvc,
		}), logger),
	}
}
