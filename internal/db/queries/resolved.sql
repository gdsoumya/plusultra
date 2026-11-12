-- name: UpsertResolvedDay :exec
INSERT INTO resolved_days (program_id, week, day, date, kind, payload)
VALUES (@program_id, @week, @day, @date, @kind, @payload)
ON CONFLICT (program_id, week, day) DO UPDATE
SET date = EXCLUDED.date, kind = EXCLUDED.kind, payload = EXCLUDED.payload;

-- name: GetResolvedDay :one
SELECT * FROM resolved_days WHERE program_id = $1 AND week = $2 AND day = $3;

-- name: ListResolvedDays :many
SELECT * FROM resolved_days WHERE program_id = $1 ORDER BY week, day;

-- name: ListResolvedDaysByWeek :many
SELECT * FROM resolved_days WHERE program_id = $1 AND week = $2 ORDER BY day;

-- name: ListResolvedDaysByDateRange :many
SELECT * FROM resolved_days
WHERE program_id = @program_id AND date BETWEEN @from_date AND @to_date
ORDER BY date;

-- name: DeleteResolvedDays :exec
DELETE FROM resolved_days WHERE program_id = $1;
