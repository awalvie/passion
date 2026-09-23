-- +goose Up
CREATE TABLE exercise (
    id                uuid        PRIMARY KEY DEFAULT uuidv7(),
    owner             uuid        NULL REFERENCES account (id) ON DELETE CASCADE,
    slug              text        NULL,

    name              text        NOT NULL,
    kind              text        NOT NULL
        CHECK (kind IN ('reps_and_sets', 'timed_reps', 'climbing', 'open')),
    notes             text,
    source            text,
    tags              text[]      NOT NULL DEFAULT '{}',

    sets              int,
    reps              int,
    set_rest_seconds  int,
    rep_seconds       int,
    rep_rest_seconds  int,
    prep_seconds      int,
    duration_seconds  int,

    video_url         text,
    thumbnail_url     text,

    retired_at        timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

-- Postgres treats NULLs as distinct, so a single (owner, slug) index would let
-- the shipped catalog hold the same slug twice.
CREATE UNIQUE INDEX exercise_shipped_slug ON exercise (slug)
    WHERE owner IS NULL AND slug IS NOT NULL;
CREATE UNIQUE INDEX exercise_owner_slug ON exercise (owner, slug)
    WHERE owner IS NOT NULL AND slug IS NOT NULL;
CREATE INDEX exercise_owner_idx ON exercise (owner);

CREATE TRIGGER exercise_touch BEFORE UPDATE ON exercise
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose Down
DROP TABLE exercise;
