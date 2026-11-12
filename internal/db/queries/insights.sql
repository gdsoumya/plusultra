-- name: ListPlannedWorkoutDates :many
SELECT DISTINCT rd.date
FROM resolved_days rd
JOIN programs p ON p.id = rd.program_id
WHERE p.user_id = @user_id AND p.kind = 'workout' AND p.deleted_at IS NULL
  AND p.status IN ('active', 'finished')
  AND rd.date BETWEEN @from_date AND @to_date
ORDER BY rd.date;

-- name: ListFinalizedWorkoutRecords :many
SELECT * FROM workout_records
WHERE user_id = @user_id AND date BETWEEN @from_date AND @to_date
  AND finalized AND deleted_at IS NULL
ORDER BY date, started_at NULLS LAST, id;
