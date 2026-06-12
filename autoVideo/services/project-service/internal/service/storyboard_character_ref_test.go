package service

import (
	"strings"
	"testing"
)

func TestPickStoryboardCharacterReferenceImage_PrefersSinglePanelOverComposite(t *testing.T) {
	composite := "https://cdn.example.com/asset_12_composite.jpg"
	front := "https://cdn.example.com/asset_12_front.jpg"
	panels := []string{front, "https://cdn.example.com/asset_12_closeup.jpg", "", ""}

	got := pickStoryboardCharacterReferenceImage(composite, panels)
	if got != front {
		t.Fatalf("expected front panel, got %q", got)
	}
}

func TestPickStoryboardCharacterReferenceImage_SkipsCompositeWhenNoPanels(t *testing.T) {
	got := pickStoryboardCharacterReferenceImage("https://cdn.example.com/asset_9_composite.jpg", nil)
	if got != "" {
		t.Fatalf("expected empty when only composite exists, got %q", got)
	}
}

func TestSanitizeCharacterAssetPromptForStoryboard(t *testing.T) {
	in := "male chef, character reference sheet, full-body front view, white apron, turnaround sheet style"
	got := sanitizeCharacterAssetPromptForStoryboard(in)
	if got == "" {
		t.Fatal("expected some wardrobe cues to remain")
	}
	if characterSheetPromptNoise.MatchString(got) {
		t.Fatalf("expected sheet jargon removed, got %q", got)
	}
	if !strings.Contains(got, "white apron") {
		t.Fatalf("expected wardrobe cue kept, got %q", got)
	}
}

func TestFilterStoryboardReferenceURLs(t *testing.T) {
	in := []string{
		"https://cdn.example.com/asset_1_front.jpg",
		"https://cdn.example.com/asset_1_composite.jpg",
		"https://cdn.example.com/asset_1_front.jpg",
	}
	got := filterStoryboardReferenceURLs(in)
	if len(got) != 1 || got[0] != in[0] {
		t.Fatalf("unexpected filtered refs: %#v", got)
	}
}
