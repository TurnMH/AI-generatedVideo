-- Add scene_description to video_tasks for richer video prompt generation
ALTER TABLE video_tasks ADD COLUMN IF NOT EXISTS scene_description TEXT NOT NULL DEFAULT '';
