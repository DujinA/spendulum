-- Category notes, unique names per kind, and idempotency keys for creating categories. User-owned tables carry user_id (ADR 0009).

-- +goose Up
ALTER TABLE categories ADD COLUMN note text NOT NULL DEFAULT '';

-- Set by the application with namekey.Of; nullable so the previous image, which never sets it, still creates accounts (ADR 0008).
ALTER TABLE categories ADD COLUMN name_key text;

-- Until now only the create-account operation made categories, so every existing name is an ASCII "Other".
UPDATE categories SET name_key = lower(regexp_replace(name, '[^A-Za-z0-9]', '', 'g'));

CREATE UNIQUE INDEX categories_user_id_kind_name_key_key ON categories (user_id, kind, name_key);

CREATE TABLE category_idempotency_keys (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    key text NOT NULL,
    request_hash bytea NOT NULL,
    category_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, key),
    FOREIGN KEY (user_id, category_id) REFERENCES categories (user_id, id) ON DELETE CASCADE
);
