-- FOR SHARE waits for a password reset in flight, so a login that verified the old password inserts nothing.
-- name: CreateSession :execrows
INSERT INTO sessions (id, user_id, token_hash, expires_at)
SELECT @id, @user_id, @token_hash, @expires_at
WHERE EXISTS (
    SELECT 1 FROM users WHERE id = @user_id AND password_hash = @verified_hash FOR SHARE
);

-- unscoped: the session token is what identifies the user.
-- name: SessionByTokenHash :one
SELECT id, user_id, expires_at FROM sessions WHERE token_hash = $1 AND expires_at > $2;

-- name: RenewSession :exec
UPDATE sessions SET expires_at = $3 WHERE user_id = $1 AND id = $2;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE user_id = $1 AND id = $2;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = $1;
