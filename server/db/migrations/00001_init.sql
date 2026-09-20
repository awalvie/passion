-- +goose Up
CREATE TABLE account (
    id            uuid        PRIMARY KEY DEFAULT uuidv7(),
    email         text        NOT NULL,
    password_hash text        NOT NULL,
    display_name  text        NOT NULL,
    timezone      text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX account_email_key ON account (lower(email));

CREATE TABLE auth_token (
    id         uuid        PRIMARY KEY DEFAULT uuidv7(),
    account_id uuid        NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    token_hash bytea       NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    CONSTRAINT auth_token_expires_after_created CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX auth_token_hash_key    ON auth_token (token_hash);
CREATE        INDEX auth_token_account_idx ON auth_token (account_id);

-- Without an ORM there is no single place in Go that every UPDATE passes through,
-- so the database keeps updated_at honest.
-- +goose StatementBegin
CREATE FUNCTION touch_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER account_touch BEFORE UPDATE ON account
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose Down
DROP TABLE auth_token;
DROP TABLE account;
DROP FUNCTION touch_updated_at;
