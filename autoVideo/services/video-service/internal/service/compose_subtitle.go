package service

import (
	"context"
	"os"
	"strings"

	"go.uber.org/zap"
)

// burnPerClipAlignedSubtitles burns subtitles timed to each clip window.
func (s *VideoService) burnPerClipAlignedSubtitles(
	ctx context.Context,
	videoPath string,
	dialogues []string,
	clipDurations []float64,
	transitions []string,
	transitionDurations []float64,
	style SubtitleStyle,
) (string, error) {
	if s.ffmpeg == nil || len(dialogues) == 0 {
		return videoPath, nil
	}
	if len(clipDurations) == 0 {
		clipDurations = make([]float64, len(dialogues))
	}
	srt := buildPerClipTimedSRT(dialogues, clipDurations, transitions, transitionDurations)
	if strings.TrimSpace(srt) == "" {
		return videoPath, nil
	}
	return s.ffmpeg.addSubtitleFromSRTContent(ctx, videoPath, srt, style)
}

// probeRemoteClipDurations downloads/probes each remote clip URL for duration.
func (s *VideoService) probeRemoteClipDurations(ctx context.Context, clipURLs []string) []float64 {
	if s.ffmpeg == nil || len(clipURLs) == 0 {
		return nil
	}
	out := make([]float64, len(clipURLs))
	for i, url := range clipURLs {
		local, err := downloadToTemp(ctx, s.ffmpeg.TempDir, url)
		if err != nil {
			s.logger.Warn("probe clip duration: download failed",
				zap.Int("clip", i),
				zap.Error(err))
			continue
		}
		if d, dErr := s.ffmpeg.ProbeDuration(ctx, local); dErr == nil && d > 0 {
			out[i] = d
		}
		_ = os.Remove(local)
	}
	return out
}

// resolveSubtitleClipDurations picks the best available per-clip durations for
// subtitle alignment: muxed durations from per-clip compose, else remote probes.
func (s *VideoService) resolveSubtitleClipDurations(
	ctx context.Context,
	perClipAudioUsed bool,
	muxedDurations []float64,
	clipURLs []string,
) []float64 {
	if perClipAudioUsed && len(muxedDurations) > 0 {
		return muxedDurations
	}
	probed := s.probeRemoteClipDurations(ctx, clipURLs)
	hasAny := false
	for _, d := range probed {
		if d > 0 {
			hasAny = true
			break
		}
	}
	if hasAny {
		return probed
	}
	return muxedDurations
}
