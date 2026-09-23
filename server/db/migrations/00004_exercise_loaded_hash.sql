-- +goose Up
-- A hash of what the catalog loader last wrote to the row. A load skips a file
-- whose hash has not changed, so a start with no edited files writes nothing.
ALTER TABLE exercise ADD COLUMN loaded_hash text NULL;

-- +goose Down
ALTER TABLE exercise DROP COLUMN loaded_hash;
