package service

import (
	"fmt"
	"strings"
)

// ======================== Model Language Detection ========================

// IsChineseModel returns true for models that expect Chinese-language prompts.
// Checks for known Chinese model names (星图/xingtu, 可图/kolors, 混元/hunyuan,
// 通义/tongyi/wanx). All other models default to English.
func IsChineseModel(name string) bool {
	lower := strings.ToLower(name)
	for _, kw := range []string{"xingtu", "星图", "kolors", "可图", "hunyuan", "混元", "tongyi", "通义", "wanx"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// ======================== Fixed Base Terms ========================

// Character prompts — portrait / reference sheet style
const (
	fixedCharZH = "真实摄影, 电影级RAW原片, 包含角色完整的正面头肩肖像, 全身正面+侧面+背面三视图并排, " +
		"(超写实主义:1.5), 8k分辨率, 纯白背景, (画面极其纯净:1.5), " +
		"(无文字:2.0), 无水印, 硬光照明, 伦布朗光, 极致细节, 富士胶片感"
	fixedCharEN = "photorealistic, cinematic RAW photo, full character portrait plus front/side/back three-view reference sheet, " +
		"(hyperrealism:1.5), 8k resolution, pure white background, (ultra clean frame:1.5), " +
		"(no text:2.0), no watermark, hard lighting, Rembrandt lighting, extreme detail, Fujifilm aesthetic"
)

// Scene prompts — empty environment
const (
	fixedSceneZH = "纯净场景空镜，画面中绝对不能出现任何人类、角色或生物，空无一人, empty scene, absolutely no people, " +
		"(色调统一:1.5), (光影连续性:1.5), 电影级布光, 8k超高清, " +
		"(无文字:2.0), 无水印"
	fixedSceneEN = "纯净场景空镜，画面中绝对不能出现任何人类、角色或生物，空无一人, empty scene, absolutely no people, " +
		"(unified color palette:1.5), (continuous lighting:1.5), cinematic lighting, 8k ultra-high definition, " +
		"(no text:2.0), no watermark"
)

// Item / prop prompts — product photography style
const (
	fixedItemZH = "真实摄影, 产品实拍, 柔和布光, 纯白背景, 无杂物, " +
		"(无文字:2.0), 无水印, 8k分辨率, 极致真实"
	fixedItemEN = "photorealistic, product photography, soft lighting, pure white background, clutter-free, " +
		"(no text:2.0), no watermark, 8k resolution, extreme realism"
)

// ======================== Prompt Builders ========================

// BuildCharacterPrompt constructs a T4-standard two-segment prompt for a character asset.
// keywords should contain: age_body, appearance, clothing, emotion_baseline (all English strings).
func BuildCharacterPrompt(name string, keywords map[string]interface{}, zh bool) string {
	var supp []string
	if name != "" {
		if zh {
			supp = append(supp, name)
		} else {
			supp = append(supp, name)
		}
	}
	for _, key := range []string{"age_body", "appearance", "clothing", "emotion_baseline"} {
		if v, ok := keywords[key]; ok {
			if s, _ := v.(string); s != "" {
				supp = append(supp, s)
			}
		}
	}

	fixed := fixedCharEN
	if zh {
		fixed = fixedCharZH
	}
	supplement := strings.Join(supp, ", ")
	if supplement == "" {
		return fixed
	}
	return fmt.Sprintf("%s, %s", supplement, fixed)
}

// BuildScenePrompt constructs a T4-standard two-segment prompt for a scene/location.
// keywords should contain: location, time, lighting, atmosphere, spatial, color_palette (all English strings).
func BuildScenePrompt(name string, keywords map[string]interface{}, direction, viewAngle string, zh bool) string {
	var supp []string
	if direction != "" {
		supp = append(supp, direction)
	}
	if viewAngle != "" {
		supp = append(supp, viewAngle)
	}
	if name != "" {
		supp = append(supp, name)
	}
	for _, key := range []string{"location", "spatial", "atmosphere", "color_palette", "time", "lighting"} {
		if v, ok := keywords[key]; ok {
			if s, _ := v.(string); s != "" {
				supp = append(supp, s)
			}
		}
	}

	fixed := fixedSceneEN
	if zh {
		fixed = fixedSceneZH
	}
	supplement := strings.Join(supp, ", ")
	if supplement == "" {
		return fixed
	}
	return fmt.Sprintf("%s, %s", supplement, fixed)
}

// BuildItemPrompt constructs a T4-standard two-segment prompt for a prop/item.
// keywords should contain: material, condition, features (all English strings).
func BuildItemPrompt(name string, keywords map[string]interface{}, zh bool) string {
	var supp []string
	if name != "" {
		supp = append(supp, name)
	}
	for _, key := range []string{"material", "features", "condition"} {
		if v, ok := keywords[key]; ok {
			if s, _ := v.(string); s != "" {
				supp = append(supp, s)
			}
		}
	}

	fixed := fixedItemEN
	if zh {
		fixed = fixedItemZH
	}
	supplement := strings.Join(supp, ", ")
	if supplement == "" {
		return fixed
	}
	return fmt.Sprintf("%s, %s", supplement, fixed)
}

// BuildSceneImagePrompt builds a T4-standard cinematic scene prompt from scene fields.
// Used by GenerateSceneImage when constructing the prompt for the image-service call.
func BuildSceneImagePrompt(description, setting, emotion string, storyboard []interface{}, zh bool) string {
	// Prefer the first storyboard shot's visual description for rich detail
	if len(storyboard) > 0 {
		if shot, ok := storyboard[0].(map[string]interface{}); ok {
			kw := make(map[string]interface{})
			for _, key := range []string{"lighting", "atmosphere"} {
				if v, ok := shot[key]; ok {
					kw[key] = v
				}
			}
			// Derive location/spatial/color from the shot
			if v, ok := shot["visual_desc"]; ok {
				kw["spatial"] = v
			}
			if v, ok := shot["composition"]; ok {
				kw["location"] = v
			}
			angle, _ := shot["angle"].(string)
			shotType, _ := shot["shot_type"].(string)
			return BuildScenePrompt(description, kw, angle, shotType, zh)
		}
	}

	// Fallback: build from basic scene fields
	kw := make(map[string]interface{})
	if setting != "" {
		kw["location"] = setting
	}
	if emotion != "" {
		kw["atmosphere"] = emotion
	}
	return BuildScenePrompt(description, kw, "", "", zh)
}
