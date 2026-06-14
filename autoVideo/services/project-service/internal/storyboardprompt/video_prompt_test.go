package storyboardprompt

import "testing"

func TestVideoSceneDescription_PrefersSceneDescription(t *testing.T) {
	got := VideoSceneDescription(
		"王大发在包子铺后厨揉面，刘师傅专注按压面团。",
		"cinematic portrait of a baker in apron, 8k",
		"旁白：天还没亮。",
	)
	if !containsAll(got, "王大发", "揉面") {
		t.Fatalf("unexpected video scene description: %q", got)
	}
	if contains(got, "旁白") || contains(got, "8k") {
		t.Fatalf("video scene should not include dialogue or image prompt: %q", got)
	}
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !contains(text, part) {
			return false
		}
	}
	return true
}

func contains(text, part string) bool {
	return len(part) == 0 || (len(text) >= len(part) && indexOf(text, part) >= 0)
}

func indexOf(text, part string) int {
	for i := 0; i+len(part) <= len(text); i++ {
		if text[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
