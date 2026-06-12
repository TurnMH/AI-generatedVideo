package service

import (
	"regexp"
	"strings"
)

// Character panel order from character-service: front, closeup, side, back.
var storyboardCharacterPanelPreference = []int{0, 1, 2, 3}

var characterSheetPromptNoise = regexp.MustCompile(`(?i)(character\s+reference\s+sheet|character\s+design\s+sheet|turnaround\s+sheet|model\s+sheet|multi-?view|front\s+view|side\s+view|back\s+view|a-?pose|四视图|三视图|多视图|设定图|角色设定|正面全身|侧面全身|背面全身|reference\s+sheet)`)

func isCompositeCharacterSheetURL(url string) bool {
	u := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(u, "_composite.")
}

func pickStoryboardCharacterReferenceImage(imageURL string, panelImages []string) string {
	for _, idx := range storyboardCharacterPanelPreference {
		if idx >= len(panelImages) {
			continue
		}
		if picked := strings.TrimSpace(panelImages[idx]); picked != "" && !isCompositeCharacterSheetURL(picked) {
			return picked
		}
	}
	if imageURL = strings.TrimSpace(imageURL); imageURL != "" && !isCompositeCharacterSheetURL(imageURL) {
		return imageURL
	}
	return ""
}

func filterStoryboardReferenceURLs(urls []string) []string {
	if len(urls) == 0 {
		return urls
	}
	out := make([]string, 0, len(urls))
	seen := make(map[string]struct{}, len(urls))
	for _, raw := range urls {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || isCompositeCharacterSheetURL(trimmed) {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func resolveStoryboardStyleReferenceURL(primary string, extras []string) string {
	if picked := strings.TrimSpace(primary); picked != "" && !isCompositeCharacterSheetURL(picked) {
		return picked
	}
	for _, raw := range extras {
		if picked := strings.TrimSpace(raw); picked != "" && !isCompositeCharacterSheetURL(picked) {
			return picked
		}
	}
	return ""
}

func sanitizeCharacterAssetPromptForStoryboard(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return ""
	}
	segments := regexp.MustCompile(`[,.;|]+`).Split(prompt, -1)
	kept := make([]string, 0, len(segments))
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" || characterSheetPromptNoise.MatchString(seg) {
			continue
		}
		kept = append(kept, seg)
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, ", ")
}

func sanitizeCharacterPromptMapForStoryboard(prompts map[string]string) {
	if len(prompts) == 0 {
		return
	}
	for key, value := range prompts {
		prompts[key] = sanitizeCharacterAssetPromptForStoryboard(value)
	}
}

func appendStoryboardAntiSheetNegativeTokens(baseNeg string) string {
	extras := []string{
		"character reference sheet", "character design sheet", "turnaround sheet", "model sheet",
		"multi-panel layout", "split screen collage", "four views", "front side back views",
		"设定图", "四视图", "三视图", "多视图",
	}
	if strings.TrimSpace(baseNeg) == "" {
		return strings.Join(extras, ", ")
	}
	return baseNeg + ", " + strings.Join(extras, ", ")
}
