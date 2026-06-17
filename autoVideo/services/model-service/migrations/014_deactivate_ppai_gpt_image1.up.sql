-- 014: 停用 ppapi 池上的 gpt-image-1 / gpt-image-1.5
-- 背景：image-service 中 openai_keys 全部走 api.ppai.pro；该代理账号（用户 ID 34）
-- 返回 403 shell_api_error「已被封禁」，是分镜失败的主因。gpt-image-2 使用独立
-- gpt_image2 多渠道 failover，不受此池影响。

UPDATE models SET
  is_active      = false,
  failure_reason = 'ppapi 代理账号已封禁（403 用户 ID 34 shell_api_error）；请更换 openai_keys 后手动恢复',
  api_endpoint   = 'https://api.ppai.pro/v1/images/generations',
  api_key_ref    = 'runtime.image.openai',
  updated_at     = NOW()
WHERE model_key IN ('gpt-image-1', 'gpt-image-1.5') AND type = 'image';
