-- name: UpsertUserExerciseData :one
INSERT INTO user_exercise_data (user_id, exercise_slug, rm1, rounding, notes, unilateral)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, exercise_slug) DO UPDATE
SET rm1 = EXCLUDED.rm1, rounding = EXCLUDED.rounding, notes = EXCLUDED.notes, unilateral = EXCLUDED.unilateral, updated_at = now()
RETURNING *;

-- name: GetUserExerciseData :one
SELECT * FROM user_exercise_data WHERE user_id = $1 AND exercise_slug = $2;

-- name: ListUserExerciseData :many
SELECT * FROM user_exercise_data WHERE user_id = $1 ORDER BY exercise_slug;

-- name: DeleteUserExerciseData :exec
DELETE FROM user_exercise_data WHERE user_id = $1 AND exercise_slug = $2;
