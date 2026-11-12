-- +goose Up
ALTER TABLE user_exercise_data ADD COLUMN unilateral boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE user_exercise_data DROP COLUMN unilateral;
