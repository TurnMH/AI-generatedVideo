package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/autovideo/project-service/internal/productionmode"
	"github.com/autovideo/project-service/internal/speechtext"
)

func (s *EpisodeService) postProcessScenes(scenes []llmScene, clipDuration int, speechPace string, profile productionmode.Profile) []llmScene {
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

func refitSceneDialogue(scene *llmScene, clipDuration int, speechPace string, commentary bool) {
	if scene == nil {
		return
	}
	maxRunes := sceneSpeechMaxRunes(*scene, clipDuration, speechPace)
	if commentary {
		scene.Dialogue = speechtext.FinalizeCommentaryDialogueWithLimit(scene.Dialogue, maxRunes)
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

	const minNarrationRunes = 12
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
	scenes = s.postProcessScenes(scenes, clipDuration, speechPace, profile)
	if profile.IsCommentaryComic() {
		scenes = alignCommentaryScenesWithSource(episodeContent, scenes)
		scenes = ensureCommentaryNarrationCoverage(episodeContent, scenes, clipDuration, speechPace)
		scenes = expandCommentaryScenesForClipLimit(scenes, clipDuration, speechPace)
		scenes = consolidateShortCommentaryDialogue(scenes, clipDuration, speechPace)
		maxRunes := sceneSpeechMaxRunes(llmScene{Duration: clipDuration}, clipDuration, speechPace)
		for i := range scenes {
			if dlg := strings.TrimSpace(scenes[i].Dialogue); dlg != "" {
				scenes[i].Dialogue = speechtext.FinalizeCommentaryDialogueWithLimit(dlg, maxRunes)
			}
			syncSceneDurationFromDialogue(&scenes[i], clipDuration, speechPace)
		}
		scenes = consolidateOrphanCommentaryDialogue(scenes, clipDuration, speechPace)
		scenes = dropEmptyCommentaryScenes(scenes)
	}
	return scenes
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
	if sceneRunes >= sourceRunes*55/100 {
		return scenes
	}
	return supplementCommentaryScenesFromSource(source, scenes, units, clipDuration, speechPace)
}

func packCommentaryScenesFromSource(source string, units []string, clipDuration int, speechPace string, hints []llmScene) []llmScene {
	maxRunes := sceneSpeechMaxRunes(llmScene{Duration: clipDuration}, clipDuration, speechPace)
	dialogues := speechtext.PackSpeechUnitsToMaxRunes(units, maxRunes)
	if len(dialogues) == 0 {
		return hints
	}
	scenes := make([]llmScene, 0, len(dialogues))
	for i, dlg := range dialogues {
		sc := llmScene{
			Dialogue: dlg,
			Duration: clipDuration,
		}
		if hint := findCommentaryDescriptionHint(dlg, hints); hint != "" {
			sc.Description = hint
		} else {
			sc.Description = defaultCommentarySceneDescription(dlg, i+1)
		}
		syncSceneDurationFromDialogue(&sc, clipDuration, speechPace)
		scenes = append(scenes, sc)
	}
	_ = source
	return scenes
}

func expandCommentaryScenesForClipLimit(scenes []llmScene, clipDuration int, speechPace string) []llmScene {
	maxRunes := sceneSpeechMaxRunes(llmScene{Duration: clipDuration}, clipDuration, speechPace)
	out := make([]llmScene, 0, len(scenes))
	for _, scene := range scenes {
		raw := strings.TrimSpace(scene.Dialogue)
		if raw == "" {
			out = append(out, scene)
			continue
		}
		cleaned := strings.TrimSpace(speechtext.ExtractNarrationForSpeech(raw))
		if cleaned == "" {
			cleaned = strings.TrimSpace(raw)
		}
		if cleaned == "" {
			out = append(out, scene)
			continue
		}
		cleanedRunes := utf8.RuneCountInString(cleaned)
		if cleanedRunes <= maxRunes {
			scene.Dialogue = cleaned
			out = append(out, scene)
			continue
		}
		chunks := speechtext.PackSpeechUnitsToMaxRunes(speechtext.SplitSpeechUnitsForPacking(cleaned), maxRunes)
		if len(chunks) == 0 {
			chunks = speechtext.PackSpeechUnitsToMaxRunes([]string{cleaned}, maxRunes)
		}
		if len(chunks) <= 1 {
			scene.Dialogue = cleaned
			out = append(out, scene)
			continue
		}
		for _, chunk := range chunks {
			sc := scene
			sc.Dialogue = chunk
			syncSceneDurationFromDialogue(&sc, clipDuration, speechPace)
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

func findCommentaryDescriptionHint(dialogue string, hints []llmScene) string {
	key := normalizeCommentaryDialogueKey(dialogue)
	for _, hint := range hints {
		if hint.Description == "" {
			continue
		}
		if normalizeCommentaryDialogueKey(hint.Dialogue) == key {
			return hint.Description
		}
	}
	for _, hint := range hints {
		if hint.Description == "" || hint.Dialogue == "" {
			continue
		}
		hKey := normalizeCommentaryDialogueKey(hint.Dialogue)
		if hKey != "" && (strings.Contains(key, hKey) || strings.Contains(hKey, key)) {
			return hint.Description
		}
	}
	return ""
}

func defaultCommentarySceneDescription(dialogue string, seq int) string {
	snippet := dialogue
	if runes := []rune(snippet); len(runes) > 72 {
		snippet = string(runes[:72]) + "…"
	}
	return fmt.Sprintf("解说镜头 %d：%s", seq, snippet)
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
