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

func TestLooksLikeSceneDescription(t *testing.T) {
	if !LooksLikeSceneDescription("德聚楼后厨，傍晚，木质灶台上散落着面粉。") {
		t.Fatal("expected location-led scene description")
	}
	if LooksLikeSceneDescription("三个月前，正是这个声音在德聚楼后厨当众宣布了解雇。") {
		t.Fatal("narration sentence should not be classified as scene description")
	}
}
