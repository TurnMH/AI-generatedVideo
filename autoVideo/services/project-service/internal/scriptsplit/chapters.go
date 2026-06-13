package scriptsplit

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// DraftEpisode is a split episode before persistence.
type DraftEpisode struct {
	Title   string
	Summary string
	Excerpt string
}

var (
	classicChapterLineRe = regexp.MustCompile(
		`(?m)^[　 \t]*(` +
			`第[零一二三四五六七八九十百千万\d]+[回章节集卷幕]` +
			`|【?第[零一二三四五六七八九十百千万\d]+[回章节集卷幕】]?` +
			`|Chapter\s+\d+` +
			`|CHAPTER\s+\d+` +
			`|序[章言幕]` +
			`|楔\s*子` +
			`|引\s*子` +
			`|尾\s*声` +
			`|终\s*章` +
			`|番\s*外` +
			`)[　 \t]*(.*)$`,
	)
	numericChapterLineRe = regexp.MustCompile(`(?m)^[　 \t]*(\d{1,3})[　 \t]*$`)
)

type chapterMarker struct {
	pos   int
	title string
}

// findNextChapterMarker returns the byte index of the next chapter marker in text, or -1.
func findNextChapterMarker(text string) int {
	markers := findChapterMarkers(text)
	if len(markers) == 0 {
		return -1
	}
	return markers[0].pos
}

func findChapterMarkers(text string) []chapterMarker {
	var markers []chapterMarker

	for _, loc := range classicChapterLineRe.FindAllStringIndex(text, -1) {
		lineEnd := strings.IndexByte(text[loc[0]:], '\n')
		titleLine := text[loc[0]:]
		if lineEnd >= 0 {
			titleLine = titleLine[:lineEnd]
		}
		if !isLikelyChapterHeadingLine(titleLine) {
			continue
		}
		markers = append(markers, chapterMarker{pos: loc[0], title: strings.TrimSpace(titleLine)})
	}

	numericMatches := numericChapterLineRe.FindAllStringSubmatchIndex(text, -1)
	if len(numericMatches) >= 2 {
		for _, loc := range numericMatches {
			lineEnd := strings.IndexByte(text[loc[0]:], '\n')
			titleLine := text[loc[0]:]
			if lineEnd >= 0 {
				titleLine = titleLine[:lineEnd]
			}
			markers = append(markers, chapterMarker{pos: loc[0], title: strings.TrimSpace(titleLine)})
		}
	}

	if len(markers) == 0 {
		return nil
	}

	sort.Slice(markers, func(i, j int) bool {
		if markers[i].pos == markers[j].pos {
			return len(markers[i].title) > len(markers[j].title)
		}
		return markers[i].pos < markers[j].pos
	})

	deduped := markers[:0]
	for _, m := range markers {
		if len(deduped) > 0 && deduped[len(deduped)-1].pos == m.pos {
			continue
		}
		deduped = append(deduped, m)
	}
	return deduped
}

// SplitByChapters splits text at chapter boundaries. Preamble before the first marker
// is merged into the first episode when it looks narrative.
func SplitByChapters(text string) []DraftEpisode {
	text = strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n")
	if text == "" {
		return nil
	}

	markers := findChapterMarkers(text)
	if len(markers) == 0 {
		return nil
	}

	var preamble string
	if markers[0].pos > 0 {
		preamble = strings.TrimSpace(text[:markers[0].pos])
	}

	var episodes []DraftEpisode
	for i, marker := range markers {
		start := marker.pos
		end := len(text)
		if i+1 < len(markers) {
			end = markers[i+1].pos
		}

		chapterText := strings.TrimSpace(text[start:end])
		if chapterText == "" {
			continue
		}

		if i == 0 && preamble != "" && shouldAttachPreambleToFirstEpisode(preamble) {
			chapterText = strings.TrimSpace(preamble + "\n\n" + chapterText)
		}

		title := marker.title
		if utf8.RuneCountInString(title) > 50 {
			title = string([]rune(title)[:50])
		}

		body := chapterText
		if firstNewline := strings.IndexAny(chapterText, "\n\r"); firstNewline > 0 {
			body = strings.TrimSpace(chapterText[firstNewline:])
		}
		summary := body
		if utf8.RuneCountInString(summary) > 200 {
			summary = string([]rune(summary)[:200]) + "..."
		}

		episodes = append(episodes, DraftEpisode{
			Title:   title,
			Summary: summary,
			Excerpt: chapterText,
		})
	}

	return validateChapterSplit(episodes)
}

func isLikelyChapterHeadingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if strings.ContainsAny(line, "。！？；") {
		return false
	}
	if utf8.RuneCountInString(line) > 80 {
		return false
	}
	return true
}

func validateChapterSplit(episodes []DraftEpisode) []DraftEpisode {
	if len(episodes) == 0 {
		return nil
	}
	total := 0
	for _, ep := range episodes {
		total += utf8.RuneCountInString(strings.TrimSpace(ep.Excerpt))
	}
	if total/len(episodes) < 60 {
		return nil
	}
	for _, ep := range episodes {
		if utf8.RuneCountInString(strings.TrimSpace(ep.Excerpt)) < 20 {
			return nil
		}
	}
	return episodes
}

func shouldAttachPreambleToFirstEpisode(preamble string) bool {
	preamble = strings.TrimSpace(preamble)
	if preamble == "" {
		return false
	}
	if looksLikeNarrativePrologue(preamble) {
		return true
	}
	// Short narrative lead-ins (e.g. kept 【导语】 blocks) belong with chapter 01.
	return utf8.RuneCountInString(preamble) <= 800 && strings.ContainsAny(preamble, "，。！？")
}
