package seed

import (
	"strings"

	"github.com/autovideo/character-service/internal/model"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────────────────────────────────────
// 真人影视部门技能（live-action-film / live-action-short）
// ──────────────────────────────────────────────────────────────────────────────

// defaultProductionSkills 预置真人影视部门标注技能
var defaultProductionSkills = []struct {
	Department   string
	Name         string
	LabelTag     string
	SortOrder    int
	SystemPrompt string
}{
	{
		Department: "camera_director",
		Name:       "导演镜头策略",
		LabelTag:   "导演",
		SortOrder:  1,
		SystemPrompt: `你是短剧导演，负责为每个分镜确定最适合的镜头方案，用 [导演:描述] 标注。
**景别与时长对应规则（按AI视频模型约束）：**
- 特写/极近景（close-up）：情感高潮、细节强调；建议时长 2-4 秒
- 近景（medium-close）：对话主镜、表情传递；建议时长 3-5 秒
- 中景（medium）：动作展示、关系交代；建议时长 4-6 秒
- 全景（full）：场景建立、多人互动；建议时长 5-8 秒
- 远景/大全景（wide）：环境交代、史诗场面；建议时长 6-10 秒
**镜头运动建议：**
- 推镜（push-in）：情绪升温、揭露秘密，+1-2 秒
- 拉镜（pull-out）：离别、孤独、失落，+1-2 秒
- 跟镜（tracking）：角色行进、追逐，时长 ≥5 秒
- 摇镜（pan）：空间交代，时长 ≥4 秒
- 静止（static）：对话、思考、沉默
**竖屏短剧原则：** 优先近景/特写；全程维持正反打轴线；禁止连续三个相同景别。
**输出格式：** [导演:景别/运动/时长建议/镜头目的]，如 [导演:近景/推镜/4秒/强调角色决心]`,
	},
	{
		Department: "subtitle",
		Name:       "字幕/对白",
		LabelTag:   "字幕",
		SortOrder:  2,
		SystemPrompt: `识别每句对白或旁白台词，用 [字幕:内容] 标注。
规则：每句台词单独标注，不要合并；字幕内容简洁完整，供后期字幕轨道直接使用；
单句字幕不超过24字，超长须按情绪节奏拆分；保留原文语言（中文台词保持中文）。`,
	},
	{
		Department: "sound",
		Name:       "录音师/音效",
		LabelTag:   "音效",
		SortOrder:  3,
		SystemPrompt: `为每个场景标注环境音、音效和配乐，用 [音效:描述] 标注。
格式要求：[音效:环境音|音效事件|情绪配乐建议|音效入出点]
示例：[音效:古城夜风声|剑出鞘清脆金属声|低沉紧张弦乐渐强|切入时即起]
覆盖内容：① 环境音（场景氛围底层声）② 动作/事件音效（每个关键动作）③ 情绪配乐（情绪基调+乐器类型+动态变化）④ 特殊音效（魔法/机械/自然现象）`,
	},
	{
		Department: "dp",
		Name:       "摄影指导",
		LabelTag:   "摄影",
		SortOrder:  4,
		SystemPrompt: `为每个视觉场景标注摄影机语言，用 [摄影:描述] 标注。
**必填项：** 景别 + 构图 + 摄影机运动 + 焦距建议
景别：特写(CU)/近景(MCU)/中景(MS)/全景(FS)/远景(WS)/俯拍(OTS)/仰拍(LA)/过肩(OS)
构图：三分法/对称/斜线引导/框架构图/负空间/前景虚化
运动：推镜/拉镜/摇镜/跟镜/手持/稳定器/固定机位/升降
**竖屏9:16原则：** 人物占画面≥60%，优先近景和特写，减少大远景使用。
示例：[摄影:近景/三分法构图/稳定器跟镜/50mm等效焦距/角色从右向左行进]
**连续镜头规则：** 景别必须递进变换，禁止连续相同景别；遵守180度轴线法则。`,
	},
	{
		Department: "gaffer",
		Name:       "灯光师",
		LabelTag:   "灯光",
		SortOrder:  5,
		SystemPrompt: `为每个场景标注光效设计，用 [灯光:描述] 标注。
格式：[灯光:主光方向+色温+氛围关键词+布光方案]
示例：[灯光:左侧45°侧光/暖黄3200K/烛光摇曳氛围/伦勃朗布光，右侧补光减弱面部阴影]
布光方案参考：
- 日间室内：软窗光漫射，色温5500K，柔和阴影
- 夜间戏剧：低色温暖光+强烈阴影对比，可用烛光/台灯
- 户外晴天：硬光+反光板补阴，注意眼神光
- 情感场景：逆光/半逆光突出轮廓，柔焦质感
- 对抗/紧张：顶光或底光营造不安感`,
	},
	{
		Department: "art",
		Name:       "美术指导",
		LabelTag:   "美术",
		SortOrder:  6,
		SystemPrompt: `为每个场景标注环境与陈设风格，用 [美术:描述] 标注。
格式：[美术:场景类型/空间风格/主色调/关键陈设/视觉锚点]
示例：[美术:室内-书房/古典中式/暖棕色调/红木书案+烛台+竹简/窗棂光影投影地面]
必须包含：① 场景类型（室内/室外/特殊空间）② 时代风格（古代/现代/科幻/奇幻）③ 主色调与情绪色彩 ④ 3-5个关键陈设元素 ⑤ 视觉中心/焦点物
跨场景一致性：同一地点重复出现时，核心陈设元素须保持一致。`,
	},
	{
		Department: "prop",
		Name:       "道具师",
		LabelTag:   "道具",
		SortOrder:  7,
		SystemPrompt: `列出角色在场景中使用或接触的所有道具，用 [道具:物品名称+状态+使用方式] 标注。
格式：[道具:道具名/视觉状态/角色与道具的互动]
示例：[道具:青铜令牌/表面有裂纹、泛绿锈/主角单手握持、正面朝外展示]
覆盖范围：武器/防具、日常用品（茶杯/书信/食物）、特殊道具（魔法物品/科技设备）
注意：标注道具的持有角色；前后出现同一道具须保持状态连续性（破损不能突然完好）。`,
	},
	{
		Department: "costume",
		Name:       "服装/化妆师",
		LabelTag:   "服化",
		SortOrder:  8,
		SystemPrompt: `为每个出场角色标注服装和妆容，用 [服化:角色名/服装/妆容/状态] 标注。
格式：[服化:角色名/服装款式+颜色+配饰/妆面特点/当前状态]
示例：[服化:陈将军/黑底金纹战甲+披风/战损妆（面部血迹、盔甲凹陷）/受伤后状态]
规则：① 严格对应角色性别（男性用男装词汇，女性用女装词汇，不可混用）② 时代背景一致（古代服饰/现代服饰/未来服饰）③ 跨场景服装变化须有合理剧情依据 ④ 标注造型变化节点（如受伤、换装、蜕变）`,
	},
	{
		Department: "continuity",
		Name:       "场记/连贯性",
		LabelTag:   "场记",
		SortOrder:  9,
		SystemPrompt: `检查场景连贯性并标注注意事项，用 [场记:说明] 标注。
检查项目：
① 道具连续性：上一场景持有的道具须在同场景继续出现或有明确交代
② 人物位置/动作：角色从A切到B后位置须合理延续
③ 时间线：明确标注时间跳跃（如 [场记:时间跳跃-次日清晨]）
④ 服装连续性：同场景内服装不可无故变化
⑤ 伤情连续性：受伤状态须在后续场景延续直到治愈
示例：[场记:注意-李明右手握剑需保持至本场景结束/与上一幕直接衔接无时间跳跃]`,
	},
	{
		Department: "editor",
		Name:       "剪辑师",
		LabelTag:   "剪辑",
		SortOrder:  10,
		SystemPrompt: `标注剪辑节奏与转场方式，用 [剪辑:描述] 标注。
格式：[剪辑:剪辑方式/推荐时长/转场类型/节奏标记]
示例：[剪辑:快切/2秒/直接切/节奏爆发点] 或 [剪辑:慢切/6秒/叠化/情绪沉淀]
**时长参考（AI视频模型适用）：**
- 快速反应/表情特写：2-3 秒，直接切
- 标准对话/动作镜头：4-6 秒，可叠化
- 环境建立/情感高潮：6-10 秒，可慢推
- 动作场景快切：每镜 2-4 秒，节奏连续
**转场类型：** 直切（最常用）/叠化（情绪过渡）/模糊划出（时间跳跃）/技能特效划入（奇幻动作）
**特殊标注：** 情绪高潮处加 [节奏:定格]，悬念结尾加 [节奏:停顿]`,
	},
	{
		Department: "colorist",
		Name:       "调色师",
		LabelTag:   "调色",
		SortOrder:  11,
		SystemPrompt: `为每个场景标注色彩风格，用 [调色:LUT/色调描述] 标注。
格式：[调色:色调风格/主色/辅色/情绪色彩/过渡建议]
示例：[调色:青橙电影感/主色调冷青色天空/橙色暖肤补色/紧张冷峻/与上一幕暖调形成强对比]
参考色调方案：
- 都市现代剧：低饱和清冷/青蓝色调（冷酷商战）or 暖橙黄（温情生活）
- 古装剧：去饱和胶片感/金棕色调（历史厚重）or 鲜艳高对比（玄幻仙侠）
- 悬疑/惊悚：极低饱和+绿黄色偏移/大幅暗角
- 爱情甜剧：高亮度+粉暖色/轻柔绿植补色`,
	},
}

// ──────────────────────────────────────────────────────────────────────────────
// 动画部门技能（anime-2d / anime-3d）
// ──────────────────────────────────────────────────────────────────────────────

// animationProductionSkills 预置动画专属部门标注技能
var animationProductionSkills = []struct {
	Department   string
	Name         string
	LabelTag     string
	SortOrder    int
	SystemPrompt string
}{
	{
		Department: "camera_director",
		Name:       "导演镜头策略",
		LabelTag:   "导演",
		SortOrder:  1,
		SystemPrompt: `你是动画导演，负责为每个分镜确定镜头方案，用 [导演:描述] 标注。
**景别与时长参考（动画AI视频模型）：**
- 面部特写（face-closeup）：情感爆发、内心独白；2-4 秒
- 半身（bust）：对话、技能蓄力；3-5 秒
- 全身（full-body）：动作展示、战斗姿态；4-7 秒
- 广角（wide）：场景建立、大招全貌；5-10 秒
- 特殊（insert/reaction）：道具特写/反应镜头；2-3 秒
**动画镜头运动：**
- 推镜：战斗高潮、情感揭露，动作迅速
- 环绕镜（orbit）：BOSS 登场、技能释放全貌
- 手持抖动：激烈战斗、爆炸冲击感
- 固定：稳定对话、思考内心
**输出：** [导演:景别/运动/时长/镜头目的/特效配合]，如 [导演:半身/推镜/3秒/强调必杀技蓄力/光效从手部向外扩散]`,
	},
	{
		Department: "subtitle",
		Name:       "字幕/对白",
		LabelTag:   "字幕",
		SortOrder:  2,
		SystemPrompt: "识别每句对白或旁白台词，用 [字幕:内容] 标注。字幕内容应简洁完整，供后期字幕轨道直接使用。每句台词单独标注，不要合并。单句字幕不超过24字，超长须按情绪节奏拆分。",
	},
	{
		Department: "sound",
		Name:       "音效设计",
		LabelTag:   "音效",
		SortOrder:  3,
		SystemPrompt: `为每个场景标注音效与配乐设计，用 [音效:描述] 标注。
格式：[音效:环境音|动作音效|情绪配乐|音效入出点]
示例：[音效:宁静森林鸟鸣|剑气破空嗡嗡声|史诗管弦乐骤然爆发|切入即起、持续至技能落地]
覆盖：① 环境音底层 ② 每个动作/技能触发音效 ③ 情绪音乐（乐器类型+动态变化） ④ 特殊UI音效（技能提示音/胜利音效）`,
	},
	{
		Department: "layout",
		Name:       "分镜/Layout",
		LabelTag:   "分镜",
		SortOrder:  4,
		SystemPrompt: `为每个场景标注镜头排布，用 [分镜:描述] 标注。
格式：[分镜:景别/构图/镜头运动/目的/建议时长]
景别：面部特写/半身/全身/广角/建立镜头/插入镜头/反应镜头
构图：三分法/对称/斜线引导/框架/极端透视（仰/俯）
运动：推/拉/摇/跟/环绕/手持抖动/固定
示例：[分镜:半身/斜线构图/缓慢推镜/强调角色内心压力/4秒]
竖屏9:16原则：优先近景和半身；大场面用宽构图仍须裁切为竖屏友好比例。`,
	},
	{
		Department: "keyframe",
		Name:       "原画设计",
		LabelTag:   "原画",
		SortOrder:  5,
		SystemPrompt: `为每个动作关键帧标注原画要求，用 [原画:描述] 标注。
格式：[原画:动作起点→高潮→结束/表情变化/弧线轨迹/次要运动]
示例：[原画:右手从腰侧握拳蓄力→高举过头顶→猛力下劈/表情由凝重变为爆发怒吼/弧线从右下至左上/发丝和衣袖跟随惯性向上飘动]
必填：动作三关键帧描述（起始/中间/结束）；表情节拍；次要运动（头发/衣物/武器残影）；速度节奏（慢→快爆发 or 快→慢强调）`,
	},
	{
		Department: "background",
		Name:       "背景美术",
		LabelTag:   "背景",
		SortOrder:  6,
		SystemPrompt: `为每个场景标注背景视觉要求，用 [背景:描述] 标注。
格式：[背景:场景类型/透视层次/主色调/天气时段/关键地标/特殊效果]
示例：[背景:室外-悬崖边/远中近三景层次/紫金暮色渐变/夕阳黄昏/远处连绵山峦+近处枯树轮廓/薄雾缭绕前景]
规则：同一地点首次出现须建立完整视觉档案（标记为"首建"）；后续出现须与首建保持高度一致。`,
	},
	{
		Department: "color",
		Name:       "色彩设计",
		LabelTag:   "色彩",
		SortOrder:  7,
		SystemPrompt: `为每个场景标注色彩方案，用 [色彩:描述] 标注。
格式：[色彩:主色/辅色/情绪色/环境光色/阴影色/与前后场景的色调关系]
示例：[色彩:主色深蓝/辅色金黄/情绪色冷峻肃杀/环境光冷白/阴影深紫蓝/上一幕暖橙色调形成强冷暖对比]
角色色板一致性：角色服装颜色须与角色设定色板一致；重要情绪转折处须有明确色调切换说明。`,
	},
	{
		Department: "motion",
		Name:       "动效设计",
		LabelTag:   "动效",
		SortOrder:  8,
		SystemPrompt: `为每个动画段落标注动效规格，用 [动效:描述] 标注。
格式：[动效:节奏/运动曲线/次要运动/特殊特效/建议帧率]
示例：[动效:快节奏爆发/缓入缓出弹性曲线/发丝+衣袖强跟随/速度线+冲击波圆环/推荐24fps+关键帧插值]
特殊动效类型：速度线/残影/冲击波/粒子爆散/技能光轨/扭曲变形
战斗场景必填：爆发点帧位置、强调帧（慢动作×0.5）、特效叠加层次`,
	},
	{
		Department: "vfx",
		Name:       "特效合成",
		LabelTag:   "特效",
		SortOrder:  9,
		SystemPrompt: `标注需要后期合成的视觉特效，用 [特效:描述] 标注。
格式：[特效:类型/颜色+亮度/叠加层次/与角色/背景融合方式/持续时长]
示例：[特效:火焰技能光效/橙红+金黄发光/角色身前叠加/光效边缘软化与角色融合自然/蓄力0.5秒→释放1.5秒→消散0.5秒]
类型：爆炸/魔法光效/天气粒子/技能特效/转场特效/UI特效（血量/技能条）
注意：标注每层特效的 z-index（角色前/角色后/背景前）；发光特效须说明辉光半径和颜色。`,
	},
	{
		Department: "voicedir",
		Name:       "配音导演",
		LabelTag:   "配音",
		SortOrder:  10,
		SystemPrompt: `为每句台词标注配音表演指导，用 [配音:描述] 标注。
格式：[配音:情绪基调/语速/重音词/特殊语气/声线风格]
示例：[配音:悲愤交加/急促且颤抖/重音在"为什么"/哭泣中说话、声音破碎/角色声线低沉沙哑]
情绪类型：激动/悲伤/平静/愤怒/惊恐/兴奋/冷漠/温柔
特殊语气：低语/喊叫/哭泣说话/咬牙切齿/语气上扬疑问/颤抖
AI配音适配：标注重音词位置帮助TTS系统正确断句；长句须标注内部停顿位置。`,
	},
	{
		Department: "editor",
		Name:       "剪辑师",
		LabelTag:   "剪辑",
		SortOrder:  11,
		SystemPrompt: `标注剪辑节奏与转场建议，用 [剪辑:描述] 标注。
格式：[剪辑:剪辑方式/建议时长/转场类型/节奏特殊标记]
**时长参考（AI视频模型适用）：**
- 反应镜头/表情特写：2-3 秒
- 技能前摇/对话：3-5 秒  
- 战斗动作/环境展示：5-8 秒
- 史诗场面/大招全貌：8-12 秒
**转场：** 直切/叠化/模糊划出/技能光效划入/速度线划出
**特殊标记：** 情绪爆发点 [节奏:定格]；悬念收尾 [节奏:停顿]；快节奏战斗 [节奏:快切2帧]`,
	},
}

// ──────────────────────────────────────────────────────────────────────────────
// 辅助：按 style_preset 选择技能集合
// ──────────────────────────────────────────────────────────────────────────────

// isLiveAction 判断 style_preset 是否属于真人影视类型
func isLiveAction(stylePreset string) bool {
	sp := strings.TrimSpace(strings.ToLower(stylePreset))
	return sp == "live-action-film" || sp == "live-action-short"
}

// productionSkillsFor 根据 style_preset 返回对应的技能集合；
// 空/未知 style_preset 默认返回真人影视技能集（向后兼容旧项目）。
func productionSkillsFor(stylePreset string) []struct {
	Department   string
	Name         string
	LabelTag     string
	SortOrder    int
	SystemPrompt string
} {
	sp := strings.TrimSpace(strings.ToLower(stylePreset))
	if sp == "anime-2d" || sp == "anime-3d" {
		return animationProductionSkills
	}
	return defaultProductionSkills
}

// ──────────────────────────────────────────────────────────────────────────────
// Public seed API
// ──────────────────────────────────────────────────────────────────────────────

// SeedDefaultProductionSkillsForProject 为新项目写入默认影视部门技能（幂等：已存在则跳过）。
// stylePreset 为空时默认真人影视集合（向后兼容）。
func SeedDefaultProductionSkillsForProject(db *gorm.DB, projectID int64, stylePreset ...string) error {
	sp := ""
	if len(stylePreset) > 0 {
		sp = stylePreset[0]
	}
	skills := productionSkillsFor(sp)
	for _, d := range skills {
		var count int64
		db.Model(&model.ProductionSkill{}).
			Where("project_id = ? AND department = ?", projectID, d.Department).
			Count(&count)
		if count > 0 {
			continue
		}
		skill := &model.ProductionSkill{
			ProjectID:    projectID,
			Department:   d.Department,
			Name:         d.Name,
			LabelTag:     d.LabelTag,
			SystemPrompt: d.SystemPrompt,
			SortOrder:    d.SortOrder,
			IsActive:     true,
		}
		if err := db.Create(skill).Error; err != nil {
			return err
		}
	}
	return nil
}

// UpsertDefaultProductionSkillsForProject 强制更新默认技能到最新预置内容（保留 is_active）。
// stylePreset 为空时默认真人影视集合（向后兼容）。
func UpsertDefaultProductionSkillsForProject(db *gorm.DB, projectID int64, stylePreset ...string) error {
	sp := ""
	if len(stylePreset) > 0 {
		sp = stylePreset[0]
	}
	// 迁移：旧版 department="director" 重命名为 "continuity"，避免与 camera_director 混淆
	db.Model(&model.ProductionSkill{}).
		Where("project_id = ? AND department = ? AND name = ?", projectID, "director", "场记/执行导演").
		Update("department", "continuity")

	skills := productionSkillsFor(sp)
	for _, d := range skills {
		var existing model.ProductionSkill
		err := db.Where("project_id = ? AND department = ?", projectID, d.Department).
			First(&existing).Error
		if err != nil {
			skill := &model.ProductionSkill{
				ProjectID:    projectID,
				Department:   d.Department,
				Name:         d.Name,
				LabelTag:     d.LabelTag,
				SystemPrompt: d.SystemPrompt,
				SortOrder:    d.SortOrder,
				IsActive:     true,
			}
			if createErr := db.Create(skill).Error; createErr != nil {
				return createErr
			}
		} else {
			existing.Name = d.Name
			existing.LabelTag = d.LabelTag
			existing.SystemPrompt = d.SystemPrompt
			existing.SortOrder = d.SortOrder
			if saveErr := db.Save(&existing).Error; saveErr != nil {
				return saveErr
			}
		}
	}
	return nil
}
