-- name: UpsertFoodConcept :exec
INSERT INTO food_concepts (id, name, aliases, category, preferred_source, preferred_source_id)
VALUES (@id, @name, coalesce(@aliases::text[], '{}'), @category, @preferred_source, @preferred_source_id)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name, aliases = EXCLUDED.aliases, category = EXCLUDED.category,
    preferred_source = EXCLUDED.preferred_source, preferred_source_id = EXCLUDED.preferred_source_id;

-- name: AssignConceptMembers :many
UPDATE foods SET concept_id = @concept_id
WHERE user_id IS NULL AND source || ':' || source_id = ANY(@keys::text[])
RETURNING source, source_id;

-- name: ClearStaleConceptMembers :exec
UPDATE foods SET concept_id = NULL
WHERE concept_id = @concept_id AND source || ':' || source_id <> ALL(@keys::text[]);

-- name: PruneFoodConcepts :exec
DELETE FROM food_concepts WHERE id <> ALL(@ids::text[]);

-- name: ListConceptMembers :many
SELECT f.* FROM foods f JOIN food_concepts c ON c.id = f.concept_id
WHERE f.concept_id = @concept_id
ORDER BY
  (c.preferred_source = f.source AND c.preferred_source_id = f.source_id) DESC,
  (f.source IN ('ifct', 'usda_foundation')) DESC,
  f.source, f.source_id;

-- name: ResolveCustomFood :one
SELECT * FROM foods
WHERE user_id = @user_id AND source = 'custom'
  AND (lower(name) = @name::text OR EXISTS (SELECT 1 FROM unnest(aliases) a WHERE lower(a) = @name::text))
ORDER BY (lower(name) = @name::text) DESC, id
LIMIT 1;

-- name: ResolveFoodConcept :one
SELECT * FROM food_concepts c
WHERE lower(c.name) = @name::text OR EXISTS (SELECT 1 FROM unnest(c.aliases) a WHERE lower(a) = @name::text)
ORDER BY (lower(c.name) = @name::text) DESC, c.id
LIMIT 1;

-- name: ResolveCatalogFood :many
-- Exact name or alias matches first (best source first), else the foods whose name starts with @name.
SELECT * FROM foods
WHERE user_id IS NULL
  AND (lower(name) = @name::text OR EXISTS (SELECT 1 FROM unnest(aliases) a WHERE lower(a) = @name::text)
       OR starts_with(lower(name), @name::text))
ORDER BY (lower(name) = @name::text) DESC,
  EXISTS (SELECT 1 FROM unnest(aliases) a WHERE lower(a) = @name::text) DESC,
  (source IN ('ifct', 'usda_foundation')) DESC, source, source_id
LIMIT 50;

-- name: GetFoodBySource :one
SELECT * FROM foods
WHERE source = @source AND source_id = @source_id AND (user_id IS NULL OR user_id = @user_id);

-- name: SuggestFoodNames :many
-- Closest names among concepts (by name and alias), catalog foods and the user's foods.
SELECT name::text AS name FROM (
  SELECT c.name, greatest(word_similarity(@name::text, c.name),
    (SELECT coalesce(max(word_similarity(@name::text, a)), 0) FROM unnest(c.aliases) a)) AS s
  FROM food_concepts c
  UNION ALL
  SELECT f.name, word_similarity(@name::text, f.name) FROM foods f WHERE f.user_id IS NULL OR f.user_id = @user_id
) m
WHERE s > 0.4
GROUP BY name
ORDER BY max(s) DESC, length(name), name
LIMIT 3;

-- name: GetFoodConcept :one
SELECT * FROM food_concepts WHERE id = $1;
