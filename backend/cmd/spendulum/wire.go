package main

import (
	"github.com/DujinA/spendulum/backend/internal/auth"
	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/platform/db"
	"github.com/DujinA/spendulum/backend/internal/settings"
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
