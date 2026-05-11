package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddAudioAttachesExternalTrackToSilentVideo(t *testing.T) {
	tempDir := t.TempDir()
	argsLog := filepath.Join(tempDir, "ffmpeg.args")
	ffmpegPath := filepath.Join(tempDir, "fake-ffmpeg.sh")
	script := "#!/bin/sh\nprintf '%s\n' \"$@\" > \"" + argsLog + "\"\neval \"last=\\${$#}\"\n: > \"$last\"\n"
	if err := os.WriteFile(ffmpegPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake ffmpeg: %v", err)
	}
	ffprobePath := filepath.Join(tempDir, "fake-ffprobe.sh")
	ffprobeScript := "#!/bin/sh\neval \"last=\\${$#}\"\nif printf '%s' \"$*\" | grep -q 'format=duration'; then\n  case \"$last\" in\n    *audio.mp3|*voice.mp3) printf '12.500\\n' ;;\n    *) printf '5.000\\n' ;;\n  esac\n  exit 0\nfi\nexit 0\n"
	if err := os.WriteFile(ffprobePath, []byte(ffprobeScript), 0o755); err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}

	videoPath := filepath.Join(tempDir, "merged.mp4")
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write video: %v", err)
	}

	audioServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("audio"))
	}))
	defer audioServer.Close()

	svc := NewFFmpegService(tempDir, ffmpegPath)
	output, err := svc.AddAudio(context.Background(), videoPath, audioServer.URL+"/voice.mp3?token=test")
	if err != nil {
		t.Fatalf("AddAudio returned error: %v", err)
	}
	if output != filepath.Join(tempDir, "with_audio.mp4") {
		t.Fatalf("unexpected output path %q", output)
	}

	argsBytes, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("read ffmpeg args: %v", err)
	}
	args := string(argsBytes)
	for _, want := range []string{
		"-filter_complex",
		"tpad=stop_mode=clone:stop_duration=7.500",
		"-map\n[v]",
		"-map\n1:a:0",
		"-t\n12.500",
		"-c:a\naac",
		"audio.mp3",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("ffmpeg args %q do not contain %q", args, want)
		}
	}
}

func TestConcatClipsAddsSilentAudioTrackWhenInputHasNoAudio(t *testing.T) {
	tempDir := t.TempDir()
	argsLog := filepath.Join(tempDir, "concat.args")
	ffmpegPath := filepath.Join(tempDir, "fake-ffmpeg.sh")
	ffmpegScript := "#!/bin/sh\nprintf '%s\n' \"$@\" >> \"" + argsLog + "\"\nprintf '%s\n' '---' >> \"" + argsLog + "\"\neval \"last=\\${$#}\"\nmkdir -p \"$(dirname \"$last\")\"\n: > \"$last\"\n"
	if err := os.WriteFile(ffmpegPath, []byte(ffmpegScript), 0o755); err != nil {
		t.Fatalf("write fake ffmpeg: %v", err)
	}
	ffprobePath := filepath.Join(tempDir, "fake-ffprobe.sh")
	ffprobeScript := "#!/bin/sh\nif printf '%s' \"$*\" | grep -q 'stream=codec_type'; then\n  exit 0\nfi\nprintf '5.000\\n'\n"
	if err := os.WriteFile(ffprobePath, []byte(ffprobeScript), 0o755); err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}

	clipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("clip"))
	}))
	defer clipServer.Close()

	svc := NewFFmpegService(tempDir, ffmpegPath)
	if _, err := svc.ConcatClips(context.Background(), []string{clipServer.URL + "/clip1.mp4"}, ""); err != nil {
		t.Fatalf("ConcatClips returned error: %v", err)
	}

	argsBytes, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("read ffmpeg args: %v", err)
	}
	args := string(argsBytes)
	for _, want := range []string{
		"anullsrc=channel_layout=stereo:sample_rate=44100",
		"-map\n1:a:0",
		"-shortest",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("ffmpeg args %q do not contain %q", args, want)
		}
	}
}

func TestBuildXfadeFilterComplexSupportsPerCutTransitions(t *testing.T) {
	fc := buildXfadeFilterComplex(
		[]float64{4.0, 5.0, 6.0},
		[]string{"wipeleft", "fade"},
		[]float64{0.22, 0.45},
	)

	for _, want := range []string{
		"xfade=transition=wipeleft:duration=0.220:offset=3.780",
		"xfade=transition=fade:duration=0.450:offset=8.330",
		"acrossfade=d=0.220",
		"acrossfade=d=0.450",
	} {
		if !strings.Contains(fc, want) {
			t.Fatalf("filter_complex %q does not contain %q", fc, want)
		}
	}
}
