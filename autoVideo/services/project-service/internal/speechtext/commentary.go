package speechtext

import (
	"strings"
	"unicode/utf8"
)

// CountSubtitleTags returns how many [字幕:…] annotations exist in text.
func CountSubtitleTags(text string) int {
	return len(subtitleExtractPattern.FindAllString(text, -1))
}

// ExtractSubtitleSegments returns speakable narration lines from [字幕:…] tags in order.
func ExtractSubtitleSegments(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	segments := make([]string, 0, 8)
	for _, m := range subtitleExtractPattern.FindAllStringSubmatch(text, -1) {
		if len(m) < 2 {
			continue
		}
		if seg := strings.TrimSpace(m[1]); seg != "" {
			segments = append(segments, seg)
		}
	}
	return segments
}

// FinalizeCommentaryDialogue normalizes a single storyboard dialogue field for commentary TTS.
func FinalizeCommentaryDialogue(dialogue string) string {
	return FinalizeCommentaryDialogueWithLimit(dialogue, DefaultMaxClipDialogueRunes)
}

// FinalizeCommentaryDialogueWithLimit normalizes commentary dialogue and caps it to maxRunes.
func FinalizeCommentaryDialogueWithLimit(dialogue string, maxRunes int) string {
	dialogue = strings.TrimSpace(dialogue)
	if dialogue == "" {
		return ""
	}
	extracted := ExtractNarrationForSpeech(dialogue)
	if extracted == "" {
		extracted = strings.TrimSpace(SanitizeForSpeech(dialogue))
	}
	if extracted == "" ||
		LooksLikeSceneDescription(extracted) ||
		LooksLikeStoryboardVisualDescription(extracted) {
		return ""
	}
	return CompactClipDialogue(extracted, maxRunes)
}

func dialogueNeedsSourceRepair(dialogue string) bool {
	return FinalizeCommentaryDialogue(dialogue) == ""
}

// AlignCommentaryScenesWithSource repairs scene dialogue using authoritative [字幕:] segments
// from the episode source text. Scenes that already have valid narration are preserved.
func AlignCommentaryScenesWithSource(source string, scenes []SceneDialogue) []SceneDialogue {
	if len(scenes) == 0 {
		return scenes
	}
	segments := ExtractSubtitleSegments(source)
	segIdx := 0
	for i := range scenes {
		if finalized := FinalizeCommentaryDialogue(scenes[i].Dialogue); finalized != "" {
			scenes[i].Dialogue = finalized
			continue
		}
		if segIdx < len(segments) {
			scenes[i].Dialogue = segments[segIdx]
			segIdx++
			continue
		}
		scenes[i].Dialogue = ""
	}
	return scenes
}

// ExtractParagraphNarration pulls speakable commentary from a plain paragraph when no [字幕:] exists.
func ExtractParagraphNarration(paragraph string) string {
	paragraph = strings.TrimSpace(paragraph)
	if paragraph == "" {
		return ""
	}
	if narr := FinalizeCommentaryDialogue(paragraph); narr != "" {
		return narr
	}
	lines := strings.Split(paragraph, "\n")
	var parts []string
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || sceneSluglinePattern.MatchString(line) {
			continue
		}
		if CountSubtitleTags(line) > 0 {
			if narr := FinalizeCommentaryDialogue(line); narr != "" {
				parts = append(parts, narr)
			}
			continue
		}
		if LooksLikeSceneDescription(line) || LooksLikeStoryboardVisualDescription(line) {
			continue
		}
		if utf8.RuneCountInString(line) >= 10 && LooksLikeCompleteUtterance(line) {
			parts = append(parts, line)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// SceneDialogue is the minimal shape needed for commentary dialogue alignment.
type SceneDialogue struct {
	Dialogue string
}
