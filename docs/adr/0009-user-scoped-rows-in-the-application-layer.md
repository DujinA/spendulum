# Every user-owned row carries user_id, scoped in the application layer

v1 has one user, but every user-owned table (entries, vendors, categories, budget target versions, settings, idempotency keys, sessions, imports) has `user_id NOT NULL` from its first migration, and every uniqueness rule includes it. Only currencies and exchange rates are shared. Scoping is enforced in the application layer: every repository method takes the user id first and every query on a user-owned table filters on it. References between user-owned rows are composite foreign keys that include `user_id` (for example `entries(user_id, category_id)` → `categories(user_id, id)`), so no row can point at another user's data even if application code is wrong.

Sign-ups on this instance are a later effort that v1 must not make expensive. Under forward-only additive migrations (ADR-0008), adding `user_id` to every table afterwards would be an expand-then-contract change on each one, with a backfill, while the app is in daily use. Adding it before the first row exists costs one column and a few wider indexes.

## Considered Options

- **No `user_id` until sign-ups**: rejected; the cheapest moment to add it is before any data exists, and every query would have to be revisited later anyway.
- **PostgreSQL row-level security**: not in v1. It needs a non-owner database role and `SET LOCAL app.user_id` in every transaction, and it complicates migrations and the operator CLI. Two-user tests and a query lint catch the same mistakes here. It stays possible later as defence in depth.

## Consequences

- Each repository has a two-user integration test proving one user cannot read or change another's rows; CI fails on any query on a user-owned table that has no `user_id` predicate.
- The API never carries a user id; the user comes only from the session. A row belonging to another user is answered with `404`, never `403`.
- Ids are UUIDv7, never sequential.
- User-owned defaults (the Other category of each kind, default settings) are created by the create-account operation, never seeded by a migration.
