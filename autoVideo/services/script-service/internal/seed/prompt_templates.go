package seed

import (
	"errors"

	"github.com/autovideo/script-service/internal/model"
	"gorm.io/gorm"
)

// defaultPromptTemplates 内置提示词模板，仅在 prompt_templates 表为空时写入
// 模板基于专业影视概念美术规范，参考真实摄影/2D动漫/3D动漫三种画风标准。
var defaultPromptTemplates = []model.PromptTemplate{
	// ── 人物（真实摄影三视图）─────────────────────────────────────────────────
	{
		Name:        "人物图片生成（真实摄影·三视图）",
		StyleKey:    "character_realistic_3view",
		Description: "真实摄影风格人物三视图参考图，九头身黄金比例，正面头肩肖像+全身正侧背并排，适合真人短剧/现代都市题材",
		ResourceType: "character",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    10,
		Content: `真实摄影, 电影级RAW原片, {name}, 包含角色完整的正面头肩肖像（头部及颈部完整可见，占画面左侧约三分之一），全身正面、侧面、背面三视图, 并排排列, {role}, {age_height}, 九头身黄金比例, 极度修长的双腿, 头身比1:9, {body_type}, {hairstyle}, {hair_color}, (超写实主义:1.5), 8k分辨率, {face_shape}, {eyes}, {skin_tone}, {outfit_top}, {outfit_bottom}, {accessories}, 纯白背景, (画面极其纯净:1.5), (无文字:2.0), 无水印, 无图表, 无标注, 硬光照明, 伦布朗光, 摄影布光, 极致细节, 富士胶片`,
	},
	{
		Name:        "人物图片生成（真实摄影·电影级）",
		StyleKey:    "character_cinematic",
		Description: "具有电影级质感的真实摄影人物形象，适合宣传图/特写海报场景",
		ResourceType: "character",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    11,
		Content: `真实摄影, 电影级RAW原片, {appearance}, 戏剧性布光, 伦布朗光, 浅景深, 背景虚化, 胶片颗粒感, 高端摄影, 超写实主义, 8k分辨率, 极致细节, (画面极其纯净:1.5), (无文字:2.0), 无水印`,
	},
	// ── 人物（二维动漫三视图）────────────────────────────────────────────────
	{
		Name:        "人物图片生成（二维动漫·三视图）",
		StyleKey:    "character_anime2d_3view",
		Description: "二维动漫风格人物三视图参考图，九头身比例，正面头肩肖像+全身正侧背并排，适合动漫/漫画题材",
		ResourceType: "character",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    12,
		Content: `二维动漫风格, {name}, 包含角色完整的正面头肩肖像（头部及颈部完整可见，占画面左侧约三分之一），全身正面、侧面、背面三视图, 并排排列, {role}, {age_height}, 九头身黄金比例, 极度修长的双腿, 头身比1:9, {body_type}, {hairstyle}, {hair_color}, (高质量:1.5), 8k分辨率, {face_shape}, {eyes}, {skin_tone}, {outfit_top}, {outfit_bottom}, {accessories}, 纯白背景, (画面极其纯净:1.5), (无文字:2.0), 无水印, 无图表, 无标注, 面部柔光照明, 蝴蝶光, 极致细节`,
	},
	// ── 人物（三维动漫三视图）────────────────────────────────────────────────
	{
		Name:        "人物图片生成（三维动漫·三视图）",
		StyleKey:    "character_anime3d_3view",
		Description: "三维动漫CG风格人物三视图参考图，皮克斯级渲染质感，适合3D动漫/游戏题材",
		ResourceType: "character",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    13,
		Content: `三维动漫CG风格, 皮克斯级渲染, {name}, 包含角色完整的正面头肩肖像（头部及颈部完整可见，占画面左侧约三分之一），全身正面、侧面、背面三视图, 并排排列, {role}, {age_height}, 九头身黄金比例, 极度修长的双腿, 头身比1:9, {body_type}, {hairstyle}, {hair_color}, (高质量:1.5), 8k分辨率, {face_shape}, {eyes}, {skin_tone}, {outfit_top}, {outfit_bottom}, {accessories}, 纯白背景, 环境光遮蔽, 全局光照, (画面极其纯净:1.5), (无文字:2.0), 无水印, 极致细节`,
	},

	// ── 场景（360°空间蓝图）──────────────────────────────────────────────────
	{
		Name:        "场景图片生成（真实摄影·360°空间蓝图）",
		StyleKey:    "scene_realistic_360",
		Description: "真实摄影风格场景，面向指定方位平视，无人空镜，电影级布光，适合真实摄影/真人短剧场景资产生成",
		ResourceType: "scene",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    20,
		Content: `面向正北, 平视, 真实摄影, 电影级RAW原片, 纯净场景空镜，画面中绝对不能出现任何人类、角色或生物，空无一人, empty scene, absolutely no people, (色调统一:1.5), (光影连续性:1.5), 电影级布光, 8k超高清, {scene_name}, {direction_feature}, {center_anchor}, {material_props}, {lighting_atmosphere}, (画面极其纯净:1.5), (无文字:2.0), 无水印`,
	},
	{
		Name:        "场景图片生成（二维动漫·360°空间蓝图）",
		StyleKey:    "scene_anime2d_360",
		Description: "二维动漫风格场景，面向指定方位平视，无人空镜，适合动漫题材场景资产生成",
		ResourceType: "scene",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    21,
		Content: `面向正北, 平视, 二维动漫风格, 纯净场景空镜，画面中绝对不能出现任何人类、角色或生物，空无一人, empty scene, absolutely no people, (色调统一:1.5), (光影连续性:1.5), 电影级布光, 8k超高清, {scene_name}, {direction_feature}, {center_anchor}, {material_props}, {lighting_atmosphere}, (画面极其纯净:1.5), (无文字:2.0), 无水印`,
	},
	{
		Name:        "场景图片生成（概念艺术·奇幻科幻）",
		StyleKey:    "scene_concept_fantasy",
		Description: "概念艺术风格场景，适合奇幻/科幻/仙侠题材的宏大场景生成",
		ResourceType: "scene",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    22,
		Content: `{scene_description}, concept art, matte painting, epic scale, dramatic sky, detailed architecture, trending on artstation, 4k, fantasy environment, pure environment shot, completely empty scene without any humans or creatures, absolutely no people, uninhabited, zero characters`,
	},

	// ── 分镜 ──────────────────────────────────────────────────────────────────
	{
		Name:        "分镜图片生成（真实摄影·电影感）",
		StyleKey:    "storyboard_cinematic",
		Description: "电影级真实摄影分镜图像，强调构图、光影与情感传递，适合真人短剧/现代题材",
		ResourceType: "storyboard",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    30,
		Content: `{scene}, {characters}, {action}, {mood}, 电影感构图, 戏剧性布光, 浅景深, 摄影级构图, 黄金比例, 散景背景, 变形镜头光晕, 超写实, 8k, 极致细节, 获奖摄影`,
	},
	{
		Name:        "分镜图片生成（二维动漫）",
		StyleKey:    "storyboard_anime2d",
		Description: "二维动漫风格分镜生成，动态构图，表情丰富",
		ResourceType: "storyboard",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    31,
		Content: `{scene}, {characters}, {action}, {mood}, 二维动漫风格, 线条清晰, 色彩鲜明, 表情丰富, 动态构图, 漫画分镜风格, 高质量动漫插画, 详细背景`,
	},
	{
		Name:        "分镜图片生成（三维动漫）",
		StyleKey:    "storyboard_anime3d",
		Description: "三维动漫CG风格分镜，皮克斯级质感",
		ResourceType: "storyboard",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    32,
		Content: `{scene}, {characters}, {action}, {mood}, 三维动漫CG风格, 皮克斯级渲染, 环境光遮蔽, 全局光照, 动态构图, 8k超高清, 极致细节`,
	},
	{
		Name:        "分镜图片生成（中国水墨国风）",
		StyleKey:    "storyboard_chinese_ink",
		Description: "中国风/古风水墨画风格分镜，适合仙侠/古装题材",
		ResourceType: "storyboard",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    33,
		Content: `{scene}, {characters}, {action}, {mood}, 中国水墨画风格, 中国传统艺术, 写意笔触, 大气透视, 云山雾绕, 古典建筑, 国风美学, 传世佳作`,
	},

	// ── 物品/道具 ──────────────────────────────────────────────────────────────
	{
		Name:        "物品图片生成（真实摄影）",
		StyleKey:    "item_realistic",
		Description: "真实摄影风格道具/物品设定图，白背景，适合写实题材",
		ResourceType: "item",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    40,
		Content: `真实摄影, 道具设定图, {item_name}, {material}, {texture}, {luster}, {wear_detail}, 柔和布光, 纯白背景, 无杂物, (无文字:2.0), 无水印, 8k分辨率, 极致真实`,
	},
	{
		Name:        "物品图片生成（二维动漫）",
		StyleKey:    "item_anime2d",
		Description: "二维动漫风格道具立绘，白背景，适合动漫题材",
		ResourceType: "item",
		ModelBinding: "",
		Version:      "1.0",
		IsActive:     true,
		SortOrder:    41,
		Content: `二维动漫风格, 道具设定图, {item_name}, {material}, {texture}, {luster}, {wear_detail}, 柔和布光, 纯白背景, 无杂物, (无文字:2.0), 无水印, 8k分辨率, 极致细节`,
	},
}

// SeedPromptTemplates 若 prompt_templates 表为空则写入内置默认模板
func SeedPromptTemplates(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.PromptTemplate{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return db.CreateInBatches(defaultPromptTemplates, 20).Error
}

// ForceReseedPromptTemplates 强制写入内置默认模板（按 style_key upsert，不删除用户自定义模板）
func ForceReseedPromptTemplates(db *gorm.DB) error {
	for i := range defaultPromptTemplates {
		tpl := defaultPromptTemplates[i]
		var existing model.PromptTemplate
		err := db.Where("style_key = ?", tpl.StyleKey).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// not found — create
			if err := db.Create(&tpl).Error; err != nil {
				return err
			}
		} else {
			// found — update fields to latest defaults
			existing.Name = tpl.Name
			existing.Description = tpl.Description
			existing.Content = tpl.Content
			existing.ResourceType = tpl.ResourceType
			existing.Version = tpl.Version
			existing.SortOrder = tpl.SortOrder
			existing.IsActive = tpl.IsActive
			if err := db.Save(&existing).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
