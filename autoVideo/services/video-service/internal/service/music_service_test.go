package service

import (
	"strings"
	"testing"
)

func TestBuildMusicPrompt_AllFields(t *testing.T) {
	km := &MusicKafkaMessage{
		Title:       "战斗BGM",
		Mood:        "intense, epic",
		Instruments: "drums, brass",
		HasVocals:   false,
		Prompt:      "for climactic battle scene",
		DurationSec: 60,
	}
	got := buildMusicPrompt(km)

	expects := []string{"战斗BGM", "intense, epic", "drums, brass", "for climactic battle scene", "60 seconds"}
	for _, want := range expects {
		if !strings.Contains(got, want) {
			t.Errorf("buildMusicPrompt: expected %q in output, got: %q", want, got)
		}
	}
}

func TestBuildMusicPrompt_WithVocalsAndLyrics(t *testing.T) {
	km := &MusicKafkaMessage{
		Title:     "情歌",
		HasVocals: true,
		Lyrics:    "月亮代表我的心",
	}
	got := buildMusicPrompt(km)

	if !strings.Contains(got, "Has vocals: yes") {
		t.Errorf("expected 'Has vocals: yes' in prompt, got: %q", got)
	}
	if !strings.Contains(got, "月亮代表我的心") {
		t.Errorf("expected lyrics in prompt, got: %q", got)
	}
}

func TestBuildMusicPrompt_EmptyMessage(t *testing.T) {
	km := &MusicKafkaMessage{}
	got := buildMusicPrompt(km)
	// Empty message should produce empty or whitespace-only prompt without panic.
	if strings.TrimSpace(got) != "" {
		t.Errorf("expected empty prompt for zero-value message, got: %q", got)
	}
}

func TestBuildMusicPrompt_NoVocals_NotInPrompt(t *testing.T) {
	km := &MusicKafkaMessage{
		Title:     "纯音乐",
		HasVocals: false,
	}
	got := buildMusicPrompt(km)
	if strings.Contains(got, "Has vocals") {
		t.Errorf("expected no vocals mention when HasVocals=false, got: %q", got)
	}
}

func TestMusicKafkaMessage_JSONFields(t *testing.T) {
	// Verify struct fields cover the Kafka payload shape used by task-service.
	km := MusicKafkaMessage{
		TaskID:      42,
		ProjectID:   7,
		ModelName:   "stable-audio-open-1.0",
		Title:       "test",
		Prompt:      "test prompt",
		Mood:        "calm",
		Instruments: "piano",
		DurationSec: 30,
		HasVocals:   false,
		Lyrics:      "",
	}
	if km.TaskID != 42 {
		t.Fatalf("unexpected TaskID: %d", km.TaskID)
	}
}
