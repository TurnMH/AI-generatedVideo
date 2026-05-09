package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/autovideo/project-service/internal/model"
)

func TestIsTransientStoryboardGenerationError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "timeout", err: errors.New("image generation timed out for task 1"), want: true},
		{name: "dns", err: errors.New("lookup api.easyart.cc: no such host"), want: true},
		{name: "http 503", err: errors.New("image-service error 503: overloaded"), want: true},
		{name: "validation", err: errors.New("invalid prompt"), want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isTransientStoryboardGenerationError(tc.err); got != tc.want {
				t.Fatalf("isTransientStoryboardGenerationError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSetProjectGenerationScopeClonesEpisodeID(t *testing.T) {
	t.Parallel()

	svc := NewStoryboardService(nil)
	episodeID := uint64(42)

	svc.setProjectGenerationScope(7, &episodeID)
	episodeID = 99

	scope, ok := svc.getProjectGenerationScope(7)
	if !ok {
		t.Fatal("expected project scope to be stored")
	}
	if scope.EpisodeID == nil {
		t.Fatal("expected episode scope to be present")
	}
	if *scope.EpisodeID != 42 {
		t.Fatalf("stored episode id = %d, want 42", *scope.EpisodeID)
	}
}

func TestBuildImagePromptIncludesCinematicStructure(t *testing.T) {
	t.Parallel()

	prompt := buildImagePrompt(StoryboardGenerateRequest{
		SceneDescription: "A woman hums softly while walking from the bed toward the bathroom in early morning light",
		Characters:       []string{"young woman"},
		Location:         "bedroom to bathroom",
		CameraMovement:   "tracking",
		AspectRatio:      "16:9",
		PromptUsed:       "keep the morning atmosphere gentle and intimate",
	}, "", "live-action film look, realistic environments", "")

	for _, want := range []string{
		"Create a single polished storyboard keyframe",
		// PromptUsed is English and not a full generated prompt, so it becomes the primary beat.
		"Primary dramatic beat: keep the morning atmosphere gentle and intimate",
		"Featured subjects: young woman.",
		"Camera language: tracking shot feel",
		"Framing target: 16:9 widescreen composition",
		"Project visual direction: live-action film look, realistic environments",
		"no text, no subtitle, no watermark",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt %q does not contain %q", prompt, want)
		}
	}
	// PromptUsed was already used as primary, so no "Additional visual requirements:" expected.
	if strings.Contains(prompt, "Additional visual requirements:") {
		t.Fatalf("prompt should not repeat PromptUsed as additional requirements")
	}
}

func TestBuildImagePromptFallsBackToSceneDescWhenPromptUsedIsFullGenerated(t *testing.T) {
	t.Parallel()

	// A "full generated" prompt starts with "Create a single" — the saved-back output of a prior run.
	fullGenPrompt := "Create a single polished storyboard keyframe. Primary dramatic beat: old prompt."
	prompt := buildImagePrompt(StoryboardGenerateRequest{
		SceneDescription: "New translated scene description",
		PromptUsed:       fullGenPrompt,
		AspectRatio:      "16:9",
	}, "", "", "")

	// Should fall back to SceneDescription as primary beat, not the full generated prompt.
	if !strings.Contains(prompt, "Primary dramatic beat: New translated scene description") {
		t.Fatalf("prompt should use SceneDescription when PromptUsed is a full generated prompt, got: %q", prompt)
	}
	if strings.Contains(prompt, "Additional visual requirements:") {
		t.Fatalf("full generated PromptUsed should not appear as additional requirements, got: %q", prompt)
	}
}

func TestBuildStoryboardNegativePromptForLiveActionAvoidsAnime(t *testing.T) {
	t.Parallel()

	project := &model.Project{
		StoryboardConfig: []byte(`{"style_preset":"live-action-film"}`),
	}

	prompt := buildStoryboardNegativePrompt(project)
	for _, want := range []string{"anime", "cartoon", "watermark"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("negative prompt %q does not contain %q", prompt, want)
		}
	}
}
