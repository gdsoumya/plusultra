-- +goose Up
ALTER TABLE users ADD COLUMN measurement_unit text NOT NULL DEFAULT 'cm' CHECK (measurement_unit IN ('cm', 'in'));

-- +goose Down
ALTER TABLE users DROP COLUMN measurement_unit;
