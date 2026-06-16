-- 012 down: 还原 doubao-seedance 元数据为 1.5-Pro
UPDATE models SET
  name        = '豆包 Seedance 1.5-Pro (ByteDance)',
  description = '豆包 Seedance 1.5-Pro 视频生成；content 数组格式，model=doubao-seedance-1-5-pro-251215；key=cd0cd314',
  updated_at  = NOW()
WHERE model_key = 'doubao-seedance' AND type = 'video';
