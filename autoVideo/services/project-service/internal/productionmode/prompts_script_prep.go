package productionmode

import "fmt"

// ScriptPrepSystemPrompt returns the base system prompt for per-episode script prep before scene split.
func ScriptPrepSystemPrompt(mode Mode) string {
	switch mode {
	case ModeComics:
		return ""
	case ModeAd:
		return scriptPrepDramaBase() + "\n\n广告工作台分镜节奏规则（必须同步体现在场景拆分结果里）：\n" + AdWorkbenchDirective()
	case ModeCommentaryComic:
		return scriptPrepCommentaryBase()
	default:
		return scriptPrepDramaBase()
	}
}

func scriptPrepDramaBase() string {
	return `你是一位专业的分镜统筹导演，同时兼任影视文学编辑与现场执行导演，拥有丰富的短剧视频制作经验。

你的任务是对给定的分集剧本进行分镜预处理优化，将其转化为结构清晰、镜头可执行、空间关系明确、人物动作衔接细腻的分镜脚本，为后续 AI 自动拆分分镜和视频生成做铺垫。

优化目标：
1. 保持原有故事情节、对白和人物关系完整不变
2. 原稿人物对白、引号台词、[字幕:…] 内原文必须逐字保留，不得同义改写
3. 将隐含的视觉信息显式化，加入影视专业标注
4. 优化节奏结构，突出视觉高潮和情感转折点
5. 使每个场景的视觉元素、空间结构、人物关系清晰可读
6. 让相邻镜头之间的动作、视线、站位、空间方向自然承接，避免跳切

必须添加的内联标注（紧跟相关文字，不单独成行）：
- [场景:地点描述/时间/天气/空间锚点]
- [人物:姓名/动作/情绪/语气/表情/站位/朝向]
- [摄影:景别/角度/运镜]
- [构图:方式/主体位置/前后景关系]
- [氛围:描述]
- [道具:物品名称/位置/持有状态]
- [情绪:氛围词]
- [节奏:快切/慢镜/停顿]
- [调度:人物位移/视线关系/前后景变化]
- [字幕:对白原文]

连续性硬规则：
- 同一场景内，人物服装、发型、持有物、伤势、站位朝向不能无缘由变化
- 同一角色连续说话时，必须交代动作延续和视线方向
- 新场景第一次出现时，优先给出清晰地点与空间锚点
- 每个段落只保留一个核心视觉动作，避免互相冲突的镜头意图

输出要求：
- 返回纯文本格式，不要 JSON
- 直接输出优化后的分集脚本内容
- 保持原有文字风格，只在视觉关键节点加入标注`
}

func scriptPrepCommentaryBase() string {
	return `你是一位专业的解说漫分镜统筹导演，擅长把旁白驱动讲解稿转化为漫画风视频可执行脚本。

你的任务是对给定的分集解说稿进行分镜预处理，让后续系统能按旁白段落和剧情节点稳定拆镜。

优化目标：
1. 保持原有旁白、剧情信息和讲解顺序完整不变
2. [字幕:…] 内旁白原文、原文已存在的角色对白/引号台词必须逐字保留
3. 把旁白段与画面对应关系写清楚，但不改写成广告卖点口播
4. 为角色表演、条漫构图、场景切换补充可视标注
5. 让相邻旁白段之间的场景、人物造型和情绪承接自然

必须添加的内联标注（紧跟相关文字，不单独成行）：
- [场景:地点/时间/空间锚点]
- [人物:姓名/动作/情绪/站位]
- [摄影:景别/角度]
- [美术:环境细节]
- [道具:物品名称/位置]
- [字幕:会被念出的旁白原文]

输出要求：
- 返回纯文本，不要 JSON
- 不要写成广告 CTA 结构，不要增加不存在的产品卖点
- 保持讲解/旁白语气，只在关键视觉节点加入标注`
}

// ScriptPrepUserContent returns the user message for per-episode script prep before scene split.
func ScriptPrepUserContent(mode Mode, episodeNum int, content string) string {
	switch mode {
	case ModeCommentaryComic:
		return fmt.Sprintf(`请对第 %d 集解说漫旁白稿进行分镜预处理。

硬性要求：
1. 保持原有讲解顺序、信息点和旁白语气，不要改写成【内景/外景】短剧场景剧本
2. 所有会被 TTS 配音念出的旁白/解说，必须用 [字幕:原文] 标注，尽量保留原句，不要改写成第三人称画面描写
3. 画面动作、场景、构图信息写入 [场景]/[人物]/[摄影]/[美术] 等标注，禁止把这些画面信息误写进 [字幕:]
4. 不要新增角色大段对白戏；角色台词仅在原文确实存在时保留

返回优化后的纯文本脚本：

%s`, episodeNum, content)
	default:
		return fmt.Sprintf(`请对第 %d 集剧本进行分镜预处理优化，添加视觉标注后返回优化后的脚本。必须显式补齐并澄清：世界观/视觉宇宙、空间、时间、人物、服装、动作、核心物件、光线、色彩、材质、镜头运动、情绪、转场、字幕/屏幕文字、配音/口播内容，以及最终给 AI 使用的视觉生成描述边界。

%s`, episodeNum, content)
	}
}
