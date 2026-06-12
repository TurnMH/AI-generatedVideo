package service

import (
	"strings"
	"testing"

	"github.com/autovideo/project-service/internal/productionmode"
)

func TestPostProcessCommentaryScenes_ExtractSubtitleAndMergeShort(t *testing.T) {
	svc := &EpisodeService{}
	scenes := []llmScene{
		{
			Description: "德聚楼后厨，傍晚，木质灶台。",
			Dialogue:    "[字幕:德聚楼的灶台前，刘师傅正低头揉面，面粉散落在木质台面上。]",
			Duration:    4,
		},
		{
			Description: "王大发站在门口。",
			Dialogue:    "他当时确实有点尴尬呢",
			Duration:    4,
		},
		{
			Description: "回忆画面。",
			Dialogue:    "[字幕:三个月前，正是这个声音在德聚楼后厨当众宣布了解雇。]",
			Duration:    4,
		},
	}

	got := svc.postProcessScenes(scenes, 4, "normal", productionmode.Profile{Mode: productionmode.ModeCommentaryComic})
	if len(got) != 2 {
		t.Fatalf("expected merged scenes, got %d", len(got))
	}
	if got[0].Dialogue == "" {
		t.Fatal("expected first scene to retain narration after merge")
	}
	if !strings.Contains(got[0].Dialogue, "他当时确实有点尴尬呢") {
		t.Fatalf("expected short narration merged into previous scene, got %q", got[0].Dialogue)
	}
	if got[1].Dialogue == "" || got[1].Dialogue != "三个月前，正是这个声音在德聚楼后厨当众宣布了解雇。" {
		t.Fatalf("unexpected second scene dialogue: %q", got[1].Dialogue)
	}
}

func TestPostProcessCommentaryScenes_DoesNotFallbackToDescription(t *testing.T) {
	svc := &EpisodeService{}
	scenes := []llmScene{
		{
			Description: "德聚楼后厨，傍晚，刘师傅正低头揉面，面粉飞散。",
			Dialogue:    "",
			Duration:    4,
		},
	}

	got := svc.postProcessScenes(scenes, 4, "normal", productionmode.Profile{Mode: productionmode.ModeCommentaryComic})
	if len(got) != 1 {
		t.Fatalf("expected 1 scene, got %d", len(got))
	}
	if got[0].Dialogue != "" {
		t.Fatalf("expected empty dialogue without description fallback, got %q", got[0].Dialogue)
	}
}

func TestAlignCommentaryScenesWithSource_RepairsVisualDialogue(t *testing.T) {
	source := `[字幕:德聚楼的灶台前，刘师傅正低头揉面。] 后厨灯光昏黄。
[字幕:三个月前，正是这个声音在德聚楼后厨当众宣布了解雇。] 回忆画面。`
	scenes := []llmScene{
		{Dialogue: "包子铺内部，刘师傅神情沉着冷静，画面近景突出两人表情，环境光线柔和。"},
		{Dialogue: "三个月前，正是这个声音在德聚楼后厨当众宣布了解雇。"},
	}
	got := alignCommentaryScenesWithSource(source, scenes)
	if got[0].Dialogue != "德聚楼的灶台前，刘师傅正低头揉面。" {
		t.Fatalf("scene 0 dialogue=%q", got[0].Dialogue)
	}
	if got[1].Dialogue != "三个月前，正是这个声音在德聚楼后厨当众宣布了解雇。" {
		t.Fatalf("scene 1 dialogue=%q", got[1].Dialogue)
	}
}

func TestPostProcessCommentaryScenes_DropsVisualDialogue(t *testing.T) {
	svc := &EpisodeService{}
	scenes := []llmScene{
		{
			Description: "后厨近景。",
			Dialogue:    "包子铺内部，刘师傅神情沉着冷静，画面近景突出两人表情，环境光线柔和。",
			Duration:    4,
		},
	}

	got := svc.postProcessScenes(scenes, 4, "normal", productionmode.Profile{Mode: productionmode.ModeCommentaryComic})
	if got[0].Dialogue != "" {
		t.Fatalf("expected visual-only dialogue to be dropped, got %q", got[0].Dialogue)
	}
}
