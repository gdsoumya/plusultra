-- name: CreateAPIKey :one
INSERT INTO api_keys (user_id, name, prefix, key_hash, scopes, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys
WHERE key_hash = $1
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now());

-- name: ListAPIKeys :many
SELECT * FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC, id DESC;

-- name: TouchAPIKey :exec
UPDATE api_keys SET last_used_at = now() WHERE id = $1;

-- name: RevokeAPIKey :one
UPDATE api_keys SET revoked_at = now()
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
RETURNING *;
