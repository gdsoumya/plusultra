-- Sync reads rows whatever their state: tombstones take part in last-write-wins.

-- name: GetDayLogForSync :one
SELECT * FROM day_logs WHERE user_id = $1 AND date = $2;

-- name: GetWorkoutRecordForSync :one
SELECT * FROM workout_records WHERE id = $1 AND user_id = $2;

-- name: GetMealLogForSync :one
SELECT * FROM meal_logs WHERE id = $1 AND user_id = $2;

-- name: GetCheckinForSync :one
SELECT * FROM checkins WHERE id = $1 AND user_id = $2;

-- ListResolvedDaysFromCursor returns a floating program's days from (next_week, next_day) on,
-- then, when wrap is set, the days before it.
-- name: ListResolvedDaysFromCursor :many
SELECT * FROM resolved_days
WHERE program_id = @program_id AND (@wrap::boolean OR (week, day) >= (@next_week::int, @next_day::int))
ORDER BY (week, day) < (@next_week::int, @next_day::int), week, day
LIMIT @max_rows;
