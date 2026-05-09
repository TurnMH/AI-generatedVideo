package service

// MotionPromptAuditor is a lightweight auditor for video motion prompts.
// It runs two passes (no LLM call) to keep motion prompt generation fast:
//  1. Rule-based sensitive word replacement.
//  2. Jaccard similarity deduplication with variation injection.

import (
	"fmt"
	"math"
	"strings"
	"unicode"

	"go.uber.org/zap"
)

// motionSensitiveEntries is a focused list for motion prompt sanitization.
// Motion prompts are short English camera/action directions and rarely contain
// sensitive content, so only the most likely triggers are included.
var motionSensitiveEntries = []struct{ word, replacement string }{
	{"blood", "fluid motion"},
	{"gore", "intense action"},
	{"nude", "figure"},
	{"naked", "figure"},
	{"corpse", "motionless subject"},
	{"dead body", "fallen subject"},
	{"torture", "intense scene"},
	{"suicide", "desperate action"},
	{"blood splatter", "dramatic spray"},
	{"brutal", "forceful"},
}

// MotionPromptAuditor audits motion prompts for sensitive content and deduplication.
type MotionPromptAuditor struct {
	dupThreshold float64 // Jaccard threshold; default 0.70
	logger       *zap.Logger
}

// newMotionPromptAuditor creates a lightweight auditor for motion prompts.
func newMotionPromptAuditor(logger *zap.Logger) *MotionPromptAuditor {
	return &MotionPromptAuditor{dupThreshold: 0.70, logger: logger}
}

// Audit processes all motion prompts: sensitive replacement then dedup variation.
// All prompts always produce output — sensitive words are replaced, never rejected.
func (a *MotionPromptAuditor) Audit(prompts []string) []string {
	if len(prompts) == 0 {
		return prompts
	}
	out := make([]string, len(prompts))
	copy(out, prompts)

	// Pass 1: sensitive word replacement.
	changed := 0
	for i, p := range out {
		cleaned, flags := motionScanReplace(p)
		if len(flags) > 0 {
			out[i] = cleaned
			changed++
			if a.logger != nil {
				a.logger.Info("motion_auditor: sensitive replacement",
					zap.Int("clip", i),
					zap.Strings("flags", flags),
				)
			}
		}
	}

	// Pass 2: deduplication — add variation suffix to near-duplicate prompts.
	dupVariations := []string{
		", camera angle shifted slightly left",
		", adjusting focus depth",
		", maintaining momentum from previous cut",
		", mirror-framed continuation",
		", zoom breathing adjusted",
		", lens compression varied",
	}
	variationIdx := 0
	for i := 1; i < len(out); i++ {
		for j := 0; j < i; j++ {
			if motionJaccard(out[i], out[j]) >= a.dupThreshold {
				suffix := dupVariations[variationIdx%len(dupVariations)]
				out[i] = strings.TrimRight(out[i], ".,") + suffix
				variationIdx++
				changed++
				if a.logger != nil {
					a.logger.Info("motion_auditor: diversified duplicate",
						zap.Int("clip", i),
						zap.Int("similar_to", j),
						zap.String("suffix", suffix),
					)
				}
				break
			}
		}
	}

	if a.logger != nil && changed > 0 {
		a.logger.Info("motion_auditor: audit complete",
			zap.Int("total", len(prompts)),
			zap.Int("changed", changed),
		)
	}
	return out
}

// ── helpers ──────────────────────────────────────────────────────────────────

func motionScanReplace(prompt string) (string, []string) {
	lower := strings.ToLower(prompt)
	result := prompt
	var flags []string
	for _, e := range motionSensitiveEntries {
		if strings.Contains(lower, e.word) {
			result = motionReplaceCI(result, e.word, e.replacement)
			lower = strings.ToLower(result)
			flags = append(flags, fmt.Sprintf("sensitive:%s", e.word))
		}
	}
	return result, flags
}

func motionReplaceCI(s, old, replacement string) string {
	lowerS := strings.ToLower(s)
	var b strings.Builder
	for {
		idx := strings.Index(lowerS, old)
		if idx < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:idx])
		b.WriteString(replacement)
		s = s[idx+len(old):]
		lowerS = strings.ToLower(s)
	}
	return b.String()
}

func motionJaccard(a, b string) float64 {
	setA := motionTokenSet(a)
	setB := motionTokenSet(b)
	if len(setA) == 0 && len(setB) == 0 {
		return 1.0
	}
	var inter int
	for w := range setA {
		if setB[w] {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return math.Round(float64(inter)/float64(union)*100) / 100
}

func motionTokenSet(text string) map[string]bool {
	set := map[string]bool{}
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			set[strings.ToLower(string(cur))] = true
			cur = cur[:0]
		}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
	return set
}
