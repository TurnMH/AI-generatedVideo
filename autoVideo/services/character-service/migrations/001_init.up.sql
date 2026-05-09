-- migrations/001_init.up.sql
-- character_db schema and seed data

CREATE TABLE IF NOT EXISTS characters (
    id                  BIGSERIAL PRIMARY KEY,
    project_id          BIGINT NOT NULL,
    name                VARCHAR(128) NOT NULL,
    role_desc           TEXT,
    appearance_desc     TEXT,
    reference_image_url TEXT,
    style_preset        VARCHAR(64) DEFAULT 'anime',
    style_reference_url TEXT,
    lora_model_id       VARCHAR(128),
    ip_adapter_config   JSONB DEFAULT '{}',
    fixed_seed          BIGINT,
    extra_config        JSONB DEFAULT '{}',
    created_at          TIMESTAMPTZ DEFAULT NOW(),
    updated_at          TIMESTAMPTZ DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_characters_project_id ON characters (project_id);
CREATE INDEX IF NOT EXISTS idx_characters_deleted_at ON characters (deleted_at);

CREATE TABLE IF NOT EXISTS style_presets (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(64) UNIQUE NOT NULL,
    description     TEXT,
    preview_url     TEXT,
    prompt_suffix   TEXT,
    negative_prompt TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Trigger: auto-update updated_at on characters
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_characters_updated_at ON characters;
CREATE TRIGGER trg_characters_updated_at
    BEFORE UPDATE ON characters
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Seed: system style presets
INSERT INTO style_presets (name, description, prompt_suffix, negative_prompt) VALUES
('anime',     '日系动漫风格',   'anime style, vibrant colors, clean lines, manga aesthetic',                           'realistic, photo, 3d render'),
('realistic', '写实风格',       'photorealistic, detailed, cinematic lighting, 8k resolution',                        'cartoon, anime, illustration'),
('comic_ink', '漫画墨线风格',   'comic book style, ink lines, bold outlines, flat colors',                            'realistic, photo, blur'),
('watercolor','水彩插画风格',   'watercolor illustration, soft edges, pastel colors, artistic',                       'sharp lines, photo, harsh lighting'),
('3d_render', '3D渲染风格',     '3d render, octane render, volumetric lighting, high detail',                         'flat, 2d, cartoon'),
('cyberpunk', '赛博朋克风格',   'cyberpunk aesthetic, neon lights, dark atmosphere, futuristic city',                 'rural, nature, bright daylight'),
('ink_wash',  '水墨国画风格',   'Chinese ink wash painting, traditional, brush strokes, minimalist',                 'colorful, western, digital art'),
('pixel_art', '像素艺术风格',   'pixel art, retro game style, 16bit, pixelated',                                      'realistic, smooth, high res')
ON CONFLICT (name) DO NOTHING;
