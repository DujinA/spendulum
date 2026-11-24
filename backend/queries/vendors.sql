-- name: UseVendor :one
INSERT INTO vendors (id, user_id, name, key)
VALUES (@id, @user_id, @name, @key)
ON CONFLICT (user_id, key) DO UPDATE SET archived = false WHERE vendors.user_id = @user_id
RETURNING id;

-- name: ArchiveVendor :execrows
UPDATE vendors SET archived = true WHERE user_id = $1 AND id = $2;

-- name: Vendors :many
SELECT id, name, key, archived FROM vendors WHERE user_id = $1;

-- name: VendorUses :many
SELECT DISTINCT ON (e.vendor_id)
    e.vendor_id::uuid AS vendor_id,
    count(*) OVER (PARTITION BY e.vendor_id) AS use_count,
    e.category_id,
    c.name AS category_name,
    e.currency
FROM entries e
JOIN categories c ON c.user_id = e.user_id AND c.id = e.category_id
WHERE e.user_id = @user_id AND e.kind = @kind AND e.vendor_id IS NOT NULL AND e.deleted_at IS NULL
ORDER BY e.vendor_id, e.entry_date DESC, e.created_at DESC, e.id DESC;
