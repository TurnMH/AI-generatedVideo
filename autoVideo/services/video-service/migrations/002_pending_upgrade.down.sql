-- migrations/002_pending_upgrade.down.sql
-- Revert Phase 3 columns

ALTER TABLE video_tasks
    DROP COLUMN IF EXISTS video_mode,
    DROP COLUMN IF EXISTS hls_url,
    DROP COLUMN IF EXISTS render_config,
    DROP COLUMN IF EXISTS export_format;
