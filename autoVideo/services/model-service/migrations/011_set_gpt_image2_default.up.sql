-- 011: 将 gpt-image-2 设为默认图片生成模型，并接入可用渠道（天衍 / shubiaobiao）
-- 背景：原 easyart 渠道 key 已封禁，gpt-image-2 实际由 image-service 的 gpt_image2 专属渠道生成。
-- 这里同步更新 model_db 中的展示端点 / 优先级 / 默认标记，使前端默认选中 gpt-image-2。

-- 1. 清除其余图片模型的默认标记（确保唯一默认）
UPDATE models SET is_default = false WHERE type = 'image' AND is_default = true;

-- 2. gpt-image-2 接入可用渠道并设为默认 / 最高优先级
UPDATE models SET
  api_endpoint   = 'https://api2img.shubiaobiao.com/v1/images/generations',
  api_key_ref    = 'runtime.image.gptimage2',
  is_active      = true,
  is_default     = true,
  priority       = 100,
  sort_order     = 100,
  failure_reason = NULL,
  description    = 'OpenAI GPT Image 2 — 默认图片生成模型（天衍 shubiaobiao 渠道，images/generations 兼容）',
  updated_at     = NOW()
WHERE model_key = 'gpt-image-2' AND type = 'image';

-- 3. 同步 gpt-image-2 变体端点（保持可用渠道一致）
UPDATE models SET
  api_endpoint   = 'https://api2img.shubiaobiao.com/v1/images/generations',
  api_key_ref    = 'runtime.image.gptimage2',
  is_active      = true,
  failure_reason = NULL,
  updated_at     = NOW()
WHERE model_key IN ('gpt-image-2-all', 'gpt-image-2-u') AND type = 'image';
