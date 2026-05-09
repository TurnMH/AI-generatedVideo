ALTER TABLE projects ADD COLUMN IF NOT EXISTS project_type VARCHAR(20) NOT NULL DEFAULT 'video';

UPDATE projects
SET project_type = CASE
  WHEN 'media:comics' = ANY(style_tags) THEN 'comics'
  WHEN 'media:music' = ANY(style_tags) THEN 'music'
  ELSE 'video'
END
WHERE project_type IS NULL
   OR project_type = ''
   OR project_type NOT IN ('video', 'comics', 'music');

CREATE INDEX IF NOT EXISTS idx_projects_user_type ON projects (user_id, project_type);
