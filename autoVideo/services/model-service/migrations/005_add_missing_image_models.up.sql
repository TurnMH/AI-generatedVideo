-- Migration 005: Add missing image models + fix model_key mismatches
-- Adds all image generators registered in image-service/cmd/main.go that
-- were absent from the model-service DB. Also fixes model_key values that
-- didn't match the generator key used by selectGenerator().

-- ─── Fix model_key mismatches ──────────────────────────────────────────────
-- 通义万象-Plus: auto-generated key '通义万象-plus' ≠ generator key 'wanx2.1-t2i-plus'
UPDATE models SET model_key = 'wanx2.1-t2i-plus'
  WHERE name = '通义万象-Plus' AND type = 'image';

-- Flux.1-dev: auto-generated key 'flux.1-dev' ≠ generator key 'flux'
UPDATE models SET model_key = 'flux'
  WHERE name = 'Flux.1-dev' AND type = 'image';

-- ─── GPT-Image-1 (OpenAI native image model) ──────────────────────────────
INSERT INTO models (
  name, provider, type, is_active, priority, cost_per_unit, unit,
  model_key, speed_rating, capability_tags,
  supports_consistency, consistency_method,
  max_resolution, supported_ratios, description
) VALUES (
  'GPT-Image-1 (OpenAI)',
  'openai',
  'image',
  TRUE, 10, 0.04, 'image',
  'gpt-image-1',
  'balanced',
  ARRAY['text-to-image', 'image-to-image', 'consistency'],
  TRUE, 'ip-adapter',
  '1536x1536',
  ARRAY['1:1', '16:9', '9:16', '4:3'],
  'OpenAI GPT-Image-1 — 原生多模态图像生成，支持文生图与图生图'
)
ON CONFLICT (name, provider) DO UPDATE SET
  is_active   = TRUE,
  model_key   = EXCLUDED.model_key,
  description = EXCLUDED.description;

-- ─── CogView-3 Plus (智谱AI) ──────────────────────────────────────────────
INSERT INTO models (
  name, provider, type, is_active, priority, cost_per_unit, unit,
  model_key, speed_rating, capability_tags,
  supports_consistency, consistency_method,
  max_resolution, supported_ratios, description
) VALUES (
  'CogView-3 Plus (智谱AI)',
  'zhipu',
  'image',
  TRUE, 7, 0.014, 'image',
  'cogview-3-plus',
  'balanced',
  ARRAY['text-to-image', 'consistency'],
  TRUE, 'ip-adapter',
  '1024x1024',
  ARRAY['1:1', '16:9', '9:16', '3:2'],
  '智谱AI CogView-3 Plus — OpenAI兼容格式，高质量中文理解图像生成'
)
ON CONFLICT (name, provider) DO UPDATE SET
  is_active   = TRUE,
  model_key   = EXCLUDED.model_key,
  description = EXCLUDED.description;

-- ─── CogView-4 (智谱AI) ───────────────────────────────────────────────────
INSERT INTO models (
  name, provider, type, is_active, priority, cost_per_unit, unit,
  model_key, speed_rating, capability_tags,
  supports_consistency, consistency_method,
  max_resolution, supported_ratios, description
) VALUES (
  'CogView-4 (智谱AI)',
  'zhipu',
  'image',
  TRUE, 8, 0.018, 'image',
  'cogview-4',
  'balanced',
  ARRAY['text-to-image', 'image-to-image', 'consistency'],
  TRUE, 'ip-adapter',
  '1440x1440',
  ARRAY['1:1', '16:9', '9:16', '3:2', '4:3'],
  '智谱AI CogView-4 — 升级版多模态图像生成，支持图生图与风格迁移'
)
ON CONFLICT (name, provider) DO UPDATE SET
  is_active   = TRUE,
  model_key   = EXCLUDED.model_key,
  description = EXCLUDED.description;

-- ─── 豆包SeedDream (ByteDance Ark image) ──────────────────────────────────
INSERT INTO models (
  name, provider, type, is_active, priority, cost_per_unit, unit,
  model_key, speed_rating, capability_tags,
  supports_consistency, consistency_method,
  max_resolution, supported_ratios, description
) VALUES (
  '豆包SeedDream (ByteDance)',
  'bytedance',
  'image',
  TRUE, 8, 0.012, 'image',
  'doubao-image',
  'fast',
  ARRAY['text-to-image', 'consistency'],
  TRUE, 'ip-adapter',
  '1440x1440',
  ARRAY['1:1', '16:9', '9:16', '4:3'],
  '字节跳动豆包 SeedDream 4.0 图像生成 (doubao-seedream-4-0-250828) — 高速文生图'
)
ON CONFLICT (name, provider) DO UPDATE SET
  is_active   = TRUE,
  model_key   = EXCLUDED.model_key,
  description = EXCLUDED.description;

-- ─── 通义万象-Turbo (DashScope fast) ──────────────────────────────────────
INSERT INTO models (
  name, provider, type, is_active, priority, cost_per_unit, unit,
  model_key, speed_rating, capability_tags,
  supports_consistency, consistency_method,
  max_resolution, supported_ratios, description
) VALUES (
  '通义万象-Turbo',
  'aliyun',
  'image',
  TRUE, 6, 0.004, 'image',
  'wanx2.1-t2i-turbo',
  'fast',
  ARRAY['text-to-image'],
  FALSE, 'none',
  '1024x1024',
  ARRAY['1:1', '16:9', '9:16'],
  '阿里云通义万象 wanx2.1-t2i-turbo — 高速低成本文生图'
)
ON CONFLICT (name, provider) DO UPDATE SET
  is_active   = TRUE,
  model_key   = EXCLUDED.model_key,
  description = EXCLUDED.description;

-- ─── 通义万象 i2i (DashScope image-to-image) ──────────────────────────────
INSERT INTO models (
  name, provider, type, is_active, priority, cost_per_unit, unit,
  model_key, speed_rating, capability_tags,
  supports_consistency, consistency_method,
  max_resolution, supported_ratios, description
) VALUES (
  '通义万象 i2i',
  'aliyun',
  'image',
  TRUE, 6, 0.008, 'image',
  'wan2.5-i2i-preview',
  'balanced',
  ARRAY['image-to-image', 'consistency'],
  TRUE, 'ip-adapter',
  '1024x1024',
  ARRAY['1:1', '16:9', '9:16'],
  '阿里云通义万象 wan2.5-i2i-preview — 图生图，支持风格迁移与人物一致性'
)
ON CONFLICT (name, provider) DO UPDATE SET
  is_active   = TRUE,
  model_key   = EXCLUDED.model_key,
  description = EXCLUDED.description;
