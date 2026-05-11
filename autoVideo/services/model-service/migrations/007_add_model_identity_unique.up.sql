-- Migration 007: ensure model identity can be upserted safely
-- 004/005 use ON CONFLICT (name, provider), so model-service needs a unique
-- index on that key to make later seed migrations re-runnable.

CREATE UNIQUE INDEX IF NOT EXISTS idx_models_name_provider_unique
ON models (name, provider);
