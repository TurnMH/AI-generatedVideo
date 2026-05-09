package service

import (
	"strings"
)

// VideoPromptParts represents structured parts of a video prompt.
// These map to the 10 cinematography fields requested for structured video generation:
// 主体/景别/氛围/环境/运镜/视角/特殊拍摄手法/构图/风格统一/动态控制
type VideoPromptParts struct {
	// SubjectAnchor — 主体: who/what is in the frame (e.g. "李明，黑色西装")
	SubjectAnchor string
	// OpeningState — 开场状态: initial frame state
	OpeningState string
	// ShotSize — 景别: e.g. 特写/近景/中景/全景/远景
	ShotSize string
	// ViewAngle — 视角: e.g. 平视/仰拍/俯拍/POV
	ViewAngle string
	// Action — 动作: what the subject physically does
	Action string
	// CameraMotion — 运镜: e.g. 推/拉/摇/跟/固定/环绕
	CameraMotion string
	// SpecialTechnique — 特殊拍摄手法: e.g. 慢动作/手持抖动/浅景深/变焦
	SpecialTechnique string
	// Composition — 构图: e.g. 三分法/对称/斜线引导/框架构图
	Composition string
	// Environment — 环境: brief scene/location description
	Environment string
	// Atmosphere — 氛围: mood/emotional tone e.g. 压抑紧张/温暖明亮
	Atmosphere string
	// StyleAnchor — 风格统一: style consistency anchor across all clips
	StyleAnchor string
	// MotionControl — 动态控制: speed, amplitude, trajectory guidance
	MotionControl string
	// Continuity — scene-to-scene continuity constraint
	Continuity string
	// Negatives — things to avoid
	Negatives string
}

// composeVideoPrompt builds a video prompt from structured parts.
// Order: subject > opening > shot-size > angle > action > camera > technique > composition > environment > atmosphere > style > motion-control > continuity > negatives
func composeVideoPrompt(parts VideoPromptParts) string {
	var segments []string

	if parts.SubjectAnchor != "" {
		segments = append(segments, parts.SubjectAnchor)
	}
	if parts.OpeningState != "" {
		segments = append(segments, parts.OpeningState)
	}
	if parts.ShotSize != "" {
		segments = append(segments, parts.ShotSize)
	}
	if parts.ViewAngle != "" {
		segments = append(segments, parts.ViewAngle)
	}
	if parts.Action != "" {
		segments = append(segments, parts.Action)
	}
	if parts.CameraMotion != "" {
		segments = append(segments, parts.CameraMotion)
	}
	if parts.SpecialTechnique != "" {
		segments = append(segments, parts.SpecialTechnique)
	}
	if parts.Composition != "" {
		segments = append(segments, parts.Composition)
	}
	if parts.Environment != "" {
		segments = append(segments, parts.Environment)
	}
	if parts.Atmosphere != "" {
		segments = append(segments, parts.Atmosphere)
	}
	if parts.StyleAnchor != "" {
		segments = append(segments, parts.StyleAnchor)
	}
	if parts.MotionControl != "" {
		segments = append(segments, parts.MotionControl)
	}
	if parts.Continuity != "" {
		segments = append(segments, parts.Continuity)
	}
	if parts.Negatives != "" {
		segments = append(segments, "avoid: "+parts.Negatives)
	}

	return strings.Join(segments, ", ")
}

// extractCharacterAnchor extracts a short identity anchor from a character prompt
// for better video subject consistency
func extractCharacterAnchor(name, prompt string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	// Take first 100 chars as identity anchor
	if len(prompt) > 100 {
		prompt = string([]rune(prompt)[:100]) + "..."
	}
	return name + "(" + prompt + ")"
}
