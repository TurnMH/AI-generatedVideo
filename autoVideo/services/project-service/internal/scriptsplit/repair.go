package scriptsplit

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	summaryTrailerKeywordRe = regexp.MustCompile(`简介|预告|全书|整本|最终|结局|倒闭|五百万|逆袭|一口气|带你|盘点|梳理|概括|总览`)
	frontMatterTitleRe        = regexp.MustCompile(`(?i)^序$|简介|导语|引子|楔子|前言|序言|预告|文案`)
)

// RepairSplit applies rule-based fixes for common bad episode boundaries.
func RepairSplit(episodes []DraftEpisode) ([]DraftEpisode, []string) {
	if len(episodes) == 0 {
		return episodes, nil
	}

	var actions []string
	out := append([]DraftEpisode(nil), episodes...)

	for pass := 0; pass < 3; pass++ {
		changed := false
		if len(out) >= 2 && shouldMergeSummaryTrailer(out[0], out[1]) {
			merged := mergeDraftEpisodes(out[0], out[1])
			out[0] = merged
			out = append(out[:1], out[2:]...)
			actions = append(actions, "merge_summary_trailer_into_next")
			changed = true
		}
		if len(out) >= 2 && isFrontMatterEpisode(out[0]) {
			merged := mergeDraftEpisodes(out[0], out[1])
			out[0] = merged
			out = append(out[:1], out[2:]...)
			actions = append(actions, "merge_front_matter_into_next")
			changed = true
		}
		for i := 0; i+1 < len(out); i++ {
			if overlapRatio(out[i].Excerpt, out[i+1].Excerpt) >= 0.35 {
				merged := mergeDraftEpisodes(out[i], out[i+1])
				out[i] = merged
				out = append(out[:i+1], out[i+2:]...)
				actions = append(actions, "merge_high_overlap")
				changed = true
				break
			}
		}
		if !changed {
			break
		}
	}

	out = compactEmptyDraftEpisodes(out)
	return out, actions
}

// NeedsStructuralReview reports whether episodes should go through LLM split review.
func NeedsStructuralReview(episodes []DraftEpisode) bool {
	if len(episodes) < 2 {
		return false
	}
	if shouldMergeSummaryTrailer(episodes[0], episodes[1]) {
		return true
	}
	if isFrontMatterEpisode(episodes[0]) {
		return true
	}
	for i := 0; i+1 < len(episodes); i++ {
		if overlapRatio(episodes[i].Excerpt, episodes[i+1].Excerpt) >= 0.35 {
			return true
		}
	}
	return false
}

func shouldMergeSummaryTrailer(first, second DraftEpisode) bool {
	firstLen := utf8.RuneCountInString(strings.TrimSpace(first.Excerpt))
	secondLen := utf8.RuneCountInString(strings.TrimSpace(second.Excerpt))
	if firstLen == 0 || secondLen == 0 {
		return false
	}
	if firstLen > 700 {
		return false
	}
	if secondLen < firstLen*2 {
		return false
	}
	text := first.Title + "\n" + first.Excerpt
	if summaryTrailerKeywordRe.MatchString(text) {
		return true
	}
	if !strings.ContainsAny(first.Excerpt, "“”「」『』\"") && firstLen < 500 {
		return summaryTrailerKeywordRe.MatchString(first.Excerpt) || strings.Contains(first.Title, "预告")
	}
	return false
}

func isFrontMatterEpisode(ep DraftEpisode) bool {
	title := strings.TrimSpace(ep.Title)
	if title == "" {
		return false
	}
	if !frontMatterTitleRe.MatchString(title) {
		return false
	}
	return utf8.RuneCountInString(strings.TrimSpace(ep.Excerpt)) <= 900
}

func mergeDraftEpisodes(a, b DraftEpisode) DraftEpisode {
	title := strings.TrimSpace(b.Title)
	if title == "" {
		title = strings.TrimSpace(a.Title)
	}
	excerpt := strings.TrimSpace(strings.TrimSpace(a.Excerpt) + "\n\n" + strings.TrimSpace(b.Excerpt))
	summary := strings.TrimSpace(b.Summary)
	if summary == "" {
		summary = strings.TrimSpace(a.Summary)
	}
	if utf8.RuneCountInString(summary) > 200 {
		summary = string([]rune(summary)[:200]) + "..."
	}
	return DraftEpisode{Title: title, Summary: summary, Excerpt: excerpt}
}

func compactEmptyDraftEpisodes(episodes []DraftEpisode) []DraftEpisode {
	out := episodes[:0]
	for _, ep := range episodes {
		if strings.TrimSpace(ep.Excerpt) == "" {
			continue
		}
		out = append(out, ep)
	}
	return out
}

func overlapRatio(a, b string) float64 {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return 0
	}
	shorter, longer := a, b
	if utf8.RuneCountInString(a) > utf8.RuneCountInString(b) {
		shorter, longer = b, a
	}
	runes := []rune(shorter)
	if len(runes) < 40 {
		return 0
	}
	window := minInt(len(runes), 120)
	sample := string(runes[:window])
	if strings.Contains(longer, sample) {
		return float64(window) / float64(maxInt(utf8.RuneCountInString(longer), 1))
	}
	return 0
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// LooksLikeSynopsisOnly reports whether text is likely non-narrative front matter.
func LooksLikeSynopsisOnly(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if strings.ContainsAny(text, "“”「」『』\"") {
		return false
	}
	if utf8.RuneCountInString(text) >= 700 {
		return false
	}
	return summaryTrailerKeywordRe.MatchString(text)
}
