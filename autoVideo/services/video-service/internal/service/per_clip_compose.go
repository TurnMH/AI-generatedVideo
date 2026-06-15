package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/autovideo/video-service/internal/model"
	"go.uber.org/zap"
)

// shouldAttachExternalDubbing decides whether to mux TTS / dubbing audio onto the
// final video. Native-audio models (e.g. seedance) skip external dubbing by default
// only when the user explicitly requested model-native speech (generate_audio=true).
// Commentary/drama projects with dialogues should still get TTS even on those models.
func shouldAttachExternalDubbing(nativeAudio, attachDubbingExplicit, generateNative bool, dialogues []string) bool {
	if attachDubbingExplicit {
		return true
	}
	if !nativeAudio {
		return true
	}
	if generateNative {
		return false
	}
	return hasAnyNonEmpty(dialogues)
}

// hasAnyNonEmpty returns true when at least one string in dialogues is not
// whitespace-only — the trigger for the per-clip audio compose path.
func hasAnyNonEmpty(dialogues []string) bool {
	for _, d := range dialogues {
		if strings.TrimSpace(d) != "" {
			return true
		}
	}
	return false
}

// tryPerClipAudioCompose attempts to synthesize per-clip audio, mux each clip,
// then concat into a single merged mp4 with audio aligned to the storyboard.
// Returns (mergedLocalPath, perClipDurations, true) on success. On any failure,
// returns ("", nil, false) so the caller can fall back to plain concat.
//
// Pre-generated storyboard dubbing (from the dubbing tab) is preferred over
// on-the-fly TTS so the final video matches what the user already approved.
func (s *VideoService) tryPerClipAudioCompose(
	ctx context.Context,
	task *model.VideoTask,
	clipURLs []string,
	perClipDialogues []string,
	transitions []string,
	transitionDurations []float64,
) (string, []float64, bool) {
	if s.dubbing == nil || s.ffmpeg == nil {
		return "", nil, false
	}

	voiceModel, voiceRate, voicePitch, voiceVolume := s.repo.FindDubbingVoiceConfig(ctx, task.ProjectID, task.EpisodeID)
	charVoiceBindings := s.dubbing.fetchCharacterVoiceBindings(ctx, task.ProjectID)
	if v := renderConfigString(task.RenderConfig, "voice_model"); v != "" {
		voiceModel = v
	}
	if v := renderConfigString(task.RenderConfig, "voice_rate"); v != "" {
		voiceRate = v
	}
	if v := renderConfigString(task.RenderConfig, "voice_pitch"); v != "" {
		voicePitch = v
	}
	if v := renderConfigString(task.RenderConfig, "voice_volume"); v != "" {
		voiceVolume = v
	}
	if voiceModel == "" {
		voiceModel = "default"
	}

	dialogues := make([]string, len(clipURLs))
	for i := range clipURLs {
		if i < len(perClipDialogues) {
			dialogues[i] = perClipDialogues[i]
		}
	}

	storyboardIDs := extractStoryboardIDs(task.RenderConfig, len(clipURLs))
	audioPaths, err := s.resolvePerClipAudioPaths(
		ctx, task, storyboardIDs, dialogues,
		voiceModel, voiceRate, voicePitch, voiceVolume,
	)
	if err != nil {
		s.logger.Warn("per-clip audio compose: audio resolution failed, falling back",
			zap.Int64("task_id", task.ID), zap.Error(err))
		return "", nil, false
	}
	defer func() {
		for _, p := range audioPaths {
			if p != "" {
				_ = os.Remove(p)
			}
		}
	}()

	hasAudio := false
	for _, p := range audioPaths {
		if p != "" {
			hasAudio = true
			break
		}
	}
	if !hasAudio && !hasAnyNonEmpty(dialogues) {
		return "", nil, false
	}

	s.logger.Info("per-clip audio compose: start",
		zap.Int64("task_id", task.ID),
		zap.Int("clips", len(clipURLs)),
		zap.String("voice_model", voiceModel),
		zap.Int("character_voice_bindings", len(charVoiceBindings)),
		zap.Int("storyboard_ids", len(storyboardIDs)))

	workDir, err := os.MkdirTemp(s.ffmpeg.TempDir, "perclip-*")
	if err != nil {
		s.logger.Warn("per-clip audio compose: mkdir failed, falling back",
			zap.Int64("task_id", task.ID), zap.Error(err))
		return "", nil, false
	}

	clipDurations := make([]float64, len(clipURLs))
	muxed := make([]string, 0, len(clipURLs))
	for i, url := range clipURLs {
		var audio string
		if i < len(audioPaths) {
			audio = audioPaths[i]
		}
		local, err := s.ffmpeg.MuxClipAudioNormalized(ctx, url, audio, workDir, i, task.VideoMode)
		if err != nil {
			s.logger.Warn("per-clip audio compose: mux failed, falling back",
				zap.Int64("task_id", task.ID),
				zap.Int("clip", i),
				zap.Error(err))
			_ = os.RemoveAll(workDir)
			return "", nil, false
		}
		muxed = append(muxed, local)
		if d, dErr := s.ffmpeg.ProbeDuration(ctx, local); dErr == nil && d > 0 {
			clipDurations[i] = d
		}
	}

	merged, err := s.ffmpeg.ConcatLocalNormalizedClipsWithTransitionPlan(ctx, muxed, transitions, transitionDurations)
	if err != nil {
		s.logger.Warn("per-clip audio compose: concat failed, falling back",
			zap.Int64("task_id", task.ID), zap.Error(err))
		_ = os.RemoveAll(workDir)
		return "", nil, false
	}

	finalDir, err := os.MkdirTemp(s.ffmpeg.TempDir, "perclip-final-*")
	if err == nil {
		dst := filepath.Join(finalDir, "merged.mp4")
		if err := copyFile(merged, dst); err == nil {
			_ = os.RemoveAll(workDir)
			s.logger.Info("per-clip audio compose: done",
				zap.Int64("task_id", task.ID),
				zap.Int("muxed_clips", len(muxed)),
				zap.String("final", dst))
			return dst, clipDurations, true
		}
	}

	s.logger.Info("per-clip audio compose: done (in-workdir)",
		zap.Int64("task_id", task.ID),
		zap.Int("muxed_clips", len(muxed)))
	return merged, clipDurations, true
}

// resolvePerClipAudioPaths prefers pre-generated storyboard dubbing audio and
// only synthesizes TTS for clips that still lack audio.
func (s *VideoService) resolvePerClipAudioPaths(
	ctx context.Context,
	task *model.VideoTask,
	storyboardIDs []int64,
	dialogues []string,
	voiceModel, voiceRate, voicePitch, voiceVolume string,
) ([]string, error) {
	results := make([]string, len(dialogues))
	sbAudio := s.repo.FindStoryboardDubbingAudios(ctx, task.ProjectID, storyboardIDs)

	for i := range dialogues {
		if i >= len(storyboardIDs) || storyboardIDs[i] <= 0 {
			continue
		}
		url := sbAudio[storyboardIDs[i]]
		if url == "" {
			continue
		}
		local, err := downloadToTemp(ctx, s.ffmpeg.TempDir, url)
		if err != nil {
			s.logger.Warn("per-clip audio compose: storyboard dubbing download failed",
				zap.Int64("task_id", task.ID),
				zap.Int("clip", i),
				zap.Int64("storyboard_id", storyboardIDs[i]),
				zap.Error(err))
			continue
		}
		results[i] = local
		s.logger.Info("per-clip audio compose: reused storyboard dubbing",
			zap.Int64("task_id", task.ID),
			zap.Int("clip", i),
			zap.Int64("storyboard_id", storyboardIDs[i]))
	}

	ttsDialogues := make([]string, len(dialogues))
	needTTS := false
	for i, dialogue := range dialogues {
		if results[i] != "" {
			continue
		}
		ttsDialogues[i] = dialogue
		if strings.TrimSpace(dialogue) != "" {
			needTTS = true
		}
	}
	if !needTTS {
		return results, nil
	}

	synthPaths, err := s.dubbing.SynthesizeClipAudios(
		ctx, task.ProjectID,
		deref(task.EpisodeID),
		ttsDialogues, voiceModel, voiceRate, voicePitch, voiceVolume,
	)
	if err != nil {
		return nil, err
	}
	for i, p := range synthPaths {
		if results[i] == "" && p != "" {
			results[i] = p
		}
	}
	return results, nil
}

func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func extractStoryboardIDs(renderConfig model.RenderConfig, n int) []int64 {
	result := make([]int64, n)
	if len(renderConfig) == 0 {
		return result
	}
	raw, ok := renderConfig["storyboard_ids"]
	if !ok {
		return result
	}
	switch v := raw.(type) {
	case []int64:
		for i := 0; i < n && i < len(v); i++ {
			result[i] = v[i]
		}
	case []int:
		for i := 0; i < n && i < len(v); i++ {
			result[i] = int64(v[i])
		}
	case []float64:
		for i := 0; i < n && i < len(v); i++ {
			result[i] = int64(v[i])
		}
	case []interface{}:
		for i := 0; i < n && i < len(v); i++ {
			switch id := v[i].(type) {
			case float64:
				result[i] = int64(id)
			case int64:
				result[i] = id
			case int:
				result[i] = int64(id)
			}
		}
	}
	return result
}
