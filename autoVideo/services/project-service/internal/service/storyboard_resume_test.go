package service

import "testing"

func TestResolveProjectGenerationScopePrefersOverride(t *testing.T) {
	t.Parallel()

	svc := NewStoryboardService(nil)
	stored := uint64(42)
	override := uint64(7)
	svc.setProjectGenerationScope(1, &stored)

	got := svc.resolveProjectGenerationScope(1, &override)
	if got == nil {
		t.Fatal("expected override scope")
	}
	if *got != 7 {
		t.Fatalf("resolved episode id = %d, want 7", *got)
	}

	override = 99
	if *got != 7 {
		t.Fatalf("resolved scope should be cloned, got %d", *got)
	}
}

func TestResolveProjectGenerationScopeFallsBackToStoredScope(t *testing.T) {
	t.Parallel()

	svc := NewStoryboardService(nil)
	stored := uint64(42)
	svc.setProjectGenerationScope(9, &stored)

	got := svc.resolveProjectGenerationScope(9, nil)
	if got == nil {
		t.Fatal("expected stored scope")
	}
	if *got != 42 {
		t.Fatalf("resolved episode id = %d, want 42", *got)
	}

	stored = 100
	if *got != 42 {
		t.Fatalf("resolved scope should be cloned, got %d", *got)
	}
}

func TestResolveProjectGenerationScopeAllowsProjectWideResume(t *testing.T) {
	t.Parallel()

	svc := NewStoryboardService(nil)
	if got := svc.resolveProjectGenerationScope(3, nil); got != nil {
		t.Fatalf("resolved scope = %d, want nil", *got)
	}
}
