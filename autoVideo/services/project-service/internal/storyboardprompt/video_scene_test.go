package storyboardprompt

import "testing"

func TestCompactVideoSceneDescription_PrefersActionTags(t *testing.T) {
	got := CompactVideoSceneDescription(
		"[动作:刘师傅双手按压面团，面粉飞散。] 包子铺内，王大发身穿深色西装，神情紧张。",
		"",
	)
	if got == "" || !containsAll(got, "按压", "面团") {
		t.Fatalf("expected action tag extraction, got %q", got)
	}
}

func TestCompactVideoSceneDescription_IgnoresImagePromptFallback(t *testing.T) {
	got := CompactVideoSceneDescription("", "Create a single 3D anime CG-style storyboard keyframe")
	if got != "" {
		t.Fatalf("image prompt_used must not become video prompt: %q", got)
	}
}

func TestCompactVideoSceneDescription_ExtractsActionOnly(t *testing.T) {
	got := CompactVideoSceneDescription(
		"[字幕:我在揉面，没抬头。] [动作:刘师傅低头揉面，王大发推门而入。]",
		"",
	)
	if contains(got, "字幕") || contains(got, "没抬头") {
		t.Fatalf("subtitle tags must stay out of video visual prompt: %q", got)
	}
	if !contains(got, "揉面") {
		t.Fatalf("expected action tag extraction: %q", got)
	}
}

func TestCompactVideoSceneDescription_PrunesAppearanceCatalog(t *testing.T) {
	got := CompactVideoSceneDescription(
		"包子铺内，刘师傅专注揉面，动作稳健。王大发站在门口，身穿深色西装，黑发中带花白，脸型圆润，气氛紧张。",
		"",
	)
	if contains(got, "身穿") || contains(got, "脸型") {
		t.Fatalf("appearance noise should be pruned: %q", got)
	}
	if !contains(got, "揉面") {
		t.Fatalf("action clause should remain: %q", got)
	}
}
