package speechtext

import "strings"

// MaxRunesForClipDuration estimates how many Chinese characters fit naturally in one clip.
func MaxRunesForClipDuration(durationSec int, speechPace string) int {
	duration := durationSec
	if duration <= 0 {
		duration = 5
	}
	if duration < 3 {
		duration = 3
	}
	if duration > 20 {
		duration = 20
	}
	charsPer10Sec := SpeechPaceCharsPer10Sec(speechPace)
	maxRunes := charsPer10Sec * duration / 10
	if maxRunes < 16 {
		maxRunes = 16
	}
	if maxRunes > 120 {
		maxRunes = 120
	}
	return maxRunes
}

// SpeechPaceCharsPer10Sec mirrors the storyboard split speech pace hints.
func SpeechPaceCharsPer10Sec(speechPace string) int {
	switch strings.TrimSpace(strings.ToLower(speechPace)) {
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
