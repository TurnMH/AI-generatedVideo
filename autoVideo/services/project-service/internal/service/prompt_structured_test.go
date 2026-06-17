package service

import (
	"strings"
	"testing"
)

func TestBuildMultiCharacterBlockingCue(t *testing.T) {
	t.Parallel()

	got := buildMultiCharacterBlockingCue(
		[]string{"刘师傅", "王大发"},
		"包子铺门外的街边带着晨雾，王大发神色狼狈地跪在门口，西装褶皱凌乱；铺内暖黄灯光映出刘师傅平静的身影，反差强烈。",
		"包子铺门外的街边带着晨雾 | 王大发神色狼狈地跪在门口",
	)
	if got == "" {
		t.Fatal("expected per-character blocking cue")
	}
	for _, want := range []string{"刘师傅:", "王大发:", "跪", "铺内"} {
		if !strings.Contains(got, want) {
			t.Fatalf("blocking %q should contain %q", got, want)
		}
	}
}

func TestEnrichStoryboardImagePromptWithConstraints(t *testing.T) {
	t.Parallel()

	prompt := enrichStoryboardImagePromptWithConstraints(
		"Single anime frame.",
		StoryboardGenerateRequest{
			SceneDescription: "包子铺门外的街边带着晨雾，王大发神色狼狈地跪在门口；铺内暖黄灯光映出刘师傅平静的身影。",
			SpatialAnchor:    "王大发神色狼狈地跪在门口",
			Characters:       []string{"刘师傅", "王大发"},
		},
		[]string{"刘师傅", "王大发"},
		nil,
		nil,
	)
	for _, want := range []string{
		"Per-character blocking:",
		"never swap who kneels",
		"Pose and body language",
		"Spatial blocking",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt %q should contain %q", prompt, want)
		}
	}
}
