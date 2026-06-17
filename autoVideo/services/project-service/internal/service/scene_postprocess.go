package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/autovideo/project-service/internal/productionmode"
	"github.com/autovideo/project-service/internal/speechtext"
)

func (s *EpisodeService) postProcessScenes(scenes []llmScene, clipDuration int, speechPace string, profile productionmode.Profile) []llmScene {
	if profile.SkipPostProcessing {
		return scenes
	}
	if profile.ShouldPostProcessMergeScenes() {
		return s.postProcessAdScenes(scenes, clipDuration, speechPace)
	}
	if profile.IsCommentaryComic() {
		return postProcessCommentaryScenes(scenes, clipDuration, speechPace)
	}
	return postProcessRhythmicScenes(scenes, clipDuration, speechPace)
}

func sceneSpeechMaxRunes(scene llmScene, clipDuration int, speechPace string) int {
	duration := scene.Duration
	if duration <= 0 {
		duration = clipDuration
	}
	if duration <= 0 {
		duration = 5
	}
	return speechtext.MaxRunesForClipDuration(duration, speechPace)
}

func commentarySceneSpeechBounds(clipDuration int, speechPace string) (min, targetMax, hardMax int) {
	return speechtext.CommentaryClipRunesBounds(clipDuration, speechPace)
}

func syncCommentarySceneDuration(scene *llmScene, clipDuration int) {
	if scene == nil {
		return
	}
	if clipDuration <= 0 {
		clipDuration = 5
	}
	scene.Duration = clipDuration
}

func commentaryLocationKey(sc *llmScene) string {
	if sc == nil {
		return ""
	}
	loc := strings.TrimSpace(sc.Location)
	zone := strings.TrimSpace(sc.LocationZone)
	if loc == "" && zone == "" {
		return ""
	}
	return loc + "\x00" + zone
}

func commentaryLocationKeyForUnit(unit string, hints []llmScene) string {
	if hint := findCommentarySceneHint(unit, hints); hint != nil {
		return commentaryLocationKey(hint)
	}
	return ""
}

func commentaryScenesShareLocation(a, b llmScene) bool {
	aKey := commentaryLocationKey(&a)
	bKey := commentaryLocationKey(&b)
	if aKey == "" || bKey == "" {
		return true
	}
	return aKey == bKey
}

func commentaryScenesCanMergeDialogue(a, b llmScene) bool {
	if !commentaryScenesShareLocation(a, b) {
		return false
	}
	return !commentaryPlotDescriptionsConflict(a, b)
}

func refitSceneDialogue(scene *llmScene, clipDuration int, speechPace string, commentary bool) {
	if scene == nil {
		return
	}
	maxRunes := sceneSpeechMaxRunes(*scene, clipDuration, speechPace)
	if commentary {
		_, _, hardMax := commentarySceneSpeechBounds(clipDuration, speechPace)
		scene.Dialogue = speechtext.FinalizeCommentaryDialogueWithLimit(scene.Dialogue, hardMax)
		return
	}
	scene.Dialogue = speechtext.CompactClipDialogue(strings.TrimSpace(speechtext.SanitizeForSpeech(scene.Dialogue)), maxRunes)
}

func syncSceneDurationFromDialogue(scene *llmScene, clipDuration int, speechPace string) {
	if scene == nil {
		return
	}
	dialogue := strings.TrimSpace(scene.Dialogue)
	if dialogue == "" {
		if scene.Duration <= 0 {
			scene.Duration = normalizeAdSceneDuration(0, clipDuration)
		}
		scene.Duration = clampDuration(scene.Duration, 2, 12)
		return
	}
	scene.Duration = inferSceneDurationFromDialogue(dialogue, clipDuration, speechPace)
}

func sanitizeStoryboardDialogue(text string) string {
	return speechtext.SanitizeForSpeech(text)
}

func postProcessRhythmicScenes(scenes []llmScene, clipDuration int, speechPace string) []llmScene {
	if len(scenes) == 0 {
		return scenes
	}
	out := make([]llmScene, len(scenes))
	copy(out, scenes)
	for i := range out {
		out[i].Dialogue = strings.TrimSpace(speechtext.SanitizeForSpeech(out[i].Dialogue))
		out[i].Description = sanitizeUserSceneDescription(out[i].Description)
		out[i].Duration = inferSceneDurationFromDialogue(out[i].Dialogue, clipDuration, speechPace)
		refitSceneDialogue(&out[i], clipDuration, speechPace, false)
	}
	return out
}

func postProcessCommentaryScenes(scenes []llmScene, clipDuration int, speechPace string) []llmScene {
	if len(scenes) == 0 {
		return scenes
	}
	out := make([]llmScene, len(scenes))
	copy(out, scenes)

	minNarrationRunes, _, _ := commentarySceneSpeechBounds(clipDuration, speechPace)
	for i := range out {
		out[i].Dialogue = speechtext.FinalizeCommentaryDialogue(out[i].Dialogue)
		out[i].Description = sanitizeUserSceneDescription(out[i].Description)
		out[i].Duration = inferSceneDurationFromDialogue(out[i].Dialogue, clipDuration, speechPace)
	}

	merged := make([]llmScene, 0, len(out))
	for _, scene := range out {
		dlg := strings.TrimSpace(scene.Dialogue)
		dlgRunes := utf8.RuneCountInString(dlg)
		if len(merged) > 0 && (dlgRunes == 0 || (dlgRunes < minNarrationRunes && !speechtext.LooksLikeCompleteUtterance(dlg))) {
			prev := &merged[len(merged)-1]
			if !commentaryScenesCanMergeDialogue(*prev, scene) {
				merged = append(merged, scene)
				continue
			}
			if prev.Description != "" && scene.Description != "" && prev.Description != scene.Description {
				merged = append(merged, scene)
				continue
			}
			if dlg != "" {
				if prev.Dialogue == "" {
					prev.Dialogue = dlg
				} else {
					prev.Dialogue = strings.TrimSpace(prev.Dialogue + "\n" + dlg)
				}
			}
			if scene.Description != "" {
				if prev.Description == "" {
					prev.Description = scene.Description
				} else {
					prev.Description = strings.TrimSpace(prev.Description + "；" + scene.Description)
				}
			}
			if prev.Location == "" {
				prev.Location = scene.Location
			}
			continue
		}
		merged = append(merged, scene)
	}
	for i := range merged {
		syncSceneDurationFromDialogue(&merged[i], clipDuration, speechPace)
		refitSceneDialogue(&merged[i], clipDuration, speechPace, true)
		syncSceneDurationFromDialogue(&merged[i], clipDuration, speechPace)
	}
	return merged
}

func alignCommentaryScenesWithSource(source string, scenes []llmScene) []llmScene {
	if len(scenes) == 0 {
		return scenes
	}
	payload := make([]speechtext.SceneDialogue, len(scenes))
	for i := range scenes {
		payload[i].Dialogue = scenes[i].Dialogue
	}
	payload = speechtext.AlignCommentaryScenesWithSource(source, payload)
	for i := range scenes {
		scenes[i].Dialogue = payload[i].Dialogue
	}
	return scenes
}

func (s *EpisodeService) postProcessAndAlignCommentaryScenes(
	episodeContent string,
	scenes []llmScene,
	clipDuration int,
	speechPace string,
	profile productionmode.Profile,
) []llmScene {
	if profile.SkipPostProcessing {
		for i := range scenes {
			scenes[i].Dialogue = strings.TrimSpace(scenes[i].Dialogue)
			scenes[i].Description = sanitizeUserSceneDescription(scenes[i].Description)
			if scenes[i].Duration <= 0 {
				scenes[i].Duration = inferSceneDurationFromDialogue(scenes[i].Dialogue, clipDuration, speechPace)
			}
		}
		return scenes
	}
	scenes = s.postProcessScenes(scenes, clipDuration, speechPace, profile)
	if profile.IsCommentaryComic() {
		scenes = alignCommentaryScenesWithSource(episodeContent, scenes)
		llmHints := append([]llmScene(nil), scenes...)
		// 解说漫：旁白必须逐字来自原文。LLM 只提供画面描述提示，dialogue 始终按原文顺序切分，
		// 不再依赖「80% 逐字匹配」判定——片段化/乱序的 LLM 输出也会误判为合格。
		if units := speechtext.ExtractCommentarySpeechUnits(episodeContent); len(units) > 0 {
			scenes = packCommentaryScenesFromSource(episodeContent, units, clipDuration, speechPace, llmHints)
		} else if !commentaryDialogueIsVerbatimFromSource(episodeContent, scenes) {
			scenes = ensureCommentaryNarrationCoverage(episodeContent, scenes, clipDuration, speechPace)
		}
		scenes = expandCommentaryScenesForClipLimit(episodeContent, scenes, clipDuration, speechPace)
		scenes = consolidateOrphanCommentaryDialogue(scenes, clipDuration, speechPace)
		_, _, hardMax := commentarySceneSpeechBounds(clipDuration, speechPace)
		for i := range scenes {
			if dlg := strings.TrimSpace(scenes[i].Dialogue); dlg != "" {
				scenes[i].Dialogue = speechtext.FinalizeCommentaryDialogueWithLimit(dlg, hardMax)
			}
			syncCommentarySceneDuration(&scenes[i], clipDuration)
		}
		scenes = dropEmptyCommentaryScenes(scenes)
	}
	return scenes
}

// commentaryVerbatimFidelityThreshold —— 分镜 dialogue 中"逐字来自原文"的最低占比。
// 低于该值视为 LLM 进行了改写/优化，需要从原文重建以恢复逐字与完整性。
const commentaryVerbatimFidelityThreshold = 0.80

// commentaryDialogueIsVerbatimFromSource reports whether the scene dialogues are (mostly)
// verbatim contiguous fragments of the episode source. Faithful splitting/merging keeps
// dialogue as a substring of the punctuation-stripped source key; word-level rewriting does not.
func commentaryDialogueIsVerbatimFromSource(source string, scenes []llmScene) bool {
	units := speechtext.ExtractCommentarySpeechUnits(source)
	if len(units) == 0 {
		// 无法从原文抽取可念内容时，不强制重建，交由后续覆盖度逻辑处理。
		return true
	}
	var sb strings.Builder
	for _, unit := range units {
		sb.WriteString(normalizeCommentaryDialogueKey(unit))
	}
	sourceKey := sb.String()
	if sourceKey == "" {
		return true
	}

	totalRunes := 0
	matchedRunes := 0
	for i := range scenes {
		sceneKey := normalizeCommentaryDialogueKey(scenes[i].Dialogue)
		keyRunes := utf8.RuneCountInString(sceneKey)
		if keyRunes == 0 {
			continue
		}
		totalRunes += keyRunes
		if strings.Contains(sourceKey, sceneKey) {
			matchedRunes += keyRunes
			continue
		}
		matchedRunes += longestCommonSubstringRunes(sceneKey, sourceKey)
	}
	if totalRunes == 0 {
		// 没有任何可念 dialogue —— 让覆盖度逻辑去补全，不在此处判定。
		return true
	}
	return float64(matchedRunes) >= float64(totalRunes)*commentaryVerbatimFidelityThreshold
}

// longestCommonSubstringRunes returns the rune length of the longest common contiguous
// substring of a and b. Used to give partial credit when a scene is only partially rewritten.
func longestCommonSubstringRunes(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 || len(br) == 0 {
		return 0
	}
	// 防御：超长原文截断到合理范围，避免 O(n*m) 过大。
	const maxLen = 8000
	if len(br) > maxLen {
		br = br[:maxLen]
	}
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	best := 0
	for i := 1; i <= len(ar); i++ {
		for j := 1; j <= len(br); j++ {
			if ar[i-1] == br[j-1] {
				cur[j] = prev[j-1] + 1
				if cur[j] > best {
					best = cur[j]
				}
			} else {
				cur[j] = 0
			}
		}
		prev, cur = cur, prev
		for j := range cur {
			cur[j] = 0
		}
	}
	return best
}

func ensureCommentaryNarrationCoverage(source string, scenes []llmScene, clipDuration int, speechPace string) []llmScene {
	units := speechtext.ExtractCommentarySpeechUnits(source)
	if len(units) == 0 {
		return scenes
	}
	if len(scenes) == 0 {
		return packCommentaryScenesFromSource(source, units, clipDuration, speechPace, nil)
	}
	sourceRunes := speechtext.CommentarySpeechRunes(source)
	if sourceRunes <= 0 {
		return scenes
	}
	sceneRunes := sumSceneDialogueRunes(scenes)
	// 解说漫强调内容完整：分镜旁白总量低于原文可念内容 80% 时，
	// 视为 LLM 拆分时漏抄/概括，触发从原文逐句补全以恢复完整性。
	if sceneRunes >= sourceRunes*80/100 {
		return scenes
	}
	return supplementCommentaryScenesFromSource(source, scenes, units, clipDuration, speechPace)
}

func packCommentaryScenesFromSource(source string, units []string, clipDuration int, speechPace string, hints []llmScene) []llmScene {
	_, targetMax, _ := commentarySceneSpeechBounds(clipDuration, speechPace)
	dialogues := packCommentaryDialoguesFromUnits(units, targetMax, hints)
	if len(dialogues) == 0 {
		return hints
	}
	scenes := make([]llmScene, 0, len(dialogues))
	for _, dlg := range dialogues {
		sc := llmScene{
			Dialogue: dlg,
			Duration: clipDuration,
		}
		
		// Find the matched LLM scene hint to inherit its metadata (Location, LocationZone, Characters, ShotType, Mood, etc.)
		if matchedHint := findCommentarySceneHint(dlg, hints); matchedHint != nil {
			sc.Description = matchedHint.Description
			sc.Location = matchedHint.Location
			sc.LocationZone = matchedHint.LocationZone
			sc.Characters = matchedHint.Characters
			sc.CharacterStates = matchedHint.CharacterStates
			sc.Items = matchedHint.Items
			sc.ShotType = matchedHint.ShotType
			sc.Mood = matchedHint.Mood
		}
		finalizeCommentarySceneDescription(source, &sc)
		syncCommentarySceneDuration(&sc, clipDuration)
		scenes = append(scenes, sc)
	}
	return scenes
}

func packCommentaryDialoguesFromUnits(units []string, targetRunes int, hints []llmScene) []string {
	if len(units) == 0 {
		return nil
	}
	var packed []string
	var current strings.Builder
	currentRunes := 0
	currentLoc := ""
	flush := func() {
		if current.Len() == 0 {
			return
		}
		packed = append(packed, strings.TrimSpace(current.String()))
		current.Reset()
		currentRunes = 0
		currentLoc = ""
	}
	for _, unit := range units {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		unitRunes := utf8.RuneCountInString(unit)
		unitLoc := commentaryLocationKeyForUnit(unit, hints)
		if currentRunes == 0 && unitRunes > targetRunes {
			current.WriteString(unit)
			flush()
			continue
		}
		if currentRunes > 0 {
			if currentRunes+unitRunes > targetRunes {
				flush()
			} else if currentLoc != "" && unitLoc != "" && currentLoc != unitLoc {
				flush()
			}
		}
		if currentRunes == 0 && unitLoc != "" {
			currentLoc = unitLoc
		}
		current.WriteString(unit)
		currentRunes += unitRunes
		if currentRunes >= targetRunes {
			flush()
		}
	}
	flush()
	return packed
}

func expandCommentaryScenesForClipLimit(source string, scenes []llmScene, clipDuration int, speechPace string) []llmScene {
	_, _, hardMax := commentarySceneSpeechBounds(clipDuration, speechPace)
	out := make([]llmScene, 0, len(scenes))
	for _, scene := range scenes {
		raw := strings.TrimSpace(scene.Dialogue)
		if raw == "" {
			out = append(out, scene)
			continue
		}
		cleaned := strings.TrimSpace(raw)
		if speechtext.LooksLikeStoryboardVisualDescription(cleaned) {
			cleaned = strings.TrimSpace(speechtext.ExtractNarrationForSpeechEx(raw, true))
		}
		if cleaned == "" {
			cleaned = strings.TrimSpace(raw)
		}
		if cleaned == "" {
			out = append(out, scene)
			continue
		}
		cleanedRunes := utf8.RuneCountInString(cleaned)
		if cleanedRunes <= hardMax {
			scene.Dialogue = cleaned
			finalizeCommentarySceneDescription(source, &scene)
			out = append(out, scene)
			continue
		}
		chunks := speechtext.PackSpeechUnitsToMaxRunes(speechtext.SplitSpeechUnitsForPacking(cleaned), hardMax)
		if len(chunks) == 0 {
			chunks = speechtext.PackSpeechUnitsToMaxRunes([]string{cleaned}, hardMax)
		}
		if len(chunks) <= 1 {
			scene.Dialogue = cleaned
			out = append(out, scene)
			continue
		}
		for _, chunk := range chunks {
			sc := scene
			sc.Dialogue = chunk
			finalizeCommentarySceneDescription(source, &sc)
			syncCommentarySceneDuration(&sc, clipDuration)
			out = append(out, sc)
		}
	}
	return out
}

func sumSceneDialogueRunes(scenes []llmScene) int {
	total := 0
	for _, scene := range scenes {
		dlg := speechtext.FinalizeCommentaryDialogue(scene.Dialogue)
		total += utf8.RuneCountInString(dlg)
	}
	return total
}

func findCommentarySceneHint(dialogue string, hints []llmScene) *llmScene {
	key := normalizeCommentaryDialogueKey(dialogue)
	if key == "" {
		return nil
	}
	// 1. Exact match first (highest priority)
	for i := range hints {
		hint := &hints[i]
		if hint.Description == "" {
			continue
		}
		if normalizeCommentaryDialogueKey(hint.Dialogue) == key {
			return hint
		}
	}
	// 2. Substring match with length constraint to prevent false positives on short names/words
	for i := range hints {
		hint := &hints[i]
		if hint.Description == "" || hint.Dialogue == "" {
			continue
		}
		hKey := normalizeCommentaryDialogueKey(hint.Dialogue)
		if hKey == "" {
			continue
		}
		// If either key is too short, do not use substring matching (must be exact match)
		if utf8.RuneCountInString(hKey) < 5 || utf8.RuneCountInString(key) < 5 {
			continue
		}
		if strings.Contains(key, hKey) || strings.Contains(hKey, key) {
			return hint
		}
	}
	return nil
}

func findCommentaryDescriptionHint(dialogue string, hints []llmScene) string {
	if hint := findCommentarySceneHint(dialogue, hints); hint != nil {
		return hint.Description
	}
	return ""
}

func defaultCommentarySceneDescription(source, dialogue string, seq int, scene *llmScene) string {
	narrator := inferNarratorFromSource(source)
	if narrator == "" && scene != nil {
		narrator = pickCommentaryPOVCharacter(scene.Characters)
	}
	if built := extractCommentaryVisualDescriptionFromSource(source, dialogue, scene, narrator); built != "" {
		return built
	}
	if built := extractCommentaryVisualDescriptionFromDialogue(dialogue, scene, narrator); built != "" {
		return built
	}
	snippet := trimVisualDescriptionLength(dialogue, 72)
	if seq > 0 {
		return fmt.Sprintf("解说镜头 %d：%s", seq, snippet)
	}
	return snippet
}

func dropEmptyCommentaryScenes(scenes []llmScene) []llmScene {
	out := make([]llmScene, 0, len(scenes))
	for _, scene := range scenes {
		if strings.TrimSpace(scene.Dialogue) == "" {
			continue
		}
		out = append(out, scene)
	}
	return out
}

func normalizeCommentaryDialogueKey(dialogue string) string {
	dialogue = speechtext.FinalizeCommentaryDialogue(dialogue)
	var b strings.Builder
	for _, r := range dialogue {
		switch r {
		case ' ', '\t', '\n', '\r', '　', '，', ',', '。', '.', '！', '!', '？', '?', '；', ';', '：', ':', '“', '”', '「', '」', '『', '』', '"', '\'':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
