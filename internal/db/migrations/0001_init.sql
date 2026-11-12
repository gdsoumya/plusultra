-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username      text NOT NULL UNIQUE,
    email         text UNIQUE,
    password_hash text NOT NULL,
    display_name  text NOT NULL DEFAULT '',
    units         text NOT NULL DEFAULT 'kg' CHECK (units IN ('kg', 'lb')),
    timezone      text NOT NULL DEFAULT 'UTC',
    is_admin      boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    token_hash   bytea NOT NULL UNIQUE,
    expires_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    user_agent   text NOT NULL DEFAULT ''
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

CREATE TABLE api_keys (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    name         text NOT NULL,
    prefix       text NOT NULL,
    key_hash     bytea NOT NULL UNIQUE,
    scopes       text[] NOT NULL DEFAULT '{}',
    expires_at   timestamptz,
    revoked_at   timestamptz,
    last_used_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX api_keys_user_idx ON api_keys (user_id);

CREATE TABLE invites (
    code       text PRIMARY KEY,
    created_by uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    used_by    uuid REFERENCES users ON DELETE SET NULL,
    used_at    timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE exercises (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug              text NOT NULL UNIQUE,
    name              text NOT NULL,
    equipment         text NOT NULL DEFAULT '',
    aliases           text[] NOT NULL DEFAULT '{}',
    primary_muscles   text[] NOT NULL DEFAULT '{}',
    secondary_muscles text[] NOT NULL DEFAULT '{}',
    category          text NOT NULL DEFAULT '',
    default_warmup    jsonb,
    images            text[] NOT NULL DEFAULT '{}',
    source            text NOT NULL DEFAULT '',
    user_id           uuid REFERENCES users ON DELETE CASCADE
);
CREATE INDEX exercises_name_trgm_idx ON exercises USING gin (name gin_trgm_ops);

-- array_to_string is only STABLE, which generated columns reject.
CREATE FUNCTION immutable_join(text[]) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE
    AS $$ SELECT array_to_string($1, ' ') $$;

CREATE TABLE foods (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source    text NOT NULL,
    source_id text NOT NULL DEFAULT '',
    name      text NOT NULL,
    aliases   text[] NOT NULL DEFAULT '{}',
    barcode   text,
    per100g   jsonb NOT NULL DEFAULT '{}',
    units     jsonb NOT NULL DEFAULT '{}',
    user_id   uuid REFERENCES users ON DELETE CASCADE,
    search    tsvector GENERATED ALWAYS AS (
        to_tsvector('simple', name || ' ' || immutable_join(aliases))
    ) STORED
);
CREATE INDEX foods_search_idx ON foods USING gin (search);
CREATE INDEX foods_barcode_idx ON foods (barcode) WHERE barcode IS NOT NULL;
CREATE UNIQUE INDEX foods_source_idx ON foods (source, source_id) WHERE source_id <> '';

CREATE TABLE programs (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('workout', 'meal')),
    name       text NOT NULL,
    script     text NOT NULL,
    revision   int NOT NULL DEFAULT 1,
    structure  jsonb NOT NULL DEFAULT '{}',
    status     text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'finished', 'archived')),
    start_date date,
    end_date   date,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX programs_user_status_idx ON programs (user_id, status);

CREATE TABLE program_revisions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    program_id uuid NOT NULL REFERENCES programs ON DELETE CASCADE,
    revision   int NOT NULL,
    script     text NOT NULL,
    structure  jsonb NOT NULL DEFAULT '{}',
    created_by text NOT NULL CHECK (created_by IN ('user', 'api', 'mcp')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (program_id, revision)
);

CREATE TABLE resolved_days (
    program_id uuid NOT NULL REFERENCES programs ON DELETE CASCADE,
    week       int NOT NULL,
    day        int NOT NULL,
    date       date,
    kind       text NOT NULL DEFAULT '',
    payload    jsonb NOT NULL DEFAULT '{}',
    PRIMARY KEY (program_id, week, day)
);
CREATE INDEX resolved_days_date_idx ON resolved_days (program_id, date) WHERE date IS NOT NULL;

CREATE TABLE program_cursor (
    program_id uuid PRIMARY KEY REFERENCES programs ON DELETE CASCADE,
    next_week  int NOT NULL,
    next_day   int NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE progression_state (
    program_id uuid NOT NULL REFERENCES programs ON DELETE CASCADE,
    exercise   text NOT NULL,
    label      text NOT NULL DEFAULT '',
    state      jsonb NOT NULL DEFAULT '{}',
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (program_id, exercise, label)
);

CREATE TABLE user_exercise_data (
    user_id       uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    exercise_slug text NOT NULL,
    rm1           numeric,
    rounding      jsonb,
    notes         text NOT NULL DEFAULT '',
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, exercise_slug)
);

CREATE TABLE equipment (
    user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    name    text NOT NULL,
    config  jsonb NOT NULL DEFAULT '{}',
    PRIMARY KEY (user_id, name)
);

CREATE TABLE photos (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    taken_at     timestamptz NOT NULL DEFAULT now(),
    bytes        bytea NOT NULL,
    content_type text NOT NULL
);
CREATE INDEX photos_user_idx ON photos (user_id);

CREATE TABLE day_logs (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    date       date NOT NULL,
    status     text NOT NULL CHECK (status IN ('planned', 'done', 'partial', 'skipped', 'rest')),
    notes      text NOT NULL DEFAULT '',
    readiness  jsonb,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    device_id  text NOT NULL DEFAULT '',
    version    int NOT NULL DEFAULT 1,
    deleted_at timestamptz,
    UNIQUE (user_id, date)
);
CREATE INDEX day_logs_changed_idx ON day_logs (user_id, updated_at, id);

CREATE TABLE workout_records (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    date         date NOT NULL,
    program_id   uuid REFERENCES programs ON DELETE SET NULL,
    resolved_day jsonb NOT NULL DEFAULT '{}',
    entries      jsonb NOT NULL DEFAULT '[]',
    started_at   timestamptz,
    finished_at  timestamptz,
    finalized    boolean NOT NULL DEFAULT false,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    device_id    text NOT NULL DEFAULT '',
    version      int NOT NULL DEFAULT 1,
    deleted_at   timestamptz
);
CREATE INDEX workout_records_date_idx ON workout_records (user_id, date);
CREATE INDEX workout_records_changed_idx ON workout_records (user_id, updated_at, id);

CREATE TABLE meal_logs (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    date       date NOT NULL,
    program_id uuid REFERENCES programs ON DELETE SET NULL,
    target     jsonb NOT NULL DEFAULT '{}',
    meals      jsonb NOT NULL DEFAULT '[]',
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    device_id  text NOT NULL DEFAULT '',
    version    int NOT NULL DEFAULT 1,
    deleted_at timestamptz
);
CREATE INDEX meal_logs_date_idx ON meal_logs (user_id, date);
CREATE INDEX meal_logs_changed_idx ON meal_logs (user_id, updated_at, id);

CREATE TABLE checkins (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    date         date NOT NULL,
    weight       numeric,
    body_fat     numeric,
    measurements jsonb NOT NULL DEFAULT '{}',
    photo_id     uuid REFERENCES photos ON DELETE SET NULL,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    device_id    text NOT NULL DEFAULT '',
    version      int NOT NULL DEFAULT 1,
    deleted_at   timestamptz
);
CREATE INDEX checkins_date_idx ON checkins (user_id, date);
CREATE INDEX checkins_changed_idx ON checkins (user_id, updated_at, id);

-- +goose Down
DROP TABLE checkins, meal_logs, workout_records, day_logs, photos, equipment, user_exercise_data,
    progression_state, program_cursor, resolved_days, program_revisions, programs, foods, exercises,
    invites, api_keys, sessions, users;
DROP FUNCTION immutable_join(text[]);
