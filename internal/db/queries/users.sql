-- name: CreateUser :one
INSERT INTO users (username, email, password_hash, display_name, units, timezone, is_admin)
VALUES (@username, @email, @password_hash, @display_name, @units, @timezone, @is_admin)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: LockRegistration :exec
-- Serializes registrations so only the first user becomes admin.
SELECT pg_advisory_xact_lock(hashtext('plusultra.register'));

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at, id;

-- name: UpdateUserSettings :one
-- A null argument keeps the current value.
UPDATE users SET
    display_name     = coalesce(sqlc.narg(display_name), display_name),
    units            = coalesce(sqlc.narg(units), units),
    timezone         = coalesce(sqlc.narg(timezone), timezone),
    measurement_unit = coalesce(sqlc.narg(measurement_unit), measurement_unit)
WHERE id = @id
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
