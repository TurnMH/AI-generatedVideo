package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MuxClipAudioNormalized downloads the clip from clipURL, optionally attaches
// audioLocalPath (pads video with freeze-frame OR audio with silence to match
// the longer track), and produces a re-encoded normalized output of
// (resolution × 24fps × yuv420p × aac stereo 44.1kHz) ready for concat.
//
// When audioLocalPath is empty, the output still gets a silent aac track so
// concat/xfade with other audio-bearing clips stays balanced.
//
// Returns the normalized local MP4 path inside workDir.
func (f *FFmpegService) MuxClipAudioNormalized(
	ctx context.Context,
	clipURL, audioLocalPath, workDir string,
	idx int,
	videoMode string,
) (string, error) {
	rawPath := filepath.Join(workDir, fmt.Sprintf("raw%03d.mp4", idx))
	if err := f.DownloadFile(ctx, clipURL, rawPath); err != nil {
		return "", fmt.Errorf("download clip %d: %w", idx, err)
	}

	tw, th := videoModeScale(videoMode)
	scaleFilter := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=24",
		tw, th, tw, th,
	)
	outPath := filepath.Join(workDir, fmt.Sprintf("clip%03d.mp4", idx))

	// No external audio → keep clip's own audio (or inject silence if missing).
	if strings.TrimSpace(audioLocalPath) == "" {
		hasAudio, err := f.HasAudioStream(ctx, rawPath)
		if err != nil {
			hasAudio = true
		}
		args := []string{"-i", rawPath}
		if !hasAudio {
			args = append(args,
				"-f", "lavfi",
				"-i", "anullsrc=channel_layout=stereo:sample_rate=44100",
			)
		}
		args = append(args, "-vf", scaleFilter, "-map", "0:v:0")
		if hasAudio {
			args = append(args, "-map", "0:a:0")
		} else {
			args = append(args, "-map", "1:a:0", "-shortest")
		}
		args = append(args,
			"-c:v", "libx264", "-preset", "fast", "-crf", "20",
			"-c:a", "aac", "-ar", "44100", "-ac", "2",
			"-pix_fmt", "yuv420p", "-movflags", "+faststart",
			outPath,
		)
		if err := f.RunFFmpeg(ctx, args...); err != nil {
			return "", fmt.Errorf("normalize clip %d (no external audio): %w", idx, err)
		}
		return outPath, nil
	}

	// External audio present — align with the clip's video length.
	videoDur, vErr := f.ProbeDuration(ctx, rawPath)
	audioDur, aErr := f.ProbeDuration(ctx, audioLocalPath)
	if vErr != nil || aErr != nil || videoDur <= 0 || audioDur <= 0 {
		// Fallback: let ffmpeg decide via -shortest with the provided audio.
		args := []string{
			"-i", rawPath, "-i", audioLocalPath,
			"-vf", scaleFilter,
			"-map", "0:v:0", "-map", "1:a:0",
			"-c:v", "libx264", "-preset", "fast", "-crf", "20",
			"-c:a", "aac", "-ar", "44100", "-ac", "2",
			"-shortest",
			"-pix_fmt", "yuv420p", "-movflags", "+faststart",
			outPath,
		}
		if err := f.RunFFmpeg(ctx, args...); err != nil {
			return "", fmt.Errorf("mux clip %d (fallback): %w", idx, err)
		}
		return outPath, nil
	}

	// Case A — audio is longer: pad video tail by cloning last frame so no dialogue is cut.
	if videoDur+0.05 < audioDur {
		padDur := audioDur - videoDur
		filter := fmt.Sprintf("[0:v]%s,tpad=stop_mode=clone:stop_duration=%s[v]",
			scaleFilter, formatFFmpegDuration(padDur))
		args := []string{
			"-i", rawPath, "-i", audioLocalPath,
			"-filter_complex", filter,
			"-map", "[v]", "-map", "1:a:0",
			"-c:v", "libx264", "-preset", "fast", "-crf", "20",
			"-c:a", "aac", "-ar", "44100", "-ac", "2",
			"-t", formatFFmpegDuration(audioDur),
			"-pix_fmt", "yuv420p", "-movflags", "+faststart",
			outPath,
		}
		if err := f.RunFFmpeg(ctx, args...); err != nil {
			return "", fmt.Errorf("mux clip %d (pad video): %w", idx, err)
		}
		return outPath, nil
	}

	// Case B — video is longer: pad audio with silence so no visual is lost.
	if audioDur+0.05 < videoDur {
		padDur := videoDur - audioDur
		filter := fmt.Sprintf("[0:v]%s[v];[1:a]apad=pad_dur=%s[a]",
			scaleFilter, formatFFmpegDuration(padDur))
		args := []string{
			"-i", rawPath, "-i", audioLocalPath,
			"-filter_complex", filter,
			"-map", "[v]", "-map", "[a]",
			"-c:v", "libx264", "-preset", "fast", "-crf", "20",
			"-c:a", "aac", "-ar", "44100", "-ac", "2",
			"-t", formatFFmpegDuration(videoDur),
			"-pix_fmt", "yuv420p", "-movflags", "+faststart",
			outPath,
		}
		if err := f.RunFFmpeg(ctx, args...); err != nil {
			return "", fmt.Errorf("mux clip %d (pad audio): %w", idx, err)
		}
		return outPath, nil
	}

	// Case C — durations match within tolerance.
	target := videoDur
	if audioDur > target {
		target = audioDur
	}
	args := []string{
		"-i", rawPath, "-i", audioLocalPath,
		"-vf", scaleFilter,
		"-map", "0:v:0", "-map", "1:a:0",
		"-c:v", "libx264", "-preset", "fast", "-crf", "20",
		"-c:a", "aac", "-ar", "44100", "-ac", "2",
		"-t", formatFFmpegDuration(target),
		"-pix_fmt", "yuv420p", "-movflags", "+faststart",
		outPath,
	}
	if err := f.RunFFmpeg(ctx, args...); err != nil {
		return "", fmt.Errorf("mux clip %d (match): %w", idx, err)
	}
	return outPath, nil
}

// ConcatLocalNormalizedClips concatenates already-normalized local clip files
// (produced by MuxClipAudioNormalized) into a single MP4. Assumes all clips
// share resolution, framerate and codec settings — it skips the download +
// re-encode stage to save ~N× encode cost.
//
// When transition is empty, uses the concat demuxer (stream copy, fast).
// When transition is set, applies video xfade + audio acrossfade.
func (f *FFmpegService) ConcatLocalNormalizedClips(
	ctx context.Context,
	localPaths []string,
	transition string,
	transitionDur float64,
) (string, error) {
	if len(localPaths) == 0 {
		return "", fmt.Errorf("no clips to concat")
	}
	if len(localPaths) == 1 {
		return localPaths[0], nil
	}
	workDir := filepath.Dir(localPaths[0])
	merged := filepath.Join(workDir, "merged.mp4")

	if transition == "" || transition == "none" {
		concatFile := filepath.Join(workDir, "concat.txt")
		var sb strings.Builder
		for _, p := range localPaths {
			sb.WriteString(fmt.Sprintf("file '%s'\n", p))
		}
		if err := os.WriteFile(concatFile, []byte(sb.String()), 0o644); err != nil {
			return "", fmt.Errorf("write concat.txt: %w", err)
		}
		if err := f.RunFFmpeg(ctx,
			"-f", "concat", "-safe", "0",
			"-i", concatFile,
			"-c", "copy",
			"-movflags", "+faststart",
			merged,
		); err != nil {
			return "", fmt.Errorf("concat local ffmpeg: %w", err)
		}
		return merged, nil
	}

	// xfade + acrossfade path — same algorithm as ConcatClipsWithTransitions but
	// inputs are already normalized.
	if transitionDur <= 0 {
		transitionDur = 0.5
	}
	durations := make([]float64, len(localPaths))
	for i, p := range localPaths {
		d, err := f.ProbeDuration(ctx, p)
		if err != nil || d <= 0 {
			d = 5.0
		}
		durations[i] = d
	}
	var inputs []string
	for _, p := range localPaths {
		inputs = append(inputs, "-i", p)
	}
	var fc strings.Builder
	var offset float64
	prevV := "[0:v]"
	prevA := "[0:a]"
	for i := 1; i < len(localPaths); i++ {
		offset += durations[i-1] - transitionDur
		nextV := fmt.Sprintf("[v%d]", i)
		nextA := fmt.Sprintf("[a%d]", i)
		if i == len(localPaths)-1 {
			nextV = "[vout]"
			nextA = "[aout]"
		}
		fmt.Fprintf(&fc, "%s[%d:v]xfade=transition=%s:duration=%.3f:offset=%.3f%s;",
			prevV, i, transition, transitionDur, offset, nextV)
		fmt.Fprintf(&fc, "%s[%d:a]acrossfade=d=%.3f%s;",
			prevA, i, transitionDur, nextA)
		prevV = nextV
		prevA = nextA
	}
	fcStr := strings.TrimRight(fc.String(), ";")
	args := inputs
	args = append(args,
		"-filter_complex", fcStr,
		"-map", "[vout]", "-map", "[aout]",
		"-c:v", "libx264", "-preset", "fast", "-crf", "20",
		"-c:a", "aac", "-ar", "44100", "-ac", "2",
		"-pix_fmt", "yuv420p", "-movflags", "+faststart",
		merged,
	)
	if err := f.RunFFmpeg(ctx, args...); err != nil {
		return "", fmt.Errorf("xfade concat (local): %w", err)
	}
	return merged, nil
}
