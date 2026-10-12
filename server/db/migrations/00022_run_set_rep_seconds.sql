-- +goose Up
-- The seconds of each rep, for a set whose reps differ, as a ladder's 3, 6
-- then 9. seconds holds one time for every rep, so a set uses one or the other.
ALTER TABLE run_set ADD COLUMN rep_seconds int[] NULL
    CHECK (0 <= ALL (rep_seconds));
ALTER TABLE run_set ADD CONSTRAINT run_set_one_time
    CHECK (seconds IS NULL OR rep_seconds IS NULL);

-- +goose Down
ALTER TABLE run_set DROP CONSTRAINT run_set_one_time;
ALTER TABLE run_set DROP COLUMN rep_seconds;
