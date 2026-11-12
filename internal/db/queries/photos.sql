-- name: CreatePhoto :one
INSERT INTO photos (user_id, bytes, content_type) VALUES ($1, $2, $3) RETURNING *;

-- name: GetPhoto :one
SELECT * FROM photos WHERE id = $1 AND user_id = $2;

-- name: DeletePhoto :exec
DELETE FROM photos WHERE id = $1 AND user_id = $2;

-- name: ListPhotosByID :many
SELECT * FROM photos WHERE user_id = $1 AND id = ANY(@ids::uuid[]);
