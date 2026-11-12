-- +goose Up
-- The program's progression state as it was just before the record's finalize ran its
-- progressions; edits to a finalized record replay from it (R79).
ALTER TABLE workout_records ADD COLUMN state_before jsonb;

-- +goose Down
ALTER TABLE workout_records DROP COLUMN state_before;
