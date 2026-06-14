package speechtext

import "strings"

// FitStoryboardDialogue compacts a storyboard dialogue field to what the clip duration can carry.
func FitStoryboardDialogue(dialogue string, durationSec int, speechPace string, commentary bool) string {
	dialogue = strings.TrimSpace(dialogue)
	if dialogue == "" {
		return ""
	}
	maxRunes := MaxRunesForClipDuration(durationSec, speechPace)
	if commentary {
		return FinalizeCommentaryDialogueWithLimit(dialogue, maxRunes)
	}
	return CompactClipDialogue(SanitizeForSpeech(dialogue), maxRunes)
}
