package seed

import (
	"github.com/autovideo/character-service/internal/model"
	"gorm.io/gorm"
)

// defaultSkills 新项目自动写入的默认技能列表
var defaultSkills = []struct {
	Name        string
	SkillType   string
	UseCase     string
	Description string
}{
	// ── 分镜生成辅助（导演方法论）──────────────────────────────────────────────
	{
		Name:      "动作场景增强",
		SkillType: "combat",
		UseCase:   "storyboard",
		Description: "增强战斗/动作场景的视觉张力，提示词中注入动态模糊、运动感、肾上腺素元素，每个动作镜头独立成帧，禁止同一镜头混合剧烈外部动作与台词",
	},
	{
		Name:      "情感场景增强",
		SkillType: "social",
		UseCase:   "storyboard",
		Description: "增强情感/对话场景的氛围，注入柔和光线、面部表情特写、越肩镜头交替，情绪高潮时推近至特写，维护正反打轴线不跳轴",
	},
	{
		Name:      "环境场景增强",
		SkillType: "exploration",
		UseCase:   "storyboard",
		Description: "增强环境/探索场景的空间感，注入广角构图、环境细节、大气透视，进入新场景时先用空镜全景交代环境与人物位置关系",
	},
	{
		Name:      "神秘场景增强",
		SkillType: "special",
		UseCase:   "storyboard",
		Description: "增强神秘/奇幻场景的魔法感，注入粒子特效、神秘光晕、超自然元素，运用低角度仰视强调力量，高角度俯视展示脆弱",
	},
	{
		Name:      "景别递进原则",
		SkillType: "special",
		UseCase:   "storyboard",
		Description: "连续镜头必须递进变换景别（远景→全景→中景→近景→特写），禁止连续两个镜头使用相同景别；景别变化服务于情绪递进而非技术惯例",
	},
	{
		Name:      "空间轴线维护",
		SkillType: "social",
		UseCase:   "storyboard",
		Description: "严格遵守180度轴线法则，对话场景交替使用越肩镜头；角色位置、朝向、持物状态须在镜头间严格继承；切换视角时物理位置不变",
	},
	{
		Name:      "情感蓝图设计",
		SkillType: "social",
		UseCase:   "storyboard",
		Description: "每个场景须先确定情绪目标（紧张/温暖/悲伤/愤怒），规划情绪强度曲线（低→高→低），明确情绪起点、转折点、高潮点，再设计镜头语言",
	},
	{
		Name:      "竖屏短剧构图",
		SkillType: "special",
		UseCase:   "storyboard",
		Description: "竖构图下人物占画面比例更大，以近景/特写为主；出镜人数严格遵守剧情需求；多人对话须有足够多人同框中景/全景，不可全程单人特写",
	},

	// ── 资源提取辅助 ──────────────────────────────────────────────────────────
	{
		Name:      "人物三视图规格提取",
		SkillType: "social",
		UseCase:   "extraction",
		Description: "提取角色外貌时须覆盖三视图所需全部维度：发型发色、脸型、五官细节、体型、服装上下装（含鞋子）、饰品、肤色，以便生成九头身三视图参考图",
	},
	{
		Name:      "场景空间蓝图提取",
		SkillType: "exploration",
		UseCase:   "extraction",
		Description: "提取场景时须描述360°空间特征：东西南北四个方向的视觉边界（室内为四面墙特征，室外为四方向地标）、中心锚点、地面材质、光线色温，构建空间蓝图",
	},
	{
		Name:      "时代背景侦测",
		SkillType: "special",
		UseCase:   "extraction",
		Description: "根据剧本关键词判断时代背景（古代实拍/现代都市/科幻未来），自动补全缺失的年代服饰特征、场景材质、道具造型，确保资产在同一时空内视觉统一；服饰描述须按角色性别分开——男性用男装（如圆领袍衫/幞头），女性用女装（如齐胸裙/帔帛），绝不混用",
	},
	{
		Name:      "性别服饰一致性",
		SkillType: "special",
		UseCase:   "extraction",
		Description: "提取角色服饰描述时必须严格对应角色性别：男性角色只使用男性服饰（圆领袍衫、幞头、布靴、儒衫、道袍、长衫等），绝不出现女性服饰词汇（齐胸裙、帔帛、步摇、高髻裙、钗环）；女性角色同理；若剧本未明确服饰，按角色性别和时代补全合理默认服饰",
	},
	{
		Name:      "跨章节人物一致性",
		SkillType: "social",
		UseCase:   "extraction",
		Description: "同名角色在不同章节中的外貌描述必须保持核心特征一致：发型发色、五官特征、体型、标志性服饰保持不变；仅允许因剧情明确描述而产生的合理变化（如受伤、换装、成长蜕变）；禁止同一角色前后外貌矛盾或年龄跨度不合逻辑",
	},

	// ── 提示词优化辅助 ────────────────────────────────────────────────────────
	{
		Name:      "写实风格优化",
		SkillType: "special",
		UseCase:   "prompt",
		Description: "附加写实摄影风格：真实摄影, 电影级RAW原片, 超写实主义, 8k分辨率, 硬光照明, 伦布朗光, 摄影布光, 极致细节, 富士胶片, (画面极其纯净:1.5), (无文字:2.0)",
	},
	{
		Name:      "动漫风格优化",
		SkillType: "special",
		UseCase:   "prompt",
		Description: "附加二维动漫风格：二维动漫风格, 高质量, 8k分辨率, 面部柔光照明, 蝴蝶光, 极致细节, (画面极其纯净:1.5), (无文字:2.0), 无水印, 无图表, 无标注",
	},
	{
		Name:      "3D动漫风格优化",
		SkillType: "special",
		UseCase:   "prompt",
		Description: "附加三维动漫CG风格：三维动漫CG风格, 皮克斯级渲染, 高质量, 8k分辨率, 环境光遮蔽, 全局光照, (画面极其纯净:1.5), (无文字:2.0), 无水印",
	},
	{
		Name:      "纯净背景强制",
		SkillType: "special",
		UseCase:   "prompt",
		Description: "强制纯白背景约束：纯白背景, 影棚级纯白无缝背景, (画面极其纯净:1.5), (无文字:2.0), 无水印, 无图表, 无标注，适用于角色/物品资产图抠图场景",
	},

	// ── 剧本/分集文本优化 ──────────────────────────────────────────────────────
	{
		Name:      "短剧节奏优化",
		SkillType: "special",
		UseCase:   "writing",
		Description: "压缩叙事节拍，每集开头3秒内设置强钩子（冲突/反转/悬念），每集结尾留情感落点或悬念，强化爽感，加入逆袭/打脸/反转场景，适合竖屏短剧节奏",
	},
	{
		Name:      "场景可视化增强",
		SkillType: "exploration",
		UseCase:   "writing",
		Description: "将叙述性文字改写为富含可视化细节的场景描述：包含光线色温、空间层次、人物位置关系，便于AI生图执行，特别标注空镜切场节点",
	},
	{
		Name:      "人物台词自然化",
		SkillType: "social",
		UseCase:   "writing",
		Description: "优化人物对话：去除书面语和文言痕迹，贴近口播配音节奏，每句台词控制在8-24字（超长须按情绪点拆分），突出人物性格差异，英文台词保留英文",
	},
	{
		Name:      "情感弧线强化",
		SkillType: "social",
		UseCase:   "writing",
		Description: "规划每集的情绪强度曲线（低→高→低），明确情绪起点、转折点、高潮点和落点，确保编剧逻辑完整、角色动机合理、因果关系清晰，提升观众代入感",
	},
	{
		Name:      "分集摘要润色",
		SkillType: "special",
		UseCase:   "writing",
		Description: "将分集 summary 改写为更具吸引力的简介文案，突出核心冲突和看点，适合平台展示；从编剧、导演、影评家三重视角审视内容的逻辑性、视觉性和商业性",
	},
	{
		Name:      "台词拆镜节奏",
		SkillType: "social",
		UseCase:   "writing",
		Description: "根据台词语义和情绪节奏判断拆分点，单镜头台词不超过24字；多人对话场景须有充分的多人同框描述；台词原文保真，不可合并、删减或修改任何对白",
	},

	// ── 分镜预处理（剧本→视觉分镜脚本转化）───────────────────────────────────
	{
		Name:      "视觉标注强制",
		SkillType: "special",
		UseCase:   "storyboard_prep",
		Description: "每个场景切换处必须插入[场景:地点/时间/氛围]标注；每个角色出场或状态变化时插入[人物:姓名/动作/情绪]标注；缺少视觉信息的段落须补全光线色温和空间层次",
	},
	{
		Name:      "摄影建议注入",
		SkillType: "special",
		UseCase:   "storyboard_prep",
		Description: "情感高潮处插入[摄影:特写/仰拍]等建议；大场面引入时使用[摄影:远景/俯拍]空镜交代；对话场景在关键情绪转折处加入推近/拉远建议；禁止连续三个镜头同景别",
	},
	{
		Name:      "节奏标注注入",
		SkillType: "special",
		UseCase:   "storyboard_prep",
		Description: "动作冲突段落在句间加入[节奏:快切]标注；感情流露、回忆闪回段落加入[节奏:慢镜]；人物思考停顿、场景静默处加入[节奏:停顿]；爽感高潮结算时加入[节奏:定格]",
	},
	{
		Name:      "道具与场景强化",
		SkillType: "exploration",
		UseCase:   "storyboard_prep",
		Description: "提取剧情中出现的关键道具，用[道具:物品名称]标注并补充其视觉特征；场景中缺乏空间感的描写须补入建筑结构、装饰风格、光线方向等视觉细节",
	},
	{
		Name:      "情绪弧线可视化",
		SkillType: "social",
		UseCase:   "storyboard_prep",
		Description: "在段落开头插入[情绪:氛围词]标注（如[情绪:紧张对峙][情绪:温情告别]）；每个情绪段落须对应描述表情/肢体语言/光线/背景音效变化，使情绪变化可被AI拆镜识别",
	},
	{
		Name:      "导演镜头统筹",
		SkillType: "special",
		UseCase:   "storyboard_prep",
		Description: "在每个场景添加[导演:景别/镜头运动/建议时长]标注，指导AI视频模型拆镜：特写2-4秒/对话近景3-5秒/动作中景4-7秒/环境全景6-10秒；连续镜头须递进变换景别；在关键情绪节点注明镜头运动方式（推/拉/摇/跟/固定）",
	},

	// ── 视频模型时长适配（影响拆镜粒度）──────────────────────────────────────
	{
		Name:      "Kling/可灵时长适配",
		SkillType: "special",
		UseCase:   "storyboard",
		Description: "生成针对可灵(Kling)模型的分镜时：每个镜头建议时长3-10秒，对话场景4-6秒，动作场景3-5秒，环境建立6-10秒；可灵擅长人物动作连续性和面部表情，对话镜头优先近景/特写；避免超过10秒的超长镜头，复杂场景拆分为多个短镜头",
	},
	{
		Name:      "Wan/万象时长适配",
		SkillType: "special",
		UseCase:   "storyboard",
		Description: "生成针对万象(Wan/通义)模型的分镜时：建议时长3-8秒，该模型擅长大场景环境渲染和大气透视；环境建立镜头5-8秒以充分展示景深；人物动作镜头保持3-5秒防止漂移；远景/全景场景可适当延长；人物近景控制在4秒以内保证连贯性",
	},
	{
		Name:      "Vidu时长适配",
		SkillType: "special",
		UseCase:   "storyboard",
		Description: "生成针对Vidu模型的分镜时：建议时长4-8秒，该模型擅长真实物理运动和流体动态；动作场景4-6秒以展示完整运动弧线；慢镜头/情感场景可到8秒；快速切换场景保持4秒以保证帧间一致性；避免过快切换造成画面跳变",
	},
	{
		Name:      "Doubao/豆包时长适配",
		SkillType: "special",
		UseCase:   "storyboard",
		Description: "生成针对豆包(Doubao/Seedance)模型的分镜时：建议时长3-12秒，该模型支持原生音频/语音同步；有台词的镜头时长须与台词朗读时长匹配（约每3-4字1秒），无台词场景3-5秒；擅长人物口型同步，对话镜头优先近景；长对白场景可延至10-12秒",
	},
}


// SeedDefaultSkillsForProject 为新项目写入默认技能，若该项目已有技能则跳过
func SeedDefaultSkillsForProject(db *gorm.DB, projectID int64) error {
	var count int64
	if err := db.Model(&model.Skill{}).Where("project_id = ?", projectID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	skills := make([]*model.Skill, 0, len(defaultSkills))
	for _, d := range defaultSkills {
		skills = append(skills, &model.Skill{
			ProjectID:   projectID,
			Name:        d.Name,
			SkillType:   d.SkillType,
			UseCase:     d.UseCase,
			Description: d.Description,
			IsActive:    true,
		})
	}
	return db.CreateInBatches(skills, 20).Error
}

// UpsertDefaultSkillsForProject 为指定项目补充缺失的默认技能（按名称判重，不覆盖已有技能）
func UpsertDefaultSkillsForProject(db *gorm.DB, projectID int64) error {
	// 获取该项目已有技能名称集合
	var existing []string
	if err := db.Model(&model.Skill{}).Where("project_id = ?", projectID).Pluck("name", &existing).Error; err != nil {
		return err
	}
	existingSet := make(map[string]struct{}, len(existing))
	for _, name := range existing {
		existingSet[name] = struct{}{}
	}

	var toCreate []*model.Skill
	for _, d := range defaultSkills {
		if _, ok := existingSet[d.Name]; ok {
			continue // 已存在，跳过
		}
		toCreate = append(toCreate, &model.Skill{
			ProjectID:   projectID,
			Name:        d.Name,
			SkillType:   d.SkillType,
			UseCase:     d.UseCase,
			Description: d.Description,
			IsActive:    true,
		})
	}
	if len(toCreate) == 0 {
		return nil
	}
	return db.CreateInBatches(toCreate, 20).Error
}
