-- +goose Up
-- Days the person picked for goals that run once, such as a trip. The client
-- picks the id. With no name, they are known by their dates.
CREATE TABLE goal_span (
    id          uuid        PRIMARY KEY,
    owner       uuid        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    name        text        NOT NULL DEFAULT '',
    starts      date        NOT NULL,
    -- The last day, inclusive.
    ends        date        NOT NULL CHECK (ends >= starts),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, owner)
);

CREATE TRIGGER goal_span_touch BEFORE UPDATE ON goal_span
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- A goal the person sets for every week, month or year until they remove it,
-- or once over a span. The client picks the id, so a retried write makes one.
-- Its period cannot change, so old ticks keep their meaning.
CREATE TABLE goal (
    id          uuid        PRIMARY KEY,
    owner       uuid        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    text        text        NOT NULL,
    period      text        NOT NULL CHECK (period IN ('week', 'month', 'year', 'custom')),
    -- The day it was set, in the person's time zone. The period holding it is
    -- its first.
    starts      date        NOT NULL,
    -- A custom goal's days.
    span        uuid        NULL,
    -- The day it was removed. It still counts in the periods that ended
    -- before then.
    removed     date        NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, owner),
    FOREIGN KEY (span, owner) REFERENCES goal_span (id, owner) ON DELETE CASCADE,
    CHECK ((period = 'custom') = (span IS NOT NULL))
);

CREATE INDEX goal_owner ON goal (owner, created_at);

CREATE TRIGGER goal_touch BEFORE UPDATE ON goal
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- One row for each period the goal was met, named by the period's first day:
-- for a custom goal, its span's first day. It keeps the goal's text as it
-- read then, so a later edit does not change what was met.
CREATE TABLE goal_tick (
    goal          uuid        NOT NULL,
    owner         uuid        NOT NULL,
    period_start  date        NOT NULL,
    text          text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (goal, period_start),
    FOREIGN KEY (goal, owner) REFERENCES goal (id, owner) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE goal_tick;
DROP TABLE goal;
DROP TABLE goal_span;
