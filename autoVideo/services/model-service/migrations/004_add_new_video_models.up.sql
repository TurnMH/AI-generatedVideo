-- Migration 004: Add new video generator models (doubao, vidu, suanneng, gaga)
-- These correspond to the new generators added in video-service.

-- ─── Doubao V4.0 (xingguang-3.0, ByteDance Ark) ────────────────────────────
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

-- ─── Doubao SeedDream (doubao-seedance, ByteDance Ark) ──────────────────────
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

-- ─── Vidu Q3 Pro (xingcheng-2.6) ────────────────────────────────────────────
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

-- ─── Vidu Q3 Mix (xingchen-3.1) ─────────────────────────────────────────────
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

-- ─── Suanneng Seedance 1.5 Pro (xingguang-2.5, SophNet) ─────────────────────
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

-- ─── Gaga-1 (xingdian2.0) ────────────────────────────────────────────────────
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
