-- +goose Up
-- A climb keeps each go in order, an attempt or a send, so a problem done in
-- loops can hold many sends.
ALTER TABLE climb ADD COLUMN goes text[] NOT NULL DEFAULT '{}'
    CHECK (goes <@ ARRAY['attempt', 'send']::text[]);

UPDATE climb SET goes =
    CASE WHEN outcome IN ('onsight', 'flash', 'redpoint')
        THEN array_fill('attempt'::text, ARRAY[greatest(coalesce(attempts, 1) - 1, 0)]) || ARRAY['send']
        ELSE array_fill('attempt'::text, ARRAY[coalesce(attempts, CASE WHEN outcome IS NULL THEN 0 ELSE 1 END)])
    END;

ALTER TABLE climb DROP COLUMN outcome, DROP COLUMN attempts;

-- +goose Down
ALTER TABLE climb
    ADD COLUMN outcome text NULL
        CHECK (outcome IN ('onsight', 'flash', 'redpoint', 'hangdog', 'working')),
    ADD COLUMN attempts int NULL CHECK (attempts >= 1);

UPDATE climb SET
    outcome = CASE
        WHEN goes = ARRAY['send'] THEN 'flash'
        WHEN 'send' = ANY (goes) THEN 'redpoint'
        WHEN cardinality(goes) > 0 THEN 'working'
    END,
    attempts = nullif(cardinality(goes), 0);

ALTER TABLE climb DROP COLUMN goes;
