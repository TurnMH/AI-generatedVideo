-- migrations/003_soft_delete.up.sql
-- Add soft-delete support and compose stage tracking to video_tasks

ALTER TABLE video_tasks
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE video_tasks
    ADD COLUMN IF NOT EXISTS compose_stage VARCHAR(32) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_video_tasks_deleted_at ON video_tasks (deleted_at);
