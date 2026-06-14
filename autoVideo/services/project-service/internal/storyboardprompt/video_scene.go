package storyboardprompt

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	inlineActionTagPattern = regexp.MustCompile(`\[动作[:：]([^\]]+)\]`)
	appearanceNoisePattern = regexp.MustCompile(`(?:身穿|身着|穿着|发型|黑发|花白|脸型|圆润|商人气息|环境光线|背景简洁|神情|面露|身形对比|气氛紧张|近景突出|远景)`)
)

// CompactVideoSceneDescription builds a motion-focused visual prompt for video models.
// Dialogue is handled separately via per-clip dialogues / VoiceText.
// Image prompt_used is intentionally ignored — it is only for storyboard image generation.
func CompactVideoSceneDescription(sceneDescription, _ string) string {
	scene := strings.TrimSpace(sceneDescription)
	if scene == "" {
		return ""
	}

	if actions := extractInlineActionTags(scene); len(actions) > 0 {
		return joinClauses(actions)
	}

	scene = pruneAppearanceCatalog(scene)
	if scene != "" {
		return scene
	}
	return strings.TrimSpace(sceneDescription)
}

// VideoSceneDescription prefers narrative scene_description over image prompt_used.
func VideoSceneDescription(sceneDescription, promptUsed, _ string) string {
	return CompactVideoSceneDescription(sceneDescription, promptUsed)
}

func extractInlineActionTags(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, m := range inlineActionTagPattern.FindAllStringSubmatch(text, -1) {
		if len(m) < 2 {
			continue
		}
		action := strings.TrimSpace(m[1])
		if action == "" {
			continue
		}
		if _, ok := seen[action]; ok {
			continue
		}
		seen[action] = struct{}{}
		out = append(out, action)
	}
	return out
}

func pruneAppearanceCatalog(desc string) string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return ""
	}

	clauses := splitDescriptionClauses(desc)
	kept := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		if appearanceNoisePattern.MatchString(clause) && !hasActionVerb(clause) {
			continue
		}
		kept = append(kept, clause)
	}
	if len(kept) == 0 {
		return strings.TrimSpace(desc)
	}
	return joinClauses(kept)
}

func splitDescriptionClauses(desc string) []string {
	desc = strings.ReplaceAll(desc, "；", "，")
	desc = strings.ReplaceAll(desc, "。", "，")
	parts := strings.Split(desc, "，")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func hasActionVerb(clause string) bool {
	for _, verb := range []string{"揉", "站", "走", "拿", "放", "推", "拉", "看", "转", "抬", "低", "开", "关", "切", "递", "接", "坐", "蹲", "跑", "握", "拍", "敲", "揉面", "开门", "探头"} {
		if strings.Contains(clause, verb) {
			return true
		}
	}
	return false
}

func joinClauses(clauses []string) string {
	if len(clauses) == 0 {
		return ""
	}
	out := strings.Join(clauses, "，")
	if !strings.HasSuffix(out, "。") && utf8.RuneCountInString(out) >= 8 {
		out += "。"
	}
	return out
}
