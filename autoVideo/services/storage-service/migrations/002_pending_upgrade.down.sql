DROP INDEX IF EXISTS idx_files_project_category;
DROP INDEX IF EXISTS idx_files_category;
DROP INDEX IF EXISTS idx_files_project_id;

ALTER TABLE files DROP COLUMN IF EXISTS label;
ALTER TABLE files DROP COLUMN IF EXISTS related_type;
ALTER TABLE files DROP COLUMN IF EXISTS related_id;
ALTER TABLE files DROP COLUMN IF EXISTS version_number;
ALTER TABLE files DROP COLUMN IF EXISTS is_current;
ALTER TABLE files DROP COLUMN IF EXISTS category;
ALTER TABLE files DROP COLUMN IF EXISTS project_id;
