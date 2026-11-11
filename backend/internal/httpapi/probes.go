package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
	"github.com/DujinA/spendulum/backend/internal/httpapi/problem"
)

type probes struct {
	db     Pinger
	logger *slog.Logger
}

func (probes) GetHealthz(context.Context, gen.GetHealthzRequestObject) (gen.GetHealthzResponseObject, error) {
	return gen.GetHealthz200JSONResponse{Status: gen.Ok}, nil
}

func (p probes) GetReadyz(ctx context.Context, _ gen.GetReadyzRequestObject) (gen.GetReadyzResponseObject, error) {
	if err := p.db.Ping(ctx); err != nil {
		p.logger.WarnContext(ctx, "readiness: database ping failed", slog.Any("error", err))

		return gen.GetReadyz503ApplicationProblemPlusJSONResponse(
			problem.New(http.StatusServiceUnavailable, "The database does not answer."),
		), nil
	}

	return gen.GetReadyz200JSONResponse{Status: gen.Ok}, nil
}
