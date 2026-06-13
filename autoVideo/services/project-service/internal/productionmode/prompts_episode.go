package productionmode

// EpisodeSplitDirective returns mode-specific system-prompt additions for LLM episode splitting.
func EpisodeSplitDirective(mode Mode) string {
	switch mode {
	case ModeAd:
		return AdWorkbenchDirective()
	case ModeCommentaryComic:
		return commentaryEpisodeSplitDirective()
	case ModeScriptDrama:
		return scriptDramaEpisodeSplitDirective()
	default:
		return ""
	}
}

// EpisodeEnrichDirective returns mode-specific additions for episode title/summary enrichment.
func EpisodeEnrichDirective(mode Mode) string {
	switch mode {
	case ModeAd:
		return AdWorkbenchDirective()
	default:
		return ""
	}
}

// OptimizeBeforeSplitMessage returns the progress message shown before script optimization.
func OptimizeBeforeSplitMessage(mode Mode) string {
	switch mode {
	case ModeAd:
		return "正在按所选风格优化广告文案，为自动切分做准备…"
	default:
		return ""
	}
}

// OptimizeAfterSplitMessage returns the progress message shown after script optimization.
func OptimizeAfterSplitMessage(mode Mode) string {
	switch mode {
	case ModeAd:
		return "广告文案优化完成，正在根据时长自动计算分集数…"
	default:
		return ""
	}
}

// AdWorkbenchDirective is the shared ad-workbench rhythm rules block.
func AdWorkbenchDirective() string {
	return `- 面向广告转化，不写成长剧情拖沓节奏；每个片段都要快速建立场景、动作、卖点与 CTA 关系。
- 镜头节奏优先"开场钩子 → 痛点/场景 → 解决方案/产品展示 → 证据/细节 → CTA 收束"。
- 广告拆分的最高优先级是"先按台词 / 口播承载量满足当前目标单分镜时长，再决定是否继续拆分"；如果同一段口播/卖点在当前时长内可以完整表达，应优先合并为一个主分镜或少量连续分镜，禁止为了追求最小动作单位而过度拆镜。
- 只有在以下情况才允许继续拆分：1）明确切换到新空间/新场景；2）明确进入新卖点/新产品信息；3）说话主体发生变化；4）当前段内容明显超过当前单分镜时长可承载的信息量；5）确实需要一个极短强调镜头，但数量必须严格受限。
- 广告口播项目中，大多数分镜都应承载明确的 dialogue / 字幕 / 卖点信息；无 dialogue 镜头只能作为极短辅助镜头，不能连续出现，也不能成为主体。
- 轻微的表情变化、手部动作变化、视线变化、镜头轻推拉，不足以单独拆成新分镜；若不引入新的信息点，应继续留在当前分镜内完成表达。
- 广告优化、文案拆分、分镜预处理、Prompt 生成时，必须明确以下 14 个维度：1）世界观/故事发生的视觉宇宙；2）空间（在哪里）；3）时间（几点/白天夜晚/时序）；4）人物（谁）；5）服装（穿什么）；6）动作（做什么）；7）核心物件/镜头重点；8）光线（怎么打光）；9）色彩（什么色调）；10）材质（表面质感）；11）镜头运动（怎么拍）；12）情绪（传达什么感觉）；13）转场（怎么切）；14）字幕/屏幕文字、配音/口播内容，以及最终给 AI 的生成 Prompt 描述。
- 场景描述必须明确时间、空间、人物位置关系、关键道具和镜头焦点，避免后续素材生成漂移。
- 台词、字幕、口播、屏幕文字必须各司其职：台词/配音负责说什么，字幕负责屏幕上显示什么，Prompt 负责给生成模型什么视觉/镜头描述，不要互相混写。
- 光线、色彩、材质必须可视化，不要只写抽象情绪词；例如要明确暖金逆光、冷白顶光、哑光塑料、拉丝金属、玻璃反光、柔雾肤感等。
- 同一个广告项目内，人物外观、服饰、场景主色调、道具、品牌表达、口播身份和语气必须稳定延续。`
}

func commentaryEpisodeSplitDirective() string {
	return `- 面向解说漫/旁白驱动视频，按"旁白段落 + 剧情节点"拆分，而不是按广告卖点或 CTA 节奏拆分。
- 分集优先级（强制）：1）原文章节/幕/段落标题；2）用户提供的分集关键词；3）剧情转折与讲解主题变化。
- 每一集应覆盖一段完整讲解主题（人物线、剧情段、设定块或盘点单元），保证旁白叙述连贯。
- 若原文存在"第X章/回/节/集"等章节标题，必须优先按章节边界分集，不要为凑时长或集数而跨章节合并。
- 允许同一集内包含多个场景，但不要为了凑时长把无关主题硬合并到同一集。
- 【简介】/全书概括/结局剧透不得单独成第 1 集；有现场感的【导语】应并入第一章/第 1 集正文。
- 分集摘要应突出"本集讲了什么故事/设定/人物"，而不是产品卖点或转化话术。`
}

func scriptDramaEpisodeSplitDirective() string {
	return `- 面向影视剧本/短剧叙事，按剧情的起承转合拆分，每集应有完整叙事弧（开端-发展-高潮/悬念）。
- 分集优先级（强制）：1）原文章节/幕/段落标题；2）用户提供的分集关键词；3）情节转折、冲突升级、场景切换。
- 若原文存在"第X章/回/节/集"等章节标题，必须优先按章节边界分集，不要为均分字数而截断章节。
- 每集字数尽量均匀，但不要为均分而截断对白或动作链。
- 分集摘要应覆盖主要角色行动、冲突、情感变化和情节转折。`
}

// EpisodeSplitReviewSystemPrompt returns the system prompt for project-level split boundary review.
func EpisodeSplitReviewSystemPrompt(mode Mode) string {
	base := `你是专业的长篇剧本分集结构审查员。你会收到已经初步拆好的分集列表（标题、字数、首尾片段），需要判断分集边界是否合理。

**重点检查：**
1. 是否存在"简介/预告/全书概括"被单独拆成第 1 集，而真正的叙事正文从第 2 集才开始
2. 是否存在仅含【简介】【导语】营销文案、没有具体场景/对白的空集
3. 相邻两集是否大量重复或前后重叠
4. 第 1 集是否过短（通常 <500 字）且只有结局/投资/逆袭等概括句，缺少具体人物动作

**issue 类型：** front_matter_as_episode | summary_trailer_episode | duplicate_with_next | empty_episode | other

**action 类型：**
- merge_into_next：将 episode_index 指定集并入下一集（仅 index 0..n-2）
- drop：删除指定集（仅当该集纯前言/简介、且下一集已包含正文）

返回严格 JSON（不要 markdown 代码块）：
{
  "passed": true,
  "issues": [{"type": "summary_trailer_episode", "episode_index": 0, "severity": "critical", "detail": "..."}],
  "actions": [{"type": "merge_into_next", "episode_index": 0}]
}

若分集边界合理，passed=true 且 actions=[]。`
	switch mode {
	case ModeCommentaryComic:
		return base + "\n\n**解说漫补充：** 第 1 集应能直接开拍/开录旁白，不能只是全书剧情预告；有现场感的导语应并入正文第 1 集，而不是单独成集。"
	default:
		return base
	}
}
