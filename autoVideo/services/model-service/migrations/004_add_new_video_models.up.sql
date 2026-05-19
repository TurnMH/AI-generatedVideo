
-- Clean up duplicate seed rows first, then enforce a stable identity on name/provider.
DROP INDEX IF EXISTS idx_models_model_key_unique;

CREATE TEMP TABLE model_dedup_map AS
WITH ranked AS (
  SELECT
    id,
    name,
    provider,
    row_number() OVER (PARTITION BY name, provider ORDER BY id DESC) AS rn,
    first_value(id) OVER (PARTITION BY name, provider ORDER BY id DESC) AS keep_id
  FROM models
  WHERE name IS NOT NULL
    AND provider IS NOT NULL
)
SELECT id AS duplicate_id, keep_id
FROM ranked
WHERE rn > 1;

UPDATE model_healths mh
SET model_id = m.keep_id
FROM model_dedup_map m
WHERE mh.model_id = m.duplicate_id;

UPDATE usage_records ur
SET model_id = m.keep_id
FROM model_dedup_map m
WHERE ur.model_id = m.duplicate_id;

DELETE FROM models
WHERE id IN (SELECT duplicate_id FROM model_dedup_map);

CREATE UNIQUE INDEX IF NOT EXISTS idx_models_name_provider_unique
ON models (name, provider);

INSERT INTO models (
  name, provider, type, is_active, priority, cost_per_unit, unit,
  model_key, speed_rating, capability_tags,
  supports_consistency, consistency_method,
  video_mode, max_resolution, supported_ratios, description
) VALUES (
  '星光3.0 (Doubao V4.0)',
  'bytedance',
  'video',
  TRUE, 9, 0.40, 'second',
  'doubao',
  'balanced',
  ARRAY['image-to-video', 'text-to-video'],
  TRUE, 'reference-image',
  'api_generation',
  '1080p',
  ARRAY['16:9', '9:16', '1:1'],
  '字节豆包 V4.0 图生视频 — 星光3.0渠道，ByteDance Ark API'
)
ON CONFLICT (name, provider) DO UPDATE SET
  is_active = TRUE,
  model_key  = EXCLUDED.model_key,
  description = EXCLUDED.description;

INSERT INTO models (
  name, provider, type, is_active, priority, cost_per_unit, unit,
  model_key, speed_rating, capability_tags,
  supports_consistency, consistency_method,
  video_mode, max_resolution, supported_ratios, description
) VALUES (
  '星图 (Doubao SeedDream)',
  'bytedance',
  'video',
  TRUE, 10, 0.30, 'second',
  'doubao-seedance',
  'balanced',
  ARRAY['image-to-video'],
  TRUE, 'reference-image',
  'api_generation',
  '1080p',
  ARRAY['16:9', '9:16', '1:1'],
  '字节豆包 SeedDream 4.0 图生视频 — 星图渠道，ByteDance Ark API'
)
ON CONFLICT (name, provider) DO UPDATE SET
  is_active = TRUE,
  model_key  = EXCLUDED.model_key,
  description = EXCLUDED.description;

INSERT INTO models (
  name, provider, type, is_active, priority, cost_per_unit, unit,
  model_key, speed_rating, capability_tags,
  supports_consistency, consistency_method,
  video_mode, max_resolution, supported_ratios, description
) VALUES (
  '星成2.6 (Vidu Q3 Pro)',
  'vidu',
  'video',
  TRUE, 8, 0.50, 'second',
  'vidu',
  'balanced',
  ARRAY['image-to-video'],
  TRUE, 'reference-image',
  'api_generation',
  '1080p',
  ARRAY['16:9', '9:16', '1:1'],
  'Vidu Q3 Pro 图生视频 — 星成2.6渠道，Vidu Enterprise v2 API'
)
ON CONFLICT (name, provider) DO UPDATE SET
  is_active = TRUE,
  model_key  = EXCLUDED.model_key,
  description = EXCLUDED.description;

INSERT INTO models (
  name, provider, type, is_active, priority, cost_per_unit, unit,
  model_key, speed_rating, capability_tags,
  supports_consistency, consistency_method,
  video_mode, max_resolution, supported_ratios, description
) VALUES (
  '星辰3.1 (Vidu Q3 Mix)',
  'vidu',
  'video',
  TRUE, 7, 0.45, 'second',
  'vidu-mix',
  'fast',
  ARRAY['image-to-video'],
  TRUE, 'reference-image',
  'api_generation',
  '1080p',
  ARRAY['16:9', '9:16', '1:1'],
  'Vidu Q3 Mix 图生视频 — 星辰3.1渠道，Vidu Enterprise v2 API'
)
ON CONFLICT (name, provider) DO UPDATE SET
  is_active = TRUE,
  model_key  = EXCLUDED.model_key,
  description = EXCLUDED.description;

INSERT INTO models (
  name, provider, type, is_active, priority, cost_per_unit, unit,
  model_key, speed_rating, capability_tags,
  supports_consistency, consistency_method,
  video_mode, max_resolution, supported_ratios, description
) VALUES (
  '星光2.5 (Seedance 1.5 Pro)',
  'sophnet',
  'video',
  TRUE, 9, 0.35, 'second',
  'suanneng',
  'balanced',
  ARRAY['image-to-video'],
  TRUE, 'reference-image',
  'api_generation',
  '1080p',
  ARRAY['16:9', '9:16', '1:1'],
  'Sophnet Seedance 1.5 Pro 图生视频 — 星光2.5渠道'
)
ON CONFLICT (name, provider) DO UPDATE SET
  is_active = TRUE,
  model_key  = EXCLUDED.model_key,
  description = EXCLUDED.description;

INSERT INTO models (
  name, provider, type, is_active, priority, cost_per_unit, unit,
  model_key, speed_rating, capability_tags,
  supports_consistency, consistency_method,
  video_mode, max_resolution, supported_ratios, description
) VALUES (
  '星点2.0 (Gaga)',
  'gaga',
  'video',
  TRUE, 6, 0.25, 'second',
  'gaga',
  'fast',
  ARRAY['image-to-video'],
  TRUE, 'reference-image',
  'api_generation',
  '720p',
  ARRAY['16:9', '9:16', '1:1'],
  'Gaga 图生视频 — 星点2.0渠道'
)
ON CONFLICT (name, provider) DO UPDATE SET
  is_active = TRUE,
  model_key  = EXCLUDED.model_key,
  description = EXCLUDED.description;
