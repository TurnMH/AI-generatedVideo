-- migrations/001_init.up.sql
-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT        NOT NULL,
    title       VARCHAR(256)  NOT NULL,
    description TEXT          NOT NULL DEFAULT '',
    cover_url   TEXT          NOT NULL DEFAULT '',
    status      VARCHAR(32)   NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_projects_user_id   ON projects (user_id);
CREATE INDEX IF NOT EXISTS idx_projects_deleted_at ON projects (deleted_at);
CREATE INDEX IF NOT EXISTS idx_projects_status     ON projects (status);

-- Episodes table
CREATE TABLE IF NOT EXISTS episodes (
    id             BIGSERIAL PRIMARY KEY,
    project_id     BIGINT       NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    episode_number INT          NOT NULL,
    title          VARCHAR(256) NOT NULL DEFAULT '',
    summary        TEXT         NOT NULL DEFAULT '',
    status         VARCHAR(32)  NOT NULL DEFAULT 'draft',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_episodes_project_id ON episodes (project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_episodes_project_number ON episodes (project_id, episode_number);

-- Project snapshots table
CREATE TABLE IF NOT EXISTS project_snapshots (
    id            BIGSERIAL PRIMARY KEY,
    project_id    BIGINT      NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version       INT         NOT NULL,
    snapshot_data JSONB       NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_snapshots_project_id ON project_snapshots (project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_snapshots_project_version ON project_snapshots (project_id, version);
