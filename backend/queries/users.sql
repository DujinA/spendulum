-- name: CreateUser :exec
INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3);

-- unscoped: finds the user that is logging in, before any user is known.
-- name: UserByEmail :one
SELECT id, email, password_hash FROM users WHERE email = $1;

-- name: UserEmail :one
SELECT email FROM users WHERE id = $1;

-- name: SetPasswordHash :execrows
UPDATE users SET password_hash = $2 WHERE id = $1;
