-- 002_serial_video.down.sql
DROP INDEX IF EXISTS idx_storyboards_scene_group_key;
ALTER TABLE storyboards
    DROP COLUMN IF EXISTS end_frame_image_url,
    DROP COLUMN IF EXISTS is_scene_first_clip,
    DROP COLUMN IF EXISTS scene_group_key;
