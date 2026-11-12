-- name: UpsertExercise :one
INSERT INTO exercises (slug, name, equipment, aliases, primary_muscles, secondary_muscles,
                       category, default_warmup, images, instructions, source, user_id)
VALUES (@slug, @name, @equipment, coalesce(@aliases::text[], '{}'), coalesce(@primary_muscles::text[], '{}'), coalesce(@secondary_muscles::text[], '{}'),
        @category, @default_warmup, coalesce(@images::text[], '{}'), coalesce(@instructions::text[], '{}'), @source, @user_id)
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name, equipment = EXCLUDED.equipment, aliases = EXCLUDED.aliases,
    primary_muscles = EXCLUDED.primary_muscles, secondary_muscles = EXCLUDED.secondary_muscles,
    category = EXCLUDED.category, default_warmup = EXCLUDED.default_warmup,
    images = EXCLUDED.images, instructions = EXCLUDED.instructions, source = EXCLUDED.source
WHERE exercises.user_id IS NOT DISTINCT FROM EXCLUDED.user_id
RETURNING *;

-- name: GetExerciseBySlug :one
SELECT * FROM exercises
WHERE slug = @slug AND (user_id IS NULL OR user_id = @user_id);

-- name: ListExercises :many
SELECT * FROM exercises
WHERE user_id IS NULL OR user_id = $1
ORDER BY name;

-- name: DeleteCustomExercise :execrows
-- One statement, so the exercise and the user's data for it go together.
WITH gone AS (
    DELETE FROM exercises WHERE exercises.slug = @slug AND exercises.user_id = @user_id RETURNING exercises.slug
), data AS (
    DELETE FROM user_exercise_data WHERE user_exercise_data.user_id = @user_id AND exercise_slug IN (SELECT slug FROM gone)
)
SELECT slug FROM gone;

-- name: ResolveExerciseName :one
-- A user's own exercise wins over a catalog one with the same name.
SELECT * FROM exercises
WHERE (user_id IS NULL OR user_id = @user_id)
  AND (lower(name) = @name::text OR EXISTS (SELECT 1 FROM unnest(aliases) a WHERE lower(a) = @name::text))
-- Own exercises first, then a name match over an alias match, then curated rows.
ORDER BY user_id IS NULL, (lower(name) = @name::text) DESC, (source = 'curated') DESC, slug
LIMIT 1;

-- name: SearchCatalog :many
-- Empty equipment, muscle or query means no filter. Name matches outrank alias-only matches.
SELECT * FROM exercises
WHERE (user_id IS NULL OR user_id = @user_id)
  AND (@equipment::text = '' OR equipment = @equipment::text)
  AND (@muscle::text = '' OR @muscle::text = ANY(primary_muscles) OR @muscle::text = ANY(secondary_muscles))
  AND (@q::text = ''
       OR name % @q::text OR strpos(lower(name), lower(@q::text)) > 0
       OR EXISTS (SELECT 1 FROM unnest(aliases) a WHERE a % @q::text OR strpos(lower(a), lower(@q::text)) > 0))
ORDER BY (lower(name) = lower(@q::text)) DESC,
         (strpos(lower(name), lower(@q::text)) > 0) DESC,
         similarity(name, @q::text) DESC,
         name
LIMIT @max_rows;

-- name: ListStaleCatalog :many
-- Catalog rows an import no longer produces, and whether a user still has data on them.
SELECT slug, EXISTS (SELECT 1 FROM user_exercise_data d WHERE d.exercise_slug = e.slug) AS referenced
FROM exercises e
WHERE e.user_id IS NULL AND e.source IN ('curated', 'free-exercise-db') AND e.slug <> ALL(@keep::text[]);

-- name: DeleteCatalogExercises :exec
DELETE FROM exercises WHERE user_id IS NULL AND slug = ANY(@slugs::text[]);
