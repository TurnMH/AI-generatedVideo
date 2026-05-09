-- 002_pending_upgrade.up.sql
-- Project extended fields
ALTER TABLE projects ADD COLUMN IF NOT EXISTS logo_url TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS script_file_url TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS script_text TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS script_file_size INTEGER DEFAULT 0;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS script_versions JSONB DEFAULT '[]';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS progress JSONB DEFAULT '{}';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS target_episodes INTEGER DEFAULT 0;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS style_tags TEXT[] DEFAULT '{}';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS text_model_id BIGINT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS image_model_id BIGINT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS video_model_id BIGINT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS tts_model_id BIGINT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS enable_dubbing BOOLEAN DEFAULT false;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS enable_subtitle BOOLEAN DEFAULT false;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS video_mode VARCHAR(30) DEFAULT 'frame_animation';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS storyboard_config JSONB DEFAULT '{}';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS watermark_config JSONB DEFAULT '{"enabled":false}';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS consistency_strength DECIMAL(3,2) DEFAULT 0.75;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS storage_used_bytes BIGINT DEFAULT 0;

-- Episodes extended fields
ALTER TABLE episodes ADD COLUMN IF NOT EXISTS script_excerpt TEXT DEFAULT '';
ALTER TABLE episodes ADD COLUMN IF NOT EXISTS word_count INTEGER DEFAULT 0;
ALTER TABLE episodes ADD COLUMN IF NOT EXISTS estimated_duration INTEGER DEFAULT 0;
ALTER TABLE episodes ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 1;

-- Script versions table
CREATE TABLE IF NOT EXISTS script_versions (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    file_url TEXT DEFAULT '',
    oss_key TEXT DEFAULT '',
    file_size INTEGER DEFAULT 0,
    is_current BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_script_versions_project ON script_versions(project_id);

-- Storyboards table
CREATE TABLE IF NOT EXISTS storyboards (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    episode_id BIGINT REFERENCES episodes(id),
    sequence_number INTEGER NOT NULL,
    scene_description TEXT DEFAULT '',
    characters TEXT[] DEFAULT '{}',
    location VARCHAR(200) DEFAULT '',
    camera_movement VARCHAR(100) DEFAULT '',
    duration INTEGER DEFAULT 4,
    aspect_ratio VARCHAR(20) DEFAULT '16:9',
    resolution VARCHAR(20) DEFAULT '1080p',
    video_mode VARCHAR(30),
    dialogue TEXT DEFAULT '',
    current_version INTEGER DEFAULT 1,
    image_url TEXT DEFAULT '',
    prompt_used TEXT DEFAULT '',
    status VARCHAR(50) DEFAULT 'pending',
    is_voided BOOLEAN DEFAULT false,
    is_manual_edited BOOLEAN DEFAULT false,
    agent_history JSONB DEFAULT '[]',
    asset_ids BIGINT[] DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_storyboards_project ON storyboards(project_id);
CREATE INDEX IF NOT EXISTS idx_storyboards_episode ON storyboards(episode_id);
CREATE INDEX IF NOT EXISTS idx_storyboards_status ON storyboards(status);

-- Storyboard versions table
CREATE TABLE IF NOT EXISTS storyboard_versions (
    id BIGSERIAL PRIMARY KEY,
    storyboard_id BIGINT NOT NULL REFERENCES storyboards(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    image_url TEXT DEFAULT '',
    oss_key TEXT DEFAULT '',
    size_bytes BIGINT DEFAULT 0,
    prompt_used TEXT DEFAULT '',
    is_current BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sb_versions_storyboard ON storyboard_versions(storyboard_id);
