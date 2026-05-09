-- 002_character_four_panels.up.sql
-- 角色四视图一致性改造：asset 增加分栏图 URL 数组、拼接图 URL、锁定种子。
-- 详见 pending.md 方案 A（分次生成 + 服务端拼接）。
ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS panel_images text[] DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS composite_image_url text,
    ADD COLUMN IF NOT EXISTS seed bigint DEFAULT -1;
