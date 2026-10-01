-- +goose Up
-- A name from the client's icon set. The client draws the first letter of
-- the name for one it does not know.
ALTER TABLE session_template ADD COLUMN icon text;

-- +goose Down
ALTER TABLE session_template DROP COLUMN icon;
