package speechtext

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var subtitleExtractPattern = regexp.MustCompile(`\[字幕[:：]\s*([^\]]+?)\s*\]`)
var quotedSpeechPattern = regexp.MustCompile(`[“「『"]([^”」』"]+)[”」』"]`)
var storyboardVisualKeywordPattern = regexp.MustCompile(`画面|构图|近景|中景|远景|景别|空镜|机位|运镜|特写|环境光线|背景简洁|神情|面露|身穿|穿着|身形对比|视觉`)

// shouldExtractQuotes checks if we should extract only the quoted text from a line.
// If there is significant narrative text before the quote (more than 5 runes),
// we should NOT extract only the quoted part, but instead keep the entire text verbatim.
func shouldExtractQuotes(text string) bool {
	quoteChars := []string{"“", "「", "『", "\""}
	firstIdx := -1
	for _, qc := range quoteChars {
		idx := strings.Index(text, qc)
		if idx != -1 && (firstIdx == -1 || idx < firstIdx) {
			firstIdx = idx
		}
	}

	if firstIdx == -1 {
		return false
	}

	before := text[:firstIdx]
	beforeClean := strings.TrimFunc(before, func(r rune) bool {
		return r == ':' || r == '：' || r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '　'
	})

	if utf8.RuneCountInString(beforeClean) > 5 {
		return false
	}

	return true
}

// ExtractNarrationForSpeech pulls speakable narration from commentary scripts or misformatted drama scripts.
func ExtractNarrationForSpeech(text string) string {
	return ExtractNarrationForSpeechEx(text, false)
}

// ExtractNarrationForSpeechEx is the extended version of ExtractNarrationForSpeech that supports skipping action/scene filters in commentary mode.
func ExtractNarrationForSpeechEx(text string, isCommentary bool) string {
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

	if shouldExtractQuotes(text) {
		for _, m := range quotedSpeechPattern.FindAllStringSubmatch(text, -1) {
			if len(m) > 1 {
				if v := strings.TrimSpace(m[1]); v != "" {
					parts = append(parts, v)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !isCommentary {
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
		}
		if m := speakerWithEmotionLine.FindStringSubmatch(line); len(m) == 3 && isRealSpeakerName(m[1]) {
			line = strings.TrimSpace(m[2])
		} else if m := speakerLinePattern.FindStringSubmatch(line); len(m) == 3 && isRealSpeakerName(m[1]) {
			line = strings.TrimSpace(m[2])
		}
		if isCommentary || utf8.RuneCountInString(line) >= 10 {
			parts = append(parts, line)
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// LooksLikeStoryboardVisualDescription detects AI storyboard staging text in dialogue fields.
func LooksLikeStoryboardVisualDescription(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if LooksLikeSceneDescription(text) {
		return true
	}
	keywords := storyboardVisualKeywordPattern.FindAllString(text, -1)
	if len(keywords) >= 2 {
		return true
	}
	if strings.Contains(text, "内部，") && (strings.Contains(text, "神情") || strings.Contains(text, "表情") || strings.Contains(text, "光线")) {
		return true
	}
	if LooksLikeCompleteUtterance(text) && len(keywords) == 0 {
		return false
	}
	if len(keywords) == 1 {
		return true
	}
	return false
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
