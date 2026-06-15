package service

import (
	"fmt"
	"regexp"
	"strings"
)

var srtTimestampLineRe = regexp.MustCompile(`(\d{2}:\d{2}:\d{2},\d{3}) --> (\d{2}:\d{2}:\d{2},\d{3})`)

// computeClipWindowStarts returns the start time of each clip in the final
// concatenated timeline, accounting for xfade/acrossfade overlap when present.
func computeClipWindowStarts(durations []float64, transitions []string, transitionDurations []float64) []float64 {
	n := len(durations)
	starts := make([]float64, n)
	if n == 0 {
		return starts
	}
	starts[0] = 0
	if n == 1 || len(transitions) == 0 {
		for i := 1; i < n; i++ {
			starts[i] = starts[i-1] + durations[i-1]
		}
		return starts
	}
	_, transDurs := normalizeXfadePlan(durations, transitions, transitionDurations)
	elapsed := 0.0
	for i := 1; i < n; i++ {
		elapsed += durations[i-1] - transDurs[i-1]
		starts[i] = elapsed
	}
	return starts
}

// buildPerClipTimedSRT assigns each clip's dialogue to that clip's timeline
// window instead of spreading all lines across the full video duration.
func buildPerClipTimedSRT(dialogues []string, clipDurations []float64, transitions []string, transitionDurations []float64) string {
	if len(dialogues) == 0 || len(clipDurations) == 0 {
		return ""
	}
	count := len(clipDurations)
	if len(dialogues) < count {
		count = len(dialogues)
	}
	starts := computeClipWindowStarts(clipDurations, transitions, transitionDurations)

	var out strings.Builder
	seq := 1
	for i := 0; i < count; i++ {
		raw := strings.TrimSpace(dialogues[i])
		if raw == "" {
			continue
		}
		text := SpokenTextForPlayback(cleanScriptForSpeech(raw))
		if text == "" {
			continue
		}
		windowDur := clipDurations[i]
		if windowDur <= 0 {
			windowDur = 5
		}
		chunk := buildTimedSRT(text, windowDur)
		rebased := rebaseSRTTimestamps(chunk, starts[i])
		for _, block := range splitSRTBlocks(rebased) {
			if strings.TrimSpace(block) == "" {
				continue
			}
			lines := strings.Split(strings.TrimSpace(block), "\n")
			if len(lines) < 3 {
				continue
			}
			fmt.Fprintf(&out, "%d\n%s\n%s\n\n", seq, lines[1], strings.Join(lines[2:], "\n"))
			seq++
		}
	}
	return out.String()
}

func rebaseSRTTimestamps(srt string, offsetSec float64) string {
	if offsetSec <= 0 {
		return srt
	}
	return srtTimestampLineRe.ReplaceAllStringFunc(srt, func(match string) string {
		parts := srtTimestampLineRe.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		start := parseSRTTimestamp(parts[1]) + offsetSec
		end := parseSRTTimestamp(parts[2]) + offsetSec
		return fmt.Sprintf("%s --> %s", secondsToSRTTimestamp(start), secondsToSRTTimestamp(end))
	})
}

func parseSRTTimestamp(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	ms := 0.0
	if commaIdx := strings.Index(value, ","); commaIdx >= 0 {
		_, _ = fmt.Sscanf(value[commaIdx+1:], "%f", &ms)
		value = value[:commaIdx]
	}
	var h, m int
	var s float64
	if _, err := fmt.Sscanf(value, "%d:%d:%f", &h, &m, &s); err != nil {
		return 0
	}
	return float64(h*3600+m*60) + s + ms/1000.0
}

func splitSRTBlocks(srt string) []string {
	normalized := strings.ReplaceAll(strings.TrimSpace(srt), "\r\n", "\n")
	if normalized == "" {
		return nil
	}
	return strings.Split(normalized, "\n\n")
}

// shouldUseEpisodeLevelAudio avoids attaching one long episode dubbing track onto
// a multi-clip video that already carries per-clip dialogue — that mismatch is
// the main cause of narration drifting away from frames.
func shouldUseEpisodeLevelAudio(perClipDialogues []string, clipCount int, perClipAudioUsed bool) bool {
	if perClipAudioUsed {
		return false
	}
	if !hasAnyNonEmpty(perClipDialogues) {
		return true
	}
	return clipCount <= 1
}
