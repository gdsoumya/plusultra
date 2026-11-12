-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, expires_at, user_agent)
VALUES ($1, $2, now() + interval '90 days', $3)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions WHERE token_hash = $1 AND expires_at > now();

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now(), expires_at = now() + interval '90 days' WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteOtherSessions :exec
DELETE FROM sessions WHERE user_id = @user_id AND token_hash <> @keep_hash;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= now();
