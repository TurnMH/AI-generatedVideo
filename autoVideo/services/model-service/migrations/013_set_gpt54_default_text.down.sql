-- 013 down: 回滚 gpt-5.4 默认文本模型设置（仅清空本次写入的渠道引用与优先级标记）
UPDATE models SET
  api_endpoint   = '',
  api_key_ref    = '',
  is_default     = false,
  priority       = 68,
  updated_at     = NOW()
WHERE model_key = 'gpt-5.4' AND type = 'llm' AND is_active = true;
