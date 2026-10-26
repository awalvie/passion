-- +goose Up
-- Free text about the person's climbing, on a day. The client picks the id, so
-- a retried write makes one entry.
CREATE TABLE journal_entry (
    id          uuid        PRIMARY KEY,
    owner       uuid        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    local_date  date        NOT NULL,
    body        text        NOT NULL,
    -- The session it was written during. An entry outlives a discarded session.
    run         uuid        NULL,
    pinned_at   timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (run, owner) REFERENCES run (id, owner) ON DELETE SET NULL (run)
);

CREATE INDEX journal_entry_owner_date ON journal_entry (owner, local_date DESC, created_at DESC);
CREATE INDEX journal_entry_run ON journal_entry (run);

CREATE TRIGGER journal_entry_touch BEFORE UPDATE ON journal_entry
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose Down
DROP TABLE journal_entry;
