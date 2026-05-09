DROP INDEX IF EXISTS idx_split_configs_script;
DROP TABLE IF EXISTS split_configs;

ALTER TABLE scenes DROP COLUMN IF EXISTS estimated_duration;
ALTER TABLE scenes DROP COLUMN IF EXISTS word_count;

ALTER TABLE scripts DROP COLUMN IF EXISTS version;
ALTER TABLE scripts DROP COLUMN IF EXISTS file_size;
