-- migrations/001_init.up.sql
-- Model Management Service – initial schema + seed data

-- ─── Extensions ──────────────────────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ─── models ──────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS models (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(128)  NOT NULL,
    provider      VARCHAR(64)   NOT NULL,
    type          VARCHAR(32)   NOT NULL,
    api_endpoint  TEXT,
    is_active     BOOLEAN       NOT NULL DEFAULT TRUE,
    priority      INT           NOT NULL DEFAULT 0,
    cost_per_unit DOUBLE PRECISION NOT NULL DEFAULT 0,
    unit          VARCHAR(32),
    config        JSONB,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_models_type_active    ON models (type, is_active);
CREATE INDEX IF NOT EXISTS idx_models_provider       ON models (provider);
CREATE INDEX IF NOT EXISTS idx_models_priority       ON models (priority DESC);

-- ─── model_healths ───────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS model_healths (
    id          BIGSERIAL PRIMARY KEY,
    model_id    BIGINT       NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    status      VARCHAR(16)  NOT NULL DEFAULT 'unknown',
    latency_ms  BIGINT       NOT NULL DEFAULT 0,
    checked_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_healths_model_id ON model_healths (model_id);

-- ─── usage_records ───────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS usage_records (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT           NOT NULL,
    model_id    BIGINT           NOT NULL REFERENCES models(id),
    task_id     VARCHAR(128),
    units_used  DOUBLE PRECISION NOT NULL DEFAULT 0,
    cost        DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_usage_records_user_id   ON usage_records (user_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_model_id  ON usage_records (model_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_created   ON usage_records (created_at);

-- ─── Auto-update updated_at ──────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$;

DROP TRIGGER IF EXISTS trg_models_updated_at ON models;
CREATE TRIGGER trg_models_updated_at
    BEFORE UPDATE ON models
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ═══════════════════════════════════════════════════════════════════════════════
--  Seed data
-- ═══════════════════════════════════════════════════════════════════════════════

-- ─── LLM models ──────────────────────────────────────────────────────────────
INSERT INTO models (name, provider, type, api_endpoint, is_active, priority, cost_per_unit, unit, config) VALUES
(
    'GPT-4o',
    'openai',
    'llm',
    'https://api.openai.com/v1/chat/completions',
    TRUE, 10, 0.005, 'token',
    '{"max_tokens":128000,"context_window":128000,"supports_vision":true,"supports_function_calling":true,"default_temperature":0.7}'
),
(
    'GPT-4o-mini',
    'openai',
    'llm',
    'https://api.openai.com/v1/chat/completions',
    TRUE, 8, 0.00015, 'token',
    '{"max_tokens":16000,"context_window":128000,"supports_vision":true,"supports_function_calling":true,"default_temperature":0.7,"model_id":"gpt-4o-mini"}'
),
(
    'Claude-3.7-Sonnet',
    'anthropic',
    'llm',
    'https://api.anthropic.com/v1/messages',
    TRUE, 9, 0.003, 'token',
    '{"max_tokens":200000,"context_window":200000,"supports_vision":true,"supports_function_calling":true,"anthropic_version":"2023-06-01","model_id":"claude-3-7-sonnet-20250219"}'
),
(
    'Qwen2.5-72B-Instruct',
    'aliyun',
    'llm',
    'https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation',
    TRUE, 7, 0.0004, 'token',
    '{"max_tokens":131072,"context_window":131072,"supports_function_calling":true,"model_id":"qwen2.5-72b-instruct","enable_search":false}'
),
(
    'DeepSeek-V3',
    'deepseek',
    'llm',
    'https://api.deepseek.com/v1/chat/completions',
    TRUE, 8, 0.00014, 'token',
    '{"max_tokens":65536,"context_window":65536,"supports_function_calling":true,"model_id":"deepseek-chat","top_p":0.95}'
);

-- ─── Image models ────────────────────────────────────────────────────────────
INSERT INTO models (name, provider, type, api_endpoint, is_active, priority, cost_per_unit, unit, config) VALUES
(
    'SDXL',
    'local',
    'image',
    'http://localhost:7860/sdapi/v1/txt2img',
    TRUE, 5, 0.0, 'image',
    '{"width":1024,"height":1024,"steps":30,"cfg_scale":7,"sampler":"DPM++ 2M Karras","negative_prompt":"low quality, blurry"}'
),
(
    'Flux.1-dev',
    'replicate',
    'image',
    'https://api.replicate.com/v1/predictions',
    TRUE, 8, 0.055, 'image',
    '{"model_id":"black-forest-labs/flux-dev","width":1024,"height":1024,"num_inference_steps":28,"guidance":3.5,"output_format":"webp","output_quality":90}'
),
(
    'DALL-E-3',
    'openai',
    'image',
    'https://api.openai.com/v1/images/generations',
    TRUE, 9, 0.04, 'image',
    '{"model_id":"dall-e-3","size":"1024x1024","quality":"standard","style":"vivid","response_format":"url"}'
),
(
    '通义万象-Plus',
    'aliyun',
    'image',
    'https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis',
    TRUE, 7, 0.008, 'image',
    '{"model_id":"wanx2.1-t2i-plus","size":"1024*1024","n":1,"style":"<auto>","prompt_extend":true}'
);

-- ─── Video models ────────────────────────────────────────────────────────────
INSERT INTO models (name, provider, type, api_endpoint, is_active, priority, cost_per_unit, unit, config) VALUES
(
    'Kling-1.6',
    'kuaishou',
    'video',
    'https://api.klingai.com/v1/videos/text2video',
    TRUE, 9, 0.35, 'second',
    '{"model_name":"kling-v1-6","duration":5,"aspect_ratio":"16:9","mode":"std","cfg_scale":0.5,"max_duration":10}'
),
(
    'Wan2.1',
    'bytedance',
    'video',
    'https://visual.volcengineapi.com/?Action=CVVideoGenerateTask',
    TRUE, 8, 0.28, 'second',
    '{"model":"wan2.1-t2v-turbo","resolution":"720p","aspect_ratio":"16:9","duration":5,"fps":16,"watermark":false}'
),
(
    'CogVideoX-5B',
    'replicate',
    'video',
    'https://api.replicate.com/v1/predictions',
    TRUE, 7, 0.20, 'second',
    '{"model_id":"THUDM/CogVideoX-5b","num_frames":49,"num_inference_steps":50,"guidance_scale":6,"fps":8,"output_format":"mp4"}'
),
(
    'Runway-Gen3-Alpha',
    'runway',
    'video',
    'https://api.dev.runwayml.com/v1/image_to_video',
    TRUE, 8, 0.50, 'second',
    '{"model":"gen3a_turbo","duration":10,"ratio":"1280:768","watermark":false}'
);

-- ─── Audio models ────────────────────────────────────────────────────────────
INSERT INTO models (name, provider, type, api_endpoint, is_active, priority, cost_per_unit, unit, config) VALUES
(
    'ElevenLabs-Multilingual-v2',
    'elevenlabs',
    'audio',
    'https://api.elevenlabs.io/v1/text-to-speech',
    TRUE, 9, 0.00003, 'token',
    '{"model_id":"eleven_multilingual_v2","output_format":"mp3_44100_128","voice_settings":{"stability":0.5,"similarity_boost":0.75,"style":0.0,"use_speaker_boost":true}}'
),
(
    'CosyVoice-v2',
    'aliyun',
    'audio',
    'https://dashscope.aliyuncs.com/api/v1/services/aigc/audio/speech/synthesis',
    TRUE, 7, 0.000025, 'token',
    '{"model":"cosyvoice-v2","format":"mp3","sample_rate":22050,"rate":1.0,"pitch":1.0,"volume":50}'
);
