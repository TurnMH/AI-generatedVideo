package scriptpreserve

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const minLockedDialogueRunes = 2

var (
	subtitleTagPattern = regexp.MustCompile(`\[(?:字幕|对白|台词|独白|旁白|内心独白|画外音|解说)[:：]\s*([^\]]+?)\s*\]`)
	quotedSpeechPattern = regexp.MustCompile(`[“「『"]([^”」』"]+)[”」』"]`)
	screenplaySpeakerPattern = regexp.MustCompile(`^([\p{Han}A-Za-z·]{1,16})(?:[（(][^)）]{0,30}[）)])?\s*$`)
	indentedDialoguePattern = regexp.MustCompile(`^[\s　]{2,}(.+)$`)
	inlineSpeakerPattern = regexp.MustCompile(`([\p{Han}A-Za-z·]{1,16})(?:[（(][^)）]{0,30}[）)])?\s*(?:说|问|答|喊|叫|道|回|叹|笑|哭|怒|吼|低语|开口|回应|反驳|继续|接着)\s*[：:]?\s*[“「『"]([^”」』"]+)[”」』"]`)
)

// LockedDialogue is a speakable line extracted from the source manuscript.
type LockedDialogue struct {
	Text         string
	Raw          string
	AnchorBefore string
}

// DialoguePreservationDirective returns shared LLM rules for locked dialogue.
func DialoguePreservationDirective() string {
	return `**人物对白锁定（强制）：**
- 原稿中已出现的人物对白、引号台词、[字幕:…] 内会被念出的原文，必须原样保留，不得删改、换说法、同义改写或合并到其他句子里
- 允许在台词前后补充动作、场景与镜头描写，但台词字句本身不可变动
- 不要把角色对白改成旁白概括，也不要用第三人称转述替代原句`
}

// ExtractLockedDialogues pulls speakable dialogue lines from source text in reading order.
func ExtractLockedDialogues(source string) []LockedDialogue {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil
	}

	seen := make(map[string]struct{})
	var out []LockedDialogue
	add := func(text, raw, anchorBefore string) {
		text = strings.TrimSpace(text)
		raw = strings.TrimSpace(raw)
		if text == "" || raw == "" {
			return
		}
		if utf8.RuneCountInString(text) < minLockedDialogueRunes {
			return
		}
		key := normalizeForCompare(text)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, LockedDialogue{
			Text:         text,
			Raw:          raw,
			AnchorBefore: strings.TrimSpace(anchorBefore),
		})
	}

	for _, loc := range subtitleTagPattern.FindAllStringSubmatchIndex(source, -1) {
		if len(loc) < 4 {
			continue
		}
		raw := source[loc[0]:loc[1]]
		text := source[loc[2]:loc[3]]
		add(text, raw, source[:loc[0]])
	}

	for _, loc := range quotedSpeechPattern.FindAllStringSubmatchIndex(source, -1) {
		if len(loc) < 4 {
			continue
		}
		raw := source[loc[0]:loc[1]]
		text := source[loc[2]:loc[3]]
		add(text, raw, source[:loc[0]])
	}

	for _, loc := range inlineSpeakerPattern.FindAllStringSubmatchIndex(source, -1) {
		if len(loc) < 6 {
			continue
		}
		text := source[loc[4]:loc[5]]
		raw := source[loc[0]:loc[1]]
		add(text, raw, source[:loc[0]])
	}

	lines := strings.Split(source, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !screenplaySpeakerPattern.MatchString(trimmed) {
			continue
		}
		if i+1 >= len(lines) {
			continue
		}
		nextLine := strings.TrimRight(lines[i+1], "\r")
		if m := indentedDialoguePattern.FindStringSubmatch(nextLine); len(m) == 2 {
			anchor := strings.Join(lines[:i+1], "\n")
			add(m[1], strings.TrimSpace(trimmed+"\n"+nextLine), anchor)
			i++
		}
	}

	return out
}

// FormatLockedDialoguePromptBlock lists locked lines for LLM prompts.
func FormatLockedDialoguePromptBlock(dialogues []LockedDialogue) string {
	if len(dialogues) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("**以下对白/台词为原稿锁定内容，必须逐字保留：**\n")
	for i, d := range dialogues {
		if i >= 40 {
			b.WriteString("- …（其余锁定对白同样不得改动）\n")
			break
		}
		b.WriteString("- ")
		b.WriteString(d.Text)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

// EnforceLockedDialogues ensures rewritten text still contains source dialogues verbatim.
func EnforceLockedDialogues(source, rewritten string) (string, int) {
	source = strings.TrimSpace(source)
	rewritten = strings.TrimSpace(rewritten)
	if source == "" || rewritten == "" || source == rewritten {
		return rewritten, 0
	}

	dialogues := ExtractLockedDialogues(source)
	if len(dialogues) == 0 {
		return rewritten, 0
	}

	restored := 0
	out := rewritten
	for _, d := range dialogues {
		if dialoguePresent(out, d) {
			continue
		}
		out = insertMissingDialogue(out, d)
		restored++
	}
	return out, restored
}

func dialoguePresent(text string, d LockedDialogue) bool {
	if strings.Contains(text, d.Raw) {
		return true
	}
	if strings.Contains(text, d.Text) {
		return true
	}
	return strings.Contains(normalizeForCompare(text), normalizeForCompare(d.Text))
}

func insertMissingDialogue(text string, d LockedDialogue) string {
	fragment := d.Raw
	if fragment == "" {
		fragment = d.Text
	}
	if idx := findAnchorInsertIndex(text, d.AnchorBefore); idx >= 0 {
		return text[:idx] + "\n" + fragment + "\n" + text[idx:]
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text + "\n" + fragment
}

func findAnchorInsertIndex(text, anchorBefore string) int {
	anchorBefore = strings.TrimSpace(anchorBefore)
	if anchorBefore == "" {
		return -1
	}
	runes := []rune(anchorBefore)
	for length := len(runes); length >= 8; length -= 4 {
		if length > len(runes) {
			length = len(runes)
		}
		snippet := string(runes[len(runes)-length:])
		if idx := strings.LastIndex(text, snippet); idx >= 0 {
			return idx + len(snippet)
		}
	}
	return -1
}

func normalizeForCompare(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '　':
			continue
		case '“', '”', '「', '」', '『', '』', '"', '\'':
			b.WriteRune('"')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
