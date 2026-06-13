package scriptsplit

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	bracketHeaderLineRe = regexp.MustCompile(`(?m)^[　 \t]*【([^】]{1,24})】[　 \t]*(.*)$`)
	plainSynopsisLineRe = regexp.MustCompile(`(?m)^[　 \t]*(简介|内容简介|作品简介)[：:][　 \t]*(.*)$`)
)

var synopsisHeaders = map[string]struct{}{
	"简介": {}, "内容简介": {}, "作品简介": {}, "文案简介": {}, "内容提要": {},
}

var dropHeaders = map[string]struct{}{
	"作者有话说": {}, "作者说": {}, "写在前面": {}, "声明": {}, "上架感言": {},
	"书籍相关": {}, "请君入坑": {}, "完结感言": {}, "更新说明": {},
}

var prologueHeaders = map[string]struct{}{
	"导语": {}, "引子": {}, "楔子": {}, "序章": {}, "序言": {}, "前言": {}, "序": {},
}

// NormalizeResult holds script text prepared for episode splitting.
type NormalizeResult struct {
	Text             string
	StrippedSynopsis string
	RemovedBlocks    []string
	Changed          bool
}

type blockKind int

const (
	blockUnknown blockKind = iota
	blockSynopsis
	blockDrop
	blockPrologue
)

func classifyHeader(header string) blockKind {
	header = strings.TrimSpace(header)
	if _, ok := synopsisHeaders[header]; ok {
		return blockSynopsis
	}
	if _, ok := dropHeaders[header]; ok {
		return blockDrop
	}
	if _, ok := prologueHeaders[header]; ok {
		return blockPrologue
	}
	return blockUnknown
}

func parseLeadingBlockHeader(firstLine string) (header, inlineBody string, ok bool) {
	firstLine = strings.TrimSpace(firstLine)
	if firstLine == "" {
		return "", "", false
	}
	if m := bracketHeaderLineRe.FindStringSubmatch(firstLine); len(m) >= 3 {
		return strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), true
	}
	if m := plainSynopsisLineRe.FindStringSubmatch(firstLine); len(m) >= 3 {
		return strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), true
	}
	return "", "", false
}

// NormalizeForEpisodeSplit strips non-narrative front matter while keeping scene-like prologues.
func NormalizeForEpisodeSplit(text string) NormalizeResult {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	original := strings.TrimSpace(text)
	var synopsisParts []string
	var removed []string

	for {
		text = strings.TrimLeft(text, "\n")
		text = strings.TrimSpace(text)
		if text == "" {
			break
		}
		if findNextChapterMarker(text) == 0 {
			break
		}

		lineEnd := strings.IndexByte(text, '\n')
		firstLine := text
		if lineEnd >= 0 {
			firstLine = text[:lineEnd]
		}

		header, inlineBody, ok := parseLeadingBlockHeader(firstLine)
		if !ok {
			break
		}

		block, consumed := extractLeadingBlock(text, header, inlineBody)
		if consumed <= 0 {
			break
		}

		switch classifyHeader(header) {
		case blockPrologue:
			if looksLikeNarrativePrologue(block) {
				goto done
			}
			if body := strings.TrimSpace(block); body != "" {
				synopsisParts = append(synopsisParts, body)
			}
			removed = append(removed, header)
		case blockSynopsis:
			if body := strings.TrimSpace(block); body != "" {
				synopsisParts = append(synopsisParts, body)
			}
			removed = append(removed, header)
		case blockDrop:
			removed = append(removed, header)
		default:
			goto done
		}

		text = strings.TrimSpace(text[consumed:])
	}

done:
	result := NormalizeResult{
		Text:          strings.TrimSpace(text),
		RemovedBlocks: removed,
		Changed:       strings.TrimSpace(text) != original,
	}
	if len(synopsisParts) > 0 {
		result.StrippedSynopsis = strings.Join(synopsisParts, "\n\n")
	}
	return result
}

func extractLeadingBlock(fullText, header, inlineBody string) (block string, consumed int) {
	if inlineBody != "" {
		lineEnd := strings.IndexByte(fullText, '\n')
		if lineEnd < 0 {
			return inlineBody, len(fullText)
		}
		return inlineBody, lineEnd + 1
	}

	lines := strings.Split(fullText, "\n")
	if len(lines) == 0 {
		return "", 0
	}

	var bodyLines []string
	consumed = len(lines[0]) + 1
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			bodyLines = append(bodyLines, line)
			consumed += len(line) + 1
			continue
		}
		if _, _, isHeader := parseLeadingBlockHeader(line); isHeader {
			break
		}
		if findNextChapterMarker(line) == 0 {
			break
		}
		bodyLines = append(bodyLines, line)
		consumed += len(line) + 1
	}
	if consumed > len(fullText) {
		consumed = len(fullText)
	}
	_ = header
	return strings.TrimSpace(strings.Join(bodyLines, "\n")), consumed
}

func looksLikeNarrativePrologue(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if strings.ContainsAny(text, "“”「」『』\"") {
		return true
	}
	narrativeHints := []string{"说", "道", "问", "喊", "叫", "唤", "看", "走", "进", "出", "站", "坐", "拿", "推", "门", "铺", "屋"}
	hits := 0
	for _, hint := range narrativeHints {
		if strings.Contains(text, hint) {
			hits++
		}
	}
	return utf8.RuneCountInString(text) >= 60 && hits >= 2
}
