package service

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const minCharacterNameMatchRunes = 2

// Character panel order from character-service: front, closeup, side, back.
var storyboardCharacterPanelPreference = []int{0, 1, 2, 3}
var storyboardCharacterCloseUpPreference = []int{1, 0, 2, 3}

var characterSheetPromptNoise = regexp.MustCompile(`(?i)(character\s+reference\s+sheet|character\s+design\s+sheet|turnaround\s+sheet|model\s+sheet|multi-?view|front\s+view|side\s+view|back\s+view|a-?pose|四视图|三视图|多视图|设定图|角色设定|正面全身|侧面全身|背面全身|reference\s+sheet)`)

// Solo-character asset prompts often forbid extra people — strip these in multi-character storyboards.
var characterSoloShotPromptNoise = regexp.MustCompile(`(?i)(无第二人物|无合影|无多余肢体|无重复|无背景元素|single[\s-]?(person|character|subject)|solo (portrait|shot)|only one (person|character|subject)|no (second|other) (person|people|character|characters)|one person only)`)

// Default standing/front-view tokens from character asset generation conflict with kneeling/sitting beats.
var characterDefaultPosePromptNoise = regexp.MustCompile(`(?i)(standing (straight|upright|pose|full body)|upright (standing|posture|stance)|front[\s-]?facing full body|正面全身|站立全身|A[\s-]?pose standing)`)

var llmThinkingLeakPattern = regexp.MustCompile(`(?is)<(?:think|thinking|redacted_thinking)[^>]*>.*?</(?:think|thinking|redacted_thinking)>`)

func isCompositeCharacterSheetURL(url string) bool {
	u := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(u, "_composite.")
}

func pickStoryboardCharacterReferenceImage(imageURL string, panelImages []string) string {
	return pickStoryboardCharacterReferenceImageWithOrder(imageURL, panelImages, storyboardCharacterPanelPreference)
}

func pickStoryboardCharacterReferenceImageForScene(imageURL string, panelImages []string, preferCloseUp bool) string {
	order := storyboardCharacterPanelPreference
	if preferCloseUp {
		order = storyboardCharacterCloseUpPreference
	}
	return pickStoryboardCharacterReferenceImageWithOrder(imageURL, panelImages, order)
}

func pickStoryboardCharacterReferenceImageWithOrder(imageURL string, panelImages []string, order []int) string {
	for _, idx := range order {
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
	return sanitizeCharacterAssetPromptForStoryboardContext(prompt, false, false)
}

func sanitizeCharacterAssetPromptForStoryboardContext(prompt string, multiCharacter, conflictingPose bool) string {
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
		if multiCharacter && characterSoloShotPromptNoise.MatchString(seg) {
			continue
		}
		if conflictingPose && characterDefaultPosePromptNoise.MatchString(seg) {
			continue
		}
		kept = append(kept, seg)
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, ", ")
}

func sanitizeCharacterPromptMapForStoryboardContext(prompts map[string]string, multiCharacter, conflictingPose bool) {
	if len(prompts) == 0 {
		return
	}
	for key, value := range prompts {
		prompts[key] = sanitizeCharacterAssetPromptForStoryboardContext(value, multiCharacter, conflictingPose)
	}
}

func sceneHasConflictingPoseBeat(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"跪", "跪下", "跪着", "跪地", "坐", "坐着", "躺", "趴", "蹲", "俯身", "弯腰",
		"kneel", "kneeling", "seated", "sitting", "crouching", "lying", "reclining",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func sanitizeLLMThinkingLeak(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	text = llmThinkingLeakPattern.ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?m)^\s*I['’]?m considering .+$`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?m)^\s*I need to translate .+$`).ReplaceAllString(text, "")
	return strings.TrimSpace(regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n"))
}

func appendReferenceImageCharacterLabels(prompt string, characters []string, images map[string]string) string {
	if len(characters) < 2 || len(images) == 0 {
		return prompt
	}
	labels := make([]string, 0, len(characters))
	idx := 0
	for _, name := range characters {
		if lookupCharacterReferenceImage(name, images) == "" {
			continue
		}
		idx++
		labels = append(labels, fmt.Sprintf("reference image %d = %s (face/outfit identity only, ignore reference pose)", idx, strings.TrimSpace(name)))
	}
	if len(labels) == 0 {
		return prompt
	}
	return strings.TrimSpace(prompt + " Reference mapping: " + strings.Join(labels, "; ") + ".")
}

func characterNamesFuzzyMatch(query, catalogKey string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	catalogKey = strings.ToLower(strings.TrimSpace(catalogKey))
	if query == "" || catalogKey == "" {
		return false
	}
	if query == catalogKey {
		return true
	}
	queryRunes := utf8.RuneCountInString(query)
	keyRunes := utf8.RuneCountInString(catalogKey)
	if queryRunes < minCharacterNameMatchRunes || keyRunes < minCharacterNameMatchRunes {
		return false
	}
	// Prefer canonical asset names embedded in longer scene names, e.g. "王大发总裁" -> "王大发".
	if strings.Contains(query, catalogKey) {
		return true
	}
	// Allow shorter scene names embedded in longer asset aliases only when the scene name is specific enough.
	if strings.Contains(catalogKey, query) && queryRunes >= 3 {
		return true
	}
	return false
}

func lookupCharacterReferenceImage(name string, images map[string]string) string {
	nameLower := strings.ToLower(strings.TrimSpace(name))
	if nameLower == "" || len(images) == 0 {
		return ""
	}
	if url := strings.TrimSpace(images[nameLower]); url != "" {
		return url
	}
	bestKey := ""
	bestLen := 0
	for key, url := range images {
		if key == "" || strings.TrimSpace(url) == "" {
			continue
		}
		if !characterNamesFuzzyMatch(nameLower, key) {
			continue
		}
		if kl := utf8.RuneCountInString(key); kl > bestLen {
			bestLen = kl
			bestKey = key
		}
	}
	if bestKey != "" {
		return images[bestKey]
	}
	return ""
}

func lookupCharacterAssetText(name string, texts map[string]string) string {
	nameLower := strings.ToLower(strings.TrimSpace(name))
	if nameLower == "" || len(texts) == 0 {
		return ""
	}
	if text := strings.TrimSpace(texts[nameLower]); text != "" {
		return text
	}
	bestKey := ""
	bestLen := 0
	for key, text := range texts {
		if key == "" || strings.TrimSpace(text) == "" {
			continue
		}
		if !characterNamesFuzzyMatch(nameLower, key) {
			continue
		}
		if kl := utf8.RuneCountInString(key); kl > bestLen {
			bestLen = kl
			bestKey = key
		}
	}
	if bestKey != "" {
		return texts[bestKey]
	}
	return ""
}

func lookupCharacterAssetID(name string, ids map[string]int64) int64 {
	nameLower := strings.ToLower(strings.TrimSpace(name))
	if nameLower == "" || len(ids) == 0 {
		return 0
	}
	if id, ok := ids[nameLower]; ok && id > 0 {
		return id
	}
	bestKey := ""
	bestLen := 0
	for key, id := range ids {
		if id <= 0 || key == "" {
			continue
		}
		if !characterNamesFuzzyMatch(nameLower, key) {
			continue
		}
		if kl := utf8.RuneCountInString(key); kl > bestLen {
			bestLen = kl
			bestKey = key
		}
	}
	if bestKey != "" {
		return ids[bestKey]
	}
	return 0
}

func resolveCharacterAssetIDs(names []string, ids map[string]int64) []int64 {
	if len(names) == 0 || len(ids) == 0 {
		return nil
	}
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(names))
	for _, name := range names {
		if id := lookupCharacterAssetID(name, ids); id > 0 {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

func characterCatalogNames(ids map[string]int64) []string {
	if len(ids) == 0 {
		return nil
	}
	names := make([]string, 0, len(ids))
	for name := range ids {
		if strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		return utf8.RuneCountInString(names[i]) > utf8.RuneCountInString(names[j])
	})
	return names
}

// inferStoryboardCharacters keeps explicit scene characters; when empty, infer from scene text using the project character catalog.
func inferStoryboardCharacters(explicit []string, sceneDesc, dialogue string, catalog []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(explicit)+2)
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	for _, name := range explicit {
		add(name)
	}
	if len(out) > 0 {
		return out
	}
	lookup := strings.ToLower(strings.TrimSpace(sceneDesc + " " + dialogue))
	if lookup == "" || len(catalog) == 0 {
		return out
	}
	for _, name := range catalog {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || utf8.RuneCountInString(key) < minCharacterNameMatchRunes {
			continue
		}
		if strings.Contains(lookup, key) {
			add(name)
		}
	}
	return out
}

func prioritizeStoryboardReferenceImagesWithCharacters(characterURLs, otherURLs []string) []string {
	const maxStoryboardRefs = 4
	seen := make(map[string]struct{}, maxStoryboardRefs)
	out := make([]string, 0, maxStoryboardRefs)
	appendUnique := func(urls []string) {
		for _, raw := range urls {
			trimmed := strings.TrimSpace(raw)
			if trimmed == "" || isCompositeCharacterSheetURL(trimmed) {
				continue
			}
			if _, dup := seen[trimmed]; dup {
				continue
			}
			seen[trimmed] = struct{}{}
			out = append(out, trimmed)
		}
	}
	appendUnique(characterURLs)
	appendUnique(otherURLs)
	if len(out) > maxStoryboardRefs {
		out = out[:maxStoryboardRefs]
	}
	return out
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
