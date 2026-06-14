package service

import "testing"

func TestEnrichSceneDescriptionForVideo_PrefersStoryAndAppendsDialogue(t *testing.T) {
	got := EnrichSceneDescriptionForVideo(
		"[字幕:王大发把塑料桶往地上一墩。] 包子铺后厨，蒸汽弥漫。",
		"王大发把塑料桶往地上一墩。",
	)
	if got == "" {
		t.Fatal("expected enriched description")
	}
	if !containsAll(got, "塑料桶", "蒸汽") {
		t.Fatalf("story description missing: %q", got)
	}
}

func TestPreparePerClipStoryDescriptions_SubtitleTagsStayOutOfVisual(t *testing.T) {
	got := preparePerClipStoryDescriptions(
		[]string{"[字幕:我在揉面，没抬头。] [动作:刘师傅低头揉面，王大发推门而入。]"},
		[]string{"旁白：我在揉面，没抬头。"},
	)
	if len(got) != 1 {
		t.Fatalf("expected 1 clip description, got %d", len(got))
	}
	if containsSubstring(got[0], "没抬头") && !containsSubstring(got[0], "揉面") {
		t.Fatalf("subtitle text should not dominate visual prompt: %q", got[0])
	}
}

func TestPreparePerClipStoryDescriptions_VisualOnly(t *testing.T) {
	got := preparePerClipStoryDescriptions(
		[]string{"包子铺内，刘师傅专注揉面，动作稳健。王大发站在门口，身穿深色西装，黑发中带花白，脸型圆润，表情圆滑带有商人气息。"},
		[]string{"旁白：我在德聚楼干了三十年，从洗碗学徒熬到主厨。"},
	)
	if len(got) != 1 {
		t.Fatalf("expected 1 clip description, got %d", len(got))
	}
	if containsSubstring(got[0], "旁白") || containsSubstring(got[0], "我在德聚楼") {
		t.Fatalf("visual prompt should not include dialogue: %q", got[0])
	}
	if !containsSubstring(got[0], "揉面") {
		t.Fatalf("expected action clause retained: %q", got[0])
	}
	if containsSubstring(got[0], "身穿") {
		t.Fatalf("appearance catalog should be pruned: %q", got[0])
	}
}

func TestEnrichSceneDescriptionForVideo_DialogueOnly(t *testing.T) {
	got := EnrichSceneDescriptionForVideo("", "旁白：刘师傅，该开工了。")
	if got != "剧情节拍：旁白：刘师傅，该开工了。" {
		t.Fatalf("unexpected dialogue-only prompt: %q", got)
	}
}

func TestMergeStoryAndMotionPrompt_KeepsStoryFirst(t *testing.T) {
	story := "王大发在包子铺后厨揉面，陈大鹏从门口探头。"
	motion := "中景固定镜头，蒸汽缓慢升腾，人物动作连贯。"
	got := MergeStoryAndMotionPrompt(story, motion)
	if !containsAll(got, "王大发", "中景固定镜头") {
		t.Fatalf("expected merged story+motion, got %q", got)
	}
	if got[:len(story)] != story {
		t.Fatalf("story beat should remain at the front: %q", got)
	}
}

func TestMergeStoryAndMotionPrompt_SkipsRedundantMotion(t *testing.T) {
	story := "王大发在包子铺后厨揉面，镜头缓慢推进。"
	motion := "王大发在包子铺后厨揉面"
	got := MergeStoryAndMotionPrompt(story, motion)
	if got != story {
		t.Fatalf("expected story only, got %q", got)
	}
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !containsSubstring(text, part) {
			return false
		}
	}
	return true
}

func containsSubstring(text, part string) bool {
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
