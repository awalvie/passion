-- +goose Up
-- The scales the client offers first. A climb keeps the scale it was logged
-- in, so changing these rewrites nothing.
ALTER TABLE account
    ADD COLUMN boulder_grades text NOT NULL DEFAULT 'font' CHECK (boulder_grades IN ('font', 'v')),
    ADD COLUMN route_grades   text NOT NULL DEFAULT 'french' CHECK (route_grades IN ('french', 'yds'));

-- +goose Down
ALTER TABLE account DROP COLUMN boulder_grades, DROP COLUMN route_grades;
