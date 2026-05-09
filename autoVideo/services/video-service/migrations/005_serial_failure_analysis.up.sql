-- Add AI serial failure analysis column to video_clips
ALTER TABLE video_clips ADD COLUMN IF NOT EXISTS chain_failure_analysis TEXT NOT NULL DEFAULT '';
