package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
)

type probes struct {
	db     Pinger
	logger *slog.Logger
}

// GetHealthz answers while the process serves requests; it never touches the database.
func (probes) GetHealthz(context.Context, gen.GetHealthzRequestObject) (gen.GetHealthzResponseObject, error) {
	return gen.GetHealthz200JSONResponse{Status: gen.Ok}, nil
}

// GetReadyz answers 200 only when a database ping succeeds.
func (p probes) GetReadyz(ctx context.Context, _ gen.GetReadyzRequestObject) (gen.GetReadyzResponseObject, error) {
	if err := p.db.Ping(ctx); err != nil {
		p.logger.WarnContext(ctx, "readiness: database ping failed", slog.Any("error", err))

		return gen.GetReadyz503ApplicationProblemPlusJSONResponse(
			problem(http.StatusServiceUnavailable, "The database does not answer."),
		), nil
	}

	return gen.GetReadyz200JSONResponse{Status: gen.Ok}, nil
}
