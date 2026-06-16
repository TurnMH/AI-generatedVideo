-- 012: 将 doubao-seedance 视频模型元数据对齐到 Seedance 2.0
-- video-service 配置 doubao_seedance_model 已指向 doubao-seedance-2-0-260128 并实测可用 (Ark, 2026-06-15)，
-- 但 model_db 展示名/描述仍停留在 1.5-Pro，这里同步元数据（model_key 不变，仍由 video-service 映射到配置模型名）。
UPDATE models SET
  name        = '豆包 Seedance 2.0 (ByteDance)',
  description = '字节跳动豆包 Seedance 2.0 视频生成 (doubao-seedance-2-0-260128) — content 数组格式，720p；较 1.5-Pro 运动流畅度与一致性更强',
  updated_at  = NOW()
WHERE model_key = 'doubao-seedance' AND type = 'video';
