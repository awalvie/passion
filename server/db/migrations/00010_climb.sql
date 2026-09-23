-- +goose Up
-- One row per climb, under a climbing step. The client picks the id, so a
-- retried write makes one climb.
CREATE TABLE climb (
    id            uuid        PRIMARY KEY,
    run           uuid        NOT NULL,
    owner         uuid        NOT NULL,
    step          uuid        NOT NULL,
    position      int         NOT NULL,
    discipline    text        NOT NULL CHECK (discipline IN ('boulder', 'sport', 'trad')),
    setting       text        NOT NULL CHECK (setting IN ('indoor', 'outdoor')),
    board         text        NULL
        CHECK (board IN ('kilter', 'moon', 'tension', 'spray', 'custom')),
    rope_style    text        NULL
        CHECK (rope_style IN ('lead', 'top_rope', 'auto_belay', 'follow')),
    -- As logged. With no grade_system it may hold an ungraded label.
    grade         text        NULL,
    grade_system  text        NULL CHECK (grade_system IN ('font', 'v', 'french', 'yds')),
    grade_rank    int         NULL,
    outcome       text        NULL
        CHECK (outcome IN ('onsight', 'flash', 'redpoint', 'hangdog', 'working')),
    attempts      int         NULL CHECK (attempts >= 1),
    seconds       int         NULL CHECK (seconds >= 0),
    stars         smallint    NULL CHECK (stars BETWEEN 1 AND 3),
    focus         text        NULL,
    notes         text        NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (run, owner) REFERENCES run (id, owner) ON DELETE CASCADE,
    CHECK ((grade_system IS NULL) = (grade_rank IS NULL)),
    CHECK (grade_system IS NULL OR grade IS NOT NULL)
);

CREATE INDEX climb_run ON climb (run);
CREATE INDEX climb_owner ON climb (owner);

CREATE TRIGGER climb_touch BEFORE UPDATE ON climb
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose Down
DROP TABLE climb;
