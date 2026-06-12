package productionmode

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var dramaSceneHeadingPattern = regexp.MustCompile(`【\s*(?:内景|外景|内外景)`)
var subtitleTagPattern = regexp.MustCompile(`\[字幕[:：][^\]]+\]`)
var dramaSpeakerLinePattern = regexp.MustCompile(`(?m)^[\p{Han}A-Za-z·]{1,8}[（(][^)）]{0,24}[）)]\s*$`)

// CommentaryFormatIssue describes one commentary-script format validation failure.
type CommentaryFormatIssue struct {
	Type        string
	Description string
	Suggestion  string
}

// NeedsCommentaryFormatRepair reports whether optimized text looks like misformatted script drama
// instead of narration-driven commentary comic output.
func NeedsCommentaryFormatRepair(text string) bool {
	return len(CommentaryFormatIssues(text)) > 0
}

// CommentaryFormatIssues returns validation failures for commentary scripts.
func CommentaryFormatIssues(text string) []CommentaryFormatIssue {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var issues []CommentaryFormatIssue
	subtitleCount := len(subtitleTagPattern.FindAllString(text, -1))
	dramaHeadingCount := len(dramaSceneHeadingPattern.FindAllString(text, -1))
	speakerCueCount := len(dramaSpeakerLinePattern.FindAllString(text, -1))

	appendIssue := func(desc string) {
		issues = append(issues, CommentaryFormatIssue{
			Type:        "narration_gap",
			Description: desc,
			Suggestion:  "改回旁白驱动结构：为所有会被配音念出的旁白添加 [字幕:…] 标注；画面动作用普通描写补充；不要写成【内景/外景】短剧场景剧本或角色（情绪）+碎台词格式",
		})
	}

	if dramaHeadingCount > 0 && subtitleCount == 0 {
		appendIssue("文稿含【内景/外景】短剧场景标题，但缺少 [字幕:] 旁白标注")
	}
	if speakerCueCount >= 2 && subtitleCount == 0 {
		appendIssue("文稿以角色（情绪）+短台词的短剧格式为主，旁白讲解结构缺失")
	}
	if subtitleCount == 0 && utf8.RuneCountInString(text) >= 200 {
		appendIssue("长文稿未标注任何 [字幕:] 可配音旁白，TTS 将无法稳定提取讲解内容")
	}
	return issues
}
