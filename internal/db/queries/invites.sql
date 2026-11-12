-- name: CreateInvite :one
INSERT INTO invites (code, created_by, expires_at) VALUES ($1, $2, $3) RETURNING *;

-- name: GetUsableInvite :one
SELECT * FROM invites
WHERE code = $1 AND used_at IS NULL AND expires_at > now()
FOR UPDATE;

-- name: ConsumeInvite :execrows
UPDATE invites SET used_by = @used_by, used_at = now()
WHERE code = @code AND used_at IS NULL AND expires_at > now();

-- name: ListInvites :many
SELECT sqlc.embed(i), u.username AS used_by_username
FROM invites i LEFT JOIN users u ON u.id = i.used_by
ORDER BY i.created_at DESC, i.code;

-- name: DeleteInvite :exec
DELETE FROM invites WHERE code = $1;
