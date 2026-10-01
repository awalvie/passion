-- +goose Up
-- A place the person climbs or trains, and the sessions they can do there.
-- The client picks the id, so a retried create makes one centre (rule 37).
CREATE TABLE centre (
    id          uuid        PRIMARY KEY,
    owner       uuid        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    name        text        NOT NULL,
    -- Session templates, shipped or the person's own. No foreign key:
    -- templates are retired, never deleted.
    sessions    uuid[]      NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX centre_owner ON centre (owner, lower(name));

CREATE TRIGGER centre_touch BEFORE UPDATE ON centre
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose Down
DROP TABLE centre;
