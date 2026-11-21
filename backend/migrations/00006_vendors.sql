-- Vendors, and the optional vendor of an entry. User-owned tables carry user_id (ADR 0009).

-- +goose Up
CREATE TABLE vendors (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name text NOT NULL,
    -- namekey.Of(name): names that differ only in case, diacritics, spaces or punctuation are one vendor.
    key text NOT NULL CHECK (key <> ''),
    archived boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, id),
    UNIQUE (user_id, key)
);

ALTER TABLE entries ADD COLUMN vendor_id uuid;

ALTER TABLE entries ADD CONSTRAINT entries_vendor_fkey FOREIGN KEY (user_id, vendor_id) REFERENCES vendors (user_id, id);

-- Use counts and the latest entry per vendor and kind.
CREATE INDEX entries_user_id_vendor_id_idx ON entries (user_id, vendor_id, kind, entry_date DESC, created_at DESC) WHERE vendor_id IS NOT NULL;
