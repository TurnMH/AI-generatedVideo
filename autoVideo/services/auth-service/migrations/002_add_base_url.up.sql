-- Add base_url to user_api_keys for per-provider endpoint configuration
ALTER TABLE user_api_keys ADD COLUMN IF NOT EXISTS base_url TEXT NOT NULL DEFAULT '';
ALTER TABLE user_api_keys ADD COLUMN IF NOT EXISTS model_scope TEXT NOT NULL DEFAULT '';
ALTER TABLE user_api_keys ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN user_api_keys.base_url IS 'API endpoint base URL for this provider';
COMMENT ON COLUMN user_api_keys.model_scope IS 'Comma-separated list of supported model names';
COMMENT ON COLUMN user_api_keys.is_system IS 'System-level key available to all users';

-- Seed valid keys (system-level, user_id=1 as admin placeholder)
-- These will be associated with the first registered admin user
-- Key values stored as plaintext here then encrypted by app on first read
-- For seeding, we insert a special marker and let migration handler process them

CREATE TABLE IF NOT EXISTS system_api_keys (
    id           BIGSERIAL PRIMARY KEY,
    provider     VARCHAR(64)  NOT NULL,
    key_alias    VARCHAR(128) NOT NULL,
    plain_key    TEXT         NOT NULL,
    base_url     TEXT         NOT NULL DEFAULT '',
    model_scope  TEXT         NOT NULL DEFAULT '',
    is_active    BOOLEAN      NOT NULL DEFAULT TRUE,
    status       VARCHAR(16)  NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_system_api_keys_provider ON system_api_keys(provider);

-- Insert verified available keys (✅ from keyImg/keys_extracted.md)
INSERT INTO system_api_keys (provider, key_alias, plain_key, base_url, model_scope, status) VALUES
-- 星启 easyart - 132+ models (LLM + Image + Video)
('easyart', '星启 easyart (LLM+图像+视频)',
 'sk-lpuJgWP2Z8nsYehtqxx4mEg9i3dzyS0uiFZh4cSY3GmGRtXW',
 'https://api.easyart.cc',
 'gemini-2.5-pro,gemini-2.5-flash,gemini-3-pro-preview,gemini-3.1-flash-image-preview,gemini-3.1-pro-preview,claude-opus-4-6,claude-sonnet-4-6,claude-sonnet-4-5-20250929,gpt-5.4,gpt-5.4-mini,gpt-5.2,gpt-5,gpt-image-1.5,sora-2,veo3,veo3.1,glm-4.7,qwen3-max,grok-4,Kimi-K2.5,MiniMax-M2.5',
 'active'),

-- 智谱 GLM 官方直连
('zhipu', '智谱 GLM 官方',
 '6d3398265b814c3eb266c3e4be146ffd.FNhS76riXtaduy0R',
 'https://open.bigmodel.cn/api/paas/v4',
 'glm-4-flash,glm-4-flashx,glm-4-plus,glm-4-air,glm-4-airx,glm-4.7,glm-z1-flash',
 'active'),

-- 阿里云 DashScope 官方
('dashscope', '阿里云 DashScope 官方',
 'sk-2cf9fae133d643c1874304e234c13484',
 'https://dashscope.aliyuncs.com/compatible-mode',
 'qwen-max,qwen3-235b-a22b,qwen-turbo-latest,qwen-vl-max,wan2.5-i2i-preview',
 'active'),

-- wcnbai 图像生成渠道
('wcnbai', 'wcnbai 图像渠道',
 'sk-OwEKOA7XwtvJ71wHqakGFUXCc977qa4o8xVXYhQg65NtfKMF',
 'http://64.32.27.150:3000',
 'gemini-3.1-flash-image-preview',
 'active'),

-- ppai.pro 图像渠道 (nano-banana / Gemini IMAGE_REFERENCE)
-- 来源：fengxi/0428all/192.168.5.108_ai_video_33061_limited.sql，越光1渠道
-- 对接方式：OpenAI chat/completions + generationConfig:{responseModalities:["TEXT","IMAGE"]}
('runtime.image.ppai', 'ppai.pro 图像渠道 (nano-banana)',
 'sk-QeXuGasPbBN7yjHa19E7A892Af2546A2BfB7D2B6B134A091',
 'https://api.ppai.pro',
 'nano-banana',
 'active')

ON CONFLICT DO NOTHING;
