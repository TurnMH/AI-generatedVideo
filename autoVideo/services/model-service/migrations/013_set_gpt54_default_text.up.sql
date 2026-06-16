-- 013: 将 gpt-5.4 设为默认文本模型并接入可用渠道（ppapi）
-- 背景：gpt-5.4 仅 ppapi / 3688 渠道可用（poloai 不提供 gpt-5.4）。
-- model_db 中 gpt-5.4 此前 api_endpoint / api_key_ref 为空，导致 project-service 解析项目选中的文本模型时
-- 无法定位运行时 key，回退到服务配置默认。这里补上 ppapi 端点与运行时 key 引用，
-- 并提高其优先级，使其在“按厂商排序”的 openai 分组中位于首位、且为唯一默认文本模型。

-- 1. 清除其余 llm 模型的默认标记（确保唯一默认）
UPDATE models SET is_default = false WHERE type = 'llm' AND is_default = true;

-- 2. gpt-5.4 接入 ppapi 渠道并设为默认 / openai 组内最高优先级
UPDATE models SET
  api_endpoint   = 'https://cld.ppapi.vip/v1/chat/completions',
  api_key_ref    = 'project-service primary llm',
  is_active      = true,
  is_default     = true,
  priority       = 110,
  failure_reason = NULL,
  description    = 'OpenAI GPT-5.4 — 默认文本模型（ppapi 渠道，OpenAI chat/completions 兼容）；用于剧本优化、分镜拆分、图片/视频提示词',
  updated_at     = NOW()
WHERE model_key = 'gpt-5.4' AND type = 'llm' AND is_active = true;
