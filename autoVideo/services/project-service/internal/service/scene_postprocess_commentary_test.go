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

	got := svc.postProcessScenes(scenes, 4, productionmode.Profile{Mode: productionmode.ModeCommentaryComic})
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
