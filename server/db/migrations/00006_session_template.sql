-- +goose Up
CREATE TABLE session_template (
    id           uuid        PRIMARY KEY DEFAULT uuidv7(),
    owner        uuid        NULL REFERENCES account (id) ON DELETE CASCADE,
    slug         text        NULL,
    file_id      uuid        NULL,
    loaded_hash  text        NULL,

    name         text        NOT NULL,
    notes        text,
    source       text,
    color        text,
    needs        text,
    tags         text[]      NOT NULL DEFAULT '{}',
    body         jsonb       NOT NULL,

    retired_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX session_template_shipped_slug ON session_template (slug)
    WHERE owner IS NULL AND slug IS NOT NULL;
CREATE UNIQUE INDEX session_template_owner_slug ON session_template (owner, slug)
    WHERE owner IS NOT NULL AND slug IS NOT NULL;
CREATE UNIQUE INDEX session_template_shipped_file_id ON session_template (file_id)
    WHERE owner IS NULL AND file_id IS NOT NULL;
CREATE UNIQUE INDEX session_template_owner_file_id ON session_template (owner, file_id)
    WHERE owner IS NOT NULL AND file_id IS NOT NULL;
CREATE INDEX session_template_owner_idx ON session_template (owner);

CREATE TRIGGER session_template_touch BEFORE UPDATE ON session_template
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose Down
DROP TABLE session_template;
