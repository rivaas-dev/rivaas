CREATE TABLE IF NOT EXISTS api_keys (
    key_hash VARCHAR(32) PRIMARY KEY NOT NULL,
    actor_id VARCHAR(255) NOT NULL,
    quota_end_date TIMESTAMP NULL,
    creation_date TIMESTAMP DEFAULT NOW() NOT NULL,
    last_modified TIMESTAMP DEFAULT NOW() NOT NULL,
    description TEXT DEFAULT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS api_keys_actor_id_key ON api_keys (actor_id);
CREATE INDEX IF NOT EXISTS api_keys_description_key ON api_keys USING GIN (description::text);
