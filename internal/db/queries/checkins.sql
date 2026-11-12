-- name: UpsertCheckin :one
INSERT INTO checkins (id, user_id, date, weight, body_fat, measurements, photo_id, device_id)
VALUES (@id, @user_id, @date, @weight, @body_fat, @measurements, @photo_id, @device_id)
ON CONFLICT (id) DO UPDATE SET
    date = EXCLUDED.date, weight = EXCLUDED.weight, body_fat = EXCLUDED.body_fat,
    measurements = EXCLUDED.measurements, photo_id = EXCLUDED.photo_id,
    device_id = EXCLUDED.device_id, version = checkins.version + 1,
    updated_at = clock_timestamp(), deleted_at = NULL
WHERE checkins.user_id = EXCLUDED.user_id
RETURNING *;

-- name: GetCheckin :one
SELECT * FROM checkins WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: ListCheckinsByDate :many
SELECT * FROM checkins
WHERE user_id = @user_id AND date BETWEEN @from_date AND @to_date AND deleted_at IS NULL
ORDER BY date, id;

-- name: DeleteCheckin :execrows
UPDATE checkins SET deleted_at = now(), updated_at = clock_timestamp(), version = version + 1
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: ListCheckinsChangedSince :many
SELECT * FROM checkins
WHERE user_id = @user_id AND (updated_at, id) > (@since::timestamptz, @since_id::uuid)
  AND updated_at < now() - interval '5 seconds'
ORDER BY updated_at, id
LIMIT @max_rows;

-- name: GetCheckinByDate :one
SELECT * FROM checkins
WHERE user_id = $1 AND date = $2 AND deleted_at IS NULL
ORDER BY updated_at DESC LIMIT 1;

-- name: ListCheckinsPage :many
SELECT * FROM checkins
WHERE user_id = @user_id AND date BETWEEN @from_date AND @to_date AND deleted_at IS NULL
  AND (date, id) > (@after_date::date, @after_id::uuid)
ORDER BY date, id
LIMIT @max_rows;
