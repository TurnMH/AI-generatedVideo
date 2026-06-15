package speechtext

import (
	"strings"
	"unicode/utf8"
)

// ExtractCommentarySpeechUnits returns ordered speakable narration units from episode source text.
func ExtractCommentarySpeechUnits(source string) []string {
	source = strings.TrimSpace(strings.ReplaceAll(source, "\r\n", "\n"))
	if source == "" {
		return nil
	}
	if segments := ExtractSubtitleSegments(source); len(segments) > 0 {
		return filterSpeakableUnits(segments)
	}
	return filterSpeakableUnits(extractOrderedSpeechUnits(source))
}

func extractOrderedSpeechUnits(source string) []string {
	var units []string
	paragraphs := strings.Split(source, "\n\n")
	if len(paragraphs) <= 1 {
		paragraphs = strings.Split(source, "\n")
	}
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" || isCommentaryStructuralHeading(paragraph) {
			continue
		}
		for _, m := range subtitleExtractPattern.FindAllStringSubmatch(paragraph, -1) {
			if len(m) > 1 {
				if v := strings.TrimSpace(m[1]); v != "" {
					units = append(units, v)
				}
			}
		}
		if CountSubtitleTags(paragraph) > 0 {
			continue
		}
		for _, m := range quotedSpeechPattern.FindAllStringSubmatch(paragraph, -1) {
			if len(m) > 1 {
				if v := strings.TrimSpace(m[1]); v != "" {
					units = append(units, v)
				}
			}
		}
		plain := subtitleExtractPattern.ReplaceAllString(paragraph, " ")
		plain = quotedSpeechPattern.ReplaceAllString(plain, " ")
		plain = strings.TrimSpace(plain)
		if plain == "" {
			continue
		}
		for _, unit := range splitSpeechUnits(plain) {
			unit = strings.TrimSpace(unit)
			if unit == "" || LooksLikeSceneDescription(unit) || LooksLikeStoryboardVisualDescription(unit) {
				continue
			}
			if utf8.RuneCountInString(unit) >= 10 || quotedSpeechPattern.MatchString(unit) {
				units = append(units, unit)
			}
		}
	}
	return units
}

func isCommentaryStructuralHeading(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}
	if strings.HasPrefix(text, "【") && strings.HasSuffix(text, "】") && utf8.RuneCountInString(text) <= 12 {
		return true
	}
	if len(text) <= 24 && strings.Contains(text, "导语") {
		return true
	}
	return false
}

// SplitSpeechUnitsForPacking exposes sentence-level splitting for clip packing.
func SplitSpeechUnitsForPacking(text string) []string {
	return splitSpeechUnits(text)
}

// MergeAdjacentSpeechUnits combines consecutive short units before clip packing.
func MergeAdjacentSpeechUnits(units []string, minRunes int) []string {
	if minRunes <= 0 {
		minRunes = 18
	}
	var out []string
	var batch []string
	batchRunes := 0
	flush := func() {
		if len(batch) == 0 {
			return
		}
		out = append(out, joinSpeechUnits(batch))
		batch = nil
		batchRunes = 0
	}
	for _, unit := range units {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		unitRunes := utf8.RuneCountInString(unit)
		if batchRunes > 0 && batchRunes+unitRunes <= minRunes*3 {
			batch = append(batch, unit)
			batchRunes += unitRunes
			if batchRunes >= minRunes {
				flush()
			}
			continue
		}
		if unitRunes < minRunes {
			if len(batch) == 0 {
				batch = append(batch, unit)
				batchRunes = unitRunes
				continue
			}
			batch = append(batch, unit)
			batchRunes += unitRunes
			if batchRunes >= minRunes {
				flush()
			}
			continue
		}
		flush()
		out = append(out, unit)
	}
	flush()
	return out
}

// PackSpeechUnitsToMaxRunes groups narration units into clip-sized dialogue chunks.
func PackSpeechUnitsToMaxRunes(units []string, maxRunes int) []string {
	if maxRunes <= 0 {
		maxRunes = DefaultMaxClipDialogueRunes
	}
	minPack := maxRunes * 50 / 100
	if minPack < 18 {
		minPack = 18
	}
	units = MergeAdjacentSpeechUnits(units, minPack)
	var packed []string
	var batch []string
	batchRunes := 0

	flush := func(force bool) {
		if len(batch) == 0 {
			return
		}
		joined := joinSpeechUnits(batch)
		if !force && utf8.RuneCountInString(joined) < minPack && len(batch) > 0 {
			return
		}
		packed = append(packed, joined)
		batch = nil
		batchRunes = 0
	}

	for _, unit := range units {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		unitRunes := utf8.RuneCountInString(unit)
		if unitRunes > maxRunes {
			flush(true)
			subUnits := splitSpeechUnits(unit)
			if len(subUnits) <= 1 {
				packed = append(packed, splitTextByRunes(unit, maxRunes)...)
			} else {
				packed = append(packed, PackSpeechUnitsToMaxRunes(subUnits, maxRunes)...)
			}
			continue
		}
		sepRunes := 0
		if batchRunes > 0 {
			sepRunes = 1
		}
		if batchRunes > 0 && batchRunes+sepRunes+unitRunes > maxRunes {
			flush(true)
		}
		batch = append(batch, unit)
		batchRunes += sepRunes + unitRunes
	}
	flush(true)
	return packed
}

// CommentarySpeechRunes counts speakable characters across extracted narration units.
func CommentarySpeechRunes(source string) int {
	total := 0
	for _, unit := range ExtractCommentarySpeechUnits(source) {
		total += utf8.RuneCountInString(unit)
	}
	return total
}

func filterSpeakableUnits(units []string) []string {
	out := make([]string, 0, len(units))
	seen := make(map[string]struct{}, len(units))
	for _, unit := range units {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		if LooksLikeSceneDescription(unit) || LooksLikeStoryboardVisualDescription(unit) {
			continue
		}
		if utf8.RuneCountInString(unit) < 6 && !quotedSpeechPattern.MatchString(unit) {
			continue
		}
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
	return out
}

func joinSpeechUnits(units []string) string {
	if len(units) == 0 {
		return ""
	}
	if len(units) == 1 {
		return strings.TrimSpace(units[0])
	}
	var b strings.Builder
	for i, unit := range units {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		if b.Len() > 0 {
			if strings.HasSuffix(strings.TrimSpace(b.String()), "。") ||
				strings.HasSuffix(strings.TrimSpace(b.String()), "！") ||
				strings.HasSuffix(strings.TrimSpace(b.String()), "？") {
				b.WriteString("")
			} else if !strings.HasSuffix(unit, "。") && !strings.HasSuffix(unit, "！") && !strings.HasSuffix(unit, "？") {
				b.WriteString("，")
			}
		}
		b.WriteString(unit)
		if i == len(units)-1 && !strings.HasSuffix(unit, "。") && !strings.HasSuffix(unit, "！") && !strings.HasSuffix(unit, "？") {
			if utf8.RuneCountInString(unit) >= 8 {
				b.WriteString("。")
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func splitTextByRunes(text string, maxRunes int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return []string{text}
	}
	var out []string
	for start := 0; start < len(runes); {
		end := start + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		window := runes[start:end]
		if end < len(runes) {
			if idx := lastBreakIndex(window); idx > maxRunes/3 {
				window = window[:idx]
				end = start + idx
			}
		}
		chunk := strings.TrimSpace(string(window))
		if chunk != "" {
			if end >= len(runes) && !strings.ContainsAny(chunk, "。！？!?") {
				out = append(out, chunk)
			} else {
				out = append(out, truncateAtSentenceBoundary(chunk))
			}
		}
		if end >= len(runes) {
			break
		}
		start = end
	}
	return out
}

func lastBreakIndex(runes []rune) int {
	for i := len(runes) - 1; i >= 0; i-- {
		switch runes[i] {
		case '，', ',', '；', ';', '、', ' ':
			return i
		}
	}
	return -1
}
