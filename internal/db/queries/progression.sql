-- name: UpsertProgressionState :exec
INSERT INTO progression_state (program_id, exercise, label, state)
VALUES ($1, $2, $3, $4)
ON CONFLICT (program_id, exercise, label) DO UPDATE
SET state = EXCLUDED.state, updated_at = now();

-- name: ListProgressionState :many
SELECT * FROM progression_state WHERE program_id = $1 ORDER BY exercise, label;

-- name: DeleteProgressionState :exec
DELETE FROM progression_state WHERE program_id = $1 AND exercise = $2 AND label = $3;
