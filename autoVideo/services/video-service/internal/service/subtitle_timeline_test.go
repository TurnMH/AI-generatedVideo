package service

import (
	"strings"
	"testing"
)

func TestComputeClipWindowStarts_NoTransition(t *testing.T) {
	starts := computeClipWindowStarts([]float64{5, 8, 3}, nil, nil)
	want := []float64{0, 5, 13}
	for i := range want {
		if starts[i] != want[i] {
			t.Fatalf("starts[%d] = %v, want %v (full=%v)", i, starts[i], want[i], starts)
		}
	}
}

func TestComputeClipWindowStarts_WithTransition(t *testing.T) {
	starts := computeClipWindowStarts(
		[]float64{5, 8},
		[]string{"dissolve"},
		[]float64{0.5},
	)
	if starts[0] != 0 {
		t.Fatalf("starts[0] = %v, want 0", starts[0])
	}
	if starts[1] != 4.5 {
		t.Fatalf("starts[1] = %v, want 4.5", starts[1])
	}
}

func TestBuildPerClipTimedSRT_AssignsClipWindows(t *testing.T) {
	srt := buildPerClipTimedSRT(
		[]string{"旁白：第一句", "旁白：第二句"},
		[]float64{5, 8},
		nil,
		nil,
	)
	if srt == "" {
		t.Fatal("expected non-empty srt")
	}
	if !strings.Contains(srt, "00:00:00,000") || !strings.Contains(srt, "第一句") ||
		!strings.Contains(srt, "00:00:05,000") || !strings.Contains(srt, "第二句") {
		t.Fatalf("unexpected srt:\n%s", srt)
	}
}

func TestShouldUseEpisodeLevelAudio(t *testing.T) {
	dialogues := []string{"旁白：hello", "旁白：world"}
	if shouldUseEpisodeLevelAudio(dialogues, 2, true) {
		t.Fatal("per-clip mux used should skip episode audio")
	}
	if shouldUseEpisodeLevelAudio(dialogues, 2, false) {
		t.Fatal("multi-clip dialogues without mux should skip episode audio")
	}
	if !shouldUseEpisodeLevelAudio(dialogues, 1, false) {
		t.Fatal("single clip may still use episode audio")
	}
	if !shouldUseEpisodeLevelAudio(nil, 3, false) {
		t.Fatal("no dialogues should allow episode audio")
	}
}
