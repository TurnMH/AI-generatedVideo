package productionmode

import (
	"fmt"
	"strings"

	"github.com/autovideo/project-service/internal/stylepreset"
)

// DefaultSpeechPace returns a sensible narration pace for the canonical style preset.
func DefaultSpeechPace(stylePreset string) string {
	switch stylepreset.Canonical(stylePreset) {
	case stylepreset.LiveActionFilm:
		return "medium_steady"
	case stylepreset.LiveActionShort:
		return "with_pauses"
	case stylepreset.Anime3D:
		return "slightly_fast"
	default:
		return "normal"
	}
}

// StylePresetLabel returns a human-readable Chinese label for prompts/logs.
func StylePresetLabel(stylePreset string) string {
	switch stylepreset.Canonical(stylePreset) {
	case stylepreset.Anime2D:
		return "二维动漫"
	case stylepreset.Anime3D:
		return "三维动漫"
	case stylepreset.LiveActionFilm:
		return "真人电影"
	case stylepreset.LiveActionShort:
		return "真人短剧"
	default:
		if trimmed := strings.TrimSpace(stylePreset); trimmed != "" {
			return trimmed
		}
		return "二维动漫"
	}
}

// StyleSplitVisualHint guides LLM scene splitting toward the project's visual style.
func StyleSplitVisualHint(stylePreset, motionMode string) string {
	var parts []string
	switch stylepreset.Canonical(stylePreset) {
	case stylepreset.Anime2D:
		parts = append(parts,
			"项目视觉风格：二维动漫（anime-2d）。",
			"description 应体现平面线条、番剧感角色表演、赛璐璐上色、二维构图；不要写成真人摄影或三维 CG 渲染。",
			"角色外观与场景氛围应服务二维叙事，不要混入写实肤质、电影实拍或游戏引擎质感。",
		)
	case stylepreset.Anime3D:
		parts = append(parts,
			"项目视觉风格：三维动漫（anime-3d）。",
			"description 应体现角色体积感、三渲二材质、CG 景深、立体空间调度；不要写成纯二维线稿或真人实拍。",
			"强调立体造型、卡通材质层次、三维镜头纵深；避免“手绘线条/平面赛璐璐”主导画面。",
		)
	case stylepreset.LiveActionFilm:
		parts = append(parts,
			"项目视觉风格：真人电影（live-action-film）。",
			"description 应体现真实场景、电影光影、银幕构图与写实材质；禁止写成动漫、插画、三渲二或卡通化表达。",
		)
	case stylepreset.LiveActionShort:
		parts = append(parts,
			"项目视觉风格：真人短剧（live-action-short）。",
			"description 应体现真实人物表演、近景对白、自然肤质与真实空间；禁止写成动漫、插画或 CG 卡通。",
		)
	default:
		parts = append(parts, "项目视觉风格："+StylePresetLabel(stylePreset)+"。")
	}
	if cue := motionModeSplitHint(motionMode); cue != "" {
		parts = append(parts, cue)
	}
	return strings.Join(parts, "\n")
}

func motionModeSplitHint(motionMode string) string {
	switch strings.TrimSpace(motionMode) {
	case "gentle":
		return "运镜气质：偏柔和克制，适合对白、情绪与氛围镜头。"
	case "dynamic":
		return "运镜气质：偏动感张力，适合动作、冲突与节奏更快的镜头。"
	case "cinematic":
		return "运镜气质：偏电影化，强调景别变化与银幕感。"
	default:
		return ""
	}
}

// SceneSplitStyleBlock returns the style section injected into scene-split user prompts.
func SceneSplitStyleBlock(p SceneSplitParams) string {
	if strings.TrimSpace(p.StyleHint) == "" {
		return ""
	}
	return fmt.Sprintf("- 视觉风格约束（必须同步遵守）：\n%s", p.StyleHint)
}

// ScriptPrepRuntimeContext prepends project runtime constraints to script prep user content.
func ScriptPrepRuntimeContext(stylePreset, motionMode string, clipDuration int, speechPace string) string {
	duration := clipDuration
	if duration <= 0 {
		duration = 5
	}
	pace := strings.TrimSpace(speechPace)
	if pace == "" {
		pace = DefaultSpeechPace(stylePreset)
	}
	return fmt.Sprintf(`项目运行时约束（后续分镜拆分、出图、视频生成都会继承，必须一致）：
- 目标视觉风格：%s（canonical=%s）
- 目标单分镜时长：%d 秒（dialogue 字数与 duration 必须匹配，禁止明显超时口播）
- 目标语速档位：%s
- 运镜气质：%s

%s`,
		StylePresetLabel(stylePreset),
		stylepreset.Canonical(stylePreset),
		duration,
		pace,
		strings.TrimSpace(motionMode),
		StyleSplitVisualHint(stylePreset, motionMode),
	)
}

// RefinePromptStyleBlock adds an explicit style anchor for post-split prompt refinement.
func RefinePromptStyleBlock(stylePreset string) string {
	return "Project visual style (every prompt must stay consistent): " + strings.ReplaceAll(StyleSplitVisualHint(stylePreset, ""), "\n", " ")
}
