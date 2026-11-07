# Migrations are forward-only and additive; rollback is a revert of the image tag

Goose migrations have no down steps. A mistake is fixed by a new migration, never by editing or reversing a merged one. Each migration must keep the previous image working (expand first, contract in a later PR), so rolling back production is a `git revert` of the image-tag commit in `deploy/overlays/prod`, and the older image runs against the newer schema. Lost or corrupted data is recovered from the CloudNativePG backups (ADR-0003), not from down migrations.

We chose this over reversible migrations because down steps are rarely run, so they are rarely correct, and a down step that drops a column destroys the data a rollback was meant to protect. With one user and a single image that ships API and client together, the cost of expand-then-contract is small.

## Considered Options

- **Reversible migrations, CI runs up → down → up**: rejected; it tests that down steps run, not that they preserve data, and it doubles the work per schema change.

## Consequences

- CI applies all migrations to an empty PostgreSQL 18 database, fails if a migration file already on `master` was changed, and fails on duplicate or out-of-order version numbers.
- A rename or type change takes two PRs: add and backfill the new shape, then remove the old one after the image that stops using it is in production.
- The API still exits at startup if migrations are pending (ADR-0004); it must not exit because the schema is *newer* than it expects.
