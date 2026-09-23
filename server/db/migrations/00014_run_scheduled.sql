-- +goose Up
-- The scheduled session a run was started from. Deleting the row, or the
-- cycle that placed it, clears this and keeps the run.
ALTER TABLE run
    ADD COLUMN scheduled uuid NULL,
    ADD FOREIGN KEY (scheduled, owner) REFERENCES scheduled_session (id, owner)
        ON DELETE SET NULL (scheduled);

CREATE INDEX run_scheduled ON run (scheduled);

-- +goose Down
DROP INDEX run_scheduled;
ALTER TABLE run DROP COLUMN scheduled;
