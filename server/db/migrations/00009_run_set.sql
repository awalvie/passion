-- +goose Up
-- One row per set. exercise is copied from the step when the set is written,
-- and it is the identity a chart groups on.
CREATE TABLE run_set (
    run        uuid         NOT NULL,
    owner      uuid         NOT NULL,
    step       uuid         NOT NULL,
    number     int          NOT NULL CHECK (number >= 1),
    exercise   uuid         NOT NULL,
    reps       int          NULL CHECK (reps >= 0),
    seconds    int          NULL CHECK (seconds >= 0),
    -- Below zero is assistance, and zero is bodyweight.
    weight_kg  numeric(6,2) NULL,
    PRIMARY KEY (run, step, number),
    FOREIGN KEY (run, owner) REFERENCES run (id, owner) ON DELETE CASCADE
);

CREATE INDEX run_set_owner_exercise ON run_set (owner, exercise);

-- +goose Down
DROP TABLE run_set;
