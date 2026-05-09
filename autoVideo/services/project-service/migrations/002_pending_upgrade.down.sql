-- 002_pending_upgrade.down.sql
-- Drop storyboard tables
DROP TABLE IF EXISTS storyboard_versions;
DROP TABLE IF EXISTS storyboards;

-- Drop script_versions table
DROP TABLE IF EXISTS script_versions;

-- Remove episode extended fields
ALTER TABLE episodes DROP COLUMN IF EXISTS script_excerpt;
ALTER TABLE episodes DROP COLUMN IF EXISTS word_count;
ALTER TABLE episodes DROP COLUMN IF EXISTS estimated_duration;
ALTER TABLE episodes DROP COLUMN IF EXISTS version;

-- Remove project extended fields
ALTER TABLE projects DROP COLUMN IF EXISTS logo_url;
ALTER TABLE projects DROP COLUMN IF EXISTS script_file_url;
ALTER TABLE projects DROP COLUMN IF EXISTS script_text;
ALTER TABLE projects DROP COLUMN IF EXISTS script_file_size;
ALTER TABLE projects DROP COLUMN IF EXISTS script_versions;
ALTER TABLE projects DROP COLUMN IF EXISTS progress;
ALTER TABLE projects DROP COLUMN IF EXISTS target_episodes;
ALTER TABLE projects DROP COLUMN IF EXISTS style_tags;
ALTER TABLE projects DROP COLUMN IF EXISTS text_model_id;
ALTER TABLE projects DROP COLUMN IF EXISTS image_model_id;
ALTER TABLE projects DROP COLUMN IF EXISTS video_model_id;
ALTER TABLE projects DROP COLUMN IF EXISTS tts_model_id;
ALTER TABLE projects DROP COLUMN IF EXISTS enable_dubbing;
ALTER TABLE projects DROP COLUMN IF EXISTS enable_subtitle;
ALTER TABLE projects DROP COLUMN IF EXISTS video_mode;
ALTER TABLE projects DROP COLUMN IF EXISTS storyboard_config;
ALTER TABLE projects DROP COLUMN IF EXISTS watermark_config;
ALTER TABLE projects DROP COLUMN IF EXISTS consistency_strength;
ALTER TABLE projects DROP COLUMN IF EXISTS storage_used_bytes;
