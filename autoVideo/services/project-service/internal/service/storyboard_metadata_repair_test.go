package service

import "testing"

func TestIsPoorSceneHintSegment(t *testing.T) {
	t.Parallel()

	cases := []struct {
		seg  string
		poor bool
	}{
		{"他跪在我包子铺门口：\"刘师傅", true},
		{"【导语】。我是德聚楼三十年的主理厨师", true},
		{"王大发神色狼狈地跪在门口", false},
		{"门口晨光斜照", false},
		{"我拉开躺椅坐下", true},
	}
	for _, tc := range cases {
		if got := isPoorSceneHintSegment(tc.seg); got != tc.poor {
			t.Fatalf("isPoorSceneHintSegment(%q) = %v, want %v", tc.seg, got, tc.poor)
		}
	}
}

func TestIsCorruptedStoryboardPromptUsed(t *testing.T) {
	t.Parallel()

	if !isCorruptedStoryboardPromptUsed("Single 2D anime <think>oops</think>") {
		t.Fatal("expected corrupted prompt")
	}
	if isCorruptedStoryboardPromptUsed("Single 2D anime still frame with clean line art.") {
		t.Fatal("expected clean prompt")
	}
}

func TestPickStoryboardCharacterReferenceImageForScenePrefersCloseUp(t *testing.T) {
	t.Parallel()

	panels := []string{
		"https://cdn.example.com/front.jpg",
		"https://cdn.example.com/closeup.jpg",
	}
	got := pickStoryboardCharacterReferenceImageForScene("", panels, true)
	if got != panels[1] {
		t.Fatalf("expected closeup panel, got %q", got)
	}
}
