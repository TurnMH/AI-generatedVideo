package service

import "testing"

func TestResolveProjectGenerationScopePrefersOverride(t *testing.T) {
	t.Parallel()

	svc := NewStoryboardService(nil)
	stored := uint64(42)
	override := uint64(7)
	svc.setProjectGenerationScope(1, storyboardGenerationScope{EpisodeID: &stored})

	got := svc.resolveProjectGenerationScope(1, &override, "", nil)
	if got.EpisodeID == nil {
		t.Fatal("expected override scope")
	}
	if *got.EpisodeID != 7 {
		t.Fatalf("resolved episode id = %d, want 7", *got.EpisodeID)
	}

	override = 99
	if *got.EpisodeID != 7 {
		t.Fatalf("resolved scope should be cloned, got %d", *got.EpisodeID)
	}
}

func TestResolveProjectGenerationScopeFallsBackToStoredScope(t *testing.T) {
	t.Parallel()

	svc := NewStoryboardService(nil)
	stored := uint64(42)
	svc.setProjectGenerationScope(9, storyboardGenerationScope{EpisodeID: &stored})

	got := svc.resolveProjectGenerationScope(9, nil, "", nil)
	if got.EpisodeID == nil {
		t.Fatal("expected stored scope")
	}
	if *got.EpisodeID != 42 {
		t.Fatalf("resolved episode id = %d, want 42", *got.EpisodeID)
	}

	stored = 100
	if *got.EpisodeID != 42 {
		t.Fatalf("resolved scope should be cloned, got %d", *got.EpisodeID)
	}
}

func TestResolveProjectGenerationScopeAllowsProjectWideResume(t *testing.T) {
	t.Parallel()

	svc := NewStoryboardService(nil)
	if got := svc.resolveProjectGenerationScope(3, nil, "", nil); got.EpisodeID != nil {
		t.Fatalf("resolved scope = %d, want nil", *got.EpisodeID)
	}
}
