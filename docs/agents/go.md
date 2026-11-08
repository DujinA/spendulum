# Go backend

Read with ADR 0010 (backend packaged by feature) and ADR 0009 (user-scoped rows).

## Structure

- One package per feature under `backend/internal/`: `entry/`, `vendor/`, `category/`, `budget/`, `report/`, `fx/`, `importing/`, `export/`, `auth/`, `settings/`. Use `entry/`, never `transaction/`: "Transaction" is an avoided term.
- Each feature package owns `domain.go` (types and rules), `service.go` (use cases), `postgres.go` and `http.go` (adapters), and their tests.
- Interfaces are declared by the consumer that needs them. Unit tests use hand-written fakes.
- `domain.go` and `service.go` never import `net/http`, pgx or the sqlc package. depguard fails the lint if they do.
- One feature calls another only through its service, never through its adapters. Code review enforces this.
- `httpapi/` composes the feature handlers into the generated strict server (`httpapi/gen`). Add an operation to `api/openapi.yaml`, run `task gen`, then implement its method on a feature handler and embed that handler in `httpapi.server`.
- `platform/` holds config, the database (`platform/db`), the sqlc code (`platform/db/gen`) and logging.
- The CLI subcommands (`backend/cmd/spendulum/`) call the same services as the API. They never write SQL of their own.

## Database

- SQL lives in `backend/queries/*.sql`; sqlc generates one package, `platform/db/gen`. No ORM, and no other way to query.
- Every Postgres adapter is built on the `*gen.Queries` from `db.DB.Queries()`. Each call runs under the `SPENDULUM_DB_TIMEOUT` deadline, so a stuck database never hangs a request.
- A transaction that spans features uses one shared helper, `platform/db.InTx(ctx, fn)` (ADR 0010). The first slice that needs a transaction adds it; never invent another pattern.
- Repository methods take the user id first, and every query on a user-owned table filters on `user_id` (ADR 0009). Ids are UUIDv7.
- Migrations: a new Goose file in `backend/migrations/`, never an edit to a merged one. No down steps (ADR 0008).

## Standards

- Money is `int64` minor units plus an ISO 4217 currency code. Never a float.
- Domain sentinel errors are mapped once, at the HTTP edge, to RFC 9457 `application/problem+json`. A row that belongs to another user answers `404`, never `403`.
- `context.Context` is the first parameter. No global state.
- Configuration comes from environment variables through `platform/config` (`caarlos0/env`). Add a field with its `env` tag and validation; invalid config exits at startup and names the variable.
- Logs use `log/slog` JSON with attributes, a constant message, and no global logger. Logs never contain amounts, vendor names or notes.
- Every create endpoint takes an `Idempotency-Key` header.
- API responses carry `Cache-Control: no-store`.
- `task lint` runs golangci-lint (pinned in `backend/tools/go.mod`) with `backend/.golangci.yml`. Fix the code; add a `//nolint:<linter> // reason` only when the linter is wrong.
