-- +goose Up
CREATE TABLE run (
    id              uuid        PRIMARY KEY DEFAULT uuidv7(),
    owner           uuid        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    template        uuid        NULL,
    name            text        NOT NULL,
    plan            jsonb       NULL,
    body            jsonb       NOT NULL,

    started_at      timestamptz NOT NULL,
    timezone        text        NOT NULL,
    local_date      date        NOT NULL,
    finished_at     timestamptz NULL,
    elapsed_seconds int         NULL CHECK (elapsed_seconds >= 0),
    place           text        NULL,
    notes           text        NULL,

    sleep           smallint    NULL CHECK (sleep BETWEEN 1 AND 5),
    energy          smallint    NULL CHECK (energy BETWEEN 1 AND 5),
    rpe             smallint    NULL CHECK (rpe BETWEEN 1 AND 10),
    focus           text        NULL
        CHECK (focus IN ('strength', 'endurance', 'technique', 'projects', 'general')),
    setting         text        NULL CHECK (setting IN ('indoor', 'outdoor')),
    went_well       text        NULL,
    next_focus      text        NULL,

    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    -- The log tables reference (id, owner), so a row can never sit under
    -- another account than its run.
    UNIQUE (id, owner)
);

CREATE INDEX run_owner_date ON run (owner, local_date DESC);
CREATE INDEX run_owner_template ON run (owner, template);

CREATE TRIGGER run_touch BEFORE UPDATE ON run
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose Down
DROP TABLE run;
