-- 002_character_four_panels.down.sql
ALTER TABLE assets
    DROP COLUMN IF EXISTS panel_images,
    DROP COLUMN IF EXISTS composite_image_url,
    DROP COLUMN IF EXISTS seed;
