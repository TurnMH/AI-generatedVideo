-- migrations/001_init.up.sql
-- video_db schema for video-service

CREATE TABLE IF NOT EXISTS video_tasks (
    id            BIGSERIAL PRIMARY KEY,
    project_id    BIGINT        NOT NULL,
    episode_id    BIGINT,
    user_id       BIGINT        NOT NULL,
    image_urls    JSONB         NOT NULL DEFAULT '[]',
    style_preset  VARCHAR(64)   NOT NULL DEFAULT 'anime',
    motion_mode   VARCHAR(32)   NOT NULL DEFAULT 'gentle',
    audio_url     TEXT          NOT NULL DEFAULT '',
    subtitle_text TEXT          NOT NULL DEFAULT '',
    model_name    VARCHAR(64)   NOT NULL DEFAULT 'kling',
    status        VARCHAR(32)   NOT NULL DEFAULT 'pending',
    result_url    TEXT          NOT NULL DEFAULT '',
    duration_sec  FLOAT,
    error_msg     TEXT          NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS video_clips (
    id               BIGSERIAL PRIMARY KEY,
    video_task_id    BIGINT       NOT NULL REFERENCES video_tasks(id) ON DELETE CASCADE,
    clip_order       INT          NOT NULL,
    source_image_url TEXT         NOT NULL,
    clip_url         TEXT         NOT NULL DEFAULT '',
    duration_sec     FLOAT,
    model_used       VARCHAR(64)  NOT NULL DEFAULT '',
    status           VARCHAR(32)  NOT NULL DEFAULT 'pending',
    error_msg        TEXT         NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_video_tasks_project_id  ON video_tasks (project_id);
CREATE INDEX IF NOT EXISTS idx_video_tasks_episode_id  ON video_tasks (episode_id);
CREATE INDEX IF NOT EXISTS idx_video_tasks_user_id     ON video_tasks (user_id);
CREATE INDEX IF NOT EXISTS idx_video_tasks_status      ON video_tasks (status);
CREATE INDEX IF NOT EXISTS idx_video_clips_task_id     ON video_clips (video_task_id);

-- Auto-update updated_at on row changes
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_video_tasks_updated_at ON video_tasks;
CREATE TRIGGER trg_video_tasks_updated_at
    BEFORE UPDATE ON video_tasks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS trg_video_clips_updated_at ON video_clips;
CREATE TRIGGER trg_video_clips_updated_at
    BEFORE UPDATE ON video_clips
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
