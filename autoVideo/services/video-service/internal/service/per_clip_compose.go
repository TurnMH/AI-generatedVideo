package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/autovideo/video-service/internal/model"
	"go.uber.org/zap"
)

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
// Returns (mergedLocalPath, true) on success. On any failure, returns ("", false)
// so the caller can fall back to the legacy concat-then-attach-audio flow.
//
// The function prefers the voice configuration from the latest DubbingTask for
// this project+episode (so the user's choice of voice model propagates without
// the user having to re-pick it when they render the video).
func (s *VideoService) tryPerClipAudioCompose(
	ctx context.Context,
	task *model.VideoTask,
	clipURLs []string,
	perClipDialogues []string,
	transition string,
	transitionDur float64,
) (string, bool) {
	if s.dubbing == nil || s.ffmpeg == nil {
		return "", false
	}

	// Resolve voice config from the matching dubbing task (if any); fall back to
	// RenderConfig overrides if the user specified them on the video task.
	voiceModel, voiceRate, voicePitch, voiceVolume := s.repo.FindDubbingVoiceConfig(ctx, task.ProjectID, task.EpisodeID)
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

	// Align slice: only synthesize for clips we actually have URLs for.
	dialogues := make([]string, len(clipURLs))
	for i := range clipURLs {
		if i < len(perClipDialogues) {
			dialogues[i] = perClipDialogues[i]
		}
	}

	s.logger.Info("per-clip audio compose: start",
		zap.Int64("task_id", task.ID),
		zap.Int("clips", len(clipURLs)),
		zap.String("voice_model", voiceModel))

	audioPaths, err := s.dubbing.SynthesizeClipAudios(
		ctx, task.ProjectID,
		deref(task.EpisodeID),
		dialogues, voiceModel, voiceRate, voicePitch, voiceVolume,
	)
	if err != nil {
		s.logger.Warn("per-clip audio compose: synthesis failed, falling back",
			zap.Int64("task_id", task.ID), zap.Error(err))
		return "", false
	}
	defer func() {
		for _, p := range audioPaths {
			if p != "" {
				_ = os.Remove(p)
			}
		}
	}()

	// Build a single work dir shared by all muxed clips so the final concat
	// demuxer / xfade pass can live next to them.
	workDir, err := os.MkdirTemp(s.ffmpeg.TempDir, "perclip-*")
	if err != nil {
		s.logger.Warn("per-clip audio compose: mkdir failed, falling back",
			zap.Int64("task_id", task.ID), zap.Error(err))
		return "", false
	}

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
			return "", false
		}
		muxed = append(muxed, local)
	}

	merged, err := s.ffmpeg.ConcatLocalNormalizedClips(ctx, muxed, transition, transitionDur)
	if err != nil {
		s.logger.Warn("per-clip audio compose: concat failed, falling back",
			zap.Int64("task_id", task.ID), zap.Error(err))
		_ = os.RemoveAll(workDir)
		return "", false
	}

	// Move merged out of workDir into a sibling so post-processing (subtitles,
	// BGM) can live alongside it without the per-clip temp litter. The caller
	// already schedules cleanup via filepath.Dir(mergedPath) removal.
	finalDir, err := os.MkdirTemp(s.ffmpeg.TempDir, "perclip-final-*")
	if err == nil {
		dst := filepath.Join(finalDir, "merged.mp4")
		if err := copyFile(merged, dst); err == nil {
			_ = os.RemoveAll(workDir)
			s.logger.Info("per-clip audio compose: done",
				zap.Int64("task_id", task.ID),
				zap.Int("muxed_clips", len(muxed)),
				zap.String("final", dst))
			return dst, true
		}
	}

	s.logger.Info("per-clip audio compose: done (in-workdir)",
		zap.Int64("task_id", task.ID),
		zap.Int("muxed_clips", len(muxed)))
	return merged, true
}

func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
