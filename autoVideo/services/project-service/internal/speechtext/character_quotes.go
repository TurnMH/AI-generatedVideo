package speechtext

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

type CharacterQuote struct {
	Speaker string
	Quote   string
}

var (
	quotedSpeechExtractPattern = regexp.MustCompile(`[“「『"]([^”」』"]+)[”」』"]`)
	speakerBeforeQuotePattern  = regexp.MustCompile(`([\p{Han}A-Za-z·]{2,8})(?:[（(][^)）]{0,16}[）)])?(?:[^"「『"]{0,32})?(?:说|喊|叫|问|答|回应|道|唤|开口|低声|轻声|沉声|冷声|怒|笑)[^"「『"]{0,12}[“「『"]`)
)

// ExtractCharacterQuotesFromScene pulls quoted character lines from scene/action descriptions.
func ExtractCharacterQuotesFromScene(sceneText string, characters []string) []CharacterQuote {
	sceneText = strings.TrimSpace(strings.ReplaceAll(sceneText, "\r\n", "\n"))
	if sceneText == "" {
		return nil
	}

	seen := make(map[string]struct{})
	out := make([]CharacterQuote, 0, 4)
	for _, m := range quotedSpeechExtractPattern.FindAllStringSubmatchIndex(sceneText, -1) {
		if len(m) < 4 {
			continue
		}
		quote := strings.TrimSpace(sceneText[m[2]:m[3]])
		if quote == "" || utf8.RuneCountInString(quote) > 120 {
			continue
		}
		if LooksLikeStoryboardVisualDescription(quote) || LooksLikeSceneDescription(quote) {
			continue
		}
		start := m[0]
		if start < 0 {
			start = 0
		}
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
		out = append(out, CharacterQuote{Speaker: speaker, Quote: quote})
	}
	return out
}

func inferSpeakerBeforeQuote(before string, characters []string, quote string) string {
	before = strings.TrimSpace(before)
	if before == "" {
		return pickFallbackSpeaker(characters, quote)
	}
	if m := speakerBeforeQuotePattern.FindStringSubmatch(before); len(m) >= 2 {
		if speaker := normalizeCharacterName(m[1]); speaker != "" && speaker != quote {
			return speaker
		}
	}
	// Last mentioned character name before the quote.
	lastIdx := -1
	lastName := ""
	for _, name := range characters {
		n := normalizeCharacterName(name)
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
		n := normalizeCharacterName(name)
		if n == "" || n == quote {
			continue
		}
		return n
	}
	return ""
}

func normalizeCharacterName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "[]()（）【】")
	return strings.TrimSpace(name)
}

func DedupeSpeechLines(lines []string) []string {
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
