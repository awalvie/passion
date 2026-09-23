-- +goose Up
-- Copied from the step when the climb is written, as on run_set. It is the
-- identity history groups on.
ALTER TABLE climb ADD COLUMN exercise uuid NULL;

UPDATE climb c SET exercise = (item -> 'step' ->> 'exercise')::uuid
FROM run r,
    jsonb_array_elements(r.body -> 'sections') AS section,
    jsonb_array_elements(section -> 'items') AS item
WHERE r.id = c.run AND item -> 'step' ->> 'id' = c.step::text;

-- A climb whose step the body no longer holds should already have been pruned.
DELETE FROM climb WHERE exercise IS NULL;

ALTER TABLE climb ALTER COLUMN exercise SET NOT NULL;
DROP INDEX climb_owner;
CREATE INDEX climb_owner_exercise ON climb (owner, exercise);

-- +goose Down
DROP INDEX climb_owner_exercise;
CREATE INDEX climb_owner ON climb (owner);
ALTER TABLE climb DROP COLUMN exercise;
