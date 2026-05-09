-- 002_serial_video.up.sql
-- 串行视频流程：分镜增加场景分组 key 和连续帧字段
-- scene_group_key: 标准化后的场景名称，用于场景内串行生成
-- is_scene_first_clip: 是否为该场景的第一个分镜（首次生成用分镜图，后续用前一视频末帧）
-- end_frame_image_url: 从前一个视频提取的末帧图 URL（作为本视频的首帧起点）

ALTER TABLE storyboards
    ADD COLUMN IF NOT EXISTS scene_group_key     VARCHAR(200) DEFAULT '',
    ADD COLUMN IF NOT EXISTS is_scene_first_clip BOOLEAN      DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS end_frame_image_url TEXT         DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_storyboards_scene_group_key
    ON storyboards(project_id, scene_group_key)
    WHERE scene_group_key != '';
