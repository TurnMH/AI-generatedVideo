package service

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/autovideo/project-service/internal/speechtext"
)

type commentarySceneInsert struct {
	sourcePos int
	scene     llmScene
}

func supplementCommentaryScenesFromSource(
	source string,
	scenes []llmScene,
	units []string,
	clipDuration int,
	speechPace string,
) []llmScene {
	if len(units) == 0 {
		return scenes
	}
	maxRunes := sceneSpeechMaxRunes(llmScene{Duration: clipDuration}, clipDuration, speechPace)
	base := append([]llmScene(nil), scenes...)
	var inserts []commentarySceneInsert

	var uncovered []string
	for _, unit := range units {
		unit = strings.TrimSpace(unit)
		if unit == "" || isSpeechUnitCovered(unit, base) {
			continue
		}
		uncovered = append(uncovered, unit)
	}
	if len(uncovered) == 0 {
		return scenes
	}
	packed := speechtext.PackSpeechUnitsToMaxRunes(uncovered, maxRunes)
	for _, chunk := range packed {
		if isSpeechUnitCovered(chunk, base) {
			continue
		}
		sourcePos := findUnitSourcePosition(source, chunk)
		sc := llmScene{
			Dialogue: chunk,
			Duration: clipDuration,
		}
		if hint := findCommentaryDescriptionHint(chunk, base); hint != "" {
			sc.Description = hint
		} else if hint := excerptCommentaryVisualHint(source, chunk); hint != "" {
			sc.Description = hint
		} else {
			sc.Description = defaultCommentarySceneDescription(chunk, 0)
		}
		syncSceneDurationFromDialogue(&sc, clipDuration, speechPace)
		inserts = append(inserts, commentarySceneInsert{sourcePos: sourcePos, scene: sc})
		base = append(base, sc)
	}
	if len(inserts) == 0 {
		return scenes
	}
	return mergeCommentaryScenesPreservingPlot(scenes, inserts, source)
}

func isSpeechUnitCovered(unit string, scenes []llmScene) bool {
	key := speechtext.NormalizeSpeechKey(unit)
	if key == "" {
		return true
	}
	minMatch := utf8.RuneCountInString(key) * 60 / 100
	if minMatch < 4 {
		minMatch = 4
	}
	for _, scene := range scenes {
		dialogueKey := collectSceneDialogueKeys(scene)
		for _, sceneKey := range dialogueKey {
			if sceneKey == "" {
				continue
			}
			if strings.Contains(sceneKey, key) || strings.Contains(key, sceneKey) {
				return true
			}
			if speechKeyOverlapRunes(key, sceneKey) >= minMatch {
				return true
			}
		}
	}
	return false
}

func collectSceneDialogueKeys(scene llmScene) []string {
	raw := strings.TrimSpace(scene.Dialogue)
	if raw == "" {
		return nil
	}
	keys := []string{speechtext.NormalizeSpeechKey(raw)}
	if cleaned := speechtext.FinalizeCommentaryDialogue(raw); cleaned != "" {
		keys = append(keys, speechtext.NormalizeSpeechKey(cleaned))
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, "："); idx > 0 {
			keys = append(keys, speechtext.NormalizeSpeechKey(line[idx+1:]))
		}
		keys = append(keys, speechtext.NormalizeSpeechKey(line))
	}
	return keys
}

func speechKeyOverlapRunes(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	shorter, longer := a, b
	if len([]rune(a)) > len([]rune(b)) {
		shorter, longer = b, a
	}
	best := 0
	runes := []rune(shorter)
	for size := len(runes); size >= 4; size-- {
		for start := 0; start+size <= len(runes); start++ {
			needle := string(runes[start : start+size])
			if strings.Contains(longer, needle) && size > best {
				best = size
			}
		}
	}
	return best
}

func findUnitSourcePosition(source, unit string) int {
	if idx := strings.Index(source, unit); idx >= 0 {
		return idx
	}
	unit = strings.TrimSpace(unit)
	runes := []rune(unit)
	for n := minInt(len(runes), 12); n >= 4; n-- {
		if idx := strings.Index(source, string(runes[:n])); idx >= 0 {
			return idx
		}
	}
	return len(source)
}

func findSceneSourcePosition(source string, scene llmScene) int {
	dialogue := strings.TrimSpace(scene.Dialogue)
	if dialogue == "" {
		return -1
	}
	if idx := strings.Index(source, dialogue); idx >= 0 {
		return idx
	}
	if cleaned := speechtext.FinalizeCommentaryDialogue(dialogue); cleaned != "" {
		if idx := strings.Index(source, cleaned); idx >= 0 {
			return idx
		}
	}
	runes := []rune(dialogue)
	for n := minInt(len(runes), 12); n >= 4; n-- {
		if idx := strings.Index(source, string(runes[:n])); idx >= 0 {
			return idx
		}
	}
	return -1
}

func mergeCommentaryScenesPreservingPlot(base []llmScene, inserts []commentarySceneInsert, source string) []llmScene {
	type orderedScene struct {
		order float64
		scene llmScene
	}
	out := make([]orderedScene, 0, len(base)+len(inserts))
	for i, scene := range base {
		out = append(out, orderedScene{
			order: float64(i) * 1000,
			scene: scene,
		})
	}
	for _, ins := range inserts {
		out = append(out, orderedScene{
			order: commentaryInsertOrder(source, base, ins.sourcePos),
			scene: ins.scene,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].order < out[j].order
	})
	merged := make([]llmScene, len(out))
	for i := range out {
		merged[i] = out[i].scene
	}
	return merged
}

func commentaryInsertOrder(source string, base []llmScene, sourcePos int) float64 {
	bestIdx := -1
	bestPos := -1
	for i, scene := range base {
		pos := findSceneSourcePosition(source, scene)
		if pos >= 0 && pos <= sourcePos && pos >= bestPos {
			bestPos = pos
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return float64(len(base))*1000 + float64(sourcePos)
	}
	return float64(bestIdx)*1000 + 500 + float64(sourcePos%500)
}

func excerptCommentaryVisualHint(source, unit string) string {
	idx := strings.Index(source, unit)
	if idx < 0 {
		runes := []rune(unit)
		for n := minInt(len(runes), 12); n >= 4; n-- {
			if idx = strings.Index(source, string(runes[:n])); idx >= 0 {
				break
			}
		}
	}
	if idx < 0 {
		return ""
	}
	start := idx - 100
	if start < 0 {
		start = 0
	}
	end := idx + len(unit) + 100
	if end > len(source) {
		end = len(source)
	}
	snippet := strings.TrimSpace(source[start:end])
	if snippet == "" {
		return ""
	}
	if len([]rune(snippet)) > 96 {
		snippet = string([]rune(snippet)[:96]) + "…"
	}
	return snippet
}

func consolidateShortCommentaryDialogue(scenes []llmScene, clipDuration int, speechPace string) []llmScene {
	return consolidateCommentaryDialogueByThreshold(scenes, clipDuration, speechPace, false)
}

func consolidateOrphanCommentaryDialogue(scenes []llmScene, clipDuration int, speechPace string) []llmScene {
	return consolidateCommentaryDialogueByThreshold(scenes, clipDuration, speechPace, true)
}

func consolidateCommentaryDialogueByThreshold(scenes []llmScene, clipDuration int, speechPace string, orphansOnly bool) []llmScene {
	if len(scenes) <= 1 {
		return scenes
	}
	maxRunes := sceneSpeechMaxRunes(llmScene{Duration: clipDuration}, clipDuration, speechPace)
	minRunes := maxRunes * 50 / 100
	if minRunes < 15 {
		minRunes = 15
	}
	shortFragmentRunes := 15
	softMaxRunes := maxRunes + 10
	out := make([]llmScene, 0, len(scenes))
	for i := 0; i < len(scenes); i++ {
		scene := scenes[i]
		dlg := strings.TrimSpace(scene.Dialogue)
		dlgRunes := utf8.RuneCountInString(dlg)
		if dlgRunes == 0 {
			out = append(out, scene)
			continue
		}
		if orphansOnly {
			if dlgRunes >= shortFragmentRunes {
				out = append(out, scene)
				continue
			}
		} else if dlgRunes >= minRunes {
			out = append(out, scene)
			continue
		}
		mergeLimit := maxRunes
		if dlgRunes < shortFragmentRunes {
			mergeLimit = softMaxRunes
		}
		if len(out) > 0 {
			prev := &out[len(out)-1]
			if !commentaryPlotDescriptionsConflict(*prev, scene) {
				merged := joinCommentaryDialogue(prev.Dialogue, dlg)
				if utf8.RuneCountInString(merged) <= mergeLimit {
					prev.Dialogue = merged
					syncSceneDurationFromDialogue(prev, clipDuration, speechPace)
					continue
				}
			}
		}
		if i+1 < len(scenes) {
			next := scenes[i+1]
			if !commentaryPlotDescriptionsConflict(scene, next) {
				merged := joinCommentaryDialogue(dlg, next.Dialogue)
				if utf8.RuneCountInString(merged) <= mergeLimit {
					next.Dialogue = merged
					if scene.Description != "" && next.Description == "" {
						next.Description = scene.Description
					}
					out = append(out, next)
					i++
					syncSceneDurationFromDialogue(&out[len(out)-1], clipDuration, speechPace)
					continue
				}
			}
		}
		out = append(out, scene)
	}
	return out
}

func commentaryPlotDescriptionsConflict(a, b llmScene) bool {
	aDesc := strings.TrimSpace(a.Description)
	bDesc := strings.TrimSpace(b.Description)
	if aDesc == "" || bDesc == "" {
		return false
	}
	return aDesc != bDesc
}

func joinCommentaryDialogue(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(strings.TrimLeft(b, "，,"))
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if strings.HasSuffix(a, "。") || strings.HasSuffix(a, "！") || strings.HasSuffix(a, "？") {
		return a + b
	}
	if strings.HasSuffix(a, "，") || strings.HasSuffix(a, ",") {
		return a + b
	}
	return a + "，" + b
}
