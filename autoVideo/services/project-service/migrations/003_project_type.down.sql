DROP INDEX IF EXISTS idx_projects_user_type;
ALTER TABLE projects DROP COLUMN IF EXISTS project_type;
