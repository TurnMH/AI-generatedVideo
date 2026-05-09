-- 004_character_groups.down.sql
DROP INDEX IF EXISTS idx_assets_group_id;
ALTER TABLE assets
    DROP COLUMN IF EXISTS asset_sort_order,
    DROP COLUMN IF EXISTS variant_name,
    DROP COLUMN IF EXISTS group_id;

DROP INDEX IF EXISTS idx_character_groups_project_id;
DROP TABLE IF EXISTS character_groups;
