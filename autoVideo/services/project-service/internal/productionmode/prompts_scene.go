package productionmode

import "fmt"

// SceneSplitParams carries shared inputs for scene-split prompt construction.
type SceneSplitParams struct {
	EpisodeNum      int
	Content         string
	RefDuration     int
	ModelDuration   string
	VisualHint      string
	SpeechHint      string
}

// SceneSplitUserPrompt returns the user prompt for a single scene-split LLM call.
func SceneSplitUserPrompt(mode Mode, p SceneSplitParams) string {
	switch mode {
	case ModeComics:
		return comicsSceneSplitPrompt(p)
	case ModeAd:
		return adSceneSplitPrompt(p)
	case ModeCommentaryComic:
		return commentarySceneSplitPrompt(p)
	default:
		return scriptDramaSceneSplitPrompt(p)
	}
}

// SceneSplitSystemPrompt returns the system prompt for scene-split LLM calls.
func SceneSplitSystemPrompt(mode Mode) string {
	switch mode {
	case ModeComics:
		return comicsSceneSplitSystemPrompt()
	case ModeAd:
		return adSceneSplitSystemPrompt()
	case ModeCommentaryComic:
		return commentarySceneSplitSystemPrompt()
	default:
		return scriptDramaSceneSplitSystemPrompt()
	}
}

func comicsSceneSplitPrompt(p SceneSplitParams) string {
	return fmt.Sprintf(`你是一位专业的漫画分镜师（漫画分格助手）。请将以下第 %d 集的内容拆分为最细粒度的漫画格（panel）。

**核心原则：最小化漫画格**
每一格应当是一个不可再拆分的叙事单元——即一个关键动作、一句对白、一个情绪节点或一个场景切换。

**拆分规则：**
- 每次人物动作变化、场景切换、对白转换、情绪转折都应独立为一格
- 不限制格数，根据内容自然拆分，宁多勿少
- description 用中文写“这一格观众看到什么”（40-120字），只写：人物动作、表情、关键道具、环境氛围、时间线索
- 禁止在 description 里写：左/右画幅、前景中景后景、机位、轴线、镜头运动、空间方位套话；这些由 shot_type 承担
- shot_type 使用漫画构图类型：face-closeup / bust / full-body / wide / establishing / insert / reaction
- characters 列出该格中出现的角色名
- character_states 每个角色的姿态和情绪（name/action/emotion）
- mood：tense / romantic / comedic / sad / epic / mysterious / action / calm / dramatic
- location：场景地点（2-20字）
- duration 固定为 0（漫画格无时长）
- dialogue：该格中的对白或心理独白（保持原文；如无则留空）。原文中引号内容、冒号引用句、[字幕:]标注均必须提取到此字段，禁止遗漏

**内联标注识别（优先级最高）：**
内容中可能包含影视标注：
- [摄影:xxx] → 映射到构图类型（shot_type），融入 description 的视角描述
- [美术:xxx] → 融入 description 的背景/环境细节
- [道具:xxx] → 在 description 中明确提及该道具
- [服化:xxx] → 在人物描述中体现服装细节
- [字幕:对白内容] → 对白内容【必须】直接填入 dialogue 字段，这是 TTS 配音的唯一数据来源

请严格按以下 JSON 格式返回：
{"scenes": [
  {"description": "中文画面描述", "shot_type": "bust", "characters": ["角色1"], "character_states": [{"name": "角色1", "action": "grips sword", "emotion": "determined"}], "mood": "tense", "location": "地点", "duration": 0, "dialogue": "对白"}
]}

第 %d 集内容：
%s`, p.EpisodeNum, p.EpisodeNum, p.Content)
}

func adSceneSplitPrompt(p SceneSplitParams) string {
	return fmt.Sprintf(`你是一位专业的分镜师和摄影指导。请将以下第 %d 集的内容按"台词 / 口播为主、画面辅助承载"的原则拆分为适合广告成片的分镜。

**核心原则：以台词 / 口播拆分为主，时长优先。**
当前目标是优先按单分镜时长判断一段台词 / 口播能否在一个镜头内完整承载，而不是追求"最小视觉单位"。如果同一段口播、同一段卖点说明在当前目标时长内可以完整表达，应优先合并为一个主分镜或少量连续分镜，不要过度拆镜。

**拆分规则（按优先级从高到低）：**
- 第一优先级：先判断当前这段台词 / 口播是否可以在目标单分镜时长内完整表达；如果可以，优先保持在同一个分镜内完成
- 只有在以下情况才拆成新分镜：
  1. 明确切换到新空间 / 新场景
  2. 明确进入新卖点 / 新产品信息 / 新展示重点
  3. 说话主体发生变化
  4. 当前段内容明显超过目标单分镜时长可承载的信息量
  5. 确实需要一个极短强调镜头，但必须严格控制数量
- 轻微的表情变化、手部动作变化、视线变化、镜头轻推拉，不足以单独拆成新分镜；如果没有新的信息点，继续留在当前分镜内
- 广告口播项目中，分镜默认必须承载明确的 dialogue / 字幕 / 卖点信息
- description 用中文写可见画面（40-120字）：人物在做什么、表情、环境、关键道具；不要写机位/方位/分层术语
- shot_type 推荐景别：close-up / medium / full / wide / overhead / low-angle / tracking / handheld
- characters / character_states / items / mood / location 按画面需要填写
- duration 该分镜的视频时长（秒数，整数），必须严格等于当前目标单分镜时长：
%s
- 构图/画面约束（必须同步遵守）：
%s
- 语速/口播承载约束（必须同步遵守）：
%s
- dialogue 该场景中的对白（保持原文语言；如无则留空字符串）

**对白提取强制规则（TTS 配音关键）：**
原文中 [字幕:…]、引号内容、冒号引用句、心理独白必须提取到 dialogue 字段。
若当前分镜 dialogue 少于约 8 个汉字/字符，且没有明确新增场景切换、主体切换或卖点切换，也必须继续并回相邻分镜。

请严格按以下 JSON 格式返回：
{"scenes": [
  {"description": "中文画面描述", "shot_type": "medium", "characters": ["角色1"], "character_states": [{"name": "角色1", "action": "presenting product", "emotion": "confident"}], "items": ["产品"], "mood": "dramatic", "location": "展示区", "duration": %d, "dialogue": "对白"}
]}

第 %d 集内容：
%s`, p.EpisodeNum, p.ModelDuration, p.VisualHint, p.SpeechHint, p.RefDuration, p.EpisodeNum, p.Content)
}

func commentarySceneSplitPrompt(p SceneSplitParams) string {
	return fmt.Sprintf(`你是一位专业的解说漫分镜师。请将以下第 %d 集的内容拆分为适合旁白驱动漫画风视频的分镜。

**核心原则：旁白段落优先，画面配合叙事，宁多勿少。**
- 以旁白/解说句群为拆分主轴：每当旁白进入新信息点、新人物、新剧情段、新场景或新情绪节点，优先拆成新分镜
- 不要把多段无关旁白硬合并到一个镜头；也不要为了凑时长把完整旁白段拆得过碎
- 允许无人物出镜的说明性画面，但 dialogue 必须承载会被念出的旁白原文
- 漫画风讲解视频强调角色表演、条漫式构图、信息图示化，但不要写成广告卖点口播稿

**拆分规则：**
- 旁白转折、场景切换、人物登场/退场、剧情节点、设定块切换 → 新分镜
- 同一旁白段若信息量超过当前单镜时长，再按句群二次拆分
- description 用中文写这一镜的可见画面（40-120字）：角色动作、表情、场景、道具、情绪；不要写左/右/机位/分层
- shot_type：close-up / medium / full / wide / establishing / insert / reaction
- characters / character_states / items / mood / location 按需填写
- duration 整数秒，必须随旁白节奏变化：短句/反应镜 3-5 秒，标准叙述 5-8 秒，信息密集段 8-12 秒；不要全部填同一个数
%s
- 构图/画面约束：
%s
- 语速/旁白承载约束：
%s
- dialogue：该镜会被 TTS 念出的旁白/对白原文（保持原文，不可省略）
- dialogue 禁止直接抄写 description 的画面描写；description 写给眼睛看，dialogue 写给配音念
- 原文若已有 [字幕:…]，dialogue 必须优先原样保留 [字幕:] 内文本，不要改写成“某某正在做什么”的旁观镜头句

**对白提取规则：**
[字幕:…]、引号内容、冒号引用句、解说性旁白段落必须进入 dialogue。
禁止把第三人称画面动作句（如“刘师傅正低头揉面”）当作旁白，除非它本来就出现在原文 [字幕:] 或讲解句中。
无旁白纯转场镜尽量少用；若确需保留，duration 应较短。

请严格按以下 JSON 格式返回：
{"scenes": [
  {"description": "中文画面描述", "shot_type": "medium", "characters": ["角色1"], "character_states": [{"name": "角色1", "action": "narrating", "emotion": "calm"}], "items": [], "mood": "calm", "location": "地点", "duration": %d, "dialogue": "旁白原文"}
]}

第 %d 集内容：
%s`, p.EpisodeNum, p.ModelDuration, p.VisualHint, p.SpeechHint, p.RefDuration, p.EpisodeNum, p.Content)
}

func scriptDramaSceneSplitPrompt(p SceneSplitParams) string {
	return fmt.Sprintf(`你是一位专业的影视分镜师和摄影指导。请将以下第 %d 集的内容拆分为适合短剧/剧本视频制作的分镜。

**核心原则：按可视叙事节拍拆分，不是广告口播合并，也不是漫画格最小化。**
- 以场景切换、人物动作链、对白回合、情绪转折、冲突升级为拆分依据
- 同一连续动作链可拆成 2-4 个递进分镜，保持动作前后承接
- 不要把大段对白强行塞进一镜；也不要为每个微表情单独拆镜
- 允许无对白的环境/反应镜头，但连续无对白镜头不宜过多

**拆分规则：**
- 新场景 / 新时空 → 新分镜；同场景内按动作链和对白回合递进拆分
- description 用中文写可见画面（40-120字）：人物动作、表情、环境、光线氛围；景别只填 shot_type，不要写机位/方位
- shot_type：close-up / medium / full / wide / overhead / low-angle / tracking / establishing
- characters / character_states / items / mood / location 按需填写
- duration 整数秒，按对白长短与动作复杂度估算：无对白反应镜 2-4 秒，短对白 4-6 秒，标准对白 5-8 秒，长对白/情绪高潮 8-12 秒：
%s
- 构图/画面约束：
%s
- dialogue：该场景对白（保持原文；无对白可留空）

**内联标注识别：**
[摄影:xxx]→shot_type；[美术:xxx]→环境；[字幕:xxx]→dialogue；[道具:xxx]→items/description

请严格按以下 JSON 格式返回：
{"scenes": [
  {"description": "中文画面描述", "shot_type": "medium", "characters": ["角色1"], "character_states": [{"name": "角色1", "action": "stands by window", "emotion": "tense"}], "items": [], "mood": "tense", "location": "室内", "duration": %d, "dialogue": "对白"}
]}

第 %d 集内容：
%s`, p.EpisodeNum, p.ModelDuration, p.VisualHint, p.RefDuration, p.EpisodeNum, p.Content)
}

func comicsSceneSplitSystemPrompt() string {
	return "你是漫画分格拆分助手，只输出JSON，不要输出其他内容。你必须只写观众能看见的画面，不要写剧情解释或抽象心理分析。漫画格拆分应宁多勿少，每次动作、对白、情绪或场景变化优先独立成格。"
}

func adSceneSplitSystemPrompt() string {
	return "你是分镜场景拆分助手，只输出JSON，不要输出其他内容。当前为广告口播模式：分镜拆分必须优先按当前目标单分镜时长判断台词/口播承载量；若同一段口播在当前时长内能完整表达，应优先保留在同一分镜中。除最后一个分镜外，默认每个分镜都必须包含可被念出或显示的 dialogue；无台词或台词过短的分镜必须并回相邻分镜。description 只写可见动作与氛围，禁止写空间方位和机位术语。"
}

func commentarySceneSplitSystemPrompt() string {
	return `你是解说漫分镜拆分助手，只输出JSON，不要输出其他内容。
当前为旁白驱动漫画风讲解模式：
- 按旁白信息点和剧情节点拆分，宁多勿少，不要把多段旁白硬合并
- dialogue 必须承载会被 TTS 念出的旁白原文；优先逐字保留原文中的 [字幕:…] 内文本
- description 只写画面，禁止写机位/方位套话，禁止把 description 复制进 dialogue
- 没有可念旁白的镜头应尽量少生成；不要为了凑镜数创造第三人称画面解说
- duration 要随旁白长短起伏，形成节奏变化`
}

func scriptDramaSceneSplitSystemPrompt() string {
	return "你是影视剧本分镜拆分助手，只输出JSON，不要输出其他内容。当前为剧本叙事模式：按场景、动作链、对白回合和情绪转折拆分，不使用广告口播合并规则。允许合理数量的无对白镜头。duration 随对白与动作复杂度变化。description 只写可见内容，禁止空间方位和机位术语。"
}
