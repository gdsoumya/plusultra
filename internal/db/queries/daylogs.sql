-- UpsertDayLog keeps a client id for a new log; a date's existing log keeps its own.
-- name: UpsertDayLog :one
INSERT INTO day_logs (id, user_id, date, status, notes, readiness, device_id)
VALUES (COALESCE(sqlc.narg('id')::uuid, gen_random_uuid()), @user_id, @date, @status, @notes, @readiness, @device_id)
ON CONFLICT (user_id, date) DO UPDATE SET
    status = EXCLUDED.status, notes = EXCLUDED.notes, readiness = EXCLUDED.readiness,
    device_id = EXCLUDED.device_id, version = day_logs.version + 1,
    updated_at = clock_timestamp(), deleted_at = NULL
RETURNING *;

-- name: GetDay :one
SELECT * FROM day_logs WHERE user_id = $1 AND date = $2 AND deleted_at IS NULL;

-- name: ListDays :many
SELECT * FROM day_logs
WHERE user_id = @user_id AND date BETWEEN @from_date AND @to_date AND deleted_at IS NULL
ORDER BY date;

-- name: DeleteDayLog :execrows
UPDATE day_logs SET deleted_at = now(), updated_at = clock_timestamp(), version = version + 1
WHERE user_id = $1 AND date = $2 AND deleted_at IS NULL;

-- name: ListDaysChangedSince :many
SELECT * FROM day_logs
WHERE user_id = @user_id AND (updated_at, id) > (@since::timestamptz, @since_id::uuid)
  AND updated_at < now() - interval '5 seconds'
ORDER BY updated_at, id
LIMIT @max_rows;
