package service

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/autovideo/project-service/internal/speechtext"
)

var sceneDescriptionBoilerplatePrefixes = []string{
	"镜头衔接：",
	"关键道具延续：",
	"氛围：",
	"角色状态——",
}

var sceneDescriptionSpatialNoise = regexp.MustCompile(`(人物位于画面(左侧|右侧|居中)|门在(左|右)后景|桌案在身侧|前景.{0,12}中景.{0,12}后景|轴线(左侧|右侧)机位|机位在|空间方位)`)

// sanitizeUserSceneDescription keeps scene descriptions readable for humans and video models.
// It strips auto-appended continuity boilerplate and excessive spatial blocking jargon.
func sanitizeUserSceneDescription(desc string) string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return ""
	}

	segments := regexp.MustCompile(`[。！？\n]+`).Split(desc, -1)
	kept := make([]string, 0, len(segments))
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if isSceneDescriptionBoilerplate(seg) {
			continue
		}
		if sceneDescriptionSpatialNoise.MatchString(seg) && utf8.RuneCountInString(seg) < 48 {
			continue
		}
		kept = append(kept, seg)
	}
	if len(kept) == 0 {
		return strings.TrimSpace(desc)
	}
	return strings.Join(kept, "。") + "。"
}

func isSceneDescriptionBoilerplate(seg string) bool {
	for _, prefix := range sceneDescriptionBoilerplatePrefixes {
		if strings.HasPrefix(seg, prefix) {
			return true
		}
	}
	if strings.HasPrefix(seg, "镜头衔接：") {
		return true
	}
	return false
}

// inferSceneDurationFromDialogue estimates clip duration from speakable text length.
// clipDuration is the project default; speechPace follows canonicalSpeechPace keys.
func inferSceneDurationFromDialogue(dialogue string, clipDuration int, speechPace string) int {
	return speechtext.InferClipDurationFromDialogue(dialogue, clipDuration, speechPace)
}
