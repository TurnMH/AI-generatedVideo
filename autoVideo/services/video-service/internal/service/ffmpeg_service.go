package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// FFmpegService provides video processing operations backed by the ffmpeg binary.
type FFmpegService struct {
	TempDir string
	Bin     string // path to ffmpeg binary, defaults to "ffmpeg"
}

// SubtitleStyle controls the visual appearance of burned-in subtitles.
type SubtitleStyle struct {
	FontName    string  // e.g. "Arial", "NotoSansCJK" (default: "Arial")
	FontSize    int     // default 24
	FontColor   string  // e.g. "white", "&H00FFFFFF" (default: "white")
	OutlineColor string // e.g. "black", "&H00000000" (default: "black")
	OutlineWidth int    // default 2
	Bold        bool
	Alignment   int     // ASS alignment (2=bottom-center, 8=top-center, default: 2)
	MarginV     int     // vertical margin in pixels (default: 20)
}

// ForceStyle returns the FFmpeg ASS force_style string for this SubtitleStyle.
func (s SubtitleStyle) ForceStyle() string {
	if s.FontName == "" {
		s.FontName = "Arial"
	}
	if s.FontSize == 0 {
		s.FontSize = 24
	}
	if s.FontColor == "" {
		s.FontColor = "&H00FFFFFF"
	} else if !strings.HasPrefix(s.FontColor, "&H") {
		// convert CSS color names
		switch strings.ToLower(s.FontColor) {
		case "white":
			s.FontColor = "&H00FFFFFF"
		case "yellow":
			s.FontColor = "&H0000FFFF"
		case "black":
			s.FontColor = "&H00000000"
		case "red":
			s.FontColor = "&H000000FF"
		default:
			s.FontColor = "&H00FFFFFF"
		}
	}
	if s.OutlineColor == "" {
		s.OutlineColor = "&H00000000"
	}
	if s.OutlineWidth == 0 {
		s.OutlineWidth = 2
	}
	if s.Alignment == 0 {
		s.Alignment = 2
	}
	if s.MarginV == 0 {
		s.MarginV = 20
	}
	bold := 0
	if s.Bold {
		bold = 1
	}
	return fmt.Sprintf(
		"FontName=%s,FontSize=%d,PrimaryColour=%s,OutlineColour=%s,Outline=%d,Bold=%d,Alignment=%d,MarginV=%d",
		s.FontName, s.FontSize, s.FontColor, s.OutlineColor, s.OutlineWidth, bold, s.Alignment, s.MarginV,
	)
}

// NewFFmpegService —— 创建 FFmpeg 服务实例，设置临时目录和二进制路径
func NewFFmpegService(tempDir, bin string) *FFmpegService {
	if bin == "" {
		bin = "ffmpeg"
	}
	if tempDir == "" {
		tempDir = "/tmp/video-service"
	}
	return &FFmpegService{TempDir: tempDir, Bin: bin}
}

// videoModeScale returns the FFmpeg scale/pad filter string for the given video_mode.
// Supported modes: "16:9" (default), "9:16", "1:1".
func videoModeScale(videoMode string) (w, h int) {
	switch videoMode {
	case "9:16":
		return 720, 1280
	case "1:1":
		return 720, 720
	default: // "16:9", "frame_animation", ""
		return 1280, 720
	}
}

// ConcatClips —— 下载所有片段并拼接为一个 MP4 文件，返回合并文件路径
// ConcatClips downloads all clip URLs and concatenates them into a single MP4.
// videoMode controls the output aspect ratio: "16:9" (default), "9:16", "1:1".
func (f *FFmpegService) ConcatClips(ctx context.Context, clipURLs []string, videoMode string) (string, error) {
	workDir, err := os.MkdirTemp(f.TempDir, "concat-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Download each clip
	var rawPaths []string
	for i, u := range clipURLs {
		dest := filepath.Join(workDir, fmt.Sprintf("raw%03d.mp4", i))
		if err := f.DownloadFile(ctx, u, dest); err != nil {
			return "", fmt.Errorf("download clip %d: %w", i, err)
		}
		rawPaths = append(rawPaths, dest)
	}

	// Re-encode each clip to a uniform format (aspect-ratio-aware, 24fps, yuv420p)
	// This ensures clips from different AI models can be cleanly concatenated.
	tw, th := videoModeScale(videoMode)
	scaleFilter := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=24",
		tw, th, tw, th,
	)
	var clipPaths []string
	for i, raw := range rawPaths {
		normalized := filepath.Join(workDir, fmt.Sprintf("clip%03d.mp4", i))
		hasAudio, audioErr := f.HasAudioStream(ctx, raw)
		if audioErr != nil {
			hasAudio = true
		}

		args := []string{"-i", raw}
		if !hasAudio {
			args = append(args,
				"-f", "lavfi",
				"-i", "anullsrc=channel_layout=stereo:sample_rate=44100",
			)
		}
		args = append(args,
			"-vf", scaleFilter,
			"-map", "0:v:0",
		)
		if hasAudio {
			args = append(args, "-map", "0:a:0")
		} else {
			args = append(args,
				"-map", "1:a:0",
				"-shortest",
			)
		}
		args = append(args,
			"-c:v", "libx264", "-preset", "fast", "-crf", "20",
			"-c:a", "aac", "-ar", "44100", "-ac", "2",
			"-pix_fmt", "yuv420p",
			"-movflags", "+faststart",
			normalized,
		)
		if err := f.RunFFmpeg(ctx, args...); err != nil {
			return "", fmt.Errorf("normalize clip %d: %w", i, err)
		}
		clipPaths = append(clipPaths, normalized)
	}

	// Write concat list
	concatFile := filepath.Join(workDir, "concat.txt")
	var sb strings.Builder
	for _, p := range clipPaths {
		sb.WriteString(fmt.Sprintf("file '%s'\n", p))
	}
	if err := os.WriteFile(concatFile, []byte(sb.String()), 0o644); err != nil {
		return "", fmt.Errorf("write concat.txt: %w", err)
	}

	merged := filepath.Join(workDir, "merged.mp4")
	if err := f.RunFFmpeg(ctx,
		"-f", "concat", "-safe", "0",
		"-i", concatFile,
		"-c", "copy",
		"-movflags", "+faststart",
		merged,
	); err != nil {
		return "", fmt.Errorf("concat ffmpeg: %w", err)
	}
	return merged, nil
}

// AddAudio —— 为视频附加外部音轨，返回新视频文件路径
// AddAudio attaches the provided audio track to the video, looping or trimming it to match the video duration.
func (f *FFmpegService) AddAudio(ctx context.Context, videoPath, audioURL string) (string, error) {
	workDir := filepath.Dir(videoPath)

	audioPath := filepath.Join(workDir, "audio"+remoteFileExt(audioURL))
	if err := f.DownloadFile(ctx, audioURL, audioPath); err != nil {
		return "", fmt.Errorf("download audio: %w", err)
	}

	output := filepath.Join(workDir, "with_audio.mp4")

	videoDuration, videoErr := f.ProbeDuration(ctx, videoPath)
	audioDuration, audioErr := f.ProbeDuration(ctx, audioPath)
	if videoErr != nil || audioErr != nil || videoDuration <= 0 || audioDuration <= 0 {
		if err := f.RunFFmpeg(ctx,
			"-i", videoPath,
			"-stream_loop", "-1",
			"-i", audioPath,
			"-map", "0:v:0",
			"-map", "1:a:0",
			"-c:v", "copy",
			"-c:a", "aac",
			"-shortest",
			"-movflags", "+faststart",
			output,
		); err != nil {
			return "", fmt.Errorf("add audio ffmpeg: %w", err)
		}
		return output, nil
	}

	targetDuration := videoDuration
	if audioDuration > targetDuration {
		targetDuration = audioDuration
	}

	// Case A — audio is longer than video: pad the video (freeze last frame) so
	// dialogue is never cut off; final output length = audio length.
	if videoDuration+0.05 < audioDuration {
		padDuration := audioDuration - videoDuration
		if err := f.RunFFmpeg(ctx,
			"-i", videoPath,
			"-i", audioPath,
			"-filter_complex", fmt.Sprintf("[0:v]tpad=stop_mode=clone:stop_duration=%s[v]", formatFFmpegDuration(padDuration)),
			"-map", "[v]",
			"-map", "1:a:0",
			"-c:v", "libx264",
			"-preset", "fast",
			"-crf", "20",
			"-c:a", "aac",
			"-t", formatFFmpegDuration(audioDuration),
			"-pix_fmt", "yuv420p",
			"-movflags", "+faststart",
			output,
		); err != nil {
			return "", fmt.Errorf("add audio ffmpeg (pad video): %w", err)
		}
		return output, nil
	}

	// Case B — video is longer than audio: pad the audio with silence so no
	// visual content is lost; final output length = video length. Truncating
	// the video to audio length (previous behavior) was the main cause of
	// 配音与视频输出对应不上 — the last clip would be chopped off.
	if audioDuration+0.05 < videoDuration {
		padDuration := videoDuration - audioDuration
		if err := f.RunFFmpeg(ctx,
			"-i", videoPath,
			"-i", audioPath,
			"-filter_complex", fmt.Sprintf("[1:a]apad=pad_dur=%s[a]", formatFFmpegDuration(padDuration)),
			"-map", "0:v:0",
			"-map", "[a]",
			"-c:v", "copy",
			"-c:a", "aac",
			"-t", formatFFmpegDuration(videoDuration),
			"-movflags", "+faststart",
			output,
		); err != nil {
			return "", fmt.Errorf("add audio ffmpeg (pad audio): %w", err)
		}
		return output, nil
	}

	// Case C — lengths match within tolerance.
	if err := f.RunFFmpeg(ctx,
		"-i", videoPath,
		"-i", audioPath,
		"-map", "0:v:0",
		"-map", "1:a:0",
		"-c:v", "copy",
		"-c:a", "aac",
		"-t", formatFFmpegDuration(targetDuration),
		"-movflags", "+faststart",
		output,
	); err != nil {
		return "", fmt.Errorf("add audio ffmpeg: %w", err)
	}
	return output, nil
}

// AddSubtitle —— 将字幕文本烧录到视频中，返回新视频文件路径
// AddSubtitle burns subtitleText into the video as a simple SRT overlay.
func (f *FFmpegService) AddSubtitle(ctx context.Context, videoPath, subtitleText string) (string, error) {
	return f.AddSubtitleWithStyle(ctx, videoPath, subtitleText, SubtitleStyle{})
}

// AddSubtitleWithStyle —— 以指定样式将字幕文本烧录到视频中
// AddSubtitleWithStyle burns subtitleText into the video with customizable style.
func (f *FFmpegService) AddSubtitleWithStyle(ctx context.Context, videoPath, subtitleText string, style SubtitleStyle) (string, error) {
	workDir := filepath.Dir(videoPath)

	srtPath := filepath.Join(workDir, "sub.srt")
	srt := buildSRT(subtitleText)
	if err := os.WriteFile(srtPath, []byte(srt), 0o644); err != nil {
		return "", fmt.Errorf("write srt: %w", err)
	}

	output := filepath.Join(workDir, "with_sub.mp4")
	subtitleFilter := fmt.Sprintf("subtitles=%s:force_style='%s'", srtPath, style.ForceStyle())
	if err := f.RunFFmpeg(ctx,
		"-i", videoPath,
		"-vf", subtitleFilter,
		output,
	); err != nil {
		return "", fmt.Errorf("add subtitle ffmpeg: %w", err)
	}
	return output, nil
}

// AddSubtitleFromVTT —— 下载 VTT 字幕文件，转换为带时间轴的 SRT 后烧录到视频
// AddSubtitleFromVTT downloads a WebVTT subtitle file, converts it to SRT with
// proper timestamps, and burns it into the video.
func (f *FFmpegService) AddSubtitleFromVTT(ctx context.Context, videoPath, vttURL string) (string, error) {
	return f.AddSubtitleFromVTTWithStyle(ctx, videoPath, vttURL, SubtitleStyle{})
}

// AddSubtitleFromVTTWithStyle —— 以指定样式将 VTT 字幕烧录到视频
func (f *FFmpegService) AddSubtitleFromVTTWithStyle(ctx context.Context, videoPath, vttURL string, style SubtitleStyle) (string, error) {
	workDir := filepath.Dir(videoPath)

	vttPath := filepath.Join(workDir, "sub.vtt")
	if err := f.DownloadFile(ctx, vttURL, vttPath); err != nil {
		return "", fmt.Errorf("download vtt: %w", err)
	}

	srtContent, err := vttToSRT(vttPath)
	if err != nil {
		return "", fmt.Errorf("convert vtt to srt: %w", err)
	}

	srtPath := filepath.Join(workDir, "sub.srt")
	if err := os.WriteFile(srtPath, []byte(srtContent), 0o644); err != nil {
		return "", fmt.Errorf("write srt: %w", err)
	}

	output := filepath.Join(workDir, "with_sub.mp4")
	subtitleFilter := fmt.Sprintf("subtitles=%s:force_style='%s'", srtPath, style.ForceStyle())
	if err := f.RunFFmpeg(ctx,
		"-i", videoPath,
		"-vf", subtitleFilter,
		output,
	); err != nil {
		return "", fmt.Errorf("add subtitle ffmpeg: %w", err)
	}
	return output, nil
}

// AddBGM —— 将背景音乐混入视频，volume 为 BGM 音量比例（0~1，默认 0.15）
// AddBGM mixes background music into the video at the given volume level.
func (f *FFmpegService) AddBGM(ctx context.Context, videoPath, bgmURL string, volume float64) (string, error) {
	if volume <= 0 {
		volume = 0.15
	}
	workDir := filepath.Dir(videoPath)

	bgmPath := filepath.Join(workDir, "bgm"+remoteFileExt(bgmURL))
	if err := f.DownloadFile(ctx, bgmURL, bgmPath); err != nil {
		return "", fmt.Errorf("download bgm: %w", err)
	}

	output := filepath.Join(workDir, "with_bgm.mp4")
	hasAudio, _ := f.HasAudioStream(ctx, videoPath)

	var filterComplex string
	if hasAudio {
		filterComplex = fmt.Sprintf(
			"[1:a]volume=%.3f[bgm];[0:a][bgm]amix=inputs=2:duration=first:dropout_transition=2[a]",
			volume,
		)
	} else {
		filterComplex = fmt.Sprintf("[1:a]volume=%.3f[a]", volume)
	}

	if err := f.RunFFmpeg(ctx,
		"-i", videoPath,
		"-stream_loop", "-1", "-i", bgmPath,
		"-filter_complex", filterComplex,
		"-map", "0:v:0",
		"-map", "[a]",
		"-c:v", "copy",
		"-c:a", "aac",
		"-shortest",
		"-movflags", "+faststart",
		output,
	); err != nil {
		return "", fmt.Errorf("add bgm ffmpeg: %w", err)
	}
	return output, nil
}

// ConcatClipsWithTransitions —— 使用 xfade 转场拼接视频片段
// transition can be: "fade", "wipeleft", "wiperight", "circleclose", "dissolve"
// transitionDur is the overlap duration in seconds (default 0.5).
// Falls back to ConcatClips when transition is empty.
func (f *FFmpegService) ConcatClipsWithTransitions(ctx context.Context, clipURLs []string, videoMode, transition string, transitionDur float64) (string, error) {
	if transition == "" {
		return f.ConcatClips(ctx, clipURLs, videoMode)
	}
	if transitionDur <= 0 {
		transitionDur = 0.5
	}

	workDir, err := os.MkdirTemp(f.TempDir, "xfade-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Download and normalize clips (same as ConcatClips)
	tw, th := videoModeScale(videoMode)
	scaleFilter := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=24",
		tw, th, tw, th,
	)
	var clipPaths []string
	for i, u := range clipURLs {
		raw := filepath.Join(workDir, fmt.Sprintf("raw%03d.mp4", i))
		if err := f.DownloadFile(ctx, u, raw); err != nil {
			return "", fmt.Errorf("download clip %d: %w", i, err)
		}
		norm := filepath.Join(workDir, fmt.Sprintf("clip%03d.mp4", i))
		hasAudio, _ := f.HasAudioStream(ctx, raw)
		args := []string{"-i", raw}
		if !hasAudio {
			args = append(args, "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=44100")
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
			norm,
		)
		if err := f.RunFFmpeg(ctx, args...); err != nil {
			return "", fmt.Errorf("normalize clip %d: %w", i, err)
		}
		clipPaths = append(clipPaths, norm)
	}

	if len(clipPaths) == 1 {
		return clipPaths[0], nil
	}

	// Probe durations for xfade offset calculation
	durations := make([]float64, len(clipPaths))
	for i, p := range clipPaths {
		d, err := f.ProbeDuration(ctx, p)
		if err != nil || d <= 0 {
			d = 5.0 // fallback
		}
		durations[i] = d
	}

	// Build xfade filter_complex
	var inputs []string
	for _, p := range clipPaths {
		inputs = append(inputs, "-i", p)
	}

	var fc strings.Builder
	var offset float64
	prevV := "[0:v]"
	prevA := "[0:a]"
	for i := 1; i < len(clipPaths); i++ {
		offset += durations[i-1] - transitionDur
		nextV := fmt.Sprintf("[v%d]", i)
		nextA := fmt.Sprintf("[a%d]", i)
		if i == len(clipPaths)-1 {
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
	// Remove trailing semicolon
	fcStr := strings.TrimRight(fc.String(), ";")

	merged := filepath.Join(workDir, "merged.mp4")
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
		return "", fmt.Errorf("xfade concat: %w", err)
	}
	return merged, nil
}

// DownloadFile —— 下载指定 URL 的文件并保存到本地路径
// DownloadFile fetches a URL and writes its body to destPath.
func (f *FFmpegService) DownloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// RunFFmpeg —— 执行 ffmpeg 命令，自动添加 -y 覆盖参数
// RunFFmpeg executes the ffmpeg binary with the given arguments.
func (f *FFmpegService) RunFFmpeg(ctx context.Context, args ...string) error {
	// prepend -y to overwrite outputs without prompt
	fullArgs := append([]string{"-y"}, args...)
	cmd := exec.CommandContext(ctx, f.Bin, fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w\n%s", err, string(out))
	}
	return nil
}

func (f *FFmpegService) ProbeDuration(ctx context.Context, mediaPath string) (float64, error) {
	out, err := f.runFFprobe(ctx,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		mediaPath,
	)
	if err != nil {
		return 0, err
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(out), 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", out, err)
	}
	return duration, nil
}

func (f *FFmpegService) HasAudioStream(ctx context.Context, mediaPath string) (bool, error) {
	out, err := f.runFFprobe(ctx,
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_type",
		"-of", "default=noprint_wrappers=1:nokey=1",
		mediaPath,
	)
	if err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(out)), "audio"), nil
}

func (f *FFmpegService) runFFprobe(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, f.ffprobeBin(), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffprobe: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func (f *FFmpegService) ffprobeBin() string {
	if strings.TrimSpace(f.Bin) == "" || f.Bin == "ffmpeg" {
		return "ffprobe"
	}
	base := filepath.Base(f.Bin)
	if strings.Contains(base, "ffmpeg") {
		return filepath.Join(filepath.Dir(f.Bin), strings.Replace(base, "ffmpeg", "ffprobe", 1))
	}
	return "ffprobe"
}

func formatFFmpegDuration(seconds float64) string {
	return fmt.Sprintf("%.3f", seconds)
}

// buildSRT —— 将纯文本转换为单条目 SRT 字幕格式
// buildSRT converts plain text into a minimal single-entry SRT file.
func buildSRT(text string) string {
	return fmt.Sprintf("1\n00:00:00,000 --> %s\n%s\n",
		"99:59:59,999", text)
}

// vttTimestampToSRT converts a VTT timestamp (00:00:00.000) to SRT format (00:00:00,000).
var vttTimestampRe = regexp.MustCompile(`(\d{2}:\d{2}:\d{2})\.(\d{3})`)

func vttTimestampToSRT(ts string) string {
	return vttTimestampRe.ReplaceAllString(ts, "$1,$2")
}

// vttTagRe strips VTT inline tags like <c>, <b>, <i>, timestamps <00:00:00.000>
var vttTagRe = regexp.MustCompile(`<[^>]+>`)

// vttToSRT —— 将 WebVTT 文件转换为带时间轴的 SRT 格式
// vttToSRT reads a local VTT file and returns SRT-formatted subtitle content.
// It handles the WEBVTT header, NOTE blocks, and inline tag stripping.
func vttToSRT(vttPath string) (string, error) {
	f, err := os.Open(vttPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var out strings.Builder
	seq := 1
	scanner := bufio.NewScanner(f)

	type cue struct {
		start, end string
		lines       []string
	}
	var current *cue

	flush := func() {
		if current == nil {
			return
		}
		text := strings.Join(current.lines, "\n")
		text = vttTagRe.ReplaceAllString(text, "")
		text = strings.TrimSpace(text)
		// Skip empty or whitespace-only cues
		isWhitespace := true
		for _, r := range text {
			if !unicode.IsSpace(r) {
				isWhitespace = false
				break
			}
		}
		if !isWhitespace {
			fmt.Fprintf(&out, "%d\n%s --> %s\n%s\n\n",
				seq,
				vttTimestampToSRT(current.start),
				vttTimestampToSRT(current.end),
				text,
			)
			seq++
		}
		current = nil
	}

	inHeader := true
	for scanner.Scan() {
		line := scanner.Text()

		// Skip WEBVTT header and NOTE blocks
		if inHeader {
			if strings.HasPrefix(line, "WEBVTT") {
				continue
			}
			inHeader = false
		}
		if strings.HasPrefix(line, "NOTE") {
			// skip until blank line
			for scanner.Scan() {
				if strings.TrimSpace(scanner.Text()) == "" {
					break
				}
			}
			continue
		}

		// Cue timing line: "00:00:00.000 --> 00:00:02.500"
		if strings.Contains(line, " --> ") {
			flush()
			parts := strings.SplitN(line, " --> ", 2)
			if len(parts) == 2 {
				// Strip VTT cue settings (e.g. "00:00:02.500 align:start position:0%")
				end := strings.Fields(parts[1])[0]
				current = &cue{start: strings.TrimSpace(parts[0]), end: end}
			}
			continue
		}

		// Blank line = end of cue
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}

		// Numeric cue ID line — skip
		if current == nil {
			if _, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
				continue
			}
		}

		if current != nil {
			current.lines = append(current.lines, line)
		}
	}
	flush()

	if seq == 1 {
		return "", fmt.Errorf("no subtitle cues found in VTT file")
	}
	return out.String(), nil
}

// EnsureTempDir —— 确保临时目录存在，不存在则创建
// EnsureTempDir creates the temp directory if it doesn't exist.
func (f *FFmpegService) EnsureTempDir() error {
	return os.MkdirAll(f.TempDir, 0o755)
}

// ─── opt-p2: Video Frame Template Overlay ──────────────────────────────────────

// VideoFrameTemplate defines overlay elements to composite onto a video:
// text boxes (title/caption/watermark) and an optional logo image.
// Applied via FFmpeg drawtext + overlay filters — no external rendering needed.
type VideoFrameTemplate struct {
	// Title text shown at the top-center of the video
	TitleText  string
	TitleFont  string // default "Arial"
	TitleSize  int    // default 36
	TitleColor string // default "white"

	// Caption / credit shown at the bottom
	CaptionText  string
	CaptionFont  string
	CaptionSize  int
	CaptionColor string

	// Watermark text (e.g. channel name) at top-right corner
	WatermarkText  string
	WatermarkColor string // default "white@0.6"
	WatermarkSize  int    // default 20

	// LogoURL is a remote PNG/SVG to overlay at top-left (downloaded to temp dir)
	LogoURL    string
	LogoWidth  int // ffmpeg scale width, default 80
	LogoHeight int
}

// ApplyFrameTemplate composites overlay elements from tpl onto the input video
// and writes the result to outputPath. Returns outputPath unchanged if tpl is
// effectively empty (no text, no logo).
func (s *FFmpegService) ApplyFrameTemplate(ctx context.Context, inputPath string, tpl VideoFrameTemplate, outputPath string) error {
	if tpl.TitleText == "" && tpl.CaptionText == "" && tpl.WatermarkText == "" && tpl.LogoURL == "" {
		// nothing to do — copy input to output
		return copyFile(inputPath, outputPath)
	}

	var filters []string
	lastLabel := "[0:v]"

	// ── drawtext overlays ──────────────────────────────────────────────────────
	if tpl.TitleText != "" {
		font := tpl.TitleFont
		if font == "" {
			font = "Arial"
		}
		size := tpl.TitleSize
		if size <= 0 {
			size = 36
		}
		color := tpl.TitleColor
		if color == "" {
			color = "white"
		}
		f := fmt.Sprintf("%sdrawtext=fontfile=/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf:fontfamily='%s':text='%s':fontsize=%d:fontcolor=%s:x=(w-text_w)/2:y=30:box=1:boxcolor=black@0.4:boxborderw=8[vt]",
			lastLabel, font, escapeFFmpegText(tpl.TitleText), size, color)
		filters = append(filters, f)
		lastLabel = "[vt]"
	}

	if tpl.CaptionText != "" {
		font := tpl.CaptionFont
		if font == "" {
			font = "Arial"
		}
		size := tpl.CaptionSize
		if size <= 0 {
			size = 28
		}
		color := tpl.CaptionColor
		if color == "" {
			color = "white"
		}
		f := fmt.Sprintf("%sdrawtext=fontfile=/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf:fontfamily='%s':text='%s':fontsize=%d:fontcolor=%s:x=(w-text_w)/2:y=h-text_h-30:box=1:boxcolor=black@0.5:boxborderw=6[vc]",
			lastLabel, font, escapeFFmpegText(tpl.CaptionText), size, color)
		filters = append(filters, f)
		lastLabel = "[vc]"
	}

	if tpl.WatermarkText != "" {
		size := tpl.WatermarkSize
		if size <= 0 {
			size = 20
		}
		color := tpl.WatermarkColor
		if color == "" {
			color = "white@0.6"
		}
		f := fmt.Sprintf("%sdrawtext=fontfile=/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf:text='%s':fontsize=%d:fontcolor=%s:x=w-text_w-20:y=20[vw]",
			lastLabel, escapeFFmpegText(tpl.WatermarkText), size, color)
		filters = append(filters, f)
		lastLabel = "[vw]"
	}

	// ── logo overlay ───────────────────────────────────────────────────────────
	var logoPath string
	if tpl.LogoURL != "" {
		lp, err := downloadToTemp(ctx, s.TempDir, tpl.LogoURL)
		if err == nil {
			logoPath = lp
			defer os.Remove(lp)
		}
	}

	args := []string{"-y", "-i", inputPath}
	if logoPath != "" {
		lw := tpl.LogoWidth
		if lw <= 0 {
			lw = 80
		}
		args = append(args, "-i", logoPath)
		scaleFilter := fmt.Sprintf("[1:v]scale=%d:-1[logo]", lw)
		overlayFilter := fmt.Sprintf("%s[logo]overlay=20:20[vl]", lastLabel)
		filters = append(filters, scaleFilter, overlayFilter)
		lastLabel = "[vl]"
	}

	if len(filters) == 0 {
		return copyFile(inputPath, outputPath)
	}

	args = append(args,
		"-filter_complex", strings.Join(filters, ";"),
		"-map", lastLabel,
		"-map", "0:a?",
		"-c:a", "copy",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		outputPath,
	)

	return s.RunFFmpeg(ctx, args...)
}

// escapeFFmpegText escapes characters that are special in ffmpeg drawtext filter expressions.
func escapeFFmpegText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `'\''`)
	s = strings.ReplaceAll(s, `:`, `\:`)
	return s
}

// copyFile copies src to dst file — used as a no-op passthrough.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// downloadToTemp downloads a URL to a temp file and returns its path.
func downloadToTemp(ctx context.Context, tempDir, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	f, err := os.CreateTemp(tempDir, "logo*"+remoteFileExt(rawURL))
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return "", err
	}
	return f.Name(), nil
}

func remoteFileExt(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		if ext := filepath.Ext(parsed.Path); ext != "" {
			return ext
		}
	}
	return ".bin"
}
