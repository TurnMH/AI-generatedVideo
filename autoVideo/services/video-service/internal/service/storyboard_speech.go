package service

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var subtitleExtractPattern = regexp.MustCompile(`\[字幕[:：]\s*([^\]]+?)\s*\]`)
var quotedSpeechPattern = regexp.MustCompile(`[“「『"]([^”」』"]+)[”」』"]`)
var storyboardVisualKeywordPattern = regexp.MustCompile(`画面|构图|近景|中景|远景|景别|空镜|机位|运镜|特写|环境光线|背景简洁|神情|面露|身穿|穿着|身形对比|视觉`)

// looksLikeSceneDescription reports whether text is likely visual/stage direction, not speakable narration.
func looksLikeSceneDescription(text string) bool {
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

// looksLikeSpeakerVisualStaging detects visual staging in speaker-labelled content.
// It avoids location-led narration false positives like "后厨里，刀光映着夕阳。".
func looksLikeSpeakerVisualStaging(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return true
	}
	keywords := storyboardVisualKeywordPattern.FindAllString(content, -1)
	if len(keywords) >= 2 {
		return true
	}
	if strings.Contains(content, "内部，") && (strings.Contains(content, "神情") || strings.Contains(content, "表情") || strings.Contains(content, "光线")) {
		return true
	}
	if looksLikeCompleteUtterance(content) && len(keywords) == 0 {
		return false
	}
	if timeTransitionLinePattern.MatchString(content) {
		return true
	}
	if screenplayActionLine.MatchString(content) && !looksLikeCompleteUtterance(content) {
		return true
	}
	if len(keywords) == 1 {
		return true
	}
	return false
}

// looksLikeStoryboardVisualDescription detects AI storyboard staging text (composition, lighting, expressions).
func looksLikeStoryboardVisualDescription(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if looksLikeSceneDescription(text) {
		return true
	}
	return looksLikeSpeakerVisualStaging(text)
}

func looksLikeCompleteUtterance(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if quotedSpeechPattern.MatchString(text) {
		return true
	}
	return strings.ContainsAny(text, "。！？!?；;") && utf8.RuneCountInString(text) >= 6
}

func extractSubtitleTagNarration(text string) string {
	var parts []string
	for _, m := range subtitleExtractPattern.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			if v := strings.TrimSpace(m[1]); v != "" {
				parts = append(parts, v)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func extractQuotedSpeech(text string) string {
	var parts []string
	for _, m := range quotedSpeechPattern.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			if v := strings.TrimSpace(m[1]); v != "" {
				parts = append(parts, v)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

type characterQuote struct {
	Speaker string
	Quote   string
}

var speakerBeforeQuotePattern = regexp.MustCompile(`([\p{Han}A-Za-z·]{2,8})(?:[（(][^)）]{0,16}[）)])?(?:[^"「『"]{0,32})?(?:说|喊|叫|问|答|回应|道|唤|开口|低声|轻声|沉声|冷声|怒|笑)[^"「『"]{0,12}[“「『"]`)

func extractCharacterQuotesFromScene(sceneText string, characters []string) []characterQuote {
	sceneText = strings.TrimSpace(strings.ReplaceAll(sceneText, "\r\n", "\n"))
	if sceneText == "" {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]characterQuote, 0, 4)
	for _, m := range quotedSpeechPattern.FindAllStringSubmatchIndex(sceneText, -1) {
		if len(m) < 4 {
			continue
		}
		quote := strings.TrimSpace(sceneText[m[2]:m[3]])
		if quote == "" || utf8.RuneCountInString(quote) > 120 {
			continue
		}
		if looksLikeStoryboardVisualDescription(quote) || looksLikeSceneDescription(quote) {
			continue
		}
		start := m[0]
		prefixStart := start - 48
		if prefixStart < 0 {
			prefixStart = 0
		}
		before := sceneText[prefixStart:start]
		speaker := inferSpeakerBeforeQuote(before, characters, quote)
		if speaker == "" {
			continue
		}
		key := speaker + "\x00" + quote
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, characterQuote{Speaker: speaker, Quote: quote})
	}
	return out
}

func inferSpeakerBeforeQuote(before string, characters []string, quote string) string {
	before = strings.TrimSpace(before)
	if before == "" {
		return pickFallbackSpeaker(characters, quote)
	}
	if m := speakerBeforeQuotePattern.FindStringSubmatch(before); len(m) >= 2 {
		if speaker := normalizeSpeakerLabel(m[1]); speaker != "" && speaker != quote {
			return speaker
		}
	}
	lastIdx := -1
	lastName := ""
	for _, name := range characters {
		n := normalizeSpeakerLabel(name)
		if n == "" || n == quote {
			continue
		}
		if idx := strings.LastIndex(before, n); idx > lastIdx {
			lastIdx = idx
			lastName = n
		}
	}
	if lastName != "" {
		return lastName
	}
	return pickFallbackSpeaker(characters, quote)
}

func pickFallbackSpeaker(characters []string, quote string) string {
	for _, name := range characters {
		n := normalizeSpeakerLabel(name)
		if n == "" || n == quote {
			continue
		}
		return n
	}
	return ""
}

func dedupeSpeechLines(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	return out
}

func formatStoryboardDubbingFromFields(dialogue, sceneDescription string, characters []string, commentary bool) string {
	dialogue = strings.TrimSpace(strings.ReplaceAll(dialogue, "\r\n", "\n"))
	sceneDescription = strings.TrimSpace(strings.ReplaceAll(sceneDescription, "\r\n", "\n"))
	var parts []string

	for _, line := range extractNarrationLinesFromDialogue(dialogue, commentary) {
		parts = append(parts, "旁白："+line)
	}
	for _, line := range extractCharacterLinesFromDialogue(dialogue) {
		parts = append(parts, line)
	}
	for _, q := range extractCharacterQuotesFromScene(sceneDescription, characters) {
		line := q.Speaker + "：" + q.Quote
		if containsSpeechLine(parts, line) {
			continue
		}
		parts = append(parts, line)
	}
	if len(parts) == 0 {
		return ""
	}
	return normalizeMislabeledNarrationSpeakers(strings.Join(parts, "\n"))
}

func containsSpeechLine(parts []string, want string) bool {
	for _, part := range parts {
		if part == want {
			return true
		}
	}
	return false
}

func extractNarrationLinesFromDialogue(dialogue string, commentary bool) []string {
	if dialogue == "" {
		return nil
	}
	if narr := extractSubtitleTagNarration(dialogue); narr != "" {
		return dedupeSpeechLines(strings.Split(narr, "\n"))
	}
	if commentary {
		var lines []string
		for _, rawLine := range strings.Split(dialogue, "\n") {
			line := strings.TrimSpace(rawLine)
			if line == "" {
				continue
			}
			if m := speakerLinePattern.FindStringSubmatch(line); len(m) == 3 {
				speaker := normalizeSpeakerLabel(m[1])
				content := strings.TrimSpace(m[2])
				if speaker != "" && content != "" && isLikelySpeakerLabel(speaker) {
					for _, hint := range autoVoiceNarratorHints {
						if strings.Contains(speaker, hint) {
							lines = append(lines, content)
							break
						}
					}
				}
				continue
			}
			if looksLikeStoryboardVisualDescription(line) || looksLikeSceneDescription(line) {
				continue
			}
			if utf8.RuneCountInString(line) >= 6 && extractQuotedSpeech(line) == "" {
				lines = append(lines, line)
			}
		}
		return dedupeSpeechLines(lines)
	}
	return dedupeSpeechLines([]string{strings.TrimSpace(extractStoryboardSpeechText(dialogue))})
}

func extractCharacterLinesFromDialogue(dialogue string) []string {
	if dialogue == "" {
		return nil
	}
	var lines []string
	for _, rawLine := range strings.Split(dialogue, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		m := speakerLinePattern.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		speaker := normalizeSpeakerLabel(m[1])
		content := strings.TrimSpace(m[2])
		if speaker == "" || content == "" || !isLikelySpeakerLabel(speaker) {
			continue
		}
		isNarrator := false
		for _, hint := range autoVoiceNarratorHints {
			if strings.Contains(speaker, hint) {
				isNarrator = true
				break
			}
		}
		if isNarrator || looksLikeSpeakerVisualStaging(content) {
			continue
		}
		lines = append(lines, speaker+"："+content)
	}
	return dedupeSpeechLines(lines)
}

// extractStoryboardSpeechText pulls speakable narration from mixed storyboard dialogue fields.
func extractStoryboardSpeechText(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return ""
	}

	if narr := extractSubtitleTagNarration(text); narr != "" {
		return narr
	}
	if quoted := extractQuotedSpeech(text); quoted != "" {
		return quoted
	}

	var parts []string
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		if m := speakerLinePattern.FindStringSubmatch(line); len(m) == 3 {
			speaker := strings.TrimSpace(m[1])
			content := strings.TrimSpace(m[2])
			if speaker == "" || content == "" || !isLikelySpeakerLabel(speaker) {
				continue
			}
			if content == "" || looksLikeSpeakerVisualStaging(content) {
				continue
			}
			parts = append(parts, speaker+"："+content)
			continue
		}

		if looksLikeStoryboardVisualDescription(line) {
			continue
		}
		if utf8.RuneCountInString(line) >= 6 && !looksLikeSceneDescription(line) {
			parts = append(parts, line)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// normalizeMislabeledNarrationSpeakers rewrites character-labelled lines that are
// actually third-person narration or visual staging back to 旁白.
func normalizeMislabeledNarrationSpeakers(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		matches := speakerLinePattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			out = append(out, line)
			continue
		}
		speaker := strings.TrimSpace(matches[1])
		content := strings.TrimSpace(matches[2])
		if shouldRelabelAsNarrator(speaker, content) {
			out = append(out, "旁白："+content)
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func shouldRelabelAsNarrator(speaker, content string) bool {
	if speaker == "" || content == "" {
		return false
	}
	for _, hint := range autoVoiceNarratorHints {
		if strings.Contains(speaker, hint) {
			return false
		}
	}
	if strings.ContainsAny(content, "你我咱") && !strings.HasPrefix(content, speaker) &&
		!looksLikeStoryboardVisualDescription(content) && !looksLikeSceneDescription(content) {
		return false
	}
	if quotedSpeechPattern.MatchString(content) {
		return false
	}
	if strings.HasPrefix(content, speaker) {
		return true
	}
	if looksLikeStoryboardVisualDescription(content) || looksLikeSceneDescription(content) {
		return true
	}
	if strings.Contains(speaker, "（") || strings.Contains(speaker, "(") {
		return true
	}
	return false
}

func isCommentaryProductionMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "commentary_comic", "commentary", "commentary-comic", "explainer-comic":
		return true
	default:
		return false
	}
}

func ensureCommentaryNarratorLabels(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return text
	}
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if matches := speakerLinePattern.FindStringSubmatch(line); len(matches) == 3 {
			speaker := normalizeSpeakerLabel(matches[1])
			content := strings.TrimSpace(matches[2])
			if speaker != "" && content != "" && isLikelySpeakerLabel(speaker) {
				return text
			}
		}
	}
	return "旁白：" + text
}

// cleanPerClipDialogue normalizes storyboard dialogue before per-clip TTS/subtitles.
func cleanPerClipDialogue(text string) string {
	return cleanPerClipDialogueForMode(text, false)
}

func cleanPerClipDialogueForMode(text string, commentary bool) string {
	if strings.Contains(text, "：") && speakerLinePattern.MatchString(text) {
		text = normalizeMislabeledNarrationSpeakers(text)
		return strings.TrimSpace(cleanScriptForSpeech(text))
	}
	text = ensureSpeakerLabelsForStoryboardDubbing(text)
	text = normalizeMislabeledNarrationSpeakers(text)
	if commentary {
		text = ensureCommentaryNarratorLabels(text)
	}
	text = strings.TrimSpace(extractStoryboardSpeechText(text))
	return strings.TrimSpace(cleanScriptForSpeech(text))
}
