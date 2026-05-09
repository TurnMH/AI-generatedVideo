package config

import "testing"

func TestRecommendedStoryboardConcurrency(t *testing.T) {
	t.Parallel()

	// 4 workers (1 key × 4), single key.
	generations, inFlight := recommendedStoryboardConcurrency(4, 1)
	if generations != 4 {
		t.Fatalf("generations = %d, want 4", generations)
	}
	if inFlight != 8 {
		t.Fatalf("inFlight = %d, want 8", inFlight)
	}

	// 3 keys adds 2 extra dispatch slots.
	generations, inFlight = recommendedStoryboardConcurrency(4, 3)
	if generations != 6 {
		t.Fatalf("generations with multi-key = %d, want 6", generations)
	}
	if inFlight != 12 {
		t.Fatalf("inFlight with multi-key = %d, want 12", inFlight)
	}

	// Large imageWorkers is capped at 48.
	generations, _ = recommendedStoryboardConcurrency(100, 1)
	if generations != 48 {
		t.Fatalf("generations capped = %d, want 48", generations)
	}
}
