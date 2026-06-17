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

func TestSanitizeCharacterAssetPromptStripsSoloConstraintsInMultiCharacter(t *testing.T) {
	t.Parallel()

	in := "2D anime style, 无第二人物, 无合影, wrinkled suit, middle-aged businessman"
	got := sanitizeCharacterAssetPromptForStoryboardContext(in, true, false)
	for _, banned := range []string{"无第二人物", "无合影"} {
		if strings.Contains(got, banned) {
			t.Fatalf("sanitized prompt %q should not contain %q", got, banned)
		}
	}
	if !strings.Contains(got, "wrinkled suit") {
		t.Fatalf("sanitized prompt %q should keep appearance tokens", got)
	}
}

func TestSanitizeLLMThinkingLeak(t *testing.T) {
	t.Parallel()

	in := "Single frame. <think>secret reasoning</think> Wang kneels outside."
	got := sanitizeLLMThinkingLeak(in)
	if strings.Contains(got, "redacted_thinking") || strings.Contains(got, "secret reasoning") {
		t.Fatalf("sanitized %q still contains thinking leak", got)
	}
	if !strings.Contains(got, "Wang kneels outside") {
		t.Fatalf("sanitized %q lost valid prompt text", got)
	}
}

func TestEntranceSplitCompositionCue(t *testing.T) {
	t.Parallel()

	got := entranceSplitCompositionCue("entrance", "王大发跪在门口，铺内暖黄灯光映出刘师傅平静的身影")
	if got == "" {
		t.Fatal("expected entrance split composition cue")
	}
	if !strings.Contains(got, "doorway split-frame") {
		t.Fatalf("cue %q missing split-frame guidance", got)
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

func TestLookupCharacterReferenceImage_ExactAndAlias(t *testing.T) {
	images := map[string]string{
		"刘师傅": "https://cdn.example.com/liu.png",
		"王大发": "https://cdn.example.com/wang.png",
	}
	if got := lookupCharacterReferenceImage("刘师傅", images); got != images["刘师傅"] {
		t.Fatalf("expected exact match, got %q", got)
	}
	if got := lookupCharacterReferenceImage("王大发总裁", images); got != images["王大发"] {
		t.Fatalf("expected alias match, got %q", got)
	}
}

func TestLookupCharacterReferenceImage_DoesNotCrossMatchShortNames(t *testing.T) {
	images := map[string]string{
		"刘师傅": "https://cdn.example.com/liu.png",
	}
	if got := lookupCharacterReferenceImage("师傅", images); got != "" {
		t.Fatalf("expected no match for short alias, got %q", got)
	}
}

func TestResolveCharacterAssetIDs(t *testing.T) {
	ids := map[string]int64{
		"刘师傅": 101,
		"王大发": 102,
	}
	got := resolveCharacterAssetIDs([]string{"刘师傅", "王大发"}, ids)
	if len(got) != 2 {
		t.Fatalf("unexpected resolved ids: %#v", got)
	}
	seen := map[int64]struct{}{}
	for _, id := range got {
		seen[id] = struct{}{}
	}
	if _, ok := seen[101]; !ok || len(seen) != 2 {
		t.Fatalf("expected ids 101 and 102, got %#v", got)
	}
}

func TestInferStoryboardCharacters_FromSceneText(t *testing.T) {
	catalog := []string{"刘师傅", "王大发", "陈大鹏"}
	got := inferStoryboardCharacters(nil, "刘师傅坐在躺椅上装旱烟", "说吧", catalog)
	if len(got) != 1 || got[0] != "刘师傅" {
		t.Fatalf("expected inferred 刘师傅, got %#v", got)
	}
}

func TestPrioritizeStoryboardReferenceImagesWithCharacters(t *testing.T) {
	characters := []string{"https://cdn.example.com/a.png", "https://cdn.example.com/b.png"}
	others := []string{"https://cdn.example.com/scene.png", "https://cdn.example.com/c.png", "https://cdn.example.com/d.png"}
	got := prioritizeStoryboardReferenceImagesWithCharacters(characters, others)
	if len(got) != 4 {
		t.Fatalf("expected 4 refs, got %#v", got)
	}
	if got[0] != characters[0] || got[1] != characters[1] {
		t.Fatalf("expected character refs first, got %#v", got)
	}
}
