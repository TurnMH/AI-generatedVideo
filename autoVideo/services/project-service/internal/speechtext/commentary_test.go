package speechtext

import "testing"

func TestAlignCommentaryScenesWithSource(t *testing.T) {
	source := `[字幕:德聚楼的灶台前，刘师傅正低头揉面。] 后厨灯光昏黄。
[字幕:三个月前，正是这个声音在德聚楼后厨当众宣布了解雇。] 回忆画面。`
	scenes := []SceneDialogue{
		{Dialogue: "包子铺内部，刘师傅神情沉着冷静，画面近景突出两人表情，环境光线柔和。"},
		{Dialogue: ""},
	}
	got := AlignCommentaryScenesWithSource(source, scenes)
	if got[0].Dialogue != "德聚楼的灶台前，刘师傅正低头揉面。" {
		t.Fatalf("scene 0 dialogue=%q", got[0].Dialogue)
	}
	if got[1].Dialogue != "三个月前，正是这个声音在德聚楼后厨当众宣布了解雇。" {
		t.Fatalf("scene 1 dialogue=%q", got[1].Dialogue)
	}
}

func TestFinalizeCommentaryDialogue_KeepsValidNarration(t *testing.T) {
	got := FinalizeCommentaryDialogue("[字幕:我在德聚楼掌勺三十年，却被老板用两千块遣退]")
	want := "我在德聚楼掌勺三十年，却被老板用两千块遣退"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFinalizeCommentaryDialogue_DropsVisual(t *testing.T) {
	visual := "包子铺内部，刘师傅神情沉着冷静，画面近景突出两人表情，环境光线柔和。"
	if got := FinalizeCommentaryDialogue(visual); got != "" {
		t.Fatalf("expected visual dialogue to be dropped, got %q", got)
	}
}

func TestCountSubtitleTags(t *testing.T) {
	if CountSubtitleTags("[字幕:第一句][字幕:第二句]") != 2 {
		t.Fatal("expected 2 subtitle tags")
	}
}
