-- 014 down: 恢复 ppapi 池 gpt-image-1 / gpt-image-1.5 为可用（需确认 openai_keys 已修复）

UPDATE models SET
  is_active      = true,
  failure_reason = NULL,
  updated_at     = NOW()
WHERE model_key IN ('gpt-image-1', 'gpt-image-1.5') AND type = 'image';

-- 还原 gpt-image-1.5 历史端点展示（gpt-image-1 原本无固定 endpoint）
UPDATE models SET
  api_endpoint = 'https://api.easyart.cc/v1/images/generations',
  api_key_ref  = 'easyart'
WHERE model_key = 'gpt-image-1.5' AND type = 'image';
