package config

import "testing"

func TestMergeKeys(t *testing.T) {
	t.Parallel()

	keys := mergeKeys(" key-a , key-b ", []string{"key-b", "key-c"}, "key-d\nkey-a")
	if got, want := len(keys), 4; got != want {
		t.Fatalf("len(keys) = %d, want %d", got, want)
	}
	if keys[0] != "key-a" || keys[1] != "key-b" || keys[2] != "key-c" || keys[3] != "key-d" {
		t.Fatalf("unexpected key order: %#v", keys)
	}
}

func TestRecommendedMaxWorkers(t *testing.T) {
	t.Parallel()

	// Explicit MaxWorkers takes priority over key count.
	cfg := &Config{}
	cfg.Concurrency.MaxWorkers = 24
	cfg.Models.OpenAIKeys = []string{"k1", "k2", "k3"}

	if got := RecommendedMaxWorkers(cfg); got != 24 {
		t.Fatalf("RecommendedMaxWorkers() = %d, want 24 (explicit override)", got)
	}
	// With no explicit override: 3 keys × 4 workers = 12.
	cfg2 := &Config{}
	cfg2.Models.OpenAIKeys = []string{"k1", "k2", "k3"}
	if got := RecommendedMaxWorkers(cfg2); got != 12 {
		t.Fatalf("RecommendedMaxWorkers() = %d, want 12 (3 keys × 4)", got)
	}
	// Single key: minimum 4 workers.
	cfg3 := &Config{}
	cfg3.Models.OpenAIKeys = []string{"k1"}
	if got := RecommendedMaxWorkers(cfg3); got != 4 {
		t.Fatalf("RecommendedMaxWorkers() = %d, want 4 (1 key × 4)", got)
	}
	// PrioritySlots should not exceed maxWorkers.
	if got := RecommendedPrioritySlots(gotClamp(RecommendedMaxWorkers(cfg2))); got > 12 {
		t.Fatalf("RecommendedPrioritySlots() = %d, should not exceed maxWorkers", got)
	}
}

func TestRecommendedLocalSlots(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.Models.ComfyUIURLs = []string{"http://node-1:8188", "http://node-2:8188"}

	if got := RecommendedLocalSlots(cfg); got != 2 {
		t.Fatalf("RecommendedLocalSlots() = %d, want 2", got)
	}
}

func TestRecommendedMaxWorkersUsesComfyPool(t *testing.T) {
	t.Parallel()

	// 4 ComfyUI nodes → keyCount=4 → 4×4=16 workers.
	cfg := &Config{}
	cfg.Models.ComfyUIURLs = []string{
		"http://node-1:8188",
		"http://node-2:8188",
		"http://node-3:8188",
		"http://node-4:8188",
	}

	if got := RecommendedMaxWorkers(cfg); got != 16 {
		t.Fatalf("RecommendedMaxWorkers() = %d, want 16 (4 nodes × 4)", got)
	}
}

func gotClamp(v int) int { return v }
