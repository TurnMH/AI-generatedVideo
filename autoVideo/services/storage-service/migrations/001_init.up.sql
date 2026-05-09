CREATE TABLE IF NOT EXISTS files (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT       NOT NULL,
    bucket       VARCHAR(64)  NOT NULL,
    object_key   VARCHAR(512) NOT NULL UNIQUE,
    filename     VARCHAR(256),
    content_type VARCHAR(128),
    size_bytes   BIGINT       DEFAULT 0,
    cdn_url      TEXT,
    width        INT          DEFAULT 0,
    height       INT          DEFAULT 0,
    metadata     JSONB,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
CREATE INDEX IF NOT EXISTS idx_files_bucket  ON files(bucket);
