package service

import (
	"regexp"
	"strings"
	"unicode/utf8"
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
	dialogue = strings.TrimSpace(dialogue)
	if dialogue == "" {
		return normalizeAdSceneDuration(0, clipDuration)
	}

	charsPer10Sec := speechPaceCharsPer10Sec(speechPace)
	runes := utf8.RuneCountInString(dialogue)
	if runes <= 0 {
		return normalizeAdSceneDuration(0, clipDuration)
	}

	needed := float64(runes) / float64(charsPer10Sec) * 10.0
	needed += 0.45 // breathing room after last syllable

	base := clipDuration
	if base <= 0 {
		base = 5
	}

	// Short narration should not inherit a long default clip.
	if needed <= float64(base)*0.55 {
		return clampDuration(int(needed+0.5), 2, base)
	}
	// Long narration needs a longer beat, capped for short-form video.
	if needed >= float64(base)*1.25 {
		maxDur := base + 4
		if maxDur > 12 {
			maxDur = 12
		}
		return clampDuration(int(needed+0.5), base, maxDur)
	}
	return clampDuration(base, 2, 12)
}

func speechPaceCharsPer10Sec(speechPace string) int {
	switch canonicalSpeechPace(speechPace) {
	case "slightly_fast":
		return 56
	case "with_pauses":
		return 38
	case "very_fast":
		return 66
	case "medium_fast":
		return 52
	case "medium_steady":
		return 42
	default:
		return 48
	}
}
