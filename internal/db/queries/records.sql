-- name: ListWorkoutRecordsPage :many
SELECT * FROM workout_records
WHERE user_id = @user_id AND date BETWEEN @from_date AND @to_date AND deleted_at IS NULL
  AND (sqlc.narg('program_id')::uuid IS NULL OR program_id = sqlc.narg('program_id'))
  AND (date, id) > (@after_date::date, @after_id::uuid)
ORDER BY date, id
LIMIT @max_rows;
