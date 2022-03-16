CREATE DATABASE IF NOT EXISTS key_management;
USE key_management;

CREATE TABLE IF NOT EXISTS api_keys
(
    key_hash       VARCHAR(32) PRIMARY KEY                   NOT NULL,
    actor_id       VARCHAR(16)                               NOT NULL,
    quota_end_date DATETIME                                  NULL,
    creation_date  DATETIME    DEFAULT NOW()                 NOT NULL,
    last_modified  DATETIME    DEFAULT NOW() ON UPDATE NOW() NOT NULL,
    description    VARCHAR(64) DEFAULT NULL,
    INDEX (actor_id)
) CHARACTER SET utf8