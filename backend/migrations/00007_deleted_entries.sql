-- Soft-deleted entries: Undo clears deleted_at, and deleted rows are never purged.

-- +goose Up
ALTER TABLE entries ADD COLUMN deleted_at timestamptz;
