CREATE TABLE IF NOT EXISTS assets (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL,
    type VARCHAR(20) NOT NULL CHECK (type IN ('character','scene','prop')),
    name VARCHAR(200) NOT NULL,
    description TEXT DEFAULT '',
    image_url TEXT DEFAULT '',
    consistency_ref JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'pending',
    is_locked BOOLEAN DEFAULT false,
    is_manual BOOLEAN DEFAULT false,
    prompt_used TEXT DEFAULT '',
    agent_history JSONB DEFAULT '[]',
    episode_ids INTEGER[] DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assets_project_id ON assets(project_id);
CREATE INDEX IF NOT EXISTS idx_assets_type ON assets(type);
CREATE INDEX IF NOT EXISTS idx_assets_status ON assets(status);
CREATE INDEX IF NOT EXISTS idx_assets_project_type ON assets(project_id, type);
