-- Users, sessions, settings and categories. User-owned tables carry user_id (ADR 0009).

-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL UNIQUE CHECK (email = lower(email)),
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE settings (
    user_id uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    primary_currency char(3) NOT NULL,
    time_zone text NOT NULL
);

CREATE TABLE categories (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('expense', 'income')),
    name text NOT NULL,
    position integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    -- The target of composite foreign keys from entries and budget targets (ADR 0009).
    UNIQUE (user_id, id)
);

CREATE INDEX categories_user_id_kind_position_idx ON categories (user_id, kind, position);
