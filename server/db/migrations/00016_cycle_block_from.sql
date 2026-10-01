-- +goose Up
-- The day the current block counts from. It is starts until the block's
-- length changes once the cycle has begun; then it is the day of that change.
ALTER TABLE cycle ADD COLUMN block_from date;
UPDATE cycle SET block_from = starts;
ALTER TABLE cycle ALTER COLUMN block_from SET NOT NULL,
    ADD CHECK (block_from >= starts);

-- +goose Down
ALTER TABLE cycle DROP COLUMN block_from;
