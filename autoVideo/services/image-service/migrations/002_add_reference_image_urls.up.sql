-- 002_add_reference_image_urls.up.sql
-- Adds reference_image_urls JSON column to carry extra character/scene
-- reference images for multi-image aware generators (Gemini parts[],
-- gpt-image-1 image[], Baidu messages.content[], Seedream image[]).
ALTER TABLE image_tasks
    ADD COLUMN IF NOT EXISTS reference_image_urls JSONB;
