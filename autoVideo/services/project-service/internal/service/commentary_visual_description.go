package service

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/autovideo/project-service/internal/speechtext"
)

var (
	commentaryQuotePattern          = regexp.MustCompile(`[""「」][^""「」]{0,120}[""「」]`)
	commentaryDialogueLeadPattern   = regexp.MustCompile(`^[^。！？]{0,8}[，,：:][""「]`)
	commentarySentenceSplitPattern  = regexp.MustCompile(`[。！？\n]+`)
	commentaryBracketPattern        = regexp.MustCompile(`\[[^\]]{0,120}\]`)
	commentaryCharacterNamePattern  = regexp.MustCompile(`[\p{Han}]{2,4}(?:师傅|总|哥|姐|大鹏)?`)
	commentaryLocationPhrasePattern = regexp.MustCompile(`[\p{Han}A-Za-z0-9]{2,12}(?:包子铺|德聚楼|后厨|堂中|门口|街边|铺内|铺子)`)
)

var commentaryCharacterNoise = map[string]struct{}{
	"三个月": {}, "两千块": {}, "三十年": {}, "一口汤": {}, "一百万": {},
	"德聚楼": {}, "包子铺": {}, "北街": {}, "门口": {}, "铺内": {}, "铺子": {},
	"这次来": {}, "当初那": {}, "处理得": {}, "确实出": {}, "没看我": {},
}

var commentaryVisualActionKeywords = []string{
	"坐", "站", "跪", "躺", "抽", "点", "放", "拿", "看", "走", "转", "低", "抬", "指",
	"敲", "揉", "擦", "搬", "开", "关", "笑", "哭", "怒", "推", "拉", "进", "出", "靠",
	"挨", "探", "躲", "挥", "握", "端", "捧", "递", "接", "踢", "装", "磕", "起", "放",
}

// isCommentaryDialoguePollutedDescription reports scene_description values that are
// raw script/dialogue excerpts rather than visual staging text.
func isCommentaryDialoguePollutedDescription(desc string) bool {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return false
	}
	if strings.HasPrefix(desc, "解说镜头 ") {
		return false
	}
	if strings.Contains(desc, "【导语】") {
		return true
	}
	// Broken excerpt: single-character fragment like "眼。"
	if runes := []rune(strings.TrimRight(desc, "。！？?!")); len(runes) <= 1 {
		return true
	}
	if commentaryDialogueLeadPattern.MatchString(desc) {
		return true
	}
	quoteCount := strings.Count(desc, `"`) + strings.Count(desc, `"`) + strings.Count(desc, `"`) +
		strings.Count(desc, `"`) + strings.Count(desc, `「`)
	if quoteCount >= 2 {
		return true
	}
	if strings.Contains(desc, `："`) || strings.Contains(desc, `: "`) {
		return true
	}
	// Long narration dump without staging vocabulary.
	if utf8.RuneCountInString(desc) > 48 &&
		speechtext.LooksLikeCompleteUtterance(desc) &&
		!speechtext.LooksLikeStoryboardVisualDescription(desc) {
		return true
	}
	return false
}

// pickCommentaryPOVCharacter chooses the first-person narrator for commentary scripts.
func pickCommentaryPOVCharacter(characters []string) string {
	if len(characters) == 0 {
		return ""
	}
	for _, name := range characters {
		if strings.Contains(name, "师傅") || strings.Contains(name, "主厨") {
			return name
		}
	}
	return strings.TrimSpace(characters[0])
}

var commentaryNarratorSubjectPattern = regexp.MustCompile(`(^|[，,。！？；;：:\s])我([把将拉坐站拿放走转低抬指看说装点上])`)

// normalizeCommentarySubjectFirstPerson maps narration-subject 我 to the POV character
// without touching object-position 我 (e.g. 没看我一眼).
func normalizeCommentarySubjectFirstPerson(text, narrator string) string {
	text = strings.TrimSpace(text)
	narrator = strings.TrimSpace(narrator)
	if text == "" || narrator == "" || !strings.Contains(text, "我") {
		return text
	}
	if strings.HasPrefix(text, "我") {
		text = narrator + strings.TrimPrefix(text, "我")
	}
	return commentaryNarratorSubjectPattern.ReplaceAllString(text, "${1}"+narrator+"${2}")
}

// normalizeCommentaryFirstPerson replaces narration "我" with the POV character name in pose hints.
func normalizeCommentaryFirstPerson(text, narrator string) string {
	text = strings.TrimSpace(text)
	narrator = strings.TrimSpace(narrator)
	if text == "" || narrator == "" || !strings.Contains(text, "我") {
		return text
	}
	replacer := strings.NewReplacer(
		"我", narrator,
	)
	segments := regexp.MustCompile(`[|；;]`).Split(text, -1)
	for i, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if strings.HasPrefix(seg, "我") || strings.Contains(seg, "我") {
			segments[i] = replacer.Replace(seg)
		}
	}
	out := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg = strings.TrimSpace(seg); seg != "" {
			out = append(out, seg)
		}
	}
	return strings.Join(out, " | ")
}

func stripQuotedDialogue(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = commentaryQuotePattern.ReplaceAllString(text, "")
	text = regexp.MustCompile(`[""「」]`).ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

func splitCommentarySourceSentences(source string) []string {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil
	}
	source = commentaryBracketPattern.ReplaceAllString(source, "")
	parts := commentarySentenceSplitPattern.Split(source, -1)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || utf8.RuneCountInString(part) < 3 {
			continue
		}
		out = append(out, part)
	}
	return out
}

func findSentenceIndexForUnit(sentences []string, unit string) int {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return -1
	}
	bestIdx := -1
	bestScore := 0
	unitKey := speechtext.NormalizeSpeechKey(unit)
	for i, sent := range sentences {
		score := 0
		if strings.Contains(sent, unit) {
			score = utf8.RuneCountInString(unit) + 1000
		} else if unitKey != "" {
			sentKey := speechtext.NormalizeSpeechKey(sent)
			if strings.Contains(sentKey, unitKey) {
				score = utf8.RuneCountInString(unitKey) + 500
			} else {
				score = speechKeyOverlapRunes(unitKey, sentKey)
			}
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestScore >= 4 {
		return bestIdx
	}
	return -1
}

func isCommentaryCharacterNoise(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}
	if _, ok := commentaryCharacterNoise[name]; ok {
		return true
	}
	if utf8.RuneCountInString(name) < 2 {
		return true
	}
	return false
}

func inferKnownCharactersFromSource(source string) []string {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil
	}
	counts := map[string]int{}
	for _, name := range commentaryCharacterNamePattern.FindAllString(source, -1) {
		name = strings.TrimSpace(name)
		if isCommentaryCharacterNoise(name) {
			continue
		}
		counts[name]++
	}
	type scored struct {
		name  string
		count int
	}
	ranked := make([]scored, 0, len(counts))
	for name, count := range counts {
		if count >= 2 || strings.Contains(name, "师傅") || strings.Contains(name, "大鹏") {
			ranked = append(ranked, scored{name: name, count: count})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].name < ranked[j].name
	})
	out := make([]string, 0, len(ranked))
	seen := map[string]struct{}{}
	for _, item := range ranked {
		if _, ok := seen[item.name]; ok {
			continue
		}
		seen[item.name] = struct{}{}
		out = append(out, item.name)
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func mergeCharacterNames(primary, extra []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(primary)+len(extra))
	for _, list := range [][]string{primary, extra} {
		for _, name := range list {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}

func inferCharactersFromVisualText(text string, knownNames []string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	out := make([]string, 0, len(knownNames))
	seen := map[string]struct{}{}
	for _, name := range knownNames {
		if name == "" || !strings.Contains(text, name) {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func sentenceHasVisualBeat(s string, knownNames []string, narrator string) bool {
	s = stripQuotedDialogue(s)
	s = strings.TrimSpace(s)
	if s == "" || isPoorSceneHintSegment(s) {
		return false
	}
	for _, kw := range commentaryVisualActionKeywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	for _, name := range knownNames {
		if name != "" && strings.Contains(s, name) {
			return true
		}
	}
	return narrator != "" && strings.Contains(s, "我")
}

func collectVisualSentenceIndices(sentences []string, centerIdx int, knownNames []string, narrator string) []int {
	if centerIdx < 0 || centerIdx >= len(sentences) {
		return nil
	}
	if sentenceHasVisualBeat(sentences[centerIdx], knownNames, narrator) {
		return []int{centerIdx}
	}
	out := []int{centerIdx}
	for _, delta := range []int{-1, 1} {
		i := centerIdx + delta
		if i < 0 || i >= len(sentences) {
			continue
		}
		if !sentenceHasVisualBeat(sentences[i], knownNames, narrator) {
			continue
		}
		out = append(out, i)
		if len(out) >= 2 {
			break
		}
	}
	sort.Ints(out)
	return out
}

func resolveThirdPersonPronoun(sentence, narrator string, knownNames []string) string {
	if !strings.Contains(sentence, "他") && !strings.Contains(sentence, "她") {
		return sentence
	}
	for _, name := range knownNames {
		if name != "" && name != narrator && strings.Contains(sentence, name) {
			return sentence
		}
	}
	for _, name := range knownNames {
		if name != "" && name != narrator {
			sentence = strings.ReplaceAll(sentence, "他", name)
			sentence = strings.ReplaceAll(sentence, "她", name)
			return sentence
		}
	}
	return sentence
}

func trimVisualDescriptionLength(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if s == "" || maxRunes <= 0 {
		return s
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	cut := string(runes[:maxRunes])
	if idx := strings.LastIndexAny(cut, "，,；;"); idx > len([]rune(cut))/2 {
		return strings.TrimRight(cut[:idx], "，,；; ")
	}
	return cut + "…"
}

func composeVisualBeat(sentence, narrator string, knownNames []string) string {
	s := stripQuotedDialogue(sentence)
	s = resolveThirdPersonPronoun(s, narrator, knownNames)
	if narrator != "" {
		s = normalizeCommentarySubjectFirstPerson(s, narrator)
	}
	s = trimVisualDescriptionLength(s, 72)
	return strings.TrimSpace(s)
}

func extractLocationPhrase(sentence string) string {
	sentence = strings.TrimSpace(sentence)
	if sentence == "" {
		return ""
	}
	if m := commentaryLocationPhrasePattern.FindString(sentence); m != "" {
		return m
	}
	for _, hint := range []string{"包子铺门口", "包子铺", "德聚楼", "后厨", "堂中", "街边", "铺内", "铺子门口"} {
		if strings.Contains(sentence, hint) {
			return hint
		}
	}
	return ""
}

func inferLocationFromSentences(sentences []string, indices []int) string {
	for _, idx := range indices {
		if idx < 0 || idx >= len(sentences) {
			continue
		}
		if loc := extractLocationPhrase(sentences[idx]); loc != "" {
			return loc
		}
	}
	return ""
}

func extractCommentaryVisualDescriptionFromSource(source, unit string, scene *llmScene, narrator string) string {
	if scene != nil && len(scene.CharacterStates) > 0 {
		if built := visualDescriptionFromCharacterStates(scene); built != "" {
			return sanitizeUserSceneDescription(built)
		}
	}
	source = strings.TrimSpace(source)
	unit = strings.TrimSpace(unit)
	if source == "" || unit == "" {
		return extractCommentaryVisualDescriptionFromDialogue(unit, scene, narrator)
	}

	knownNames := inferKnownCharactersFromSource(source)
	if scene != nil && len(scene.Characters) > 0 {
		knownNames = mergeCharacterNames(knownNames, scene.Characters)
	}
	if narrator == "" {
		narrator = pickCommentaryPOVCharacter(knownNames)
	}
	if narrator == "" {
		narrator = inferNarratorFromSource(source)
	}

	sentences := splitCommentarySourceSentences(source)
	centerIdx := findSentenceIndexForUnit(sentences, unit)
	if centerIdx < 0 {
		return extractCommentaryVisualDescriptionFromDialogue(unit, scene, narrator)
	}

	indices := collectVisualSentenceIndices(sentences, centerIdx, knownNames, narrator)
	var beats []string
	for _, idx := range indices {
		if beat := composeVisualBeat(sentences[idx], narrator, knownNames); beat != "" {
			beats = append(beats, beat)
		}
		if len(beats) >= 2 {
			break
		}
	}
	if len(beats) == 0 {
		return extractCommentaryVisualDescriptionFromDialogue(unit, scene, narrator)
	}

	out := strings.Join(beats, "；")
	loc := ""
	if scene != nil {
		loc = strings.TrimSpace(scene.Location)
	}
	if loc == "" {
		loc = inferLocationFromSentences(sentences, indices)
	}
	if loc != "" && !strings.Contains(out, loc) {
		out = loc + "，" + out
	}
	return sanitizeUserSceneDescription(out)
}

func extractCommentaryVisualDescriptionFromDialogue(unit string, scene *llmScene, narrator string) string {
	if scene != nil && len(scene.CharacterStates) > 0 {
		if built := visualDescriptionFromCharacterStates(scene); built != "" {
			return sanitizeUserSceneDescription(built)
		}
	}
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return ""
	}
	knownNames := inferKnownCharactersFromSource(unit)
	if scene != nil && len(scene.Characters) > 0 {
		knownNames = mergeCharacterNames(knownNames, scene.Characters)
	}
	if narrator == "" {
		narrator = pickCommentaryPOVCharacter(knownNames)
	}

	segments := splitCommentarySourceSentences(unit)
	if len(segments) == 0 {
		segments = []string{unit}
	}
	var beats []string
	for _, seg := range segments {
		if !sentenceHasVisualBeat(seg, knownNames, narrator) {
			continue
		}
		if beat := composeVisualBeat(seg, narrator, knownNames); beat != "" {
			beats = append(beats, beat)
		}
		if len(beats) >= 2 {
			break
		}
	}
	if len(beats) == 0 {
		if beat := composeVisualBeat(unit, narrator, knownNames); beat != "" {
			beats = append(beats, beat)
		}
	}
	if len(beats) == 0 {
		return ""
	}
	out := strings.Join(beats, "；")
	if scene != nil {
		if loc := strings.TrimSpace(scene.Location); loc != "" && !strings.Contains(out, loc) {
			out = loc + "，" + out
		}
	}
	return sanitizeUserSceneDescription(out)
}

func finalizeCommentarySceneDescription(source string, sc *llmScene) {
	if sc == nil {
		return
	}
	desc := strings.TrimSpace(sc.Description)
	if desc != "" &&
		!isCommentaryDialoguePollutedDescription(desc) &&
		!strings.HasPrefix(desc, "解说镜头 ") {
		if len(sc.Characters) == 0 {
			sc.Characters = inferCharactersFromVisualText(desc, inferKnownCharactersFromSource(source))
		}
		return
	}

	narrator := inferNarratorFromSource(source)
	if narrator == "" {
		narrator = pickCommentaryPOVCharacter(sc.Characters)
	}

	if built := visualDescriptionFromCharacterStates(sc); built != "" && !isCommentaryDialoguePollutedDescription(built) {
		sc.Description = sanitizeUserSceneDescription(built)
	} else if built := extractCommentaryVisualDescriptionFromSource(source, sc.Dialogue, sc, narrator); built != "" {
		sc.Description = built
	} else if built := extractCommentaryVisualDescriptionFromDialogue(sc.Dialogue, sc, narrator); built != "" {
		sc.Description = built
	} else if desc == "" || isCommentaryDialoguePollutedDescription(desc) || strings.HasPrefix(desc, "解说镜头 ") {
		sc.Description = defaultCommentarySceneDescription(source, sc.Dialogue, 0, sc)
	}

	if len(sc.Characters) == 0 {
		sc.Characters = inferCharactersFromVisualText(sc.Description, inferKnownCharactersFromSource(source))
	}
}

func visualDescriptionFromCharacterStates(scene *llmScene) string {
	if scene == nil || len(scene.CharacterStates) == 0 {
		return ""
	}
	var parts []string
	for _, cs := range scene.CharacterStates {
		name := strings.TrimSpace(cs.Name)
		if name == "" {
			continue
		}
		var beat []string
		if cs.Action != "" {
			beat = append(beat, cs.Action)
		}
		if cs.Emotion != "" {
			beat = append(beat, cs.Emotion)
		}
		if len(beat) > 0 {
			parts = append(parts, name+"："+strings.Join(beat, "，"))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	out := strings.Join(parts, "；")
	if loc := strings.TrimSpace(scene.Location); loc != "" {
		out = loc + "，" + out
	}
	return out
}

func repairCommentarySceneDescription(desc, dialogue string, scene llmScene, narrator string) string {
	desc = strings.TrimSpace(desc)
	if desc != "" && !isCommentaryDialoguePollutedDescription(desc) {
		return sanitizeUserSceneDescription(desc)
	}
	if built := visualDescriptionFromCharacterStates(&scene); built != "" {
		return sanitizeUserSceneDescription(built)
	}
	if built := extractCommentaryVisualDescriptionFromDialogue(dialogue, &scene, narrator); built != "" {
		return built
	}
	if loc := strings.TrimSpace(scene.Location); loc != "" {
		return sanitizeUserSceneDescription(loc + "，" + strings.TrimSpace(dialogue))
	}
	return sanitizeUserSceneDescription(defaultCommentarySceneDescription("", dialogue, 0, &scene))
}

func normalizeStoryboardPoseHints(spatial, subject string, characters []string) (string, string) {
	narrator := pickCommentaryPOVCharacter(characters)
	if narrator == "" {
		return spatial, subject
	}
	return normalizeCommentaryFirstPerson(spatial, narrator), normalizeCommentaryFirstPerson(subject, narrator)
}

func enrichMultiCharacterBlockingText(text string, characters []string) string {
	text = strings.TrimSpace(text)
	if text == "" || len(characters) < 2 {
		return text
	}
	narrator := pickCommentaryPOVCharacter(characters)
	if narrator == "" {
		return text
	}
	return normalizeCommentaryFirstPerson(text, narrator)
}
