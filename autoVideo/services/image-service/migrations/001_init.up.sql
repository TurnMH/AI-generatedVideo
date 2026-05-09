-- migrations/001_init.up.sql

CREATE TABLE IF NOT EXISTS image_tasks (
    id               BIGSERIAL PRIMARY KEY,
    scene_id         BIGINT,
    project_id       BIGINT NOT NULL,
    user_id          BIGINT NOT NULL,
    prompt           TEXT NOT NULL,
    negative_prompt  TEXT DEFAULT '',
    style_preset     VARCHAR(64) DEFAULT 'anime',
    style_reference_url TEXT,
    character_ids    JSONB DEFAULT '[]',
    model_name       VARCHAR(64) NOT NULL DEFAULT 'sdxl',
    width            INT DEFAULT 512,
    height           INT DEFAULT 768,
    steps            INT DEFAULT 20,
    cfg_scale        FLOAT DEFAULT 7.0,
    seed             BIGINT DEFAULT -1,
    status           VARCHAR(32) DEFAULT 'pending',
    result_url       TEXT,
    thumbnail_url    TEXT,
    error_msg        TEXT,
    metadata         JSONB DEFAULT '{}',
    created_at       TIMESTAMPTZ DEFAULT NOW(),
    updated_at       TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_image_tasks_project_id ON image_tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_image_tasks_scene_id   ON image_tasks(scene_id);
CREATE INDEX IF NOT EXISTS idx_image_tasks_user_id    ON image_tasks(user_id);
CREATE INDEX IF NOT EXISTS idx_image_tasks_status     ON image_tasks(status);
CREATE INDEX IF NOT EXISTS idx_image_tasks_created_at ON image_tasks(created_at DESC);
