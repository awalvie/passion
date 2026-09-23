-- +goose Up
-- The id: line of the catalog file a row came from. It stays put when the file
-- is renamed, so the loader finds the same row. Two people who load the same
-- tree each get their own row, so a file id is unique per owner, not overall.
ALTER TABLE exercise ADD COLUMN file_id uuid NULL;

CREATE UNIQUE INDEX exercise_shipped_file_id ON exercise (file_id)
    WHERE owner IS NULL AND file_id IS NOT NULL;
CREATE UNIQUE INDEX exercise_owner_file_id ON exercise (owner, file_id)
    WHERE owner IS NOT NULL AND file_id IS NOT NULL;

-- +goose Down
ALTER TABLE exercise DROP COLUMN file_id;
