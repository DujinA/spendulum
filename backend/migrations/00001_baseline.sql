-- The baseline migration. It creates nothing: it gives Goose a first version, so
-- `spendulum migrate` has a migration to apply and the API has one to check for.
-- Tables arrive with the slices that need them (ADR 0008, ADR 0009).

-- +goose Up
SELECT 1;
