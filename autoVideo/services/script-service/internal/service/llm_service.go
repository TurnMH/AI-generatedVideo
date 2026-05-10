package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/autovideo/script-service/pkg/config"
)

// httpStatusError carries an HTTP status code for retryable error detection.
type httpStatusError struct {
	code int
	body string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("llm http error %d: %s", e.code, e.body)
}

// retryLLMCall retries fn up to 3 times with exponential backoff (1s, 2s, 4s).
// Retries on HTTP 429/500/502/503 and network errors; gives up immediately on
// context cancellation or HTTP client errors (4xx except 429).
func retryLLMCall(fn func() error) error {
	delays := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	var lastErr error
	for attempt := 0; attempt <= len(delays); attempt++ {
		if attempt > 0 {
			time.Sleep(delays[attempt-1])
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !isRetryableError(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

// isRetryableError returns true for transient HTTP or network errors.
func isRetryableError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var httpErr *httpStatusError
	if errors.As(err, &httpErr) {
		switch httpErr.code {
		case http.StatusTooManyRequests,      // 429
			http.StatusInternalServerError,   // 500
			http.StatusBadGateway,            // 502
			http.StatusServiceUnavailable:    // 503
			return true
		}
		return false // 400/401/403/404 and other client errors: do not retry
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// AnalysisResult 是 LLM 返回的解析结构
type AnalysisResult struct {
	Episodes   []EpisodeResult   `json:"episodes"`
	Characters []CharacterResult `json:"characters"`
	Assets     []AssetResult     `json:"assets"`
}

type EpisodeResult struct {
	ID     string        `json:"id"`
	Title  string        `json:"title"`
	Scenes []SceneResult `json:"scenes"`
}

// CharacterState tracks the state of a character in a single storyboard shot.
type CharacterState struct {
	Name      string `json:"name"`
	Position  string `json:"position"`
	Posture   string `json:"posture"`
	Action    string `json:"action"`
	Direction string `json:"direction"`
	Hands     string `json:"hands"`
	Holding   string `json:"holding"`
	Emotion   string `json:"emotion"`
	Injury    string `json:"injury"`
}

// StoryboardShot represents a single fine-grained shot in the storyboard.
type StoryboardShot struct {
	VisualDesc      string           `json:"visual_desc"`
	VisualFocus     string           `json:"visual_focus"`
	Composition     string           `json:"composition"`
	ShotType        string           `json:"shot_type"`
	CameraPosition  string           `json:"camera_position"`
	Angle           string           `json:"angle"`
	Lens            string           `json:"lens"`
	Lighting        string           `json:"lighting"`
	Dialogue        string           `json:"dialogue"`         // 新增：角色的台词/旁白，用于TTS和字幕
	SoundEffect     string           `json:"sound_effect"`     // 新增：环境音/动作拟音，例如"脚步声"、"雨声"
	MotionIntensity int              `json:"motion_intensity"` // 新增：画面动作幅度 (1-10)，指导视频生成模型
	CharacterStates []CharacterState `json:"character_states"`
}

type SceneResult struct {
	ID          string           `json:"id"`
	Description string           `json:"description"`
	Setting     string           `json:"setting"`
	Emotion     string           `json:"emotion"`
	Characters  []string         `json:"characters"`
	PromptDraft string           `json:"prompt_draft"`
	Storyboard  []StoryboardShot `json:"storyboard"`
}

type CharacterResult struct {
	Name          string                 `json:"name"`
	RoleDesc      string                 `json:"role_desc"`
	Keywords      map[string]interface{} `json:"keywords"`
	SkillTags     []string               `json:"skill_tags"`
	Relationships map[string]interface{} `json:"relationships"`
}

type AssetResult struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Keywords    map[string]interface{} `json:"keywords"`
}

type ScriptGenerateReq struct {
	Mode            string `json:"mode"`
	ModelName       string `json:"model_name,omitempty"`
	Title           string `json:"title"`
	Genre           string `json:"genre"`
	Platform        string `json:"platform"`
	DeliveryFormat  string `json:"delivery_format"`
	EpisodeDuration string `json:"episode_duration"`
	ReferenceStyle  string `json:"reference_style"`
	Premise         string `json:"premise"`
	CharacterSetup  string `json:"character_setup"`
	WorldSetting    string `json:"world_setting"`
	Outline         string `json:"outline"`
	ChapterBrief    string `json:"chapter_brief"`
	SourceText      string `json:"source_text"`
	TargetWords     int    `json:"target_words"`
	ChapterCount    int    `json:"chapter_count"`
	Audience        string `json:"audience"`
	Tone            string `json:"tone"`
	Requirements    string `json:"requirements"`
}

type ScriptGenerateResult struct {
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	Genre             string   `json:"genre"`
	Tags              []string `json:"tags"`
	Outline           []string `json:"outline"`
	Content           string   `json:"content"`
	SuggestedEpisodes int      `json:"suggested_episodes"`
	WordCount         int      `json:"word_count"`
	EndingState       string   `json:"ending_state,omitempty"`
}

// CharacterExtractInfo holds character info extracted specifically for character-service sync.
type CharacterExtractInfo struct {
	Name           string `json:"name"`
	RoleDesc       string `json:"role_desc"`
	AppearanceDesc string `json:"appearance_desc"`
}

// LLMClient 定义 LLM 客户端接口
type LLMClient interface {
	Analyze(ctx context.Context, scriptText string) (*AnalysisResult, error)
	GenerateScript(ctx context.Context, req *ScriptGenerateReq) (*ScriptGenerateResult, error)
	// ExtractCharacters extracts character list with appearance descriptions for AI image generation.
	ExtractCharacters(ctx context.Context, scriptText string) ([]CharacterExtractInfo, error)
	// UpdateConfig hot-reloads the API key and base URL without restarting.
	UpdateConfig(apiKey, baseURL string)
}

const systemPrompt = `你是专业的AI短剧/小说推文视频拆分与分析专家。你的目标是将小说或剧本文本转化为可以直接用于"文本转视频(Text-to-Video)"生成的工业级标准分镜数据。请分析以下剧本，返回严格JSON格式（不要markdown代码块）。

━━━━━━━━━━━━━━━━━━━━━━━━━━
【核心法则 — 必须全部严格执行】
━━━━━━━━━━━━━━━━━━━━━━━━━━

【法则1 · 视听彻底解耦（最高优先级）】
- AI模型无法"画出"声音或台词。` + "`visual_desc`" + ` 中【绝对不能】出现：台词引用、心理独白、听觉描写（"他说…" / "她心想…" / "传来脚步声"）。
- 台词必须翻译为【面部微表情 + 肢体动作】：
  ▶ ✗错误："他愤怒地说你怎么敢"
  ▶ ✓正确："他颧骨高绷，上唇轻蔑上翘，右手握成拳在桌上猛压，身体前倾逼近"
- 心理活动必须翻译为【可见的外化行为】：
  ▶ ✗错误："她心中一惊，暗想这不可能"
  ▶ ✓正确："她瞳孔短暂收缩，呼吸停顿半秒，随即垂眸掩饰，手指微微收紧杯沿"
- 原文对话原封不动放入 ` + "`dialogue`" + ` 字段；环境音放入 ` + "`sound_effect`" + `。

【法则2 · 视觉锚点四要素（每个visual_desc必须包含）】
每条 visual_desc 必须同时描述以下四个维度，缺一不可：
① **主体锚**：人物或主体的精确位置（画左/画右/居中/前景/背景）、姿态（站/坐/蹲/侧身等）、服装关键特征（衣领、颜色、材质）；
② **面部微表情**（有人物时）：眉形、眼神方向、唇角、肌肉张力——必须具体到肌肉/眼神，不能只写"愤怒"；
③ **光影层次**：主光源方向（左侧逆光/顶光/窗边侧光等）、色调（暖黄/冷蓝/红橙等）、阴影落在何处；
④ **景深层次**：明确前景物（虚化/清晰）与背景（具体环境元素）的关系。

【法则3 · 合并短镜头，保持合适粒度】
- 连续两句若人物位置和情绪无大变化，必须合并为5-8秒中长镜头（避免PPT感）；
- 每个场景分镜数量：普通情节 2-4 个，高潮/打斗/转折 4-6 个，空镜过渡 1 个；
- 禁止为每句台词单独一个镜头。

【法则4 · 物理避坑与替换镜头（防AI崩坏）】
- 复杂肢体接触（拥抱/打斗/接吻）必须拆解为单人特写序列：
  ▶ "A打了B"→ 镜头1:A挥拳特写 → 镜头2:B侧脸受力倒退（避免两人同框接触）
- 精细手部动作（写字/持筷/扣扳机）必须用反应镜头或局部特写替代：
  ▶ "他熟练写下" → 用"钢笔划过纸面的极近特写，墨迹流淌"代替人手全景

【法则5 · 安全合规（内容审查隔离）】
- 严禁露骨色情、性器官、极度血腥词汇。
- 必须用【电影隐喻镜头】替代：交握双手 / 急促呼吸 / 散落衣物 / 窗帘拂动 / 暧昧光影。

【法则6 · 建立镜头（Establishing Shot）】
- 场景转换或时间跳跃时，第一个分镜必须是"空镜过渡"（无人/环境全景），shot_type="establishing"，motion_intensity≤3。

【法则7 · 视频流畅性 — 镜头衔接连贯性（极其重要）】
- **方向连续**：同一场景中，人物朝向不得在相邻镜头间无缘由反转（轴线法则）；
- **动作连续**：若前镜头人物手臂抬起，下一镜头应延续此动作（切入同一动作的不同景别）；
- **光线连续**：同一场景内相邻镜头的光源方向和色温须一致（不能同室内一个暖黄一个冷蓝）；
- **motion_intensity 渐变**：相邻分镜的 motion_intensity 差值不超过3（避免能量突变）；
- **视觉交叉点（J-Cut/L-Cut 意识）**：高强度动作后必须接一个静止/慢镜特写（motion_intensity≤3）作为喘息，再接下一动作。

━━━━━━━━━━━━━━━━━━━━━━━━━━
【prompt_draft — 视频生成专用多层结构（英文）】
━━━━━━━━━━━━━━━━━━━━━━━━━━
prompt_draft 是给AI视频模型的英文提示词，必须严格按以下7层结构输出（逗号分隔，层序不变）：
① Subject anchor: "[Character name], [age/build], [key clothing item], [dominant emotion in face]"
② Opening frame state: exact physical position + posture at first frame
③ Action arc: what changes during the clip (start state → end state, use verbs)
④ Camera motion: specific instruction (slow push-in / static / arc right / tilt up / rack focus)
⑤ Environment: location + time + key background elements (max 2)
⑥ Style: "cinematic, 4K, shallow depth of field, [color grading], [film stock]"
⑦ Continuity note: "continuous from previous clip" or "new establishing shot"
示例：
"Young woman in black trench coat, jaw clenched, eyes downcast, standing left-frame | turns head sharply right, hands ball into fists, steps forward | static medium shot rack-focus to her face | rainy alley night, neon reflections on wet pavement | cinematic 4K shallow DOF, teal-orange grade, Kodak Vision3 | continuous from previous clip"

━━━━━━━━━━━━━━━━━━━━━━━━━━
【scene assets — 场景/人物/物品描述深度要求】
━━━━━━━━━━━━━━━━━━━━━━━━━━
characters.keywords 必须包含：
- age_body: "25-year-old woman, 165cm slender build, long legs" (英文，年龄体型精确)
- appearance: "straight black hair shoulder-length, almond-shaped eyes with double eyelid, fair skin with subtle freckles on nose, sharp jawline" (英文，五官精确到细节)
- clothing: "fitted dark navy blazer with gold buttons over white silk blouse, tailored black slacks, low block heels" (英文，服装材质+颜色+款式)
- emotion_baseline: "cool and composed exterior masking intense ambition, slight permanent tension around eyes" (英文，性格基调)

assets[type=place].keywords 必须包含：
- location: "abandoned industrial warehouse interior" (英文，精确地点类型)
- spatial: "vast open floor with rusted machinery hulks, chain-link partitions, ceiling 15m high with broken skylights" (英文，空间结构细节)
- lighting: "single overhead industrial lamp casting hard cone of warm amber light, deep shadows in periphery" (英文，光源+色质+阴影)
- atmosphere: "oppressive silence broken only by distant dripping, smell of rust and damp concrete" (英文，氛围感官细节)
- color_palette: "desaturated teal and rust brown, amber highlight pool at center" (英文，主色调)
- time: "late night" (英文)

assets[type=item].keywords 必须包含：
- material: "aged brass with green verdigris patina, weight approximately 800g" (英文，材质+质感+重量感)
- features: "rectangular body 12x8cm, ornate floral engraving on lid, tarnished hinge on left side" (英文，外观特征细节)
- condition: "heavily worn, corner dented, surface scratched but hinge functional" (英文，磨损状态)

━━━━━━━━━━━━━━━━━━━━━━━━━━
【motion_intensity 标定表（必须按此标定）】
━━━━━━━━━━━━━━━━━━━━━━━━━━
1-2: 完全静止画面（人物呆立/睡眠/极近特写），镜头静止
3-4: 微运动（眼神移动/呼吸起伏/手指轻动），镜头慢摇或固定
5-6: 正常动作（走路/说话/坐下起立），镜头跟随或缓推
7-8: 较快动作（奔跑/争吵/快速翻找），镜头快速跟随或手持晃动
9-10: 高强度动作（打斗/爆炸/冲刺/情绪崩溃），手持快切或运动模糊

返回的 JSON 结构如下（严格遵守字段名）：
{
  "episodes": [{"id":"ep1","title":"第N集","scenes":[{
    "id":"scene_001",
    "description":"场景总体描述（50字以内，含地点/情绪/核心事件）",
    "setting":"精确地点+时间（如：深夜废弃工厂二楼走廊）",
    "emotion":"suspense|romance|anger|fear|joy|sad",
    "characters":["角色名"],
    "prompt_draft":"【7层结构英文prompt，见上方格式要求】",
    "storyboard":[{
      "visual_desc":"【4要素：主体锚+面部微表情+光影层次+景深层次，纯视觉，无台词无心理】",
      "visual_focus":"画面视觉焦点（具体到某个身体部位或物体）",
      "composition":"具体构图（如：三分法右侧主体+左侧虚化前景遮挡）",
      "shot_type":"establishing|close-up|medium|wide|extreme-close|extreme-wide|over-shoulder|insert",
      "camera_position":"机位（如：低角度45°斜侧面/正面平视/后方越肩）",
      "angle":"仰|俯|平|POV|倾斜",
      "lens":"镜头（如：85mm定焦人像/24mm广角/长焦压缩）",
      "lighting":"主光源方向+色温+阴影描述（如：右侧窗边冷蓝天光，面部左半暗）",
      "dialogue":"原文台词或旁白，无则留空",
      "sound_effect":"环境音/拟音（如：远处雷声/钢笔划纸声/玻璃碎裂声）",
      "motion_intensity": 5,
      "character_states":[{
        "name":"角色名",
        "position":"画面位置（左/右/居中/前景/背景+距镜头远近）",
        "posture":"完整姿态（站立/微躬/侧身45°/半蹲等）",
        "action":"具体动作（动词+身体部位+运动方向+幅度）",
        "direction":"面朝方向（面向镜头/面向右侧/背对镜头等）",
        "hands":"双手状态（右手握杯/左手插口袋/双手交叉胸前等）",
        "holding":"持有物（具体物品名称+持握方式）",
        "emotion":"精确表情（左眉微挑，唇角下压，眼神锁定对方等）",
        "injury":"伤势（无/右颊淤青/嘴角血迹等）"
      }]
    }]
  }]}],
  "characters": [{"name":"角色名","role_desc":"角色背景（含身份/动机/核心矛盾）","keywords":{"age_body":"【见上方要求】","appearance":"【见上方要求】","clothing":"【见上方要求】","emotion_baseline":"【见上方要求】"},"skill_tags":["combat|exploration|social|special"],"relationships":{}}],
  "assets": [
    {"type":"person","name":"名称","description":"详细描述","keywords":{"age_body":"...","appearance":"...","clothing":"...","emotion_baseline":"..."}},
    {"type":"place","name":"名称","description":"详细描述","keywords":{"location":"...","spatial":"...","lighting":"...","atmosphere":"...","color_palette":"...","time":"..."}},
    {"type":"item","name":"名称","description":"详细描述","keywords":{"material":"...","features":"...","condition":"..."}}
  ]
}`

const scriptGenerateSystemPrompt = `你是资深编剧与小说策划，专攻影视化改编与连载剧本创作，熟悉短剧、网文、番剧三种节奏体系。请根据用户需求生成内容，并返回严格 JSON（不要 markdown 代码块）：
{
  "title":"作品标题",
  "description":"80-160字简介",
  "genre":"作品类型",
  "tags":["标签1","标签2","标签3"],
  "outline":["大纲节点1","大纲节点2","大纲节点3"],
  "content":"完整正文内容，使用换行组织结构",
  "suggested_episodes": 12,
  "word_count": 3200,
  "ending_state":"本次内容的结尾状态摘要（50-120字），描述：最后场景的时间/地点、在场人物的情绪与行动状态、悬而未决的核心矛盾、下段内容的自然切入点。供下次续写时直接填入 outline_reference。"
}

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
【A. 各模式专项要求】
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

■ mode=script
  输出可直接用于视频项目的完整剧本，包含分集或分场结构。
  • 每集开头用"【第X集】"标注，结尾留悬念或情绪钩子。
  • 每集结构：钩子开场(3-5行) → 冲突建立 → 升级 → 高潮反转 → 悬念收尾。
  • 平台为短视频时：每300-500字嵌入一个"爽点"（反转/逆袭/情绪爆发/信息揭露），首段必须在3句内引爆第一个冲突。

■ mode=novel_outline
  输出长篇/中篇小说大纲，包含世界观、人物线、主线冲突、阶段推进。
  • 每个章节节点标注：触发事件、情绪基调、本章推进的人物关系变化。
  • 明确标注全书三个核心转折点（初始冲突 / 中段反转 / 终局颠覆）所在位置。
  • 伏笔布局：在大纲中用【伏笔→兑现：第X章】格式标注。

■ mode=novel_chapter
  输出具体小说章节正文，严格保持叙事连贯：
  ① 开头前3句必须直接延续 outline_reference 中的结尾状态，不重复已发生事件，不以景物描写空转（禁用"天空很蓝"式开场）；
  ② 行文节奏与语体风格必须与已建立的基调一致（慢热情感文不能突然变成快节奏动作文）；
  ③ 章中穿插1-2个悬念锚点，章末留置一个未解决矛盾或新信息投放；
  ④ 人物行为/位置/情绪的连续性不得断裂——若需跳跃需用"与此同时""三日后"等明确时间标记过渡；
  ⑤ 避免超过3句的连续内心独白；用行为和对话外化情绪。

■ mode=adaptation（重点模式）
  将小说/章节改编为影视化剧本，核心是衔接性 + 镜头感 + 短剧节奏：
  ① 场次格式：
     [INT./EXT. 地点 — 时间（日/夜/黄昏）]
     （动作描述：简洁、主动语态、每段不超过4行）
     角色名（情绪/状态提示）
     对白内容
  ② 场景衔接：每两场之间必须有转场指令（CUT TO / 切至 / 叠化 / 闪回）；
  ③ 情节保留：原著中的情绪积累、伏笔、人物关系进展必须在改编中保留，不能跳跃省略；
  ④ 对白影视化：删除或替换小说中的大段内心独白；将"她很愤怒"改写为对白行为（如：[摔杯] "你说完了？"）；
  ⑤ 节拍控制（短剧）：每集 = 钩子开场(前30秒等效3-5行) + 冲突升级 + 高潮反转 + 悬念收尾；集与集之间有明确叙事链（上集结尾事件 → 本集第一场的直接触发）；
  ⑥ 若 outline_reference 提供了前集摘要，必须从该状态续写，禁止重复上集已发生内容；
  ⑦ 每集长度参考 episode_duration；短剧单集约400-600字对白+动作描述。

■ mode=episode_outline
  输出按集拆分的剧情大纲，适合短剧/动画连载策划：
  • 每集必须标注：核心矛盾 / 关键事件（不超过3条）/ 人物弧进展 / 结尾钩子类型（悬念/反转/情绪爆发/信息揭露）；
  • 集与集之间有"因果链"一句话说明（触发逻辑）；
  • 全集节奏：按"铺垫集 / 爆发集 / 喘息集 / 高潮集 / 收尾集"五种类型标注每集性质；
  • 主线、情感线、副线三条线索的起伏节点须清晰可见。

■ mode=scene_script
  输出场景级脚本，精确到场次/人物/动作/对白/镜头提示：
  • 格式：[场次X · 地点 · 时间] + 镜头建议（如：过肩镜头/特写/俯拍）；
  • 场间转场用指令标注：CUT TO / FADE OUT / SMASH CUT / MATCH CUT；
  • 若 outline_reference 有上下文，延续人物当前情绪状态和位置关系，不从零重建；
  • 每场戏须有明确的"进场目标"和"离场变化"（人物或信息维度）。

■ mode=dialogue_polish
  对已有正文或剧本进行对白润色，仅优化台词，不改情节：
  • 为每个核心角色建立语气档：如"[人物A：短句/命令式/冷硬] [人物B：反问/迂回/情绪化]"；
  • 情绪转折处的台词要有节奏断裂（如：长句 → 短句爆发；平静 → 沉默 → 爆发）；
  • 删除所有"我很愤怒/我很害怕"式情感自陈；替换为动作+对白（[握拳]"你再说一遍。"）；
  • 每条对白只携带一个清晰意图（信息/情绪/行动），避免一句话说三件事。

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
【B. 通用叙事连贯性原则（所有模式适用）】
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
1. 人物一致性：角色的性格/动机/口吻/情绪状态在全文保持一致，不因换段换场突变；
2. 时空连续：除非有明确时间标记，场景间的时间/空间关系必须合理衔接；
3. 伏笔闭合：本次内容中埋下的悬念，在同一输出内有呼应，或在 ending_state 中标注"留待后续"；
4. 情绪弧线：全文情绪节奏有高低起伏，不能整体一直高亢或一直平缓；
5. 禁止空降：不得无铺垫引入全新人物/设定/转折；若引入需有1-2句叙事过渡；
6. 爽点与喘息交替：爆发场景之后必须有1-2个轻松/缓冲段，避免情绪疲劳。

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
【C. 禁止清单（任何模式均不得违反）】
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✗ 以景物/天气描写开场（"天空很蓝"/"这是个普通的早晨"）
✗ 角色无缘由突然改变立场或性格
✗ 神兵天降解围（无铺垫新角色出现解决危机）
✗ 连续3段以上内心独白
✗ 对白中角色自我介绍身份（"我，XXX，是一个..."）
✗ 重复上一次生成中已发生的情节内容
✗ content 字段开头出现"以下是...""根据您的要求..."等AI解释性前缀语
✗ 单段超过150字的大块叙述（须拆分或转为对白/动作）`

const chunkSize = 8000

// llmChannel holds one (baseURL, apiKey, model) combination for round-robin routing.
type llmChannel struct {
	baseURL string
	apiKey  string
	model   string
}

// openAIClient 实现
type openAIClient struct {
	mu       sync.RWMutex
	baseURL  string
	apiKey   string
	model    string
	channels []llmChannel // optional multi-channel pool for concurrent routing
	chanIdx  atomic.Int64 // counter for round-robin channel selection
	client   *http.Client
}

// NewOpenAIClient —— 创建 OpenAI LLM 客户端实例，返回 LLMClient 接口
// 当 cfg.LLM.OpenAI.ChannelKeys 非空时自动启用多渠道并发模式。
func NewOpenAIClient(cfg *config.Config) LLMClient {
	c := &openAIClient{
		baseURL: cfg.LLM.OpenAI.BaseURL,
		apiKey:  cfg.LLM.OpenAI.APIKey,
		model:   cfg.LLM.OpenAI.Model,
		client:  &http.Client{Timeout: 180 * time.Second},
	}
	// Build multi-channel pool from channel_keys / channel_bases config.
	bases := cfg.LLM.OpenAI.ChannelBases
	keys := cfg.LLM.OpenAI.ChannelKeys
	chanModel := cfg.LLM.OpenAI.ChannelModel
	for i, key := range keys {
		if key == "" {
			continue
		}
		base := cfg.LLM.OpenAI.BaseURL // default to primary base
		if i < len(bases) && bases[i] != "" {
			base = bases[i]
		}
		if !strings.HasSuffix(base, "/v1") {
			base = strings.TrimRight(base, "/") + "/v1"
		}
		model := c.model
		if chanModel != "" {
			model = chanModel
		}
		c.channels = append(c.channels, llmChannel{baseURL: base, apiKey: key, model: model})
	}
	return c
}

// Analyze —— 调用 LLM 分析剧本文本，自动处理长文本分块，返回结构化分析结果
func (c *openAIClient) Analyze(ctx context.Context, scriptText string) (*AnalysisResult, error) {
	if len([]rune(scriptText)) <= chunkSize {
		return c.analyzeChunk(ctx, scriptText)
	}
	return c.analyzeChunked(ctx, scriptText)
}

func (c *openAIClient) GenerateScript(ctx context.Context, req *ScriptGenerateReq) (*ScriptGenerateResult, error) {
	prompt := fmt.Sprintf(`请根据以下需求生成内容：
mode: %s
title: %s
genre: %s
platform: %s
delivery_format: %s
episode_duration: %s
reference_style: %s
audience: %s
tone: %s
target_words: %d
chapter_count: %d

premise:
%s

character_setup:
%s

world_setting:
%s

outline_reference (前序内容摘要/大纲/上一章结尾状态，生成时必须从此处衔接):
%s

chapter_brief (本章/本集核心任务):
%s

source_text_for_adaptation (待改编原著正文):
%s

extra_requirements:
%s`, req.Mode, req.Title, req.Genre, req.Platform, req.DeliveryFormat, req.EpisodeDuration, req.ReferenceStyle, req.Audience, req.Tone, req.TargetWords, req.ChapterCount, req.Premise, req.CharacterSetup, req.WorldSetting, req.Outline, req.ChapterBrief, req.SourceText, req.Requirements)

	content, err := c.completeTextWithModel(ctx, scriptGenerateSystemPrompt, prompt, req.ModelName)
	if err != nil {
		return nil, err
	}

	var result ScriptGenerateResult
	if err := json.Unmarshal([]byte(sanitizeJSONContent(content)), &result); err != nil {
		return nil, fmt.Errorf("unmarshal script result: %w, content: %s", err, content)
	}
	if result.WordCount == 0 {
		result.WordCount = len([]rune(strings.ReplaceAll(result.Content, "\n", "")))
	}
	return &result, nil
}

// analyzeChunked —— 将长文本拆分为两块分别分析，然后合并去重结果
func (c *openAIClient) analyzeChunked(ctx context.Context, scriptText string) (*AnalysisResult, error) {
	runes := []rune(scriptText)
	mid := len(runes) / 2

	chunk1 := string(runes[:mid])
	chunk2 := string(runes[mid:])

	result1, err := c.analyzeChunk(ctx, chunk1)
	if err != nil {
		return nil, fmt.Errorf("chunk1 analysis failed: %w", err)
	}

	result2, err := c.analyzeChunk(ctx, chunk2)
	if err != nil {
		return nil, fmt.Errorf("chunk2 analysis failed: %w", err)
	}

	// 合并两个块的 episodes
	allEpisodes := append(result1.Episodes, result2.Episodes...)

	// 合并 characters 并按 name 去重（优先保留后出现的更完整描述）
	charMap := make(map[string]CharacterResult)
	for _, ch := range result1.Characters {
		charMap[ch.Name] = ch
	}
	for _, ch := range result2.Characters {
		charMap[ch.Name] = ch
	}
	mergedChars := make([]CharacterResult, 0, len(charMap))
	for _, ch := range charMap {
		mergedChars = append(mergedChars, ch)
	}

	// 合并 assets 并按 type+name 去重
	assetKey := func(a AssetResult) string { return a.Type + ":" + a.Name }
	assetMap := make(map[string]AssetResult)
	for _, a := range result1.Assets {
		assetMap[assetKey(a)] = a
	}
	for _, a := range result2.Assets {
		assetMap[assetKey(a)] = a
	}
	mergedAssets := make([]AssetResult, 0, len(assetMap))
	for _, a := range assetMap {
		mergedAssets = append(mergedAssets, a)
	}

	merged := &AnalysisResult{
		Episodes:   allEpisodes,
		Characters: mergedChars,
		Assets:     mergedAssets,
	}
	return merged, nil
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// analyzeChunk —— 调用 OpenAI API 分析单个文本块，返回结构化分析结果
func (c *openAIClient) analyzeChunk(ctx context.Context, text string) (*AnalysisResult, error) {
	content, err := c.completeText(ctx, systemPrompt, text)
	if err != nil {
		return nil, err
	}
	var result AnalysisResult
	if err := json.Unmarshal([]byte(sanitizeJSONContent(content)), &result); err != nil {
		return nil, fmt.Errorf("unmarshal llm result: %w, content: %s", err, content)
	}
	return &result, nil
}

func (c *openAIClient) completeText(ctx context.Context, system, user string) (string, error) {
	return c.completeTextWithModel(ctx, system, user, "")
}

func (c *openAIClient) completeTextWithModel(ctx context.Context, system, user, modelName string) (string, error) {
	// When a multi-channel pool is configured, round-robin across channels.
	if len(c.channels) > 0 {
		n := c.chanIdx.Add(1)
		ch := c.channels[int(n-1)%len(c.channels)]
		apiKey := ch.apiKey
		baseURL := ch.baseURL
		if strings.TrimSpace(modelName) == "" {
			modelName = ch.model
		}
		reqBody := openAIRequest{
			Model: modelName,
			Messages: []openAIMessage{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return "", fmt.Errorf("marshal request: %w", err)
		}
		var result string
		err = retryLLMCall(func() error {
			var callErr error
			result, callErr = c.doHTTPCompletion(ctx, bodyBytes, apiKey, baseURL)
			return callErr
		})
		return result, err
	}

	c.mu.RLock()
	apiKey := c.apiKey
	baseURL := c.baseURL
	if strings.TrimSpace(modelName) == "" {
		modelName = c.model
	}
	c.mu.RUnlock()

	reqBody := openAIRequest{
		Model: modelName,
		Messages: []openAIMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	var result string
	err = retryLLMCall(func() error {
		var callErr error
		result, callErr = c.doHTTPCompletion(ctx, bodyBytes, apiKey, baseURL)
		return callErr
	})
	return result, err
}

// doHTTPCompletion performs a single HTTP call to the LLM API. It is called inside
// retryLLMCall so the body bytes and headers are recreated on every attempt.
func (c *openAIClient) doHTTPCompletion(ctx context.Context, bodyBytes []byte, apiKey, baseURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", &httpStatusError{code: resp.StatusCode, body: string(respBytes)}
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBytes, &openAIResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if openAIResp.Error != nil {
		return "", fmt.Errorf("openai error: %s", openAIResp.Error.Message)
	}
	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return openAIResp.Choices[0].Message.Content, nil
}

// UpdateConfig hot-reloads the API key and base URL without restarting.
func (c *openAIClient) UpdateConfig(apiKey, baseURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if apiKey != "" {
		c.apiKey = apiKey
	}
	if baseURL != "" {
		c.baseURL = baseURL
	}
}

const characterExtractSystemPrompt = `You are a character extraction assistant. Extract all characters from the provided text and return a JSON array only, no other content.`

// ExtractCharacters calls the LLM to extract characters with appearance descriptions
// suitable for AI image generation, using a 30-second timeout.
func (c *openAIClient) ExtractCharacters(ctx context.Context, scriptText string) ([]CharacterExtractInfo, error) {
	prompt := fmt.Sprintf(`从以下小说/剧本文本中提取所有角色信息，返回JSON数组格式：
[{"name":"角色名","role_desc":"角色简介","appearance_desc":"外貌描述（用英文描述，包含发型/服装/体型/面部特征等，适合用于AI图像生成）"}]
只返回JSON，不要其他内容。
文本：%s`, scriptText)

	content, err := c.completeText(ctx, characterExtractSystemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("extract characters llm call: %w", err)
	}

	var result []CharacterExtractInfo
	if err := json.Unmarshal([]byte(sanitizeJSONContent(content)), &result); err != nil {
		return nil, fmt.Errorf("unmarshal character extract result: %w, content: %s", err, content)
	}
	return result, nil
}

func sanitizeJSONContent(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content)
}

// newSimpleOpenAIClient creates a lightweight openAIClient for a single provider
// (Claude / Qwen / Zhipu etc.) that share the OpenAI-compatible chat/completions API.
func newSimpleOpenAIClient(baseURL, apiKey, model string) LLMClient {
	if !strings.HasSuffix(strings.TrimRight(baseURL, "/"), "/v1") {
		baseURL = strings.TrimRight(baseURL, "/") + "/v1"
	}
	return &openAIClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 180 * time.Second},
	}
}

// fallbackLLMClient wraps multiple LLMClient instances and tries them in order.
// If the primary client fails (after its own internal retries), the next provider
// is tried automatically based on configured weight/priority.
type fallbackLLMClient struct {
	clients []LLMClient
	labels  []string // human-readable label for each provider, for logging
}

// NewFallbackLLMClient builds a prioritised fallback chain from config.
// Order: OpenAI (gpt-5.4) → Claude → Qwen → Zhipu.
// Providers with empty API key are skipped.
// Returns the single client directly when only one provider is configured.
func NewFallbackLLMClient(cfg *config.Config) LLMClient {
	var clients []LLMClient
	var labels []string

	// Primary: OpenAI / GPT
	if cfg.LLM.OpenAI.APIKey != "" {
		clients = append(clients, NewOpenAIClient(cfg))
		labels = append(labels, "openai:"+cfg.LLM.OpenAI.Model)
	}
	// Fallback 1: Claude (Anthropic messages API is OpenAI-compatible via proxy)
	if cfg.LLM.Claude.APIKey != "" {
		clients = append(clients, newSimpleOpenAIClient(cfg.LLM.Claude.BaseURL, cfg.LLM.Claude.APIKey, cfg.LLM.Claude.Model))
		labels = append(labels, "claude:"+cfg.LLM.Claude.Model)
	}
	// Fallback 2: Qwen (DashScope OpenAI-compatible endpoint)
	if cfg.LLM.Qwen.APIKey != "" {
		clients = append(clients, newSimpleOpenAIClient(cfg.LLM.Qwen.BaseURL, cfg.LLM.Qwen.APIKey, cfg.LLM.Qwen.Model))
		labels = append(labels, "qwen:"+cfg.LLM.Qwen.Model)
	}
	// Fallback 3: Zhipu GLM
	if cfg.LLM.Zhipu.APIKey != "" {
		clients = append(clients, newSimpleOpenAIClient(cfg.LLM.Zhipu.BaseURL, cfg.LLM.Zhipu.APIKey, cfg.LLM.Zhipu.Model))
		labels = append(labels, "zhipu:"+cfg.LLM.Zhipu.Model)
	}

	if len(clients) == 0 {
		// Fallback to bare OpenAI client even if key is empty (preserves old behaviour)
		return NewOpenAIClient(cfg)
	}
	if len(clients) == 1 {
		return clients[0]
	}
	return &fallbackLLMClient{clients: clients, labels: labels}
}

func (f *fallbackLLMClient) Analyze(ctx context.Context, scriptText string) (*AnalysisResult, error) {
	var lastErr error
	for i, c := range f.clients {
		result, err := c.Analyze(ctx, scriptText)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if i < len(f.clients)-1 {
			fmt.Printf("[llm-fallback] provider %s failed (%v), trying %s\n", f.labels[i], err, f.labels[i+1])
		}
		// Do not fall through on context cancellation
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("all llm providers failed, last error: %w", lastErr)
}

func (f *fallbackLLMClient) GenerateScript(ctx context.Context, req *ScriptGenerateReq) (*ScriptGenerateResult, error) {
	var lastErr error
	for i, c := range f.clients {
		result, err := c.GenerateScript(ctx, req)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if i < len(f.clients)-1 {
			fmt.Printf("[llm-fallback] provider %s failed (%v), trying %s\n", f.labels[i], err, f.labels[i+1])
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("all llm providers failed, last error: %w", lastErr)
}

func (f *fallbackLLMClient) ExtractCharacters(ctx context.Context, scriptText string) ([]CharacterExtractInfo, error) {
	var lastErr error
	for i, c := range f.clients {
		result, err := c.ExtractCharacters(ctx, scriptText)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if i < len(f.clients)-1 {
			fmt.Printf("[llm-fallback] provider %s failed (%v), trying %s\n", f.labels[i], err, f.labels[i+1])
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("all llm providers failed, last error: %w", lastErr)
}

// UpdateConfig updates the primary (first) provider's credentials only.
func (f *fallbackLLMClient) UpdateConfig(apiKey, baseURL string) {
	if len(f.clients) > 0 {
		f.clients[0].UpdateConfig(apiKey, baseURL)
	}
}
