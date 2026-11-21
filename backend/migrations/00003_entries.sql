-- Entries, their idempotency keys, and archived categories. User-owned tables carry user_id (ADR 0009).

-- +goose Up
ALTER TABLE categories ADD COLUMN archived boolean NOT NULL DEFAULT false;

-- The target of the entries foreign key, which keeps every entry in a category of its own kind.
ALTER TABLE categories ADD CONSTRAINT categories_user_id_id_kind_key UNIQUE (user_id, id, kind);

CREATE TABLE entries (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('expense', 'income')),
    amount bigint NOT NULL CHECK (amount <> 0 AND (kind = 'expense' OR amount > 0)),
    currency char(3) NOT NULL,
    category_id uuid NOT NULL,
    entry_date date NOT NULL,
    note text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, id),
    FOREIGN KEY (user_id, category_id, kind) REFERENCES categories (user_id, id, kind)
);

CREATE INDEX entries_user_id_list_idx ON entries (user_id, entry_date DESC, created_at DESC, id DESC);

CREATE TABLE idempotency_keys (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    key text NOT NULL,
    request_hash bytea NOT NULL,
    entry_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, key),
    FOREIGN KEY (user_id, entry_id) REFERENCES entries (user_id, id) ON DELETE CASCADE
);
