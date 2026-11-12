-- name: CreateProgram :one
INSERT INTO programs (user_id, kind, name, script, structure, status, start_date, end_date)
VALUES (@user_id, @kind, @name, @script, @structure, @status, @start_date, @end_date)
RETURNING *;

-- name: UpsertProgram :one
INSERT INTO programs (id, user_id, kind, name, script, revision, structure, status, start_date, end_date)
VALUES (@id, @user_id, @kind, @name, @script, @revision, @structure, @status, @start_date, @end_date)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name, script = EXCLUDED.script, revision = EXCLUDED.revision,
    structure = EXCLUDED.structure, status = EXCLUDED.status,
    start_date = EXCLUDED.start_date, end_date = EXCLUDED.end_date,
    updated_at = now(), deleted_at = NULL
WHERE programs.user_id = EXCLUDED.user_id
RETURNING *;

-- name: GetProgram :one
SELECT * FROM programs WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: ListProgramsByUser :many
SELECT * FROM programs WHERE user_id = $1 AND deleted_at IS NULL ORDER BY created_at, id;

-- name: ListProgramsByUserAndStatus :many
SELECT * FROM programs
WHERE user_id = $1 AND status = $2 AND deleted_at IS NULL
ORDER BY created_at, id;

-- name: SetProgramStatus :one
UPDATE programs SET status = @status, updated_at = now()
WHERE id = @id AND user_id = @user_id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteProgram :execrows
UPDATE programs SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: InsertRevision :one
INSERT INTO program_revisions (program_id, revision, script, structure, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListRevisions :many
SELECT * FROM program_revisions WHERE program_id = $1 ORDER BY revision DESC;

-- name: GetRevision :one
SELECT * FROM program_revisions WHERE program_id = $1 AND revision = $2;

-- name: GetCursor :one
SELECT * FROM program_cursor WHERE program_id = $1;

-- name: UpsertCursor :one
INSERT INTO program_cursor (program_id, next_week, next_day)
VALUES ($1, $2, $3)
ON CONFLICT (program_id) DO UPDATE
SET next_week = EXCLUDED.next_week, next_day = EXCLUDED.next_day, updated_at = now()
RETURNING *;

-- name: ListActivePrograms :many
SELECT * FROM programs WHERE status = 'active' AND deleted_at IS NULL ORDER BY user_id, created_at, id;
