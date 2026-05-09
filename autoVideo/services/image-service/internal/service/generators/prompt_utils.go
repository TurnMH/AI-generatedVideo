// Package generators — shared prompt utilities used by all image generators.
// Centralising these helpers means each generator can focus on its API protocol
// while prompt logic lives in one maintainable location.
package generators

import (
	"fmt"
	"regexp"
	"strings"
)

// weightedTagRE matches SDXL/ComfyUI style weighted tag syntax, e.g. (photorealistic:1.4).
var weightedTagRE = regexp.MustCompile(`\(([^()]+):\d+\.?\d*\)`)

// imagePromptSpec is the structured intermediate representation used to build
// model-family-specific prompts. The user prompt remains the source of truth,
// while the surrounding fields add task-specific framing such as composition,
// detail control, emotional intent, and layout constraints.
type imagePromptSpec struct {
	TaskType          string
	PrimaryBrief      string
	StyleContextEN    string
	StyleContextZH    string
	QualityFrameEN    string
	QualityFrameZH    string
	SubjectEN         string
	SubjectZH         string
	CompositionEN     string
	CompositionZH     string
	CameraEN          string
	CameraZH          string
	LightingEN        string
	LightingZH        string
	DetailEN          string
	DetailZH          string
	EmotionEN         string
	EmotionZH         string
	LayoutEN          string
	LayoutZH          string
	ReferenceEN       string
	ReferenceZH       string
	TaskNegativeEN    string
	TaskNegativeZH    string
	NegativeEN        string
	NegativeZH        string
	RawNegative       string
	StylePreset       string
	Width             int
	Height            int
	HasReferenceImage bool
}

// stripSDXLWeightedTags removes SDXL weight syntax from a prompt string so that
// natural-language models (GPT-4o, DALL-E 3, Flux) receive clean text.
// Example: "(photorealistic:1.4), RAW photo" → "photorealistic, RAW photo"
func stripSDXLWeightedTags(s string) string {
	s = weightedTagRE.ReplaceAllString(s, "$1")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// canonicalStyle normalises a style_preset string to one of four canonical keys.
// Mirrors the stylepreset package in character/video services without an import.
func canonicalStyle(stylePreset string) string {
	switch strings.TrimSpace(stylePreset) {
	case "", "anime-2d", "anime", "comic-dynamic", "guofeng-myth", "ink-poetry":
		return "anime-2d"
	case "anime-3d", "fantasy-dream":
		return "anime-3d"
	case "live-action-film", "cinematic-epic", "vintage-film", "sci-fi-neon", "suspense-dark":
		return "live-action-film"
	case "live-action-short", "realistic-drama", "fashion-commercial", "documentary-natural", "urban-romance", "warm-healing":
		return "live-action-short"
	default:
		return strings.TrimSpace(stylePreset)
	}
}

// isLiveActionStyle returns true when the preset maps to a live-action canonical key.
func isLiveActionStyle(stylePreset string) bool {
	c := canonicalStyle(stylePreset)
	return c == "live-action-film" || c == "live-action-short"
}

// buildImagePromptSpec converts a raw request into a structured prompt spec.
// The user prompt is preserved as the primary brief; additional fields are model-
// aware hints derived from style preset, frame size, task-type heuristics, and
// whether a reference image is present.
func NormalizeImageTaskType(taskType, prompt string) string {
	normalized := strings.ToLower(strings.TrimSpace(taskType))
	switch normalized {
	case "", "auto":
		return inferImageTaskType(prompt)
	case "portrait", "character-sheet", "character-view", "poster", "storyboard", "scene-concept", "general":
		return normalized
	case "character_sheet", "character sheet", "model-sheet", "model sheet", "reference-sheet", "reference sheet":
		return "character-sheet"
	case "scene_concept", "scene concept", "environment", "concept-art", "concept art":
		return "scene-concept"
	case "cover":
		return "poster"
	case "story-board", "story board", "shot":
		return "storyboard"
	case "character", "character-portrait", "character portrait", "hero-shot", "hero shot":
		return "portrait"
	default:
		return inferImageTaskType(prompt)
	}
}

func buildImagePromptSpec(req GenerateReq) imagePromptSpec {
	brief := strings.TrimSpace(stripSDXLWeightedTags(req.Prompt))
	neg := strings.TrimSpace(stripSDXLWeightedTags(req.NegativePrompt))
	taskType := NormalizeImageTaskType(req.TaskType, brief)
	hasRef := len(req.AllReferenceImageURLs()) > 0

	spec := imagePromptSpec{
		TaskType:          taskType,
		PrimaryBrief:      brief,
		StyleContextEN:    NaturalLangStyleContext(req.StylePreset),
		StyleContextZH:    ChineseLangStyleContext(req.StylePreset),
		NegativeEN:        NaturalLangNegativeInstruction(req.NegativePrompt),
		NegativeZH:        ChineseLangNegativeInstruction(req.NegativePrompt),
		RawNegative:       neg,
		StylePreset:       req.StylePreset,
		Width:             req.Width,
		Height:            req.Height,
		HasReferenceImage: hasRef,
	}

	if isLiveActionStyle(req.StylePreset) {
		spec.QualityFrameEN = "Photorealistic production reference image."
		spec.QualityFrameZH = "高质量真实摄影参考图。"
		spec.LightingEN = "Natural skin detail, grounded real-world materials, and believable cinematic light."
		spec.LightingZH = "真实肤感与材质细节，光线自然可信，具有电影感。"
	} else {
		spec.QualityFrameEN = "High-quality cinematic illustration."
		spec.QualityFrameZH = "高质量电影感插画。"
		spec.LightingEN = "Clear subject separation, polished color design, and stable visual hierarchy."
		spec.LightingZH = "主体层次清晰，色彩组织稳定，画面主次明确。"
	}

	spec.SubjectEN, spec.SubjectZH = subjectHints(taskType)
	spec.CompositionEN, spec.CompositionZH = compositionHints(req.Width, req.Height, taskType)
	spec.CameraEN, spec.CameraZH = cameraHints(req.Width, req.Height, taskType)
	spec.DetailEN, spec.DetailZH = detailHints(taskType, req.StylePreset)
	spec.EmotionEN, spec.EmotionZH = emotionHints(taskType)
	spec.LayoutEN, spec.LayoutZH = layoutHints(taskType)
	spec.TaskNegativeEN, spec.TaskNegativeZH = taskNegativeHints(taskType)
	if hasRef {
		hasCharSheet := isCharacterSheetRequest(req)
		spec.ReferenceEN = referenceHints(taskType, hasCharSheet)
		spec.ReferenceZH = referenceHintsZH(taskType, hasCharSheet)
	}

	return spec
}

// isCharacterSheetRequest reports whether the request references a 4-panel
// character turnaround sheet. It honours the explicit req.IsCharacterSheet
// flag and falls back to substring detection on the reference URL set.
func isCharacterSheetRequest(req GenerateReq) bool {
	if req.IsCharacterSheet {
		return true
	}
	return containsCharacterPanelSheet(req.AllReferenceImageURLs())
}

// containsCharacterPanelSheet reports whether any reference URL points to a
// character 4-panel composite (filename pattern "asset_*_composite.jpg" from
// character-service). When true, callers should tell the generator how to
// read the sheet (close-up / front / side / back of the same character) so
// the model doesn't treat the strip as four unrelated subjects.
func containsCharacterPanelSheet(urls []string) bool {
	for _, u := range urls {
		if strings.Contains(strings.ToLower(u), "_composite.") {
			return true
		}
	}
	return false
}

func inferImageTaskType(prompt string) string {
	p := strings.ToLower(strings.TrimSpace(prompt))
	switch {
	case containsAny(p,
		"character sheet", "reference sheet", "turnaround", "front view", "side view", "back view", "model sheet",
		"角色设定图", "设定图", "三视图", "四视图", "多视图", "正面", "侧面", "背面"):
		return "character-sheet"
	case containsAny(p,
		"portrait", "character portrait", "hero shot", "close-up portrait",
		"角色图", "角色立绘", "人物立绘", "人物特写", "肖像"):
		return "portrait"
	case containsAny(p,
		"poster", "cover", "title", "headline", "typography", "infographic",
		"海报", "封面", "标题", "版式", "排版"):
		return "poster"
	case containsAny(p,
		"storyboard", "shot", "scene", "cinematic still", "film still", "frame",
		"分镜", "镜头", "电影静帧", "单帧"):
		return "storyboard"
	case containsAny(p,
		"concept art", "environment", "landscape", "establishing shot", "scene concept",
		"概念图", "场景图", "环境概念", "环境设计", "地貌", "建筑外景"):
		return "scene-concept"
	default:
		return "general"
	}
}

func subjectHints(taskType string) (string, string) {
	switch taskType {
	case "portrait":
		return "Prioritise a single clearly readable character with strong facial presence and clean silhouette.", "优先突出单一角色主体，确保面部辨识度高、人物轮廓清晰。"
	case "character-view":
		return "Generate a single full-body character; include the complete figure from head to toe with a clean, readable silhouette.", "生成单一全身角色，头顶至脚尖完整入画，人物轮廓清晰可读。"
	case "character-sheet":
		return "Treat the same character as the only subject across all views; preserve body proportion and costume consistency.", "将同一角色作为唯一主体，所有视图保持体型比例与服装一致。"
	case "scene-concept":
		return "Prioritise environment readability; if people appear, they should support scale rather than dominate the frame.", "优先保证环境主体可读性；如有人物，仅用于辅助尺度，不主导画面。"
	case "storyboard":
		return "Capture one decisive story beat with a clearly staged subject and readable action intent.", "聚焦一个明确叙事瞬间，主体调度清楚，动作意图可读。"
	case "poster":
		return "Build around one dominant hero subject with immediate first-glance readability.", "围绕一个主视觉核心主体组织画面，确保第一眼识别明确。"
	default:
		return "Keep the main subject dominant and visually unambiguous.", "保持主要主体主导且识别明确。"
	}
}

func compositionHints(width, height int, taskType string) (string, string) {
	shape := "square"
	switch {
	case width > height:
		shape = "landscape"
	case height > width:
		shape = "portrait"
	}

	switch taskType {
	case "portrait":
		if shape == "portrait" {
			return "Use a portrait-led composition with a single dominant character, clear face visibility, and limited background distraction.", "采用竖幅肖像构图，突出单一角色主体，保证脸部清晰且背景干扰少。"
		}
		return "Use a character-focused composition with one dominant subject, readable facial detail, and a clean supporting background.", "采用人物主体导向构图，确保单一角色占主导，面部细节清晰，背景只作辅助。"
	case "character-view":
		return "Use a neutral full-body vertical composition: keep the character centered with both head and feet inside the frame with a small margin at top and bottom.", "采用中性全身竖幅构图：角色居中，头顶与脚尖均完整入画并保留小边距。"
	case "character-sheet":
		return "Use a clean layout with explicit panel separation and consistent scale across all views.", "采用清晰分栏布局，各视图比例保持一致，栏位边界明确。"
	case "poster":
		return "Use a bold poster composition with a strong focal subject and deliberate negative space reserved for layout clarity.", "采用海报式构图，主体视觉中心明确，并保留有组织的留白区域。"
	case "storyboard":
		if shape == "landscape" {
			return "Compose as a cinematic storyboard frame with strong foreground-midground-background depth.", "按电影分镜单帧构图，强化前景、中景、背景的景深层次。"
		}
		return "Compose as a dramatic storyboard frame with a clear focal subject and readable scene hierarchy.", "按戏剧化分镜单帧构图，主体聚焦明确，场景层级清楚。"
	case "scene-concept":
		return "Prioritise environment readability, spatial depth, and atmospheric scale.", "优先表现环境可读性、空间纵深与氛围尺度。"
	default:
		switch shape {
		case "landscape":
			return "Use a wide cinematic composition with layered depth and stable scene readability.", "采用宽银幕式构图，强化纵深层次与画面可读性。"
		case "portrait":
			return "Use a vertical composition with a strong primary subject and clean silhouette separation.", "采用纵向构图，突出主要主体并保持轮廓清晰。"
		default:
			return "Use a balanced composition with a clear focal subject and clean spatial organization.", "采用平衡构图，主体焦点清晰，空间组织干净。"
		}
	}
}

func cameraHints(width, height int, taskType string) (string, string) {
	switch taskType {
	case "portrait":
		return "Use portrait-friendly framing that keeps the face, upper body, and pose readable without perspective distortion.", "采用适合人物展示的取景，保证脸部、上半身和姿态可读，避免透视畸变。"
	case "character-view":
		return "Use a neutral orthographic-style camera at eye level; avoid dramatic perspective, fisheye distortion, or tilted horizon.", "采用平视、接近正交投影的中性机位，避免夸张透视、鱼眼畸变或倾斜水平线。"
	case "character-sheet":
		return "Keep the camera neutral and orthographic-looking; avoid dramatic distortion or extreme perspective.", "机位保持中性、接近平视设定图效果，避免夸张透视和镜头畸变。"
	case "poster":
		return "Use deliberate framing with poster readability in mind; keep the main subject immediately legible.", "镜头取景服务于海报可读性，主视觉主体一眼可辨。"
	case "storyboard":
		return "Treat this as a film still with readable camera blocking and a clearly staged subject.", "按电影静帧处理，机位关系明确，人物调度清晰。"
	case "scene-concept":
		return "Choose a camera angle that reveals environment scale and spatial relationships.", "选择能表现环境尺度与空间关系的机位。"
	default:
		if width > height {
			return "Use cinematic framing with stable perspective and readable scene blocking.", "采用电影式取景，透视稳定，场面调度可读。"
		}
		return "Use clean framing that keeps the subject dominant and visually coherent.", "采用干净取景，保持主体主导和画面统一。"
	}
}

func detailHints(taskType, stylePreset string) (string, string) {
	if isLiveActionStyle(stylePreset) {
		switch taskType {
		case "portrait":
			return "Pay special attention to facial structure, hairline, skin texture, fabric detail, and hand cleanliness.", "重点保证面部结构、发际线、肤感、服装材质和手部细节自然稳定。"
		case "character-view":
			return "Ensure footwear, clothing hemline, accessories, hand proportions, and full-body silhouette are all clearly readable.", "保证鞋履、服装下摆、配饰、手部比例与全身轮廓均清晰可读。"
		case "character-sheet":
			return "Keep costume seams, accessories, footwear, and body proportions identical across every view.", "各视图中的服装缝线、配饰、鞋履和人体比例必须保持一致。"
		case "scene-concept":
			return "Emphasise architecture, terrain, atmosphere, material realism, and spatial depth cues.", "强化建筑、地貌、空气感、材质写实度和空间纵深线索。"
		default:
			return "Maintain stable material detail, anatomy, and edge clarity.", "保持材质细节、人体结构和画面边缘清晰稳定。"
		}
	}

	switch taskType {
	case "portrait":
		return "Pay special attention to face design, hairstyle shape, costume trim, and polished finishing detail.", "重点加强角色五官设计、发型轮廓、服装装饰和完成度。"
	case "character-view":
		return "Ensure costume shape, footwear, accessories, and full-body line quality are clean and consistent.", "保证服装造型、鞋履、配饰与全身线条质量干净一致。"
	case "character-sheet":
		return "Keep line quality, costume shape language, and color blocking consistent across every panel.", "所有分栏保持线条质量、服装造型语言和色块关系一致。"
	case "scene-concept":
		return "Emphasise environmental design, atmosphere, scale, and layered world-building detail.", "强化环境设计、氛围、尺度感和分层世界观细节。"
	default:
		return "Maintain stable design language, clean detailing, and polished finish.", "保持设计语言稳定、细节干净、完成度高。"
	}
}

func emotionHints(taskType string) (string, string) {
	switch taskType {
	case "portrait":
		return "The expression should be intentional, readable, and emotionally coherent.", "表情必须明确、可读，并与角色情绪一致。"
	case "storyboard":
		return "The emotional beat should be readable immediately from pose, expression, and staging.", "需要通过姿态、表情和调度快速传达当前情绪点。"
	case "poster":
		return "The mood should be bold, memorable, and suitable for promotional key art.", "情绪氛围要强烈、鲜明，适合作为宣传主视觉。"
	default:
		return "", ""
	}
}

func layoutHints(taskType string) (string, string) {
	switch taskType {
	case "character-view":
		return "Use a clean minimal background that keeps all attention on the character silhouette.", "使用简洁极简背景，将所有注意力集中于角色轮廓。"
	case "character-sheet":
		return "Panel order and structure must remain explicit and unambiguous.", "栏位顺序与结构必须明确，不可混乱或合并。"
	case "poster":
		return "Preserve clean layout hierarchy suitable for title and cover presentation.", "保持适合标题与封面展示的清晰版式层级。"
	case "portrait":
		return "Keep the frame clean and avoid unnecessary secondary elements competing with the character.", "保持画面干净，避免无关次要元素抢夺人物主体。"
	default:
		return "", ""
	}
}

func referenceHints(taskType string, hasCharacterSheet bool) string {
	base := ""
	switch taskType {
	case "character-sheet":
		base = "Maintain the exact same character identity, body proportion, costume structure, hairstyle, and color blocking across every view and panel."
	case "character-view":
		base = "Match the character identity from the reference image: replicate the facial structure, hairstyle, skin tone, costume design, and overall body proportions."
	case "portrait":
		base = "Maintain the same character identity, facial structure, hairstyle, costume language, and overall visual continuity as the reference image."
	default:
		base = "Maintain the same character identity, facial structure, hairstyle, costume language, and overall visual continuity as the reference image."
	}
	if hasCharacterSheet {
		return base + " The provided character reference is a 4-panel reference sheet of ONE single character, arranged left-to-right as: (1) head-and-upper-body close-up with clear face (use this for facial features, eyes, skin tone, hairstyle front); (2) front full-body (use this for body proportion, costume front, pose silhouette); (3) side full-body profile (use this for body depth, costume side, hair and accessory silhouette); (4) back full-body (use this for hairstyle back, costume back, and rear silhouette). Treat all 4 panels as the SAME person and reconstruct that person accordingly — do NOT treat them as four different characters and do NOT stitch multiple people into the output."
	}
	return base
}

func referenceHintsZH(taskType string, hasCharacterSheet bool) string {
	base := ""
	switch taskType {
	case "character-sheet":
		base = "所有分栏都必须保持同一角色身份一致，体型比例、服装结构、发型和主色块关系不得漂移。"
	case "character-view":
		base = "以参考图为基础还原角色身份：精准匹配面部结构、发型、肤色、服装设计与全身体型比例。"
	case "portrait":
		base = "保持与参考图一致的人物身份、面部结构、发型、服装语言与整体视觉连续性。"
	default:
		base = "保持与参考图一致的人物身份、面部结构、发型、服装语言与整体视觉连续性。"
	}
	if hasCharacterSheet {
		return base + " 所提供的角色参考图是同一个人物的四视图合成参考表，从左到右依次为：①头部与上半身特写（用于识别五官、眼睛、肤色、发型前方）；②全身正面（用于体型比例、服装正面、站姿轮廓）；③全身正侧面（用于身体厚度、服装侧面、发型与配饰轮廓）；④全身背面（用于发型后方、服装背面、整体背影）。必须把这四栏理解为同一个人物，还原同一个角色——禁止把四栏当成四个不同的人，也禁止把多个人拼到输出里。"
	}
	return base
}

func taskNegativeHints(taskType string) (string, string) {
	switch taskType {
	case "portrait":
		return "Avoid extra people, duplicate faces, cropped foreheads, broken hands, distracting props, and cluttered backgrounds.", "避免多人抢主体、重复面孔、额头裁切、手部错误、干扰性道具和杂乱背景。"
	case "character-view":
		return "Avoid cropped feet, cropped head, partial body, multiple people, panel grid layouts, extreme perspective distortion, and cluttered backgrounds.", "避免裁切脚部、裁切头部、不完整身体、多人、多格画面布局、极端透视和背景杂乱。"
	case "character-sheet":
		return "Avoid merged panels, missing views, inconsistent outfit details, pose drift between views, and dramatic perspective distortion.", "避免分栏合并、缺少视图、服装细节漂移、各视图姿态不一致以及夸张透视。"
	case "poster":
		return "Avoid overcrowded backgrounds, weak focal hierarchy, unreadable title area, and fragmented composition.", "避免背景过满、主次不清、标题区不可用以及构图碎裂。"
	case "storyboard":
		return "Avoid poster-like posing, glamour-shot framing, unclear action, and unreadable story blocking.", "避免海报摆拍感、写真式取景、动作含义不明以及叙事调度混乱。"
	case "scene-concept":
		return "Avoid oversized foreground characters, flattened depth, random props, and environment scale confusion.", "避免前景人物过大、空间扁平、道具堆砌和环境尺度混乱。"
	default:
		return "", ""
	}
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func joinSentences(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return strings.Join(filtered, " ")
}

func joinChineseSentences(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		trimmed = strings.TrimSuffix(trimmed, "。")
		trimmed = strings.TrimSuffix(trimmed, "，")
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	if len(filtered) == 0 {
		return ""
	}
	return strings.Join(filtered, "。") + "。"
}

func buildNaturalLanguagePrompt(req GenerateReq) string {
	spec := buildImagePromptSpec(req)
	return joinSentences(
		spec.QualityFrameEN,
		spec.StyleContextEN,
		spec.SubjectEN,
		"Primary subject and scene:", spec.PrimaryBrief,
		spec.CompositionEN,
		spec.CameraEN,
		spec.LightingEN,
		spec.DetailEN,
		spec.EmotionEN,
		spec.LayoutEN,
		spec.ReferenceEN,
		spec.TaskNegativeEN,
		spec.NegativeEN,
	)
}

func buildChineseStructuredPrompt(req GenerateReq, styleHandledExternally bool) string {
	spec := buildImagePromptSpec(req)
	stylePart := spec.StyleContextZH
	if styleHandledExternally {
		stylePart = ""
	}
	return joinChineseSentences(
		spec.QualityFrameZH,
		stylePart,
		func() string {
			if spec.SubjectZH == "" {
				return ""
			}
			return fmt.Sprintf("主体控制：%s", spec.SubjectZH)
		}(),
		fmt.Sprintf("画面主体与场景：%s", spec.PrimaryBrief),
		fmt.Sprintf("镜头构图：%s", spec.CompositionZH),
		fmt.Sprintf("机位取景：%s", spec.CameraZH),
		fmt.Sprintf("光线与画面控制：%s", spec.LightingZH),
		func() string {
			if spec.DetailZH == "" {
				return ""
			}
			return fmt.Sprintf("细节要求：%s", spec.DetailZH)
		}(),
		func() string {
			if spec.EmotionZH == "" {
				return ""
			}
			return fmt.Sprintf("情绪表达：%s", spec.EmotionZH)
		}(),
		func() string {
			if spec.LayoutZH == "" {
				return ""
			}
			return fmt.Sprintf("版式要求：%s", spec.LayoutZH)
		}(),
		func() string {
			if spec.ReferenceZH == "" {
				return ""
			}
			return fmt.Sprintf("一致性要求：%s", spec.ReferenceZH)
		}(),
		func() string {
			if spec.TaskNegativeZH == "" {
				return ""
			}
			return fmt.Sprintf("任务负向限制：%s", spec.TaskNegativeZH)
		}(),
	)
}

func buildSDXLPositivePrompt(req GenerateReq) string {
	spec := buildImagePromptSpec(req)
	parts := []string{}
	if styleTags := SDXLStyleTags(req.StylePreset); styleTags != "" {
		parts = append(parts, styleTags)
	}
	if taskTags := sdxlTaskTags(spec.TaskType); taskTags != "" {
		parts = append(parts, taskTags)
	}
	parts = append(parts, spec.PrimaryBrief)
	if comp := sdxlCompositionTags(spec.Width, spec.Height, spec.TaskType); comp != "" {
		parts = append(parts, comp)
	}
	if spec.HasReferenceImage {
		if isCharacterSheetRequest(req) {
			// Compact SDXL-friendly description of the 4-panel reference sheet
			// so CLIP-text models don't treat the strip as four different
			// people. Keeps the wording dense to stay within the 77-token
			// soft limit that SDXL text encoders enforce per chunk.
			parts = append(parts,
				"(single character from 4-panel reference sheet:1.3)",
				"reference sheet shows ONE person as closeup, front, side, back",
				"(same face, same hairstyle, same costume across all panels:1.3)",
				"reconstruct the SAME person, not multiple people",
			)
		} else {
			parts = append(parts, "same character identity, consistent face, consistent costume, consistent hairstyle")
		}
	}
	return joinPromptParts(parts...)
}

func buildSDXLNegativePrompt(req GenerateReq) string {
	spec := buildImagePromptSpec(req)
	parts := []string{}
	if custom := strings.TrimSpace(req.NegativePrompt); custom != "" {
		parts = append(parts, custom)
	}
	if taskTags := sdxlTaskNegativeTags(spec.TaskType); taskTags != "" {
		parts = append(parts, taskTags)
	}
	if styleTags := SDXLNegativeTags(req.StylePreset); styleTags != "" {
		parts = append(parts, styleTags)
	}
	if spec.HasReferenceImage && isCharacterSheetRequest(req) {
		// When the reference is a 4-panel sheet, SDXL tends to literally paint
		// four side-by-side characters. Push back explicitly.
		parts = append(parts, "multiple people, duplicate characters, four figures, side-by-side characters, split panels, collage layout")
	}
	return joinPromptParts(parts...)
}

func sdxlTaskNegativeTags(taskType string) string {
	switch taskType {
	case "portrait":
		return "multiple people, duplicate face, extra face, cropped forehead, bad hands, cluttered background"
	case "character-view":
		return "cropped feet, cropped head, partial body, multiple people, panel grid, dramatic perspective, cluttered background"
	case "character-sheet":
		return "merged panels, missing view, inconsistent outfit, inconsistent hairstyle, dramatic perspective, cropped body"
	case "poster":
		return "busy background, weak focal point, unreadable layout, text covering subject, fragmented composition"
	case "storyboard":
		return "poster pose, glamour shot, unclear action, unreadable blocking, fashion editorial framing"
	case "scene-concept":
		return "oversized character, flat depth, random prop clutter, unclear environment scale"
	default:
		return ""
	}
}

func sdxlTaskTags(taskType string) string {
	switch taskType {
	case "portrait":
		return "character portrait, single subject, face focus, clean silhouette, polished detail"
	case "character-view":
		return "character full-body view, single subject, head-to-toe figure, clean silhouette, neutral orthographic framing"
	case "character-sheet":
		return "character reference sheet, model sheet, explicit panel separation, consistent scale across views"
	case "poster":
		return "poster composition, strong focal point, clean layout hierarchy"
	case "storyboard":
		return "cinematic still, storyboard frame, scene readability, staged blocking"
	case "scene-concept":
		return "environment concept art, atmospheric perspective, layered depth"
	default:
		return ""
	}
}

func sdxlCompositionTags(width, height int, taskType string) string {
	switch taskType {
	case "portrait":
		return "single dominant character, face emphasis, readable pose, clean background separation"
	case "character-view":
		return "full-body centered, neutral camera, head-to-toe framing, minimal background"
	case "character-sheet":
		return "neutral camera, no extreme perspective, full layout readability"
	case "poster":
		return "clear focal subject, readable silhouette, controlled negative space"
	}
	switch {
	case width > height:
		return "wide cinematic composition, foreground midground background separation"
	case height > width:
		return "vertical composition, strong subject silhouette, centered framing"
	default:
		return "balanced composition, clean spatial organization"
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Style context strings — consumed by DALL-E and Flux which use prose prompts.
// ─────────────────────────────────────────────────────────────────────────────

// NaturalLangStyleContext returns a concise English style description suitable
// for prepending to a GPT-family or Flux prompt.
func NaturalLangStyleContext(stylePreset string) string {
	switch canonicalStyle(stylePreset) {
	case "live-action-film":
		return "Cinematic film photography. Dramatic professional lighting, authentic real-world environment, rich color grading, film-grain texture, anamorphic lens look."
	case "live-action-short":
		return "Realistic short drama photography. Natural close-up framing, warm authentic skin tones, everyday realism, handheld feel."
	case "anime-2d":
		return "2D Japanese anime illustration style. Clean line art, vibrant cel-shaded colors, expressive character design."
	case "anime-3d":
		return "3D anime CG render style. Toon-shaded volumetric depth, stylized materials, dimensional characters."
	default:
		return ""
	}
}

// NaturalLangNegativeInstruction converts a negative-prompt tag string into a
// natural-language instruction suitable for GPT/Flux models.
func NaturalLangNegativeInstruction(negativePrompt string) string {
	cleaned := stripSDXLWeightedTags(negativePrompt)
	if cleaned == "" {
		return ""
	}
	return "Do not include: " + cleaned + "."
}

// ─────────────────────────────────────────────────────────────────────────────
// SDXL tag-style style helpers — used by SDXL/ComfyUI which is tag-based.
// ─────────────────────────────────────────────────────────────────────────────

// SDXLStyleTags returns leading quality/style tags for the given preset.
// These are prepended to the user prompt so they receive the highest attention weight.
func SDXLStyleTags(stylePreset string) string {
	switch canonicalStyle(stylePreset) {
	case "live-action-film":
		return strings.Join([]string{
			"(RAW photo:1.4)", "(photorealistic:1.5)", "(hyperrealistic:1.3)",
			"DSLR", "8K UHD", "sharp focus", "film grain",
			"cinematic color grade", "dramatic lighting", "anamorphic lens look",
			"premium production values", "no cartoon", "no anime",
		}, ", ")
	case "live-action-short":
		return strings.Join([]string{
			"(RAW photo:1.3)", "(photorealistic:1.4)",
			"DSLR", "8K UHD", "sharp focus",
			"natural skin texture", "warm tones",
			"realistic close-up framing", "no cartoon", "no anime",
		}, ", ")
	case "anime-2d":
		return "(masterpiece:1.4), (best quality:1.3), 2D anime illustration, clean line art, cel shading, vibrant colors, expressive characters, cinematic lighting"
	case "anime-3d":
		return "(masterpiece:1.4), (best quality:1.3), 3D anime render, toon shading, volumetric depth, stylized materials, CG cinematic lighting"
	default:
		if t := strings.TrimSpace(stylePreset); t != "" {
			return t + " style"
		}
		return ""
	}
}

// SDXLNegativeTags returns negative-prompt tags appropriate for the given preset.
func SDXLNegativeTags(stylePreset string) string {
	base := "lowres, blurry, out of focus, jpeg artifacts, noisy, text, watermark, logo, signature, cropped"
	switch canonicalStyle(stylePreset) {
	case "anime-2d":
		return base + ", photorealistic skin, CGI realism, plastic skin, 3D render, live-action photo"
	case "anime-3d":
		return base + ", flat 2D lineart, photorealistic live action, hand-drawn sketch"
	case "live-action-film":
		return base + ", anime, cartoon, illustration, cel shading, chibi, comic book, painted artwork, " +
			"plastic CGI skin, airbrushed skin, skin smoothing, video game render, toon shading, " +
			"stylized proportions, bad anatomy, deformed face, warped limbs"
	case "live-action-short":
		return base + ", anime, cartoon, illustration, cel shading, " +
			"plastic CGI skin, airbrushed skin, skin smoothing, video game render, " +
			"bad anatomy, deformed face"
	default:
		return base
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Chinese-language style helpers — used by CogView (ZhipuAI) and Tongyi wanx2.1.
// ─────────────────────────────────────────────────────────────────────────────

// ChineseLangStyleContext returns a Chinese-language style description for
// models trained primarily on Chinese content (CogView, Tongyi wanx2.1).
func ChineseLangStyleContext(stylePreset string) string {
	switch canonicalStyle(stylePreset) {
	case "live-action-film":
		return "电影级真实摄影，专业三点布光，ARRI摄影机胶片质感，真实肤感，景深效果，电影色彩分级，高品质制作"
	case "live-action-short":
		return "真人短剧写实风格，自然光影，近景表演，真实肤色，日常生活氛围，情感细腻"
	case "anime-2d":
		return "二维日式动漫插画风格，简洁手绘线条，鲜艳赛璐璐配色，表情丰富，角色造型清晰"
	case "anime-3d":
		return "三维动漫CG渲染风格，卡通质感材质，立体角色，景深效果，流畅运动弧线"
	default:
		return ""
	}
}

// ChineseLangNegativeInstruction converts a negative prompt to a Chinese "请勿包含" instruction.
func ChineseLangNegativeInstruction(negativePrompt string) string {
	cleaned := stripSDXLWeightedTags(negativePrompt)
	if cleaned == "" {
		return ""
	}
	return "请勿包含：" + cleaned + "。"
}

// ChineseLangNegativeByStyle returns a fully Chinese negative instruction appropriate for
// the given style preset. Used for Chinese-first models (CogView, Tongyi wanx) where a
// pure Chinese negative prompt is significantly more effective than an English one.
func ChineseLangNegativeByStyle(stylePreset string) string {
	switch canonicalStyle(stylePreset) {
	case "live-action-film":
		return "请勿包含：动漫风格、卡通风格、插画风格、赛璐璐着色、漫画画风、二次元角色、Q版造型、CG渲染皮肤、塑料质感皮肤、磨皮滤镜、过度修图、电子游戏画风、卡通着色、夸张比例、画面变形、低画质、模糊、噪点、水印、文字。"
	case "live-action-short":
		return "请勿包含：动漫风格、卡通风格、插画风格、赛璐璐着色、漫画画风、二次元角色、Q版造型、CG渲染皮肤、塑料质感皮肤、磨皮滤镜、过度修图、电子游戏画风、低画质、模糊、噪点、水印、文字。"
	case "anime-2d":
		return "请勿包含：写实摄影、照片质感、3D渲染皮肤、CGI真实感、真人拍摄风格、低画质、模糊、噪点、水印、文字。"
	case "anime-3d":
		return "请勿包含：平面手绘线稿、写实摄影、照片质感、真人拍摄风格、低画质、模糊、噪点、水印、文字。"
	default:
		return "请勿包含：低画质、模糊、噪点、水印、文字。"
	}
}
