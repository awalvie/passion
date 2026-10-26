-- +goose Up
-- Days the person is away or hurt. The client picks the id, so a retried write
-- makes one.
CREATE TABLE time_off (
    id          uuid        PRIMARY KEY,
    owner       uuid        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    kind        text        NOT NULL CHECK (kind IN ('trip', 'injury', 'illness', 'other')),
    name        text        NOT NULL,
    starts      date        NOT NULL,
    ends        date        NULL,
    pauses      boolean     NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (ends >= starts)
);

CREATE INDEX time_off_owner ON time_off (owner, starts);

CREATE TRIGGER time_off_touch BEFORE UPDATE ON time_off
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose Down
DROP TABLE time_off;
