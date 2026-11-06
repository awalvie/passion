-- +goose Up
-- How steep the board was set, in degrees past vertical.
ALTER TABLE climb ADD COLUMN angle smallint NULL CHECK (angle BETWEEN 0 AND 90);
ALTER TABLE climb ADD CONSTRAINT climb_angle_board CHECK (angle IS NULL OR board IS NOT NULL);

-- +goose Down
ALTER TABLE climb DROP COLUMN angle;
