-- name: CreateSettings :exec
INSERT INTO settings (user_id, primary_currency, time_zone) VALUES ($1, $2, $3);

-- name: Settings :one
SELECT primary_currency, time_zone FROM settings WHERE user_id = $1;
