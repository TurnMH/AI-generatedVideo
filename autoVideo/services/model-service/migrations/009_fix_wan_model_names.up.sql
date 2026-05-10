-- Migration 009: Fix Wan2.1 model configuration
-- Corrects: provider bytedance→aliyun, api_endpoint→DashScope, model name wan2.1-t2v-turbo→wanx2.1-t2v-turbo

UPDATE models SET
    provider         = 'aliyun',
    api_endpoint     = 'https://dashscope.aliyuncs.com/api/v1/services/aigc/video-generation/video-synthesis',
    model_key        = 'wan2.1-t2v',
    config           = '{"model":"wanx2.1-t2v-turbo","resolution":"720p","aspect_ratio":"16:9","duration":5,"fps":16,"watermark":false}',
    description      = 'Alibaba DashScope Wan2.1 t2v-turbo text-to-video generation'
WHERE name = 'Wan2.1';
