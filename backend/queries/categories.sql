-- name: CreateCategory :exec
INSERT INTO categories (id, user_id, kind, name, position, archived) VALUES ($1, $2, $3, $4, $5, $6);

-- name: Categories :many
SELECT id, kind, name, position, archived FROM categories WHERE user_id = $1 ORDER BY kind, position, created_at;

-- name: UnarchivedCategories :many
SELECT id, kind, name, position, archived FROM categories WHERE user_id = $1 AND NOT archived ORDER BY kind, position, created_at;
