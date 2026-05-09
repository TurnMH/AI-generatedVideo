CREATE TABLE IF NOT EXISTS script_libraries (
    id          BIGSERIAL PRIMARY KEY,
    title       VARCHAR(256) NOT NULL,
    author      VARCHAR(128) DEFAULT '',
    description TEXT DEFAULT '',
    cover_url   TEXT DEFAULT '',
    file_url    TEXT DEFAULT '',
    file_size   INT DEFAULT 0,
    word_count  INT DEFAULT 0,
    genre       VARCHAR(64) DEFAULT '',
    tags        JSONB DEFAULT '[]',
    source      VARCHAR(32) NOT NULL DEFAULT 'uploaded',
    project_id  BIGINT,
    user_id     BIGINT DEFAULT 0,
    is_public   BOOLEAN DEFAULT TRUE,
    sort_order  INT DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_script_libraries_source ON script_libraries (source);
CREATE INDEX idx_script_libraries_user_id ON script_libraries (user_id);
CREATE INDEX idx_script_libraries_project_id ON script_libraries (project_id);

-- Seed showcase scripts
INSERT INTO script_libraries (title, author, description, genre, tags, source, word_count, sort_order) VALUES
('西游记', '吴承恩', '中国四大名著之一，讲述唐僧师徒四人西天取经的故事。全书共100回，约82万字，涵盖神话、冒险、喜剧等多种元素，是漫画/动画改编的经典素材。', '古典名著', '["神话","冒险","喜剧","四大名著"]', 'showcase', 820000, 1),
('三国演义', '罗贯中', '以东汉末年为背景，描绘魏、蜀、吴三国之间的政治与军事斗争。情节宏大，人物众多，对话精彩，适合分镜改编。', '古典名著', '["历史","战争","谋略","四大名著"]', 'showcase', 730000, 2),
('哈利·波特与魔法石', 'J.K.罗琳', '一个孤儿男孩发现自己是巫师，进入霍格沃茨魔法学校的奇幻冒险。场景丰富，视觉想象力极强，非常适合漫画化。', '奇幻小说', '["魔法","校园","冒险","经典"]', 'showcase', 77000, 3),
('斗破苍穹', '天蚕土豆', '少年萧炎在斗气大陆修炼成长的热血故事。打斗场面丰富，角色鲜明，是网文改编漫画的标杆作品。', '玄幻小说', '["修炼","热血","网文","打斗"]', 'showcase', 5300000, 4),
('鬼吹灯之精绝古城', '天下霸唱', '胡八一等人组成的摸金小队深入沙漠探寻精绝古城的探险故事。悬疑紧张的氛围和丰富的场景描写极适合分镜。', '探险悬疑', '["盗墓","探险","悬疑","惊悚"]', 'showcase', 350000, 5),
('一人之下', '米二', '张楚岚卷入异人世界的都市奇幻故事。原作即为漫画，其剧本结构和分镜节奏是学习范本。', '都市奇幻', '["异能","都市","格斗","漫画改编"]', 'showcase', 0, 6);
