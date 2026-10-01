-- +goose Up
-- What the person writes about a cycle. None of it places a session.
ALTER TABLE cycle
    ADD COLUMN goals          jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN before_entries jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN after_entries  jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN notes          text;

-- +goose Down
ALTER TABLE cycle
    DROP COLUMN goals,
    DROP COLUMN before_entries,
    DROP COLUMN after_entries,
    DROP COLUMN notes;
