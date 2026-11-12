-- name: UpsertMealLog :one
INSERT INTO meal_logs (id, user_id, date, program_id, target, meals, device_id)
VALUES (@id, @user_id, @date, @program_id, @target, @meals, @device_id)
ON CONFLICT (id) DO UPDATE SET
    date = EXCLUDED.date, program_id = EXCLUDED.program_id, target = EXCLUDED.target,
    meals = EXCLUDED.meals, device_id = EXCLUDED.device_id,
    version = meal_logs.version + 1, updated_at = clock_timestamp(), deleted_at = NULL
WHERE meal_logs.user_id = EXCLUDED.user_id
RETURNING *;

-- name: GetMealLog :one
SELECT * FROM meal_logs WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: ListMealLogsByDate :many
SELECT * FROM meal_logs
WHERE user_id = @user_id AND date BETWEEN @from_date AND @to_date AND deleted_at IS NULL
ORDER BY date, id;

-- name: DeleteMealLog :execrows
UPDATE meal_logs SET deleted_at = now(), updated_at = clock_timestamp(), version = version + 1
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: ListMealLogsChangedSince :many
SELECT * FROM meal_logs
WHERE user_id = @user_id AND (updated_at, id) > (@since::timestamptz, @since_id::uuid)
  AND updated_at < now() - interval '5 seconds'
ORDER BY updated_at, id
LIMIT @max_rows;

-- name: GetMealLogByDate :one
SELECT * FROM meal_logs
WHERE user_id = $1 AND date = $2 AND deleted_at IS NULL
ORDER BY updated_at DESC LIMIT 1;

-- name: ListMealLogsPage :many
SELECT * FROM meal_logs
WHERE user_id = @user_id AND date BETWEEN @from_date AND @to_date AND deleted_at IS NULL
  AND (date, id) > (@after_date::date, @after_id::uuid)
ORDER BY date, id
LIMIT @max_rows;

-- name: UpdateMealLog :one
UPDATE meal_logs SET target = @target, meals = @meals, program_id = @program_id, device_id = @device_id,
    version = version + 1, updated_at = clock_timestamp()
WHERE id = @id AND user_id = @user_id AND deleted_at IS NULL
RETURNING *;
