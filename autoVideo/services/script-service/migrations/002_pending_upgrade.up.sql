-- Enhance scripts table
ALTER TABLE scripts ADD COLUMN IF NOT EXISTS file_size INTEGER DEFAULT 0;
ALTER TABLE scripts ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 1;

-- Enhance scenes table for split configuration
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS word_count INTEGER DEFAULT 0;
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS estimated_duration INTEGER DEFAULT 0;

-- Split configuration table
CREATE TABLE IF NOT EXISTS split_configs (
    id BIGSERIAL PRIMARY KEY,
    script_id BIGINT NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    split_method VARCHAR(50) DEFAULT 'scene_based',
    target_word_count INTEGER DEFAULT 3000,
    target_episodes INTEGER DEFAULT 0,
    custom_params JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_split_configs_script ON split_configs(script_id);
