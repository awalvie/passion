-- +goose Up
-- Before and after now belong to each goal.
ALTER TABLE cycle
    DROP COLUMN before_entries,
    DROP COLUMN after_entries;

-- +goose Down
ALTER TABLE cycle
    ADD COLUMN before_entries jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN after_entries  jsonb NOT NULL DEFAULT '[]';
