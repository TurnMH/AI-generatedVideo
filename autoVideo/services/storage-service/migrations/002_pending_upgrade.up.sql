-- Enhance files table for project-scoped tracking
ALTER TABLE files ADD COLUMN IF NOT EXISTS project_id BIGINT;
ALTER TABLE files ADD COLUMN IF NOT EXISTS category VARCHAR(50) DEFAULT 'other';
ALTER TABLE files ADD COLUMN IF NOT EXISTS is_current BOOLEAN DEFAULT true;
ALTER TABLE files ADD COLUMN IF NOT EXISTS version_number INTEGER DEFAULT 1;
ALTER TABLE files ADD COLUMN IF NOT EXISTS related_id BIGINT;
ALTER TABLE files ADD COLUMN IF NOT EXISTS related_type VARCHAR(50) DEFAULT '';
ALTER TABLE files ADD COLUMN IF NOT EXISTS label TEXT DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_files_project_id ON files(project_id);
CREATE INDEX IF NOT EXISTS idx_files_category ON files(category);
CREATE INDEX IF NOT EXISTS idx_files_project_category ON files(project_id, category);
