package speechtext

import (
	"strings"
	"unicode/utf8"
)

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

// SpeakableRunesForDuration counts characters that will actually be spoken (speaker labels excluded).
func SpeakableRunesForDuration(dialogue string) int {
	dialogue = strings.TrimSpace(SanitizeForSpeech(dialogue))
	if dialogue == "" {
		return 0
	}
	total := 0
	for _, line := range strings.Split(dialogue, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if body := speakableBodyFromLine(line); body != "" {
			total += utf8.RuneCountInString(body)
		}
	}
	if total > 0 {
		return total
	}
	return utf8.RuneCountInString(dialogue)
}

func speakableBodyFromLine(line string) string {
	if m := speakerLinePattern.FindStringSubmatch(line); len(m) >= 3 {
		return strings.TrimSpace(m[2])
	}
	if m := speakerWithEmotionLine.FindStringSubmatch(line); len(m) >= 3 {
		return strings.TrimSpace(m[2])
	}
	return line
}

// InferClipDurationFromDialogue estimates clip seconds from speakable dialogue length.
func InferClipDurationFromDialogue(dialogue string, defaultDurationSec int, speechPace string) int {
	base := defaultDurationSec
	if base <= 0 {
		base = 5
	}
	runes := SpeakableRunesForDuration(dialogue)
	if runes <= 0 {
		return clampClipDurationSec(base, 2, 12)
	}
	charsPer10Sec := SpeechPaceCharsPer10Sec(speechPace)
	needed := float64(runes)/float64(charsPer10Sec)*10.0 + 0.45
	dur := int(needed + 0.5)
	maxDur := base + 4
	if maxDur > 12 {
		maxDur = 12
	}
	return clampClipDurationSec(dur, 2, maxDur)
}

func clampClipDurationSec(d, minSec, maxSec int) int {
	if d < minSec {
		return minSec
	}
	if d > maxSec {
		return maxSec
	}
	return d
}
