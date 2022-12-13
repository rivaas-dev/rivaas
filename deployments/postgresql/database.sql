CREATE TABLE api_keys
(
    key_hash       VARCHAR(32) PRIMARY KEY NOT NULL,
    actor_id       VARCHAR(255)            NOT NULL,
    quota_end_date timestamp NULL,
    creation_date  timestamp DEFAULT NOW() NOT NULL,
    last_modified  timestamp DEFAULT NOW() NOT NULL,
    description    TEXT      DEFAULT NULL
);

CREATE INDEX api_keys_actor_id_key ON api_keys (actor_id);
CREATE INDEX api_keys_description_key ON api_keys USING GIN (description);
