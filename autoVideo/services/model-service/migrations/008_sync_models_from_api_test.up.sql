-- 修复 model_key 错误
UPDATE models SET model_key='kimi-k2.5', is_active=true WHERE model_key='Kimi-K2.5';
UPDATE models SET model_key='veo3.1', name='Veo 3.1', api_endpoint='https://api.easyart.cc/v1/chat/completions', api_key_ref='easyart', is_active=true WHERE model_key='hubagi-voe3.1';

-- 下架确认不可用模型
UPDATE models SET is_active=false WHERE model_key IN (
  'hubagi-TC-GV','sora2','gpt-image-1','MiniMax-M2.5',
  'xingrong2.5','banana2.0','banana2.1','grok-4'
);

-- 补充 cogview / wanx api_key_ref
UPDATE models SET api_key_ref='zhipu-official', api_endpoint='https://open.bigmodel.cn/api/paas/v4/images/generations'
  WHERE model_key IN ('cogview-4','cogview-3-plus');
UPDATE models SET api_key_ref='dashscope'
  WHERE model_key IN ('wanx2.1-t2i-plus','wanx2.1-t2i-turbo');
UPDATE models SET api_endpoint='https://api.easyart.cc/v1/chat/completions', api_key_ref='easyart', is_active=true
  WHERE model_key='veo3.1';
UPDATE models SET api_endpoint='https://api.easyart.cc/v1/chat/completions', api_key_ref='easyart', is_active=true
  WHERE model_key='gemini-3.1-flash-image' AND provider='google' AND is_active=true;

-- DeepSeek 系列 (easyart)
INSERT INTO models (model_key, name, type, provider, api_endpoint, api_key_ref, is_active, priority, context_window, input_price, output_price, description)
VALUES
  ('deepseek-r1',            'DeepSeek R1',             'llm','deepseek','https://api.easyart.cc/v1/chat/completions','easyart',true,75,65536,0.002,0.008,'DeepSeek R1 思维链推理'),
  ('deepseek-r1-0528',       'DeepSeek R1 (0528)',      'llm','deepseek','https://api.easyart.cc/v1/chat/completions','easyart',true,74,65536,0.002,0.008,'DeepSeek R1 0528'),
  ('deepseek-v3',            'DeepSeek V3',             'llm','deepseek','https://api.easyart.cc/v1/chat/completions','easyart',true,73,65536,0.001,0.003,'DeepSeek V3 旗舰'),
  ('deepseek-v3-0324',       'DeepSeek V3 (0324)',      'llm','deepseek','https://api.easyart.cc/v1/chat/completions','easyart',true,72,65536,0.001,0.003,'DeepSeek V3 0324'),
  ('deepseek-v3.1',          'DeepSeek V3.1',           'llm','deepseek','https://api.easyart.cc/v1/chat/completions','easyart',true,76,65536,0.001,0.003,'DeepSeek V3.1'),
  ('deepseek-v3.1-terminus', 'DeepSeek V3.1 Terminus',  'llm','deepseek','https://api.easyart.cc/v1/chat/completions','easyart',true,71,65536,0.001,0.003,'DeepSeek V3.1 Terminus'),
  ('deepseek-v3.2',          'DeepSeek V3.2',           'llm','deepseek','https://api.easyart.cc/v1/chat/completions','easyart',true,77,65536,0.001,0.003,'DeepSeek V3.2'),
  ('deepseek-v4-flash',      'DeepSeek V4 Flash',       'llm','deepseek','https://api.easyart.cc/v1/chat/completions','easyart',true,78,65536,0.0005,0.002,'DeepSeek V4 Flash'),
  ('deepseek-v4-pro',        'DeepSeek V4 Pro',         'llm','deepseek','https://api.easyart.cc/v1/chat/completions','easyart',true,80,65536,0.002,0.008,'DeepSeek V4 Pro 旗舰')
ON CONFLICT DO NOTHING;

-- 其他 easyart LLM
INSERT INTO models (model_key, name, type, provider, api_endpoint, api_key_ref, is_active, priority, context_window, input_price, output_price, description)
VALUES
  ('claude-haiku-4-5-20251001','Claude Haiku 4.5',     'llm','anthropic','https://api.easyart.cc/v1/chat/completions','easyart',true,62,200000,0.001,0.003,'Claude Haiku 4.5 快速版'),
  ('claude-opus-4-5-20251101','Claude Opus 4.5',       'llm','anthropic','https://api.easyart.cc/v1/chat/completions','easyart',true,65,200000,0.015,0.075,'Claude Opus 4.5'),
  ('gemini-2.5-flash-lite',  'Gemini 2.5 Flash Lite', 'llm','google',  'https://api.easyart.cc/v1/chat/completions','easyart',true,53,1000000,0.0001,0.0003,'Gemini 2.5 Flash 轻量版'),
  ('gemini-3-flash-preview',  'Gemini 3 Flash Preview','llm','google',  'https://api.easyart.cc/v1/chat/completions','easyart',true,58,1000000,0.0002,0.0006,'Gemini 3 Flash 预览版'),
  ('gemini-3-pro',            'Gemini 3 Pro',          'llm','google',  'https://api.easyart.cc/v1/chat/completions','easyart',true,62,1000000,0.0035,0.0105,'Gemini 3 Pro'),
  ('gpt-4o',                  'GPT-4o',                'llm','openai',  'https://api.easyart.cc/v1/chat/completions','easyart',true,60,128000,0.005,0.015,'GPT-4o 多模态旗舰'),
  ('gpt-4o-mini',             'GPT-4o Mini',           'llm','openai',  'https://api.easyart.cc/v1/chat/completions','easyart',true,55,128000,0.00015,0.0006,'GPT-4o Mini'),
  ('gpt-5-mini',              'GPT-5 Mini',            'llm','openai',  'https://api.easyart.cc/v1/chat/completions','easyart',true,63,200000,0.002,0.006,'GPT-5 Mini'),
  ('gpt-5-nano',              'GPT-5 Nano',            'llm','openai',  'https://api.easyart.cc/v1/chat/completions','easyart',true,60,200000,0.0005,0.002,'GPT-5 Nano 极速版'),
  ('gpt-5.1',                 'GPT-5.1',               'llm','openai',  'https://api.easyart.cc/v1/chat/completions','easyart',true,66,200000,0.004,0.012,'GPT-5.1'),
  ('gpt-5.3-chat',            'GPT-5.3 Chat',          'llm','openai',  'https://api.easyart.cc/v1/chat/completions','easyart',true,68,200000,0.006,0.018,'GPT-5.3 Chat'),
  ('gpt-5.5',                 'GPT-5.5',               'llm','openai',  'https://api.easyart.cc/v1/chat/completions','easyart',true,72,200000,0.008,0.025,'GPT-5.5 旗舰'),
  ('kimi-k2.6',               'Kimi K2.6',             'llm','moonshot','https://api.easyart.cc/v1/chat/completions','easyart',true,70,200000,0.003,0.009,'Kimi K2.6'),
  ('MiniMax-M2.7',            'MiniMax M2.7',          'llm','minimax', 'https://api.easyart.cc/v1/chat/completions','easyart',true,72,200000,0.003,0.009,'MiniMax M2.7'),
  ('glm-5',                   'GLM-5',                 'llm','zhipu',   'https://api.easyart.cc/v1/chat/completions','easyart',true,62,128000,0.002,0.006,'GLM-5 旗舰版'),
  ('glm-5.1',                 'GLM-5.1',               'llm','zhipu',   'https://api.easyart.cc/v1/chat/completions','easyart',true,63,128000,0.002,0.006,'GLM-5.1'),
  ('qwen3-coder',             'Qwen3 Coder',           'llm','dashscope','https://api.easyart.cc/v1/chat/completions','easyart',true,62,200000,0.002,0.006,'Qwen3 代码专用'),
  ('qwen3-vl-plus',           'Qwen3 VL Plus',         'llm','dashscope','https://api.easyart.cc/v1/chat/completions','easyart',true,60,200000,0.002,0.006,'Qwen3 视觉语言'),
  ('qwen3.5-397b-a17b',       'Qwen3.5 397B',          'llm','dashscope','https://api.easyart.cc/v1/chat/completions','easyart',true,65,128000,0.003,0.009,'Qwen3.5 397B 大参数'),
  ('qwen3.6-plus',            'Qwen3.6 Plus',          'llm','dashscope','https://api.easyart.cc/v1/chat/completions','easyart',true,66,200000,0.003,0.009,'Qwen3.6 Plus')
ON CONFLICT DO NOTHING;

-- 图片模型
INSERT INTO models (model_key, name, type, provider, api_endpoint, api_key_ref, is_active, priority, description)
VALUES
  ('gpt-image-2',              'GPT Image 2',              'image','openai', 'https://api.easyart.cc/v1/images/generations','easyart',true,72,'GPT Image 2'),
  ('gpt-image-2-all',          'GPT Image 2 All',          'image','openai', 'https://api.easyart.cc/v1/images/generations','easyart',true,73,'GPT Image 2 All 快速版'),
  ('gpt-image-2-u',            'GPT Image 2 Ultra',        'image','openai', 'https://api.easyart.cc/v1/images/generations','easyart',true,74,'GPT Image 2 Ultra'),
  ('gemini-2.5-flash-image',   'Gemini 2.5 Flash Image',   'image','google', 'https://api.easyart.cc/v1/chat/completions','easyart',true,68,'Gemini 2.5 Flash 图片 (chat)'),
  ('gemini-3-pro-image-preview','Gemini 3 Pro Image',      'image','google', 'https://api.easyart.cc/v1/chat/completions','easyart',true,72,'Gemini 3 Pro 图片预览 (chat)')
ON CONFLICT DO NOTHING;

-- 视频模型
INSERT INTO models (model_key, name, type, provider, api_endpoint, api_key_ref, is_active, priority, description)
VALUES
  ('veo3.1-fast',             'Veo 3.1 Fast',            'video','google',   'https://api.easyart.cc/v1/chat/completions','easyart',true,78,'Veo 3.1 快速版'),
  ('veo3.1-pro',              'Veo 3.1 Pro',             'video','google',   'https://api.easyart.cc/v1/chat/completions','easyart',true,80,'Veo 3.1 Pro 旗舰'),
  ('MiniMax-Hailuo-02',       'MiniMax Hailuo 02',       'video','minimax',  'https://api.easyart.cc/v1/chat/completions','easyart',true,70,'海螺视频 02'),
  ('MiniMax-Hailuo-2.3',      'MiniMax Hailuo 2.3',      'video','minimax',  'https://api.easyart.cc/v1/chat/completions','easyart',true,72,'海螺视频 2.3'),
  ('MiniMax-Hailuo-2.3-Fast', 'MiniMax Hailuo 2.3 Fast', 'video','minimax',  'https://api.easyart.cc/v1/chat/completions','easyart',true,71,'海螺视频 2.3 快速'),
  ('doubao-seedream-4-5-251128','Doubao SeedDream 4.5',   'video','bytedance','https://api.easyart.cc/v1/chat/completions','easyart',true,68,'抖音 SeedDream 4.5'),
  ('mimo-v2.5-pro',           'Mimo V2.5 Pro',           'video','google',   'https://api.easyart.cc/v1/chat/completions','easyart',true,65,'Mimo V2.5 Pro 视频')
ON CONFLICT DO NOTHING;

-- =============================================================================
-- 009 追加：补全 nano-banana/kling/viduq3/doubao-seedance 的 api_key_ref + endpoint
-- 来源：fengxi 项目 docs-credentials.md 及 0428all/*.sql 验证 (2026-04-xx)
-- =============================================================================

-- 1. doubao-seedance: 使用火山方舟 seedance 专属 key
UPDATE models SET
  api_key_ref   = 'runtime.video.doubao.seedance',
  api_endpoint  = 'https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks',
  description   = '豆包 Seedance 1.5-Pro 视频生成；content 数组格式，model=doubao-seedance-1-5-pro-251215；key=cd0cd314',
  updated_at    = NOW()
WHERE model_key = 'doubao-seedance';

-- 2. kling: 腾讯 VCLM 直连（TC3-HMAC-SHA256，SecretId:SecretKey 格式存于 plain_key）
UPDATE models SET
  api_key_ref   = 'runtime.video.vclm',
  api_endpoint  = 'https://vclm.tencentcloudapi.com',
  description   = '快手 Kling 3.0 视频生成；TC3-HMAC-SHA256鉴权，Action=SubmitImageToVideoGeneralJob，Version=2024-05-23',
  updated_at    = NOW()
WHERE model_key = 'kling';

-- 3-6. vidu 系列：Authorization: Token {vda_...key}（非 Bearer）
UPDATE models SET
  api_key_ref   = 'runtime.video.vidu',
  api_endpoint  = 'https://api.vidu.cn/ent/v2',
  description   = 'Vidu Q3-Pro；Token auth，路径 /ent/v2/text2video 或 /reference2video',
  updated_at    = NOW()
WHERE model_key = 'vidu';

UPDATE models SET
  api_key_ref   = 'runtime.video.vidu',
  api_endpoint  = 'https://api.vidu.cn/ent/v2',
  description   = 'Vidu Q3-Mix 参考图生视频；Token auth，路径 /ent/v2/reference2video，images 字段为字符串URL数组',
  updated_at    = NOW()
WHERE model_key = 'vidu-mix';

UPDATE models SET
  api_key_ref   = 'runtime.video.vidu.offpeak',
  api_endpoint  = 'https://api.vidu.cn/ent/v2',
  description   = 'Vidu Q3-Pro 低峰通道；Token auth',
  updated_at    = NOW()
WHERE model_key = 'vidu-offpeak';

UPDATE models SET
  api_key_ref   = 'runtime.video.vidu.offpeak',
  api_endpoint  = 'https://api.vidu.cn/ent/v2',
  description   = 'Vidu Q3-Mix 低峰通道；Token auth，路径 /ent/v2/reference2video',
  updated_at    = NOW()
WHERE model_key = 'vidu-mix-offpeak';

-- 7. nano-banana: Gemini 图像模型，通过 ppai.pro 代理（chat/completions + IMAGE responseModality）
INSERT INTO models (
  name, provider, type, model_key,
  api_key_ref, api_endpoint,
  is_active, priority, sort_order,
  capability_tags, description
) VALUES (
  'Nano Banana (Gemini Image via ppai)',
  'ppai', 'image', 'nano-banana',
  'runtime.image.ppai',
  'https://api.ppai.pro/v1/chat/completions',
  true, 80, 80,
  ARRAY['image_generation','image_reference'],
  'Gemini 图像参考模型，ppai.pro代理；请求格式=chat+generationConfig:{responseModalities:["TEXT","IMAGE"]}；key=sk-QeXuGa...'
) ON CONFLICT DO NOTHING;

-- 8. auth_db: ppai.pro key（写在 auth-service 迁移中，此处仅注释）
-- INSERT INTO auth_db.system_api_keys (provider, key_alias, plain_key, base_url, model_scope)
-- VALUES ('runtime.image.ppai', 'ppai.pro 图像渠道 (nano-banana)',
--         'sk-QeXuGasPbBN7yjHa19E7A892Af2546A2BfB7D2B6B134A091',
--         'https://api.ppai.pro', 'nano-banana');
