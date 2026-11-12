-- name: LockUserPrograms :exec
-- Serialises a user's program writes: one active program per kind, and meal days built from a settled
-- workout calendar. Taken before any program row lock.
SELECT 1 FROM users WHERE id = $1 FOR UPDATE;

-- name: SetProgramSchedule :one
UPDATE programs SET status = @status, start_date = @start_date, end_date = @end_date, updated_at = now()
WHERE id = @id AND user_id = @user_id AND deleted_at IS NULL
RETURNING *;

-- name: UpdateProgramScript :one
UPDATE programs SET name = @name, script = @script, structure = @structure, revision = revision + 1, updated_at = now()
WHERE id = @id AND user_id = @user_id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteResolvedDaysFrom :exec
-- Floating days have no date and always go.
DELETE FROM resolved_days WHERE program_id = @program_id AND (date IS NULL OR date >= @from_date);

-- name: LatestBodyweight :one
SELECT weight::float8 FROM checkins
WHERE user_id = $1 AND weight IS NOT NULL AND deleted_at IS NULL
ORDER BY date DESC, updated_at DESC
LIMIT 1;

-- name: ListProgramsPage :many
-- Most recently changed first, after the (updated_at, id) bound; empty kind or status matches all.
SELECT * FROM programs
WHERE user_id = @user_id AND deleted_at IS NULL
  AND (@kind::text = '' OR kind = @kind::text)
  AND (@status::text = '' OR status = @status::text)
  AND (updated_at, id) < (@before_updated::timestamptz, @before_id::uuid)
ORDER BY updated_at DESC, id DESC
LIMIT @max_rows;

-- name: ListRevisionsPage :many
SELECT * FROM program_revisions
WHERE program_id = @program_id AND revision < @before_revision
ORDER BY revision DESC
LIMIT @max_rows;

-- name: GetProgramForUpdate :one
-- Serialises writes to one program: an update cannot land between another write's read and commit.
SELECT * FROM programs WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL FOR UPDATE;
