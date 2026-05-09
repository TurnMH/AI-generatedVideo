-- 003_add_is_character_sheet.up.sql
-- Adds is_character_sheet flag to image_tasks so the image generators can
-- inject explicit 4-panel turnaround guidance into the prompt when a
-- composite character reference sheet is attached. GORM AutoMigrate covers
-- this column automatically on startup; this file makes the migration
-- explicit for environments that apply SQL migrations manually.
ALTER TABLE image_tasks
    ADD COLUMN IF NOT EXISTS is_character_sheet BOOLEAN NOT NULL DEFAULT FALSE;
