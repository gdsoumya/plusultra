-- +goose Up
CREATE TABLE food_concepts (
    id                  text PRIMARY KEY,
    name                text NOT NULL,
    aliases             text[] NOT NULL DEFAULT '{}',
    category            text NOT NULL DEFAULT '',
    preferred_source    text NOT NULL,
    preferred_source_id text NOT NULL,
    search              tsvector GENERATED ALWAYS AS (
        to_tsvector('simple', name || ' ' || immutable_join(aliases))
    ) STORED
);
CREATE INDEX food_concepts_search_idx ON food_concepts USING gin (search);

ALTER TABLE foods ADD COLUMN concept_id text REFERENCES food_concepts ON DELETE SET NULL;
CREATE INDEX foods_concept_idx ON foods (concept_id) WHERE concept_id IS NOT NULL;

-- +goose Down
ALTER TABLE foods DROP COLUMN concept_id;
DROP TABLE food_concepts;
