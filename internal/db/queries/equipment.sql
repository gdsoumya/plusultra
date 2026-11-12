-- name: UpsertEquipment :one
INSERT INTO equipment (user_id, name, config) VALUES ($1, $2, $3)
ON CONFLICT (user_id, name) DO UPDATE SET config = EXCLUDED.config
RETURNING *;

-- name: ListEquipment :many
SELECT * FROM equipment WHERE user_id = $1 ORDER BY name;

-- name: DeleteEquipment :exec
DELETE FROM equipment WHERE user_id = $1 AND name = $2;

-- name: SeedEquipment :exec
INSERT INTO equipment (user_id, name, config) VALUES ($1, $2, $3)
ON CONFLICT (user_id, name) DO NOTHING;
