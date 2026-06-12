package speechtext

import "testing"

func TestExtractNarrationForSpeech_SubtitleTags(t *testing.T) {
	text := "画面推近。[字幕:德聚楼的灶台前，刘师傅正低头揉面。] 后厨灯光昏黄。"
	got := ExtractNarrationForSpeech(text)
	want := "德聚楼的灶台前，刘师傅正低头揉面。"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestExtractNarrationForSpeech_QuotedDialogue(t *testing.T) {
	text := `王大发清了清嗓子：“刘师傅，我老了，手也抖了。”`
	got := ExtractNarrationForSpeech(text)
	if got == "" {
		t.Fatal("expected quoted dialogue extraction")
	}
}

func TestLooksLikeStoryboardVisualDescription(t *testing.T) {
	visual := "包子铺内部，刘师傅神情沉着冷静，画面近景突出两人表情，环境光线柔和。"
	if !LooksLikeStoryboardVisualDescription(visual) {
		t.Fatal("expected visual storyboard description")
	}
	if LooksLikeStoryboardVisualDescription("我在德聚楼掌勺三十年，却被老板用两千块遣退") {
		t.Fatal("narration should not be classified as visual description")
	}
}

func TestLooksLikeSceneDescription(t *testing.T) {
	if !LooksLikeSceneDescription("德聚楼后厨，傍晚，木质灶台上散落着面粉。") {
		t.Fatal("expected location-led scene description")
	}
	if LooksLikeSceneDescription("三个月前，正是这个声音在德聚楼后厨当众宣布了解雇。") {
		t.Fatal("narration sentence should not be classified as scene description")
	}
}
