package service

import (
	"strings"
	"unicode/utf8"

	"github.com/autovideo/project-service/internal/productionmode"
	"github.com/autovideo/project-service/internal/speechtext"
)

func (s *EpisodeService) postProcessScenes(scenes []llmScene, clipDuration int, speechPace string, profile productionmode.Profile) []llmScene {
	if profile.ShouldPostProcessMergeScenes() {
		return s.postProcessAdScenes(scenes, clipDuration)
	}
	if profile.IsCommentaryComic() {
		return postProcessCommentaryScenes(scenes, clipDuration, speechPace)
	}
	return postProcessRhythmicScenes(scenes, clipDuration, speechPace)
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
		if out[i].Duration <= 0 {
			out[i].Duration = inferSceneDurationFromDialogue(out[i].Dialogue, clipDuration, speechPace)
		} else {
			out[i].Duration = clampDuration(out[i].Duration, 2, 12)
		}
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
		if out[i].Duration <= 0 {
			out[i].Duration = inferSceneDurationFromDialogue(out[i].Dialogue, clipDuration, speechPace)
		} else {
			out[i].Duration = clampDuration(out[i].Duration, 2, 12)
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
	}
	return scenes
}
