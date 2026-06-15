package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/autovideo/project-service/internal/productionmode"
)

func TestRefitSceneDialogue_ThirdSubtitle(t *testing.T) {
	sc := llmScene{Dialogue: "[字幕:三个月前，正是这个声音在德聚楼后厨当众宣布了解雇。]", Duration: 4}
	refitSceneDialogue(&sc, 4, "normal", true)
	if sc.Dialogue == "" {
		t.Fatal("expected subtitle narration after refit")
	}
	t.Logf("third scene refit len=%d text=%q", utf8.RuneCountInString(sc.Dialogue), sc.Dialogue)
}

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
	if len(got) != 3 {
		t.Fatalf("expected separate plot scenes with short bridge line kept, got %d", len(got))
	}
	if got[0].Dialogue == "" {
		t.Fatal("expected first scene to retain narration after merge")
	}
	if got[2].Dialogue == "" || !strings.Contains(got[2].Dialogue, "三个月前，正是这个声音在德聚楼后厨当众宣布了解雇") {
		t.Fatalf("unexpected third scene dialogue: %q", got[2].Dialogue)
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

func TestEnsureCommentaryNarrationCoverage_RebuildsSparseScenes(t *testing.T) {
	svc := &EpisodeService{}
	source := `【导语】
我是德聚楼三十年的主理厨师。三个月了，我在北街开了个包子铺，每天凌晨四点起来和面。
王大发跪在我包子铺门口，声音发颤：「刘师傅，求你救救我！」
我抽着旱烟，指着那只黑塑料桶：「王总，以前我是一天两百的厨子；现在，一口汤一百万，不讲价。」
那只跟了我三十年的桶，桶底还留着当年德聚楼后厨的编号。我把它往桌上一放：「王总，你认得这个吗？」`
	sparse := []llmScene{
		{Description: "开场", Dialogue: "我是德聚楼三十年的主理厨师", Duration: 4},
		{Description: "求助", Dialogue: "刘师傅，求你救救我", Duration: 4},
	}
	got := svc.postProcessAndAlignCommentaryScenes(
		source,
		sparse,
		4,
		"normal",
		productionmode.Profile{Mode: productionmode.ModeCommentaryComic},
	)
	if len(got) < 8 {
		t.Fatalf("expected expanded commentary scenes, got %d", len(got))
	}
	totalDialogueRunes := 0
	for _, sc := range got {
		if sc.Dialogue == "" {
			t.Fatalf("expected non-empty dialogue in rebuilt scenes")
		}
		totalDialogueRunes += len([]rune(sc.Dialogue))
	}
	if totalDialogueRunes < 80 {
		t.Fatalf("expected much more narration coverage, got %d runes", totalDialogueRunes)
	}
	foundOpening := false
	foundHelp := false
	for _, sc := range got {
		if strings.Contains(sc.Description, "开场") {
			foundOpening = true
		}
		if strings.Contains(sc.Description, "求助") {
			foundHelp = true
		}
	}
	if !foundOpening || !foundHelp {
		t.Fatalf("expected LLM plot scene descriptions preserved, opening=%v help=%v scenes=%d", foundOpening, foundHelp, len(got))
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
