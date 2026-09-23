-- +goose Up
-- The client picks the id, so a retried create makes one cycle (rule 37).
CREATE TABLE cycle (
    id          uuid        PRIMARY KEY,
    owner       uuid        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    name        text        NOT NULL,
    starts      date        NOT NULL,
    -- The last day, inclusive.
    ends        date        NOT NULL,
    block_days  int         NOT NULL CHECK (block_days >= 1),
    body        jsonb       NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, owner),
    CHECK (ends >= starts AND ends - starts < 366)
);

CREATE INDEX cycle_owner ON cycle (owner, starts DESC);

CREATE TRIGGER cycle_touch BEFORE UPDATE ON cycle
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- One row per session on a day. A cycle writes its rows when it is built; a
-- one-off has no cycle. The unique key keeps a day from holding the same
-- session twice (rule 37).
CREATE TABLE scheduled_session (
    id          uuid        PRIMARY KEY DEFAULT uuidv7(),
    owner       uuid        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    cycle       uuid        NULL,
    -- No foreign key: templates are retired, never deleted.
    template    uuid        NOT NULL,
    local_date  date        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, owner),
    UNIQUE (owner, local_date, template),
    FOREIGN KEY (cycle, owner) REFERENCES cycle (id, owner) ON DELETE CASCADE
);

CREATE INDEX scheduled_session_cycle ON scheduled_session (cycle, local_date);

CREATE TRIGGER scheduled_session_touch BEFORE UPDATE ON scheduled_session
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose Down
DROP TABLE scheduled_session;
DROP TABLE cycle;
