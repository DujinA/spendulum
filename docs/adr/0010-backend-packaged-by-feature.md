# The backend is packaged by feature, with one sqlc package and a shared transaction helper

The Go backend is hexagonal (ports and adapters) and packaged by feature: each package under `backend/internal/` (`entry/`, `vendors/`, `category/`, `budget/`, `report/`, `fx/`, `importing/`, `export/`, `auth/`, `settings/`) owns its domain types and rules (`domain.go`), its use cases (`service.go`), and its adapters (`postgres.go`, `http.go`). Interfaces are declared by the consumer that needs them, and unit tests use hand-written fakes. `httpapi/` holds the generated oapi-codegen strict server and composes the feature handlers; `platform/` holds configuration, the database pool, logging and telemetry. All sqlc-generated code is one package in `platform/`, and every Postgres adapter is built on its `*Queries`, so `platform/db.InTx(ctx, fn)` can run adapters from several features in one database transaction. Import apply needs this, because it writes entries, vendors, categories and budget target versions atomically, and so does vendor merge.

We chose feature packages because a slice usually touches one feature end to end: in one package, that is one folder to read and one folder to change, which also keeps an agent's context small. One sqlc package with one transaction helper exists because import apply and vendor merge already need transactions that span features, and without one shared pattern each slice would invent its own.

## Considered Options

- **One package per layer** (`domain/`, `app/`, `postgres/`, `httpapi/`): rejected; every slice would touch four folders, and the layers make a small Go project hard to navigate.
- **One sqlc package per feature**: rejected; a transaction across features would need several generated `Queries` bound to the same `pgx.Tx`, and shared tables would be generated more than once.

## Consequences

- The vendor feature package is `vendors/`, not `vendor/`: the go command skips directories named `vendor` when it expands `./...`, so `go test ./...` and `golangci-lint run ./...` would never reach it.
- `golangci-lint`'s depguard matches by file glob: `**/domain.go` and `**/service.go` must not import `net/http`, pgx or the sqlc package. A red check stops drift that prose rules would not.
- A feature calls another feature only through its service, never through its adapters. Go cannot enforce this inside one package, so code review does.
- Repository methods take the user id first (ADR-0009); the shared `*Queries` does not change that.
- The operator CLI subcommands (`import`, `user`) call the same services as the API; they never write SQL of their own.
