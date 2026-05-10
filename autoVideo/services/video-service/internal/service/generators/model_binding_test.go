package generators

import "testing"

func TestKlingGenerator_CloneDoesNotMutateOriginal(t *testing.T) {
	t.Parallel()
	original := NewKlingGeneratorWithKeys("https://api.klingai.com", "ak-test")
	clone := original.Clone().WithModel("kling-v3")
	if original.ModelName != "kling-v1-6" {
		t.Fatalf("expected original model to remain kling-v1-6, got %q", original.ModelName)
	}
	if clone.ModelName != "kling-v3" {
		t.Fatalf("expected clone model kling-v3, got %q", clone.ModelName)
	}
}

func TestWanGenerator_CloneWithModelUsesRequestedModel(t *testing.T) {
	t.Parallel()
	original := NewWanGenerator("wan-key", "", "https://dashscope.aliyuncs.com")
	clone := original.CloneWithModel("wan2.6")
	if original.Model != "wanx2.1-i2v-turbo" {
		t.Fatalf("expected original model to remain default, got %q", original.Model)
	}
	if clone.Model != "wan2.6" {
		t.Fatalf("expected clone model wan2.6, got %q", clone.Model)
	}
}

func TestDoubaoGenerator_CloneWithModelPreservesAudioFlags(t *testing.T) {
	t.Parallel()
	original := NewDoubaoSeedanceGenerator("db-key", "https://ark.example.com", "doubao-seedream-4-0-250828", "doubao-seedance")
	clone := original.CloneWithModel("doubao-seedream-4-0-250901", "doubao-seedance")
	if !clone.supportsAudio || !clone.supportsRatio {
		t.Fatal("expected clone to preserve seedance capability flags")
	}
	if original.Model != "doubao-seedream-4-0-250828" {
		t.Fatalf("expected original model unchanged, got %q", original.Model)
	}
	if clone.Model != "doubao-seedream-4-0-250901" {
		t.Fatalf("expected clone model updated, got %q", clone.Model)
	}
}

func TestResolvedModelUsedPrefersActualProviderModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		actual   string
		fallback string
		want     string
	}{
		{name: "wan explicit model", actual: "wan2.6", fallback: "wan", want: "wan2.6"},
		{name: "kling explicit model", actual: "kling-v3", fallback: "kling", want: "kling-v3"},
		{name: "fallback when actual missing", actual: "", fallback: "doubao", want: "doubao"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := resolvedModelUsed(tc.actual, tc.fallback); got != tc.want {
				t.Fatalf("resolvedModelUsed(%q, %q) = %q, want %q", tc.actual, tc.fallback, got, tc.want)
			}
		})
	}
}