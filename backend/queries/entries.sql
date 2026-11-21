-- name: CreateEntry :exec
INSERT INTO entries (id, user_id, kind, amount, currency, category_id, vendor_id, entry_date, note, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: Entry :one
SELECT e.id, e.kind, e.amount, e.currency, e.category_id, c.name AS category_name, e.vendor_id, v.name AS vendor_name,
    e.entry_date, e.note, e.created_at
FROM entries e
JOIN categories c ON c.user_id = e.user_id AND c.id = e.category_id
LEFT JOIN vendors v ON v.user_id = e.user_id AND v.id = e.vendor_id
WHERE e.user_id = $1 AND e.id = $2;

-- name: Entries :many
SELECT e.id, e.kind, e.amount, e.currency, e.category_id, c.name AS category_name, e.vendor_id, v.name AS vendor_name,
    e.entry_date, e.note, e.created_at
FROM entries e
JOIN categories c ON c.user_id = e.user_id AND c.id = e.category_id
LEFT JOIN vendors v ON v.user_id = e.user_id AND v.id = e.vendor_id
WHERE e.user_id = @user_id
ORDER BY e.entry_date DESC, e.created_at DESC, e.id DESC
LIMIT @max_rows;

-- name: EntriesAfter :many
SELECT e.id, e.kind, e.amount, e.currency, e.category_id, c.name AS category_name, e.vendor_id, v.name AS vendor_name,
    e.entry_date, e.note, e.created_at
FROM entries e
JOIN categories c ON c.user_id = e.user_id AND c.id = e.category_id
LEFT JOIN vendors v ON v.user_id = e.user_id AND v.id = e.vendor_id
WHERE e.user_id = @user_id
    AND (e.entry_date, e.created_at, e.id) < (@entry_date::date, @created_at::timestamptz, @id::uuid)
ORDER BY e.entry_date DESC, e.created_at DESC, e.id DESC
LIMIT @max_rows;

-- name: DeleteExpiredIdempotencyKeys :exec
DELETE FROM idempotency_keys WHERE user_id = $1 AND created_at <= $2;

-- name: CreateIdempotencyKey :execrows
INSERT INTO idempotency_keys (user_id, key, request_hash, entry_id, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, key) DO NOTHING;

-- name: IdempotencyKey :one
SELECT request_hash, entry_id FROM idempotency_keys WHERE user_id = $1 AND key = $2;

-- unscoped: the startup exchange-rate backfill covers the entry dates of every user (#35).
-- name: EarliestEntryDate :one
SELECT entry_date FROM entries ORDER BY entry_date LIMIT 1;
