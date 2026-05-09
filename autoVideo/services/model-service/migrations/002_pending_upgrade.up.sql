-- migrations/002_pending_upgrade.up.sql
-- Add new columns to models table per pending.md specification

ALTER TABLE models ADD COLUMN IF NOT EXISTS model_key VARCHAR(200);
ALTER TABLE models ADD COLUMN IF NOT EXISTS context_window INTEGER;
ALTER TABLE models ADD COLUMN IF NOT EXISTS input_price DECIMAL(10,6);
ALTER TABLE models ADD COLUMN IF NOT EXISTS output_price DECIMAL(10,6);
ALTER TABLE models ADD COLUMN IF NOT EXISTS speed_rating VARCHAR(20) DEFAULT 'balanced';
ALTER TABLE models ADD COLUMN IF NOT EXISTS capability_tags TEXT[] DEFAULT '{}';
ALTER TABLE models ADD COLUMN IF NOT EXISTS supports_consistency BOOLEAN DEFAULT false;
ALTER TABLE models ADD COLUMN IF NOT EXISTS consistency_method VARCHAR(50) DEFAULT 'none';
ALTER TABLE models ADD COLUMN IF NOT EXISTS video_mode VARCHAR(50);
ALTER TABLE models ADD COLUMN IF NOT EXISTS max_resolution VARCHAR(50);
ALTER TABLE models ADD COLUMN IF NOT EXISTS supported_ratios TEXT[] DEFAULT '{}';
ALTER TABLE models ADD COLUMN IF NOT EXISTS is_default BOOLEAN DEFAULT false;
ALTER TABLE models ADD COLUMN IF NOT EXISTS api_key_ref VARCHAR(200);
ALTER TABLE models ADD COLUMN IF NOT EXISTS description TEXT;

-- ─── Indexes ─────────────────────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_models_model_key   ON models (model_key);
CREATE INDEX IF NOT EXISTS idx_models_is_default  ON models (type, is_default) WHERE is_default = true;
CREATE INDEX IF NOT EXISTS idx_models_speed_rating ON models (speed_rating);

-- ─── Populate model_key from name ────────────────────────────────────────────
UPDATE models SET model_key = LOWER(REPLACE(name, ' ', '-')) WHERE model_key IS NULL;

-- ─── Populate input_price from cost_per_unit ─────────────────────────────────
UPDATE models SET input_price = cost_per_unit WHERE input_price IS NULL;

-- ─── LLM models: capability_tags, speed_rating, context_window ───────────────
UPDATE models SET
    capability_tags = ARRAY['text-generation', 'vision', 'function-calling'],
    speed_rating = 'balanced',
    context_window = 128000,
    description = 'OpenAI GPT-4o multimodal model'
WHERE name = 'GPT-4o';

UPDATE models SET
    capability_tags = ARRAY['text-generation', 'vision', 'function-calling'],
    speed_rating = 'fast',
    context_window = 128000,
    description = 'OpenAI GPT-4o-mini fast and affordable model'
WHERE name = 'GPT-4o-mini';

UPDATE models SET
    capability_tags = ARRAY['text-generation', 'vision', 'function-calling'],
    speed_rating = 'balanced',
    context_window = 200000,
    description = 'Anthropic Claude 3.7 Sonnet with extended thinking'
WHERE name = 'Claude-3.7-Sonnet';

UPDATE models SET
    capability_tags = ARRAY['text-generation', 'function-calling'],
    speed_rating = 'balanced',
    context_window = 131072,
    description = 'Alibaba Qwen2.5 72B instruction-tuned model'
WHERE name = 'Qwen2.5-72B-Instruct';

UPDATE models SET
    capability_tags = ARRAY['text-generation', 'function-calling'],
    speed_rating = 'fast',
    context_window = 65536,
    description = 'DeepSeek V3 chat model'
WHERE name = 'DeepSeek-V3';

-- ─── Image models: capability_tags, max_resolution, supported_ratios ─────────
UPDATE models SET
    capability_tags = ARRAY['text-to-image'],
    speed_rating = 'slow',
    max_resolution = '1024x1024',
    supported_ratios = ARRAY['1:1', '16:9', '9:16'],
    supports_consistency = false,
    consistency_method = 'none',
    description = 'Stable Diffusion XL local model'
WHERE name = 'SDXL';

UPDATE models SET
    capability_tags = ARRAY['text-to-image', 'consistency'],
    speed_rating = 'balanced',
    max_resolution = '1024x1024',
    supported_ratios = ARRAY['1:1', '16:9', '9:16', '4:3', '3:4'],
    supports_consistency = true,
    consistency_method = 'ip-adapter',
    description = 'Black Forest Labs Flux.1 dev model via Replicate'
WHERE name = 'Flux.1-dev';

UPDATE models SET
    capability_tags = ARRAY['text-to-image'],
    speed_rating = 'balanced',
    max_resolution = '1024x1024',
    supported_ratios = ARRAY['1:1', '16:9', '9:16'],
    supports_consistency = false,
    consistency_method = 'none',
    description = 'OpenAI DALL-E 3 image generation'
WHERE name = 'DALL-E-3';

UPDATE models SET
    capability_tags = ARRAY['text-to-image'],
    speed_rating = 'balanced',
    max_resolution = '1024x1024',
    supported_ratios = ARRAY['1:1', '16:9', '9:16'],
    supports_consistency = false,
    consistency_method = 'none',
    description = 'Alibaba Wanx2.1 image generation'
WHERE name = '通义万象-Plus';

-- ─── Video models: capability_tags, video_mode, max_resolution, supported_ratios, consistency ───
UPDATE models SET
    capability_tags = ARRAY['text-to-video', 'image-to-video', 'consistency'],
    speed_rating = 'balanced',
    video_mode = 'text-to-video',
    max_resolution = '1080p',
    supported_ratios = ARRAY['16:9', '9:16', '1:1'],
    supports_consistency = true,
    consistency_method = 'reference-image',
    description = 'Kuaishou Kling 1.6 video generation'
WHERE name = 'Kling-1.6';

UPDATE models SET
    capability_tags = ARRAY['text-to-video'],
    speed_rating = 'fast',
    video_mode = 'text-to-video',
    max_resolution = '720p',
    supported_ratios = ARRAY['16:9', '9:16', '1:1'],
    supports_consistency = false,
    consistency_method = 'none',
    description = 'ByteDance Wan2.1 turbo video generation'
WHERE name = 'Wan2.1';

UPDATE models SET
    capability_tags = ARRAY['text-to-video'],
    speed_rating = 'slow',
    video_mode = 'text-to-video',
    max_resolution = '720p',
    supported_ratios = ARRAY['16:9'],
    supports_consistency = false,
    consistency_method = 'none',
    description = 'Tsinghua CogVideoX 5B open-source video model'
WHERE name = 'CogVideoX-5B';

UPDATE models SET
    capability_tags = ARRAY['image-to-video', 'consistency'],
    speed_rating = 'balanced',
    video_mode = 'image-to-video',
    max_resolution = '1080p',
    supported_ratios = ARRAY['16:9', '9:16'],
    supports_consistency = true,
    consistency_method = 'first-frame',
    description = 'Runway Gen-3 Alpha turbo video generation'
WHERE name = 'Runway-Gen3-Alpha';

-- ─── Audio models: capability_tags ───────────────────────────────────────────
UPDATE models SET
    capability_tags = ARRAY['text-to-speech', 'multilingual'],
    speed_rating = 'balanced',
    description = 'ElevenLabs multilingual text-to-speech v2'
WHERE name = 'ElevenLabs-Multilingual-v2';

UPDATE models SET
    capability_tags = ARRAY['text-to-speech', 'voice-clone'],
    speed_rating = 'fast',
    description = 'Alibaba CosyVoice v2 text-to-speech'
WHERE name = 'CosyVoice-v2';

-- ─── Set defaults (one per type) ─────────────────────────────────────────────
UPDATE models SET is_default = true WHERE name = 'GPT-4o' AND type = 'llm';
UPDATE models SET is_default = true WHERE name = 'Flux.1-dev' AND type = 'image';
UPDATE models SET is_default = true WHERE name = 'Kling-1.6' AND type = 'video';
UPDATE models SET is_default = true WHERE name = 'ElevenLabs-Multilingual-v2' AND type = 'audio';
