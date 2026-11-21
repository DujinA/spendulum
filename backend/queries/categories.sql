-- name: CreateCategory :exec
INSERT INTO categories (id, user_id, kind, name, name_key, note, position, archived)
VALUES (@id, @user_id, @kind, @name, @name_key::text, @note, @position, @archived);

-- name: Categories :many
SELECT id, kind, name, note, position, archived FROM categories WHERE user_id = $1 ORDER BY kind, position, created_at;

-- name: UnarchivedCategories :many
SELECT id, kind, name, note, position, archived FROM categories WHERE user_id = $1 AND NOT archived ORDER BY kind, position, created_at;

-- name: Category :one
SELECT id, kind, name, note, position, archived FROM categories WHERE user_id = $1 AND id = $2;

-- name: CategoryByNameKey :one
SELECT id, kind, name, note, position, archived FROM categories WHERE user_id = $1 AND kind = $2 AND name_key = @name_key::text;

-- name: UpdateCategory :execrows
UPDATE categories SET name = @name, name_key = @name_key::text, note = @note, archived = @archived
WHERE user_id = @user_id AND id = @id;

-- name: SetCategoryOrder :exec
UPDATE categories SET position = o.n - 1
FROM unnest(@ids::uuid[]) WITH ORDINALITY AS o (id, n)
WHERE categories.user_id = @user_id AND categories.id = o.id;

-- name: DeleteCategory :execrows
DELETE FROM categories WHERE user_id = $1 AND id = $2;

-- name: DeleteExpiredCategoryIdempotencyKeys :exec
DELETE FROM category_idempotency_keys WHERE user_id = $1 AND created_at <= $2;

-- name: CreateCategoryIdempotencyKey :execrows
INSERT INTO category_idempotency_keys (user_id, key, request_hash, category_id, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, key) DO NOTHING;

-- name: CategoryIdempotencyKey :one
SELECT request_hash, category_id FROM category_idempotency_keys WHERE user_id = $1 AND key = $2;
