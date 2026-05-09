-- storage service: 001_init.up.sql

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS files (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID          NOT NULL,
    bucket       VARCHAR(128)  NOT NULL,
    object_key   TEXT          NOT NULL,
    filename     VARCHAR(512)  NOT NULL,
    content_type VARCHAR(128)  NOT NULL,
    size_bytes   BIGINT        NOT NULL DEFAULT 0,
    cdn_url      TEXT,
    metadata     JSONB         NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    UNIQUE (bucket, object_key)
);

CREATE INDEX IF NOT EXISTS idx_files_user_id    ON files(user_id);
CREATE INDEX IF NOT EXISTS idx_files_bucket     ON files(bucket);
CREATE INDEX IF NOT EXISTS idx_files_created_at ON files(created_at DESC);
