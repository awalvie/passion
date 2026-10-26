-- +goose Up
-- A session on a rule of days, the way a calendar app repeats an event. The
-- client picks the id, so a retried write makes one. Its days are rows in
-- scheduled_session like a cycle's, built up to a year ahead and topped up as
-- the schedule is read.
CREATE TABLE repeat (
    id           uuid        PRIMARY KEY,
    owner        uuid        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    -- No foreign key: templates are retired, never deleted.
    template     uuid        NOT NULL,
    starts       date        NOT NULL,
    -- The last day, inclusive. Null repeats for ever.
    ends         date        NULL,
    every        int         NOT NULL CHECK (every BETWEEN 1 AND 99),
    unit         text        NOT NULL CHECK (unit IN ('day', 'week')),
    -- 1 is Monday. Only a weekly repeat has them, and it needs at least one.
    weekdays     int[]       NOT NULL DEFAULT '{}',
    built_until  date        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, owner),
    CHECK (ends >= starts),
    CHECK ((unit = 'week') = (cardinality(weekdays) > 0)),
    CHECK (weekdays <@ '{1,2,3,4,5,6,7}')
);

CREATE INDEX repeat_owner ON repeat (owner, starts DESC);

CREATE TRIGGER repeat_touch BEFORE UPDATE ON repeat
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

ALTER TABLE scheduled_session
    ADD COLUMN repeat uuid NULL,
    ADD FOREIGN KEY (repeat, owner) REFERENCES repeat (id, owner) ON DELETE CASCADE,
    ADD CHECK (cycle IS NULL OR repeat IS NULL);

CREATE INDEX scheduled_session_repeat ON scheduled_session (repeat, local_date);

-- +goose Down
ALTER TABLE scheduled_session DROP COLUMN repeat;
DROP TABLE repeat;
