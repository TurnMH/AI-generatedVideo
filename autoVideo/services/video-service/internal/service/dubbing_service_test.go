package service

import "testing"

func TestBuildDubbingChunksAutoAssignsDistinctVoices(t *testing.T) {
	chunks := buildDubbingChunks("旁白：清晨的阳光照进卧室。\n女主：我先去洗漱。\n男主：我等你。", autoVoiceModel)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0].VoiceKey != "male3" {
		t.Fatalf("expected narrator to use male3, got %q", chunks[0].VoiceKey)
	}
	if chunks[1].VoiceKey != "female1" {
		t.Fatalf("expected female lead to use female1, got %q", chunks[1].VoiceKey)
	}
	if chunks[2].VoiceKey != "default" {
		t.Fatalf("expected male lead to use default, got %q", chunks[2].VoiceKey)
	}
}

func TestBuildDubbingChunksAutoFallsBackToNarrationWithoutLabels(t *testing.T) {
	chunks := buildDubbingChunks("清晨的女人从床边起身，轻轻哼歌走向浴室。", autoVoiceModel)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].VoiceKey != "male3" {
		t.Fatalf("expected unlabeled narration to use narrator voice, got %q", chunks[0].VoiceKey)
	}
}

func TestParseSpeakerSegmentsIgnoresStageDirectionLabels(t *testing.T) {
	segments := parseSpeakerSegments("场景：卧室，清晨。\n小美：今天会是好天气。\n镜头：推近她的侧脸。")
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segments))
	}
	if segments[0].Speaker != "" {
		t.Fatalf("expected stage direction to stay unlabeled, got %q", segments[0].Speaker)
	}
	if segments[1].Speaker != "小美" {
		t.Fatalf("expected dialogue speaker 小美, got %q", segments[1].Speaker)
	}
	if segments[2].Speaker != "" {
		t.Fatalf("expected camera direction to stay unlabeled, got %q", segments[2].Speaker)
	}
}
