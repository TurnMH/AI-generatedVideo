-- Migration 009 down: Restore Wan2.1 original values
UPDATE models SET
    provider         = 'bytedance',
    api_endpoint     = 'https://visual.volcengineapi.com/?Action=CVVideoGenerateTask',
    model_key        = 'wan2.1',
    config           = '{"model":"wan2.1-t2v-turbo","resolution":"720p","aspect_ratio":"16:9","duration":5,"fps":16,"watermark":false}',
    description      = 'ByteDance Wan2.1 turbo video generation'
WHERE name = 'Wan2.1';
