# Spendulum

A personal budget tracker: one Go binary serves the API and an installable web app (PWA) used on desktop and iPhone. This file is the index of rules and commands. Detail lives in the files it links; read each one when your work touches its area.

## Read first, in this order

1. The issue, its comments, and its parent issue.
2. `GLOSSARY.md`, always. Use its terms in code, UI copy, tests and the PR, never its avoided synonyms.
3. The ADRs in `docs/adr/` for the area you touch:

| Area | ADRs |
|---|---|
| Migrations | 0004, 0008 |
| Money and currencies | 0007 |
| User scoping | 0009 |
| Backend structure | 0010 |
| Deploy and cluster | 0001, 0002, 0003, 0005 |
| Monitoring | 0006 |

4. `api/openapi.yaml` when the slice touches the API.
5. `docs/agents/go.md`, `docs/agents/web.md` and `docs/agents/testing.md` for the code you change.

## Layout

```
api/openapi.yaml            the API contract (OpenAPI 3.1); change it first
backend/                    Go module
  cmd/spendulum/            one binary: serve, migrate
  internal/<feature>/       domain.go, service.go, postgres.go, http.go (ADR 0010)
  internal/httpapi/         strict server (gen/), problem+json, middleware
  internal/webui/           embedded SPA; dist/index.html is a committed stub
  internal/platform/        config, db (pool, migrations, sqlc gen/), logging
  migrations/               Goose SQL, embedded in the binary
  queries/                  sqlc .sql files
web/                        Vite + React PWA; tests sit beside the code
deploy/                     Go module: Kustomize base, overlays/prod, argocd/ (app-of-apps), manifest tests
e2e/                        Playwright, compose.e2e.yaml, stubs/
docs/adr/  docs/agents/     decisions and agent guides
```

Generated code (`backend/internal/httpapi/gen/`, `backend/internal/platform/db/gen/`, `web/src/api/schema.gen.ts`) is marked `linguist-generated`. Never edit it by hand: run `task gen`.

## Commands

Prerequisites: Go, a C compiler (sqlc builds with cgo), Node 26 LTS, pnpm, Task and Docker.

| Command | What it does |
|---|---|
| `task dev` | PostgreSQL 18 on compose, `spendulum migrate`, the Go server on :8080 and the Vite dev server with `/api` proxied to Go |
| `task gen` | oapi-codegen (strict server), sqlc and openapi-typescript |
| `task lint` | golangci-lint (pinned, strict), eslint, `tsc`, `task manifests` |
| `task manifests` | `kustomize build` of every overlay and `deploy/argocd/`, `kubeconform -strict` with CRD schemas, and the manifest tests in `deploy/` |
| `task test` | `go test -race ./...` (PostgreSQL through testcontainers, so Docker must run) and Vitest |
| `task check` | `task gen`, `git diff --exit-code`, `task lint`, `task test`. Stage your changes first: the diff compares the tree with the index |
| `task e2e` | builds the image and runs the Playwright smoke tests on WebKit (iPhone 15) and Chromium |
| `task migrate` | applies pending migrations to the dev database |
| `task build` | builds the image as `spendulum:dev` |

Run `task check` before every push. Also run `task e2e` when a user journey or the `Dockerfile` changed.

## Comments

Names, types and tests carry the meaning. Write a comment only for what the code cannot show: a non-obvious reason, a gotcha, or the ADR that decides the behaviour. Keep it to one line. Directives (`//go:embed`, `//nolint:<linter> // reason`, `-- +goose Up`) are code. Exported Go identifiers and packages need no doc comment.

## Definition of Done

1. `task check` passes locally, and `ci-ok` passes on the PR.
2. OpenAPI changes come first; generated code is committed and never edited by hand.
3. Migrations are new files only, forward-only and additive, and keep the previous image working (ADR 0008).
4. Code, UI copy, tests and the PR use `GLOSSARY.md` terms, never their avoided synonyms.
5. No personal financial data anywhere; fixtures are fake.
6. Every acceptance criterion maps to a named test.
7. UI changes work at 390 px width and in dark mode, with screenshots in the PR.
8. The PR title is a Conventional Commit (squash merge makes it the commit message).
9. Every query on a user-owned table is scoped by user, and each repository has a two-user test (ADR 0009).

## PR body

- `Closes #n`.
- What changed, in 2 to 4 lines.
- A table that maps each acceptance criterion to its test name.
- Screenshots for UI changes.
- **Assumptions**: every judgement call you made, and every new dependency.

## Ambiguity and ADR conflicts

- **Blocking ambiguity** (readings that lead to different behaviour or data): comment specific questions on the issue, replace `ready-for-agent` with `needs-info`, and stop without a PR.
- **Minor ambiguity**: take the simplest reading and list it under **Assumptions**.
- A change that contradicts an ADR, or needs a new glossary term or a new ADR, is blocking. Never edit `docs/adr/` or `GLOSSARY.md` unless the issue says so.

## Not your job

`docs/iphone-checklist.md` is a manual check on a real iPhone for the human. Never run it, and never mention it in a PR.

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues for DujinA/spendulum, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default five-role vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.
