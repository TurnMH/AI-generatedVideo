-- migrations/002_pending_upgrade.down.sql
-- Reverse the pending upgrade: drop all added columns and indexes

DROP INDEX IF EXISTS idx_models_model_key;
DROP INDEX IF EXISTS idx_models_is_default;
DROP INDEX IF EXISTS idx_models_speed_rating;

ALTER TABLE models DROP COLUMN IF EXISTS model_key;
ALTER TABLE models DROP COLUMN IF EXISTS context_window;
ALTER TABLE models DROP COLUMN IF EXISTS input_price;
ALTER TABLE models DROP COLUMN IF EXISTS output_price;
ALTER TABLE models DROP COLUMN IF EXISTS speed_rating;
ALTER TABLE models DROP COLUMN IF EXISTS capability_tags;
ALTER TABLE models DROP COLUMN IF EXISTS supports_consistency;
ALTER TABLE models DROP COLUMN IF EXISTS consistency_method;
ALTER TABLE models DROP COLUMN IF EXISTS video_mode;
ALTER TABLE models DROP COLUMN IF EXISTS max_resolution;
ALTER TABLE models DROP COLUMN IF EXISTS supported_ratios;
ALTER TABLE models DROP COLUMN IF EXISTS is_default;
ALTER TABLE models DROP COLUMN IF EXISTS api_key_ref;
ALTER TABLE models DROP COLUMN IF EXISTS description;
