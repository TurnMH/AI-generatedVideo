-- migrations/003_soft_delete.down.sql
-- Revert soft-delete column and compose_stage

DROP INDEX IF EXISTS idx_video_tasks_deleted_at;
ALTER TABLE video_tasks DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE video_tasks DROP COLUMN IF EXISTS compose_stage;
