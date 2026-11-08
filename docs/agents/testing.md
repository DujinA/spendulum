# Testing

Test first: each acceptance criterion becomes at least one failing test before the code, and the PR maps each criterion to its test name.

## Per layer

- **Domain and services**: table-driven unit tests with hand-written fakes. Use `pgregory.net/rapid` property tests for money, conversion and rounding.
- **Postgres adapters**: integration tests on real PostgreSQL 18. Never mock SQL. Call `dbtest.Main(m)` from the package's `TestMain`: it starts one container per test binary and migrates a template database once. `dbtest.NewDatabase(t)` then gives each test a fresh, migrated database. Each repository has a two-user test proving one user cannot read or change another's rows (ADR 0009).
- **HTTP**: `httptest` tests through `httpapi.NewHandler`, with responses validated against `api/openapi.yaml` by kin-openapi (see `backend/internal/httpapi/httpapi_test.go`).
- **Web**: Vitest + Testing Library at component level, tests beside the code. Mock the API with MSW handlers typed from `src/api/schema.gen.ts`; the first slice that calls the API adds MSW.
- **Playwright** (`e2e/`): only when the slice adds or changes one of the five user journeys: login, fast entry, edit plus undo, a dashboard preset, CSV export. The one exception is `e2e/tests/smoke.spec.ts`, which checks the image itself. One test per journey; edge cases belong in Vitest and Go tests. Each test creates its own data through API helpers, with unique names. `page.clock` freezes dates. The app reaches no network: `e2e/stubs/` stands in for Frankfurter.

## Running

- `task test` runs Go with `-race` and Vitest. Docker must run for the Go tests.
- One Go package: `cd backend && go test -race ./internal/<pkg>/`.
- One web test file: `pnpm --filter @spendulum/web exec vitest run src/<file>.test.tsx`.
- `task e2e` builds the image, starts `e2e/compose.e2e.yaml` and runs Playwright in its container on WebKit (iPhone 15) and Chromium (desktop).

No coverage threshold. Fixtures are fake: never use personal financial data.
