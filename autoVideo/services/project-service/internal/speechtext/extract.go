package speechtext

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var subtitleExtractPattern = regexp.MustCompile(`\[字幕[:：]\s*([^\]]+?)\s*\]`)
var quotedSpeechPattern = regexp.MustCompile(`[“「『"]([^”」』"]+)[”」』"]`)

// ExtractNarrationForSpeech pulls speakable narration from commentary scripts or misformatted drama scripts.
func ExtractNarrationForSpeech(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	var parts []string
	for _, m := range subtitleExtractPattern.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			if v := strings.TrimSpace(m[1]); v != "" {
				parts = append(parts, v)
			}
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}

	for _, m := range quotedSpeechPattern.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			if v := strings.TrimSpace(m[1]); v != "" {
				parts = append(parts, v)
			}
		}
	}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if sceneSluglinePattern.MatchString(line) || speakerCueOnlyPattern.MatchString(line) {
			continue
		}
		if screenplayActionLine.MatchString(line) && !speakerLinePattern.MatchString(line) {
			continue
		}
		if nameOnlyLinePattern.MatchString(line) ||
			sceneSettingLinePattern.MatchString(line) ||
			locationLeadLinePattern.MatchString(line) ||
			actionOnlyLinePattern.MatchString(line) {
			continue
		}
		if m := speakerWithEmotionLine.FindStringSubmatch(line); len(m) == 3 {
			line = strings.TrimSpace(m[2])
		} else if m := speakerLinePattern.FindStringSubmatch(line); len(m) == 3 {
			line = strings.TrimSpace(m[2])
		}
		if utf8.RuneCountInString(line) >= 10 {
			parts = append(parts, line)
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// LooksLikeSceneDescription reports whether text is likely a visual/stage direction, not speakable narration.
func LooksLikeSceneDescription(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if sceneSluglinePattern.MatchString(text) {
		return true
	}
	if sceneSettingLinePattern.MatchString(text) || locationLeadLinePattern.MatchString(text) {
		return true
	}
	if screenplayActionLine.MatchString(text) && !speakerLinePattern.MatchString(text) {
		return utf8.RuneCountInString(text) >= 12
	}
	return false
}

// LooksLikeCompleteUtterance reports whether short text is still a complete speakable line.
func LooksLikeCompleteUtterance(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if quotedSpeechPattern.MatchString(text) {
		return true
	}
	return strings.ContainsAny(text, "。！？!?；;") && utf8.RuneCountInString(text) >= 6
}
