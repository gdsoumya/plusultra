-- +goose Up
ALTER TABLE exercises ADD COLUMN instructions text[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE exercises DROP COLUMN instructions;
