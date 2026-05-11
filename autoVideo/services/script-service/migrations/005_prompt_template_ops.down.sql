DROP INDEX IF EXISTS idx_prompt_templates_style_resource_model_version;

DELETE FROM prompt_templates
WHERE style_key IN (
    'storyboard_anime2d',
    'storyboard_anime3d',
    'storyboard_cinematic',
    'storyboard_live_action'
)
AND version = 'ops-v2';

ALTER TABLE prompt_templates
    DROP COLUMN IF EXISTS template_file_url,
    DROP COLUMN IF EXISTS model_binding,
    DROP COLUMN IF EXISTS resource_type;

ALTER TABLE prompt_templates
    ADD CONSTRAINT prompt_templates_style_key_key UNIQUE (style_key);