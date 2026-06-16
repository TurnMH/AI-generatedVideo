package speechtext

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const DefaultMaxClipDialogueRunes = 180

var sentenceSplitPattern = regexp.MustCompile(`[。！？!?；;\n]+`)

// CompactClipDialogue keeps a single speakable beat per clip and removes duplicate lines/sentences.
func CompactClipDialogue(text string, maxRunes int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return ""
	}
	if maxRunes <= 0 {
		maxRunes = DefaultMaxClipDialogueRunes
	}

	if segments := ExtractSubtitleSegments(text); len(segments) > 0 {
		text = strings.TrimSpace(segments[0])
		if utf8.RuneCountInString(text) <= maxRunes {
			return text
		}
	} else if utf8.RuneCountInString(text) > maxRunes {
		if first := firstQuotedSpeech(text); first != "" {
			text = first
		} else {
			text = firstSpeakableUnit(text, maxRunes)
		}
	}

	text = dedupeSpeechUnits(text)
	if runes := []rune(text); len(runes) > maxRunes {
		text = truncateAtSentenceBoundary(string(runes[:maxRunes]))
	}
	return strings.TrimSpace(text)
}

// CompactCommentaryDialogue is a commentary-safe version of CompactClipDialogue.
// It removes duplicate lines/sentences but NEVER extracts quotes or discards narrative text.
func CompactCommentaryDialogue(text string, maxRunes int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return ""
	}
	if maxRunes <= 0 {
		maxRunes = DefaultMaxClipDialogueRunes
	}

	// For commentary, we do NOT call dedupeSpeechUnits because it splits by punctuation
	// and joins with "。", destroying the original punctuation (like "……\"", "，", etc.).
	// We keep the text exactly as is.
	if runes := []rune(text); len(runes) > maxRunes {
		// Use splitTextByRunes which splits at natural punctuation boundaries
		chunks := splitTextByRunes(text, maxRunes)
		if len(chunks) > 0 {
			return chunks[0]
		}
		return string(runes[:maxRunes])
	}
	return text
}

func firstQuotedSpeech(text string) string {
	for _, m := range quotedSpeechPattern.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			if v := strings.TrimSpace(m[1]); v != "" {
				return v
			}
		}
	}
	return ""
}

func firstSpeakableUnit(text string, maxRunes int) string {
	for _, unit := range splitSpeechUnits(text) {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		if LooksLikeSceneDescription(unit) || LooksLikeStoryboardVisualDescription(unit) {
			continue
		}
		if utf8.RuneCountInString(unit) <= maxRunes {
			return unit
		}
		if runes := []rune(unit); len(runes) > maxRunes {
			return truncateAtSentenceBoundary(string(runes[:maxRunes]))
		}
	}
	units := splitSpeechUnits(text)
	if len(units) == 0 {
		return truncateAtSentenceBoundary(text)
	}
	return strings.TrimSpace(units[0])
}

func splitSpeechUnits(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var units []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, seg := range sentenceSplitPattern.Split(line, -1) {
			if seg = strings.TrimSpace(seg); seg != "" {
				units = append(units, seg)
			}
		}
	}
	return units
}

func dedupeSpeechUnits(text string) string {
	units := splitSpeechUnits(text)
	if len(units) == 0 {
		return strings.TrimSpace(text)
	}
	out := make([]string, 0, len(units))
	seen := make(map[string]struct{}, len(units))
	for _, unit := range units {
		key := normalizeSpeechKey(unit)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, unit)
	}
	if len(out) == 0 {
		return strings.TrimSpace(text)
	}
	if len(out) == 1 {
		unit := out[0]
		trimmed := strings.TrimSpace(text)
		if strings.HasSuffix(trimmed, "。") && !strings.HasSuffix(unit, "。") {
			return unit + "。"
		}
		return unit
	}
	return strings.Join(out, "。") + "。"
}

func normalizeSpeechKey(text string) string {
	return NormalizeSpeechKey(text)
}

// NormalizeSpeechKey strips punctuation/whitespace for fuzzy speech matching.
func NormalizeSpeechKey(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range text {
		switch r {
		case ' ', '\t', '\n', '\r', '　', '“', '”', '「', '」', '『', '』', '"', '\'':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func truncateAtSentenceBoundary(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if idx := strings.LastIndexAny(text, "，,；;"); idx >= utf8.RuneCountInString(text)/3 {
		if prefix := strings.TrimSpace(text[:idx]); prefix != "" {
			return prefix + "。"
		}
	}
	return strings.TrimSpace(text) + "。"
}

// splitSpeechUnitsPreservingPunctuation splits a paragraph into sentences,
// preserving the punctuation and any trailing quotes/brackets.
func splitSpeechUnitsPreservingPunctuation(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var units []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		runes := []rune(line)
		var current []rune
		for i := 0; i < len(runes); i++ {
			r := runes[i]
			current = append(current, r)
			
			// Check if r is a sentence-ending punctuation
			if r == '。' || r == '！' || r == '？' || r == '!' || r == '?' || r == '；' || r == ';' {
				// Consume any trailing quotes or brackets
				for i+1 < len(runes) {
					next := runes[i+1]
					if next == '”' || next == '」' || next == '』' || next == '"' || next == '\'' || next == ')' || next == '）' {
						current = append(current, next)
						i++
					} else {
						break
					}
				}
				units = append(units, strings.TrimSpace(string(current)))
				current = nil
			}
		}
		if len(current) > 0 {
			if s := strings.TrimSpace(string(current)); s != "" {
				units = append(units, s)
			}
		}
	}
	return units
}
