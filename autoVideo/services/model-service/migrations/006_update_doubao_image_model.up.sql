-- Update doubao image model description to reflect Seedream 4.0 (doubao-seedream-4-0-250828)
UPDATE models
SET description = '字节跳动豆包 SeedDream 4.0 图像生成 (doubao-seedream-4-0-250828) — 高速文生图'
WHERE model_key = 'doubao-image'
  AND provider = 'bytedance';
