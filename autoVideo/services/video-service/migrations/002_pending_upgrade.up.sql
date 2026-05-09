-- migrations/002_pending_upgrade.up.sql
-- Phase 3: Video enhancement columns

ALTER TABLE video_tasks
    ADD COLUMN IF NOT EXISTS video_mode     VARCHAR(20)  NOT NULL DEFAULT 'frame_animation',
    ADD COLUMN IF NOT EXISTS hls_url        TEXT         NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS render_config  JSONB        NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS export_format  VARCHAR(10)  NOT NULL DEFAULT 'mp4';
