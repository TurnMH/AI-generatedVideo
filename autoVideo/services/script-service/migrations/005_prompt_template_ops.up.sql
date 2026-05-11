-- Operationalize prompt_templates schema and seed storyboard templates that
-- match the style keys used by project-service.

ALTER TABLE prompt_templates
    ADD COLUMN IF NOT EXISTS resource_type VARCHAR(32) NOT NULL DEFAULT 'storyboard',
    ADD COLUMN IF NOT EXISTS model_binding VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS template_file_url TEXT NOT NULL DEFAULT '';

UPDATE prompt_templates
SET resource_type = 'storyboard'
WHERE COALESCE(resource_type, '') = '';

UPDATE prompt_templates
SET model_binding = ''
WHERE model_binding IS NULL;

UPDATE prompt_templates
SET template_file_url = ''
WHERE template_file_url IS NULL;

ALTER TABLE prompt_templates DROP CONSTRAINT IF EXISTS prompt_templates_style_key_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_prompt_templates_style_resource_model_version
    ON prompt_templates(style_key, resource_type, model_binding, version);

INSERT INTO prompt_templates (
    name,
    style_key,
    description,
    content,
    resource_type,
    model_binding,
    version,
    is_active,
    sort_order,
    template_file_url
)
VALUES
(
    'Storyboard Anime 2D Ops V2',
    'storyboard_anime2d',
    '可运营的 2D 动漫分镜出图模板，强调角色一致性、镜头可读性和空间层次。',
    $$masterpiece, best quality, cinematic 2D anime storyboard frame, single decisive shot, clean line art, readable staging, strong silhouette clarity,
scene: {scene},
characters: {characters},
primary action: {action},
mood: {mood},
requirements: maintain identical face, hairstyle, costume layers, accessories and subject proportions across the sequence; keep one coherent camera setup; make foreground, midground and background readable; preserve lighting direction, material texture, environment logic and emotional clarity; no text, no watermark, no collage, no duplicate characters$$,
    'storyboard',
    '',
    'ops-v2',
    TRUE,
    10,
    ''
),
(
    'Storyboard Anime 3D Ops V2',
    'storyboard_anime3d',
    '可运营的 3D 动漫分镜出图模板，强调体积、材质和空间真实感。',
    $$masterpiece, best quality, stylized 3D animation storyboard frame, cinematic keyframe, volumetric readability, controlled lens language,
scene: {scene},
characters: {characters},
primary action: {action},
mood: {mood},
requirements: maintain identical character identity, hairstyle, wardrobe layers, accessories and body scale across shots; preserve believable material response, contact shadows and lighting direction; keep the frame focused on one coherent action beat; make depth layering and environment continuity immediately readable; no text, no watermark, no split-screen, no duplicate people$$,
    'storyboard',
    '',
    'ops-v2',
    TRUE,
    20,
    ''
),
(
    'Storyboard Cinematic Ops V2',
    'storyboard_cinematic',
    '可运营的真人电影分镜出图模板，强调镜头语法、服化一致性和真实光线。',
    $$RAW photo, cinematic storyboard still, live-action film frame, grounded realism, production-ready composition,
scene: {scene},
characters: {characters},
primary action: {action},
mood: {mood},
requirements: preserve identical face, hairline, costume layers, props and body proportions across the sequence; use one clear lens choice and one readable action beat; keep practical lighting direction, environment geography, wardrobe materials and emotional focus consistent; avoid fantasy drift, extra people, duplicate limbs, text, watermark or collage$$,
    'storyboard',
    '',
    'ops-v2',
    TRUE,
    30,
    ''
),
(
    'Storyboard Live Action Short Ops V2',
    'storyboard_live_action',
    '可运营的短剧真人分镜出图模板，强调情绪传达、主体清晰和短剧节奏。',
    $$RAW photo, premium short-drama storyboard frame, live-action short-form cinematic still, emotionally readable staging,
scene: {scene},
characters: {characters},
primary action: {action},
mood: {mood},
requirements: lock identical face, hairstyle, wardrobe, accessories and subject scale across shots; keep the frame centered on one emotionally clear beat; make facial direction, body pose, environment depth and motivated lighting immediately readable; preserve wardrobe materials and scene geography; no extra people, no text, no watermark, no poster layout$$,
    'storyboard',
    '',
    'ops-v2',
    TRUE,
    40,
    ''
)
ON CONFLICT (style_key, resource_type, model_binding, version)
DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    content = EXCLUDED.content,
    is_active = EXCLUDED.is_active,
    sort_order = EXCLUDED.sort_order,
    template_file_url = EXCLUDED.template_file_url,
    updated_at = NOW();