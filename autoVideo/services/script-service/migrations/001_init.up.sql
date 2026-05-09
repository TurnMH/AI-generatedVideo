-- migrations/001_init.up.sql

CREATE TABLE IF NOT EXISTS scripts (
    id           BIGSERIAL PRIMARY KEY,
    project_id   BIGINT NOT NULL,
    episode_id   BIGINT,
    title        VARCHAR(256),
    raw_text     TEXT,
    file_url     TEXT,
    parse_status VARCHAR(32) DEFAULT 'pending',
    llm_result   JSONB,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_scripts_project_id ON scripts(project_id);
CREATE INDEX IF NOT EXISTS idx_scripts_episode_id ON scripts(episode_id);
CREATE INDEX IF NOT EXISTS idx_scripts_deleted_at ON scripts(deleted_at);

CREATE TABLE IF NOT EXISTS scenes (
    id           BIGSERIAL PRIMARY KEY,
    script_id    BIGINT NOT NULL REFERENCES scripts(id),
    episode_id   BIGINT,
    scene_order  INT NOT NULL,
    description  TEXT,
    setting      TEXT,
    emotion      VARCHAR(64),
    characters   JSONB DEFAULT '[]',
    prompt_draft TEXT,
    status       VARCHAR(32) DEFAULT 'draft',
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scenes_script_id  ON scenes(script_id);
CREATE INDEX IF NOT EXISTS idx_scenes_episode_id ON scenes(episode_id);

CREATE TABLE IF NOT EXISTS characters_extracted (
    id                 BIGSERIAL PRIMARY KEY,
    script_id          BIGINT NOT NULL REFERENCES scripts(id),
    name               VARCHAR(128) NOT NULL,
    role_desc          TEXT,
    first_appear_scene INT,
    relationships      JSONB DEFAULT '{}',
    created_at         TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_characters_script_id ON characters_extracted(script_id);

CREATE TABLE IF NOT EXISTS script_assets (
    id          BIGSERIAL PRIMARY KEY,
    script_id   BIGINT NOT NULL REFERENCES scripts(id),
    asset_type  VARCHAR(32),
    name        VARCHAR(128),
    description TEXT,
    scene_ids   JSONB DEFAULT '[]',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_script_assets_script_id ON script_assets(script_id);
