package service

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// simpleSplitByLength is the fallback episode splitter for script drama and commentary comic.
// It divides text into roughly equal-length chunks without ad semantic heuristics.
func simpleSplitByLength(scriptText string, n int) []llmEpisode {
	trimmed := strings.TrimSpace(scriptText)
	if trimmed == "" {
		return nil
	}
	if n <= 0 {
		n = 1
	}

	runes := []rune(trimmed)
	total := len(runes)
	if total == 0 {
		return nil
	}

	chunkSize := total / n
	if chunkSize < 80 {
		chunkSize = 80
	}

	var episodes []llmEpisode
	start := 0
	for epIdx := 0; epIdx < n && start < total; epIdx++ {
		end := start + chunkSize
		if epIdx == n-1 {
			end = total
		}
		if end > total {
			end = total
		}
		if start >= end {
			break
		}

		// Prefer breaking at paragraph boundaries near the target end.
		if epIdx < n-1 && end < total {
			searchStart := end
			searchEnd := end + 400
			if searchEnd > total {
				searchEnd = total
			}
			bestBreak := -1
			for i := searchStart; i < searchEnd; i++ {
				if runes[i] == '\n' {
					bestBreak = i + 1
				}
			}
			if bestBreak > start {
				end = bestBreak
			}
		}

		excerpt := strings.TrimSpace(string(runes[start:end]))
		if excerpt == "" {
			start = end
			continue
		}
		summary := excerpt
		if utf8.RuneCountInString(summary) > 100 {
			summary = string([]rune(summary)[:100]) + "..."
		}
		episodes = append(episodes, llmEpisode{
			Title:   deriveLengthSplitTitle(excerpt, len(episodes)+1),
			Summary: summary,
			Excerpt: excerpt,
		})
		start = end
	}

	if len(episodes) == 0 {
		return []llmEpisode{{
			Title:   "第1集",
			Summary: trimmed,
			Excerpt: trimmed,
		}}
	}
	return episodes
}

func deriveLengthSplitTitle(excerpt string, episodeNum int) string {
	firstLine := excerpt
	if idx := strings.IndexAny(excerpt, "\n\r"); idx >= 0 {
		firstLine = excerpt[:idx]
	}
	firstLine = strings.TrimSpace(firstLine)
	if firstLine != "" && utf8.RuneCountInString(firstLine) <= 20 {
		return firstLine
	}
	return "第" + strconv.Itoa(episodeNum) + "集"
}
