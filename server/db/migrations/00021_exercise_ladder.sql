-- +goose Up
-- The seconds of each rep in a set, as 3, 6 then 9. It sets the reps.
ALTER TABLE exercise ADD COLUMN ladder int[];

-- +goose Down
ALTER TABLE exercise DROP COLUMN ladder;
