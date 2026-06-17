package service

import (
	"regexp"
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
	_, targetMax, _ := commentarySceneSpeechBounds(clipDuration, speechPace)
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
	packed := speechtext.PackSpeechUnitsToMaxRunes(uncovered, targetMax)
	for _, chunk := range packed {
		if isSpeechUnitCovered(chunk, base) {
			continue
		}
		sourcePos := findUnitSourcePosition(source, chunk)
		sc := llmScene{
			Dialogue: chunk,
			Duration: clipDuration,
		}
		if matchedHint := findCommentarySceneHint(chunk, base); matchedHint != nil {
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
	if bestIdx >= 0 {
		return float64(bestIdx)*1000 + 500 + float64(sourcePos%500)
	}

	// If no matched scene before sourcePos, try to find the first matched scene after sourcePos
	nextIdx := -1
	nextPos := -1
	for i, scene := range base {
		pos := findSceneSourcePosition(source, scene)
		if pos >= 0 && pos >= sourcePos {
			if nextPos < 0 || pos < nextPos {
				nextPos = pos
				nextIdx = i
			}
		}
	}
	if nextIdx >= 0 {
		return float64(nextIdx)*1000 - 500 + float64(sourcePos%500)
	}

	// Fallback if no scenes matched at all
	return float64(len(base))*1000 + float64(sourcePos)
}

func excerptCommentaryVisualHint(source, unit string) string {
	narrator := inferNarratorFromSource(source)
	return extractCommentaryVisualDescriptionFromSource(source, unit, nil, narrator)
}

func inferNarratorFromSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	head := source
	if runes := []rune(head); len(runes) > 1200 {
		head = string(runes[:1200])
	}
	if strings.Contains(head, "刘师傅") && strings.Contains(head, "我") {
		return "刘师傅"
	}
	if m := regexp.MustCompile(`([\p{Han}]{2,4}师傅)`).FindStringSubmatch(head); len(m) > 1 {
		return m[1]
	}
	return ""
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
	minRunes, _, hardMax := commentarySceneSpeechBounds(clipDuration, speechPace)
	shortFragmentRunes := minRunes
	softMaxRunes := hardMax
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
		mergeLimit := hardMax
		if dlgRunes < shortFragmentRunes {
			mergeLimit = softMaxRunes
		}
		if len(out) > 0 {
			prev := &out[len(out)-1]
			if !commentaryScenesCanMergeDialogue(*prev, scene) {
				out = append(out, scene)
				continue
			}
			merged := joinCommentaryDialogue(prev.Dialogue, dlg)
			if utf8.RuneCountInString(merged) <= mergeLimit {
				prev.Dialogue = merged
				syncCommentarySceneDuration(prev, clipDuration)
				continue
			}
		}
		if i+1 < len(scenes) {
			next := scenes[i+1]
			if !commentaryScenesCanMergeDialogue(scene, next) {
				out = append(out, scene)
				continue
			}
			merged := joinCommentaryDialogue(dlg, next.Dialogue)
			if utf8.RuneCountInString(merged) <= mergeLimit {
				next.Dialogue = merged
				if scene.Description != "" && next.Description == "" {
					next.Description = scene.Description
				}
				if scene.Location != "" && next.Location == "" {
					next.Location = scene.Location
				}
				if scene.LocationZone != "" && next.LocationZone == "" {
					next.LocationZone = scene.LocationZone
				}
				out = append(out, next)
				i++
				syncCommentarySceneDuration(&out[len(out)-1], clipDuration)
				continue
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
