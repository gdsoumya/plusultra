-- name: UpsertWorkoutRecord :one
INSERT INTO workout_records (id, user_id, date, program_id, resolved_day, entries,
                             started_at, finished_at, finalized, device_id)
VALUES (@id, @user_id, @date, @program_id, @resolved_day, @entries,
        @started_at, @finished_at, @finalized, @device_id)
ON CONFLICT (id) DO UPDATE SET
    date = EXCLUDED.date, program_id = EXCLUDED.program_id, resolved_day = EXCLUDED.resolved_day,
    entries = EXCLUDED.entries, started_at = EXCLUDED.started_at, finished_at = EXCLUDED.finished_at,
    finalized = EXCLUDED.finalized, device_id = EXCLUDED.device_id,
    version = workout_records.version + 1, updated_at = clock_timestamp(), deleted_at = NULL
WHERE workout_records.user_id = EXCLUDED.user_id
RETURNING *;

-- name: GetWorkoutRecord :one
SELECT * FROM workout_records WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: ListWorkoutRecordsByDate :many
SELECT * FROM workout_records
WHERE user_id = @user_id AND date BETWEEN @from_date AND @to_date AND deleted_at IS NULL
ORDER BY date, started_at NULLS LAST, id;

-- name: DeleteWorkoutRecord :execrows
UPDATE workout_records SET deleted_at = now(), updated_at = clock_timestamp(), version = version + 1
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: ListWorkoutRecordsChangedSince :many
SELECT * FROM workout_records
WHERE user_id = @user_id AND (updated_at, id) > (@since::timestamptz, @since_id::uuid)
  AND updated_at < now() - interval '5 seconds'
ORDER BY updated_at, id
LIMIT @max_rows;

-- name: SetWorkoutRecordStateBefore :exec
UPDATE workout_records SET state_before = @state_before WHERE id = @id AND user_id = @user_id;

-- name: ListFinalizedProgramRecords :many
SELECT * FROM workout_records
WHERE user_id = @user_id AND program_id = @program_id AND finalized AND deleted_at IS NULL
ORDER BY date, id;
