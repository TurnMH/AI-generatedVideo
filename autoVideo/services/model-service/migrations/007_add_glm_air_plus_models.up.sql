-- migrations/007_add_glm_air_plus_models.up.sql
-- Add GLM-4 Air, GLM-4 Air X, GLM-4 Plus — verified available on BigModel official API
-- Tested: 2025-07, all return HTTP 200 via https://open.bigmodel.cn/api/paas/v4

INSERT INTO models (model_key, name, type, provider, api_endpoint, api_key_ref, priority, is_active)
VALUES
  ('glm-4-plus',  'GLM-4 Plus',  'llm', 'zhipu', 'https://open.bigmodel.cn/api/paas/v4/chat/completions', 'zhipu-official', 58, true),
  ('glm-4-air',   'GLM-4 Air',   'llm', 'zhipu', 'https://open.bigmodel.cn/api/paas/v4/chat/completions', 'zhipu-official', 57, true),
  ('glm-4-airx',  'GLM-4 Air X', 'llm', 'zhipu', 'https://open.bigmodel.cn/api/paas/v4/chat/completions', 'zhipu-official', 56, true);

-- Priority reference for zhipu LLM models:
--   glm-4.7 (via easyart)  = 70
--   glm-4-plus             = 58  ← new (strongest BigModel direct)
--   glm-4-air              = 57  ← new
--   glm-4-airx             = 56  ← new
--   glm-4-flash            = 55
--   glm-4-flashx           = 54
--   glm-4.7-official       = 52  (disabled, 1211 model-not-found)
--   glm-z1-flash           = 50  (thinking model, slow)
