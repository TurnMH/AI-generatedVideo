-- model service: 001_init.up.sql

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS models (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(128)  NOT NULL UNIQUE,
    provider     VARCHAR(64)   NOT NULL,
    model_type   VARCHAR(64)   NOT NULL,  -- e.g. llm, image_gen, video_gen, tts
    endpoint     TEXT          NOT NULL,
    api_key_ref  TEXT,                    -- reference to secret manager key
    config       JSONB         NOT NULL DEFAULT '{}',
    is_active    BOOLEAN       NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS model_health (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id     UUID          NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    status       VARCHAR(32)   NOT NULL DEFAULT 'unknown',  -- healthy, degraded, down, unknown
    latency_ms   INT,
    error_msg    TEXT,
    checked_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS usage_records (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id       UUID        NOT NULL REFERENCES models(id) ON DELETE SET NULL,
    user_id        UUID        NOT NULL,
    request_id     UUID        NOT NULL,
    input_tokens   INT         NOT NULL DEFAULT 0,
    output_tokens  INT         NOT NULL DEFAULT 0,
    cost_usd       NUMERIC(12,6) NOT NULL DEFAULT 0,
    duration_ms    INT,
    status         VARCHAR(32) NOT NULL DEFAULT 'success',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_health_model_id    ON model_health(model_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_model_id   ON usage_records(model_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_user_id    ON usage_records(user_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_created_at ON usage_records(created_at DESC);
