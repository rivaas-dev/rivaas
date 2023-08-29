CREATE TYPE apikey_environment AS ENUM ('production', 'sandbox');

CREATE TABLE IF NOT EXISTS api_keys
(
    id            VARCHAR(26) PRIMARY KEY NOT NULL,
    actor_id      VARCHAR(250)            NOT NULL,
    creator_id    VARCHAR(250),
    key_hash      VARCHAR(32) PRIMARY KEY NOT NULL,
    expires_at    date                    NULL,
    creation_date TIMESTAMP                        DEFAULT NOW() NOT NULL,
    last_modified TIMESTAMP                        DEFAULT NOW() NOT NULL,
    description   TEXT                             DEFAULT NULL,
    deleted_at    TIMESTAMPTZ                      DEFAULT NULL,
    contacts      JSONB                   NOT NULL DEFAULT '{}'::JSONB,
    active        BOOLEAN                 NOT NULL DEFAULT TRUE,
    metadata      JSONB                   NOT NULL DEFAULT '{}'::JSONB,
    environment   apikey_environment      NOT NULL,
    labels        JSONB                   NOT NULL DEFAULT '{}'::JSONB
);

CREATE INDEX IF NOT EXISTS api_keys_description_key ON api_keys USING GIN (description);
