package service

import (
	"strings"
	"unicode/utf8"

	"github.com/autovideo/project-service/internal/productionmode"
	"github.com/autovideo/project-service/internal/speechtext"
)

func (s *EpisodeService) postProcessScenes(scenes []llmScene, clipDuration int, profile productionmode.Profile) []llmScene {
	if profile.ShouldPostProcessMergeScenes() {
		return s.postProcessAdScenes(scenes, clipDuration)
	}
	if profile.IsCommentaryComic() {
		return postProcessCommentaryScenes(scenes, clipDuration)
	}
	return postProcessNarrativeScenes(scenes, clipDuration)
}

func sanitizeStoryboardDialogue(text string) string {
	return speechtext.SanitizeForSpeech(text)
}

func postProcessNarrativeScenes(scenes []llmScene, clipDuration int) []llmScene {
	if len(scenes) == 0 {
		return scenes
	}
	out := make([]llmScene, len(scenes))
	copy(out, scenes)
	for i := range out {
		out[i].Dialogue = strings.TrimSpace(speechtext.SanitizeForSpeech(out[i].Dialogue))
		out[i].Description = strings.TrimSpace(out[i].Description)
		if out[i].Duration <= 0 {
			out[i].Duration = normalizeAdSceneDuration(out[i].Duration, clipDuration)
		}
	}
	return out
}

func postProcessCommentaryScenes(scenes []llmScene, clipDuration int) []llmScene {
	if len(scenes) == 0 {
		return scenes
	}
	out := make([]llmScene, len(scenes))
	copy(out, scenes)

	const minNarrationRunes = 12
	for i := range out {
		rawDialogue := strings.TrimSpace(out[i].Dialogue)
		extracted := speechtext.ExtractNarrationForSpeech(rawDialogue)
		if extracted == "" {
			extracted = speechtext.ExtractNarrationForSpeech(out[i].Description)
		}
		if extracted != "" {
			out[i].Dialogue = extracted
		} else {
			out[i].Dialogue = strings.TrimSpace(speechtext.SanitizeForSpeech(rawDialogue))
		}
		if speechtext.LooksLikeSceneDescription(out[i].Dialogue) {
			out[i].Dialogue = ""
		}
		out[i].Description = strings.TrimSpace(out[i].Description)
		if out[i].Duration <= 0 {
			out[i].Duration = normalizeAdSceneDuration(out[i].Duration, clipDuration)
		}
	}

	merged := make([]llmScene, 0, len(out))
	for _, scene := range out {
		dlg := strings.TrimSpace(scene.Dialogue)
		dlgRunes := utf8.RuneCountInString(dlg)
		if len(merged) > 0 && (dlgRunes == 0 || (dlgRunes < minNarrationRunes && !speechtext.LooksLikeCompleteUtterance(dlg))) {
			prev := &merged[len(merged)-1]
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
	return merged
}
