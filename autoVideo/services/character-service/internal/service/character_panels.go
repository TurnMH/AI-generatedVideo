package service

import (
	"fmt"
	"strings"

	"github.com/autovideo/character-service/internal/stylepreset"
)

// CharacterPanel 代表角色四视图中的单个分栏。
// 新策略：将原先的"一张图含四视图"拆为 4 次独立生成 + 服务端横向拼接，
// 避免不同模型对"1x4 网格"的理解差异（常见失败：2x2、单姿势、缺特写）。
type CharacterPanel string

const (
	CharacterPanelCloseup CharacterPanel = "closeup"
	CharacterPanelFront   CharacterPanel = "front"
	CharacterPanelSide    CharacterPanel = "side"
	CharacterPanelBack    CharacterPanel = "back"
)

// characterPanelOrder 固定顺序：front → closeup → side → back，
// 左到右拼接时也沿用此顺序；Doubao 串行路径会先单独生成 closeup 作为参考图，但存储顺序以此为准。
var characterPanelOrder = []CharacterPanel{
	CharacterPanelFront,
	CharacterPanelCloseup,
	CharacterPanelSide,
	CharacterPanelBack,
}

// CharacterPanelOrder 对外暴露只读顺序副本。
func CharacterPanelOrder() []CharacterPanel {
	out := make([]CharacterPanel, len(characterPanelOrder))
	copy(out, characterPanelOrder)
	return out
}

// parseCharacterPanel 归一化字符串到枚举，未知值返回空字符串。
func parseCharacterPanel(v string) CharacterPanel {
	switch CharacterPanel(strings.ToLower(strings.TrimSpace(v))) {
	case CharacterPanelCloseup, CharacterPanelFront, CharacterPanelSide, CharacterPanelBack:
		return CharacterPanel(strings.ToLower(strings.TrimSpace(v)))
	}
	return ""
}

// panelImageAspect 返回每个分栏推荐的宽高。
// 所有分栏统一 512×768（3:4 竖幅），保证合成图纵向对齐且前端 2×2 网格不变形。
func panelImageAspect(_ CharacterPanel) (width, height int) {
	return 512, 768
}

// panelFramingEN/CN 描述给生成模型的视角关键词。
// 关键：共享身份段 + panel-specific framing；删除所有 "four panels / 1x4 grid" 字样。
func panelFramingEN(panel CharacterPanel) string {
	switch panel {
	case CharacterPanelCloseup:
		return "(pure white background:1.6), plain white seamless studio backdrop, (extra high-definition facial close-up portrait:1.7), (head-and-shoulders close-up portrait:1.6), framed from head to upper-chest, face perfectly centered in frame, fully facing camera, (identity-focused hero portrait:1.5), (sharp clear facial features:1.5), (eyes, eyebrows, nose, mouth clearly visible and in sharp focus:1.4), ultra-detailed eyes and skin texture, natural neutral expression, face fills roughly 65% to 75% of the frame height, no sunglasses, no face occlusion, no hair covering the eyes, clean character reference sheet style, studio beauty lighting, ultra sharp focus"
	case CharacterPanelFront:
		return "(pure white background:1.6), plain white seamless studio backdrop, full-body front view, head to toe fully visible, standing in a relaxed neutral A-pose, fully facing camera, both feet visible and touching ground, (consistent clothing details from head to toe:1.3), character turnaround sheet style, objective neutral depiction, no cropping of head or feet, entire body inside the frame with small margin"
	case CharacterPanelSide:
		return "(pure white background:1.6), plain white seamless studio backdrop, (full-body side profile view:1.5), (exact 90-degree side silhouette:1.5), (strict profile, not 3/4 view:1.4), (looking entirely to the side:1.4), (no eye contact with camera:1.5), head to toe fully visible, facing left, both feet visible and touching ground, hairstyle and costume silhouette clearly readable"
	case CharacterPanelBack:
		return "(pure white background:1.6), plain white seamless studio backdrop, (full-body back view, seen directly from behind:1.5), (fully facing away from camera:1.5), (no face visible:1.5), head to toe fully visible, both feet visible and touching ground, hairstyle and back of outfit clearly visible, no face visible"
	}
	return ""
}

func panelFramingCN(panel CharacterPanel) string {
	switch panel {
	case CharacterPanelCloseup:
		return "(纯白背景:1.6)，白色无缝棚拍背景，人物形象高清特写，头部及肩部特写（头顶至锁骨），面部完美居中入画，完全正脸朝向镜头，以角色身份识别为核心，五官（眼睛、眉毛、鼻子、嘴巴）必须清晰锐利、可辨认，强调眼部细节、皮肤质感与发丝细节，自然中性表情，脸部约占画面高度65%~75%，不戴墨镜，不遮挡面部，刘海不遮眼，角色设定参考图风格，棚拍级美型布光，整体超清锐利"
	case CharacterPanelFront:
		return "(纯白背景:1.6)，白色无缝棚拍背景，全身正面照，头到脚完整入画，自然站立放松 A 姿势，完全正面朝向镜头，双脚落地可见，头顶与脚底都留有小边距，严禁裁切头部或脚部"
	case CharacterPanelSide:
		return "(纯白背景:1.6)，白色无缝棚拍背景，(全身严格正侧面照:1.5)，(完全90°侧面轮廓:1.5)，(不准看镜头:1.5)，角色设定图风格，头到脚完整入画，面朝画面左侧，双脚落地可见，发型与服装侧面轮廓清晰"
	case CharacterPanelBack:
		return "(纯白背景:1.6)，白色无缝棚拍背景，(全身正背面照:1.5)，(完全背对镜头:1.5)，(绝对不能出现人脸:1.5)，角色设定图风格，头到脚完整入画，双脚落地可见，清晰展示发型与背部服饰，不得出现面部"
	}
	return ""
}

// composeCharacterPanelPrompt 为单个分栏组装完整提示词。
// 与原 composeCharacterPrompt 的主要区别：
//   - 不再出现 "four panels / 1x4 grid" 类布局术语（由后端拼接完成）。
//   - 使用共享身份段 + 固定 "identical subject" 锚点，让 4 栏指向同一角色。
//   - 分 live-action / anime 两套；live-action 强调真人质感，anime 强调线条干净。
func composeCharacterPanelPrompt(name, description, promptUsed string, isLiveAction bool, panel CharacterPanel) string {
	trimmedName := strings.TrimSpace(name)
	trimmedDescription := strings.TrimSpace(description)
	trimmedPrompt := strings.TrimSpace(promptUsed)

	framingEN := panelFramingEN(panel)
	framingCN := panelFramingCN(panel)

	identity := []string{}
	if trimmedName != "" {
		identity = append(identity, trimmedName)
	}
	identity = append(identity,
		"the same single character across all reference panels",
		"(identical subject, identical face shape, identical hairstyle, identical skin tone, identical costume, identical proportions:1.4)",
	)

	tags := []string{}
	tags = append(tags, identity...)

	if isLiveAction {
		tags = append(tags,
			// Panel-specific framing
			framingEN, framingCN,
			// Single-subject constraint
			"(single character in frame:1.5)", "(one person only:1.5)", "画面中只有一个人物",
		// Background — natural-language form for GPT-Image/Gemini, plus SDXL weights for diffusion models
		"The background must be a completely plain solid white studio backdrop (#FFFFFF). There must be no color tints, no gradients, no shadows cast onto the background, no textures, no environmental elements, and absolutely no scene behind the character.",
		"背景必须是纯白色（#FFFFFF）的白色无缝棚拍背景，不允许出现任何颜色、渐变、投影、纹理、环境元素或场景背景。",
		"pure white seamless studio backdrop", "(no background clutter:1.5)", "(pure white background:1.8)",
			// Lighting
			"professional three-point studio lighting",
			"soft-box key light at 45° above", "fill light from opposite side", "rim/separation light from behind",
			"catch light visible in eyes",
			// Realism
			"RAW photo", "真实摄影", "电影级RAW原片", "(photorealistic:1.4)", "natural skin texture", "visible skin pores", "subsurface scattering on skin",
			"no airbrushing", "no plastic CGI skin", "no skin smoothing",
			"authentic hair strands individually visible", "fabric weave and material texture clearly readable",
			// Anatomy
			"correct human proportions", "no deformed hands", "no extra fingers",
			// Resolution
			"8K UHD", "8k分辨率", "ultra-detailed", "tack-sharp focus",
			// Cleanup
			"(no text:2.0)", "no watermarks", "no labels", "no annotations", "no panel numbers",
		)
	} else {
		tags = append(tags,
			// Panel-specific framing
			framingEN, framingCN,
			// Single-subject constraint
			"(single character in frame:1.5)", "(one person only:1.5)", "画面中只有一个人物",
			// Style tokens
			"(masterpiece:1.5)", "best quality", "ultra-detailed", "highly detailed illustration",
			"2D anime illustration", "anime art style", "clean line art",
		// Background — natural-language form for GPT-Image/Gemini, plus SDXL weights for diffusion models
		"The background must be completely plain white with absolutely no color, no gradient, no texture, and no background elements of any kind.",
		"背景必须是纯白色，不得有任何颜色、渐变、纹理或背景元素。",
		"pure white background", "white seamless backdrop", "(no background:1.5)", "(pure white background:1.8)",
			"(no text:2.0)", "no watermarks", "no annotations", "no labels",
			// Anatomy
			"correct human proportions", "detailed fabric texture",
		)
	}

	if trimmedDescription != "" {
		tags = append(tags, trimmedDescription)
	}
	if trimmedPrompt != "" {
		tags = append(tags, trimmedPrompt)
	}

	// Prepend a bilingual imperative instruction block.
	// Tag-list format is ignored by instruction-tuned models (GPT-Image, Gemini);
	// placing a clear sentence BEFORE the tags ensures the background requirement
	// is respected regardless of the model family.
	preamble := "STRICT BACKGROUND REQUIREMENT: The background MUST be a plain solid white (#FFFFFF) seamless studio backdrop. " +
		"No color tints, no gradients, no shadows on the background surface, no textures, no props, no environmental scene, and no ground plane are allowed. " +
		"The character must appear to be standing or posed directly in front of a clean white infinity background. " +
		"背景严格要求：背景必须是纯白色（#FFFFFF）白色无缝棚拍背景。" +
		"不允许出现任何颜色、渐变、背景阴影、纹理、道具、环境场景或地面。" +
		"角色必须呈现为站在或摆姿势于纯白无限延伸背景前。"
	return preamble + "\n" + strings.Join(tags, ", ")
}

// composeCharacterPanelPromptWithStyle 在 composeCharacterPanelPrompt 基础上
// 按 stylePreset 判定 live-action / anime，供调用方省去 Canonical 判断。
func composeCharacterPanelPromptWithStyle(name, description, promptUsed, stylePreset string, panel CharacterPanel) string {
	canonical := stylepreset.Canonical(stylePreset)
	isLiveAction := canonical == stylepreset.LiveActionFilm || canonical == stylepreset.LiveActionShort
	return composeCharacterPanelPrompt(name, description, promptUsed, isLiveAction, panel)
}

// appendPanelSpecificNegatives 在 buildProjectImageNegativePrompt 基础上追加与分栏相关的反模式。
// - closeup 不能强制全身 → 去掉 "cropped torso / missing legs" 这类硬要求；
// - 非 closeup 应避免特写裁切。
func appendPanelSpecificNegatives(base string, panel CharacterPanel) string {
	// All panels: white background is required — reject any non-white background.
	extras := []string{
		"colorful background", "dark background", "black background", "grey background",
		"gradient background", "textured background", "scene background",
		"outdoor scene", "indoor scene", "room background", "background elements", "busy background",
	}
	switch panel {
	case CharacterPanelCloseup:
		// Close-up intentionally crops at the chest — don't penalise that.
		extras = append(extras,
			"multiple faces", "two heads", "twin portrait", "side-by-side portraits",
			"full body", "feet visible", "waist-down visible",
			"small face", "distant shot", "long shot", "tiny head", "half-figure in wide frame",
		)
	case CharacterPanelFront, CharacterPanelSide, CharacterPanelBack:
		extras = append(extras,
			"cropped body", "cropped legs", "feet out of frame", "missing feet",
			"partial body", "bust crop", "head-only",
			"multiple characters", "group shot", "two people",
		)
	}
	if len(extras) == 0 {
		return base
	}
	if strings.TrimSpace(base) == "" {
		return strings.Join(extras, ", ")
	}
	return base + ", " + strings.Join(extras, ", ")
}

// panelFileName 为拼接/上传时推荐的文件名前缀。
func panelFileName(assetID uint64, panel CharacterPanel) string {
	return fmt.Sprintf("asset_%d_panel_%s.jpg", assetID, panel)
}

// compositeFileName 为最终 4 栏横拼图推荐的文件名。
func compositeFileName(assetID uint64) string {
	return fmt.Sprintf("asset_%d_composite.jpg", assetID)
}
