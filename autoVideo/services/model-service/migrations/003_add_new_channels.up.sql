-- migrations/003_add_new_channels.up.sql
-- Register new channel providers: Gemini (banana/xingrong), Baidu BCE, aiping, Kling 3.0
-- Also backfill model_key for existing image/video models that were missing it

-- ─── Fix model_key for existing image models ─────────────────────────────────
UPDATE models SET model_key = 'gemini-3.1-flash-image'
  WHERE name IN ('gemini-3.1-flash-image-preview', 'gemini-3.1-flash-image-wcnbai') AND type = 'image';

UPDATE models SET model_key = 'gpt-image-1.5'
  WHERE name = 'gpt-image-1.5' AND type = 'image';

UPDATE models SET model_key = 'wan2.5-i2i-preview'
  WHERE name = 'wan2.5-i2i-preview' AND type = 'image';

-- ─── Fix model_key for existing video models ─────────────────────────────────
-- sora-2 generator Name() returns "sora2" (no dash)
UPDATE models SET model_key = 'sora2'
  WHERE name = 'sora-2' AND type = 'video';

-- veo3 uses hubagi generator with model "TC-GV"; Name() = "hubagi-TC-GV"
UPDATE models SET model_key = 'hubagi-TC-GV'
  WHERE name = 'veo3' AND type = 'video';

-- veo3.1 uses hubagi veo generator; Name() = "hubagi-" + veo_model (config: "voe3.1")
UPDATE models SET model_key = 'hubagi-voe3.1'
  WHERE name = 'veo3.1' AND type = 'video';

-- ─── New image channel models ─────────────────────────────────────────────────
INSERT INTO models (name, provider, type, is_active, priority, cost_per_unit, unit,
                    model_key, speed_rating, capability_tags,
                    supports_consistency, consistency_method, max_resolution, supported_ratios, description)
VALUES
(
  '香蕉2.1 (Gemini Flash)',
  'google',
  'image',
  TRUE, 9, 0.008, 'image',
  'banana2.1',
  'fast',
  ARRAY['text-to-image', 'image-to-image', 'consistency'],
  TRUE, 'ip-adapter',
  '1536x1536',
  ARRAY['1:1', '16:9', '9:16', '4:3'],
  'Gemini 3.1 Flash image generation via banana2.1 proxy — 香蕉2.1渠道'
),
(
  '星融2.5 (Gemini Flash)',
  'google',
  'image',
  TRUE, 9, 0.008, 'image',
  'xingrong2.5',
  'fast',
  ARRAY['text-to-image', 'image-to-image', 'consistency'],
  TRUE, 'ip-adapter',
  '1536x1536',
  ARRAY['1:1', '16:9', '9:16', '4:3'],
  'Gemini 3.1 Flash image generation via xingrong2.5 proxy — 星融2.5beta渠道'
),
(
  '香蕉2.0 (Gemini Pro)',
  'google',
  'image',
  TRUE, 8, 0.015, 'image',
  'banana2.0',
  'balanced',
  ARRAY['text-to-image', 'image-to-image', 'consistency'],
  TRUE, 'ip-adapter',
  '1536x1536',
  ARRAY['1:1', '16:9', '9:16', '4:3'],
  'Gemini 3 Pro image generation via banana2.0 proxy — 香蕉2.0渠道'
),
(
  '百度BCE图像',
  'baidu',
  'image',
  TRUE, 7, 0.010, 'image',
  'baidu-img',
  'fast',
  ARRAY['text-to-image', 'image-to-image'],
  FALSE, 'none',
  '1024x1024',
  ARRAY['1:1', '16:9', '9:16'],
  '百度BCE图像融合渠道 (Baidu BCE image fusion via Gemini-compatible API)'
);

-- ─── New video channel models ─────────────────────────────────────────────────
INSERT INTO models (name, provider, type, is_active, priority, cost_per_unit, unit,
                    model_key, speed_rating, capability_tags,
                    supports_consistency, consistency_method,
                    video_mode, max_resolution, supported_ratios, description)
VALUES
(
  '星澜3.0 (Kling v3)',
  'kuaishou',
  'video',
  TRUE, 10, 0.40, 'second',
  'kling',
  'balanced',
  ARRAY['text-to-video', 'image-to-video', 'consistency'],
  TRUE, 'reference-image',
  'text-to-video',
  '1080p',
  ARRAY['16:9', '9:16', '1:1'],
  '快手可灵星澜3.0 (Kling v3) — 更强的运动流畅度与画面细节'
),
(
  'aiping (K3高并发)',
  'aiping',
  'video',
  TRUE, 9, 0.38, 'second',
  'aiping',
  'fast',
  ARRAY['text-to-video', 'image-to-video', 'consistency'],
  TRUE, 'reference-image',
  'text-to-video',
  '1080p',
  ARRAY['16:9', '9:16', '1:1'],
  'aiping渠道 Kling K3/K3.0-Omni 高并发视频生成'
);

-- ─── Update Kling 1.6 description to reflect it is superseded by v3 ──────────
UPDATE models SET
  is_active = FALSE,
  description = 'Kuaishou Kling 1.6 (superseded by Kling v3 / 星澜3.0)'
WHERE name = 'Kling-1.6' AND type = 'video';
