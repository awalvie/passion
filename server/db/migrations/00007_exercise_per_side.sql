-- +goose Up
-- True when the numbers count for each side, as in 6 reps per side.
ALTER TABLE exercise ADD COLUMN per_side boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE exercise DROP COLUMN per_side;
