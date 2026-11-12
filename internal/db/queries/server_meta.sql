-- name: GetServerMeta :one
SELECT value FROM server_meta WHERE key = $1;

-- name: SetServerMeta :exec
INSERT INTO server_meta (key, value) VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
