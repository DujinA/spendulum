-- name: CreateCategory :exec
INSERT INTO categories (id, user_id, kind, name, position) VALUES ($1, $2, $3, $4, $5);

-- name: Categories :many
SELECT id, kind, name, position FROM categories WHERE user_id = $1 ORDER BY kind, position, created_at;

