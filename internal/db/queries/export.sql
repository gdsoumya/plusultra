-- name: ListCustomExercises :many
SELECT * FROM exercises WHERE user_id = $1 ORDER BY name;

-- name: ListCustomFoods :many
SELECT * FROM foods WHERE user_id = $1 ORDER BY name;

-- name: ListProgramNames :many
-- Deleted programs included: their workout records still name them.
SELECT id, name FROM programs WHERE user_id = $1;
