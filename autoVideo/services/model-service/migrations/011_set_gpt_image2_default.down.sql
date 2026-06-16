-- 011 down: 还原 gpt-image-2 默认标记，恢复 Flux.1-dev 为默认图片模型，端点回退 easyart
UPDATE models SET
  is_default = false,
  priority   = 72,
  sort_order = 0
WHERE model_key = 'gpt-image-2' AND type = 'image';

UPDATE models SET is_default = true WHERE name = 'Flux.1-dev' AND type = 'image';
