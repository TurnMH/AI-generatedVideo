-- 004_character_groups.up.sql
-- 角色组：支持同一角色的多个造型变体管理（视频串行流程）
-- 每个 CharacterGroup 代表一个"角色身份"（如：李明），其下挂多个 Asset 造型变体（日常服/战斗服等）。

CREATE TABLE IF NOT EXISTS character_groups (
    id               BIGSERIAL PRIMARY KEY,
    project_id       BIGINT        NOT NULL,
    name             VARCHAR(100)  NOT NULL,           -- 角色名，与剧本中人物名对应
    description      TEXT          DEFAULT '',
    voice_model      VARCHAR(200)  DEFAULT '',         -- 角色级音色绑定（所有造型共用）
    voice_sample_url TEXT          DEFAULT '',         -- 试听参考音频 URL
    sort_order       INT           DEFAULT 0,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_character_groups_project_id ON character_groups(project_id);

-- Asset 表新增造型组字段
ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS group_id      BIGINT  REFERENCES character_groups(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS variant_name  VARCHAR(100) DEFAULT '',   -- 造型名称，如"日常便装"
    ADD COLUMN IF NOT EXISTS asset_sort_order INT DEFAULT 0;          -- 组内造型排序

CREATE INDEX IF NOT EXISTS idx_assets_group_id ON assets(group_id);
