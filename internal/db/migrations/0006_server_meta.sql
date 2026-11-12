-- +goose Up
CREATE TABLE server_meta (
    key text PRIMARY KEY,
    value text NOT NULL
);

-- +goose Down
DROP TABLE server_meta;
