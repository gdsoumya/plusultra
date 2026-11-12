-- name: UpsertFood :one
INSERT INTO foods (source, source_id, name, aliases, barcode, per100g, units, user_id)
VALUES (@source, @source_id, @name, coalesce(@aliases::text[], '{}'), @barcode, @per100g, @units, @user_id)
ON CONFLICT (source, source_id) WHERE source_id <> '' DO UPDATE SET
    name = EXCLUDED.name, aliases = EXCLUDED.aliases, barcode = EXCLUDED.barcode,
    per100g = EXCLUDED.per100g, units = EXCLUDED.units
WHERE foods.user_id IS NOT DISTINCT FROM EXCLUDED.user_id
RETURNING *;

-- name: GetFood :one
SELECT * FROM foods WHERE id = @id AND (user_id IS NULL OR user_id = @user_id);

-- name: GetFoodByBarcode :one
SELECT * FROM foods
WHERE barcode = @barcode AND (user_id IS NULL OR user_id = @user_id)
ORDER BY user_id NULLS LAST LIMIT 1;

-- name: SearchFoods :many
-- Concept members collapse to one row: the preferred member, else the best ranked one.
-- Concept names and aliases match for every member. Hits that match only by trigram similarity
-- of the concept name (typos) come after all word and prefix matches.
WITH hits AS (
  SELECT f.id, f.concept_id, f.user_id, coalesce(c.name, f.name) AS shown,
    coalesce(c.preferred_source = f.source AND c.preferred_source_id = f.source_id, false) AS is_preferred,
    NOT (f.search @@ to_tsquery('simple', @tsq::text) OR coalesce(c.search @@ to_tsquery('simple', @tsq::text), false)) AS fuzzy,
    lower(coalesce(c.name, f.name)) = @lq::text OR coalesce(@lq::text = ANY(c.aliases), false) AS exact,
    CASE WHEN f.source IN ('indb', 'ifct', 'usda_foundation') THEN 0.05 ELSE 0 END
      + CASE WHEN f.name ~* '\mraw\M' THEN 0.1 WHEN f.name ~* '\m(cooked|fried)\M' THEN -0.1 ELSE 0 END AS tier,
    greatest(ts_rank(f.search, to_tsquery('simple', @tsq::text)), coalesce(ts_rank(c.search, to_tsquery('simple', @tsq::text)), 0))
      + CASE WHEN f.concept_id IS NULL AND f.source IN ('indb', 'ifct', 'usda_foundation') THEN 0.05 ELSE 0 END
      + CASE WHEN coalesce(c.name, f.name) ~* '\mraw\M' THEN 0.1 WHEN coalesce(c.name, f.name) ~* '\m(cooked|fried)\M' THEN -0.1 ELSE 0 END AS score
  FROM foods f LEFT JOIN food_concepts c ON c.id = f.concept_id
  WHERE (f.search @@ to_tsquery('simple', @tsq::text) OR c.search @@ to_tsquery('simple', @tsq::text) OR @lq::text <<% c.name)
    AND (f.user_id IS NULL OR f.user_id = @user_id)
    AND (@source::text = '' OR starts_with(f.source, @source::text))
), picked AS (
  SELECT *, row_number() OVER (
    PARTITION BY coalesce(concept_id, id::text) ORDER BY is_preferred DESC, tier DESC, score DESC, id
  ) AS rn
  FROM hits
)
SELECT sqlc.embed(f), p.concept_id, p.shown,
  (SELECT count(DISTINCT m.source) FROM foods m WHERE m.concept_id = p.concept_id AND m.source <> f.source)::int AS other_sources
FROM picked p JOIN foods f ON f.id = p.id
WHERE p.rn = 1
ORDER BY
  p.user_id IS NOT NULL DESC,
  p.fuzzy,
  p.concept_id IS NOT NULL DESC,
  p.exact DESC,
  starts_with(lower(p.shown), @lq::text) DESC,
  round(p.score::numeric, 1) DESC,
  length(p.shown),
  p.shown
LIMIT @max_rows;

-- name: UpdateCustomFood :one
UPDATE foods SET name = @name, aliases = coalesce(@aliases::text[], '{}'), barcode = @barcode,
    per100g = @per100g, units = @units
WHERE id = @id AND user_id = @user_id AND source = 'custom'
RETURNING *;

-- name: DeleteCustomFood :execrows
DELETE FROM foods WHERE id = $1 AND user_id = $2;
