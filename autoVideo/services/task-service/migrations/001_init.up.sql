-- tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id          BIGSERIAL PRIMARY KEY,
    task_type   VARCHAR(64)  NOT NULL,
    payload     JSONB,
    priority    INTEGER      NOT NULL DEFAULT 0,
    status      VARCHAR(32)  NOT NULL DEFAULT 'pending',
    retry_count INTEGER      NOT NULL DEFAULT 0,
    max_retries INTEGER      NOT NULL DEFAULT 3,
    user_id     BIGINT       NOT NULL,
    error_msg   TEXT,
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tasks_task_type  ON tasks (task_type);
CREATE INDEX IF NOT EXISTS idx_tasks_status     ON tasks (status);
CREATE INDEX IF NOT EXISTS idx_tasks_priority   ON tasks (priority DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_user_id    ON tasks (user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status_retry ON tasks (status, retry_count)
    WHERE status = 'failed';

-- task_progress table
CREATE TABLE IF NOT EXISTS task_progress (
    id         BIGSERIAL PRIMARY KEY,
    task_id    BIGINT       NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    progress   INTEGER      NOT NULL CHECK (progress >= 0 AND progress <= 100),
    message    TEXT,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_progress_task_id ON task_progress (task_id);
