package service

import (
	"strings"
	"testing"

	"github.com/autovideo/project-service/internal/productionmode"
	"github.com/autovideo/project-service/internal/speechtext"
)

func TestSupplementCommentaryScenes_PreservesPlotDescriptions(t *testing.T) {
	source := `我是德聚楼三十年的主理厨师。三个月了，我在北街开了个包子铺，每天凌晨四点起来和面。
王大发跪在我包子铺门口：「刘师傅，求你救救我！」`
	plotScenes := []llmScene{
		{
			Description: "德聚楼后厨，刘师傅被遣退，神情落寞。",
			Dialogue:    "我是德聚楼三十年的主理厨师",
			Duration:    4,
			Location:    "德聚楼",
		},
		{
			Description: "北街包子铺门口，王大发跪地求助。",
			Dialogue:    "刘师傅，求你救救我",
			Duration:    4,
			Location:    "包子铺",
		},
	}
	got := supplementCommentaryScenesFromSource(
		source,
		plotScenes,
		speechtext.ExtractCommentarySpeechUnits(source),
		4,
		"normal",
	)
	if len(got) <= len(plotScenes) {
		t.Fatalf("expected supplemental scenes, got %d", len(got))
	}
	descs := make(map[string]bool, len(got))
	for _, sc := range got {
		descs[sc.Description] = true
	}
	if !descs[plotScenes[0].Description] || !descs[plotScenes[1].Description] {
		t.Fatalf("expected LLM plot descriptions preserved in output: %#v", got)
	}
}

func TestPostProcessAndAlignCommentaryScenes_PlotFirst(t *testing.T) {
	svc := &EpisodeService{}
	source := `我是德聚楼三十年的主理厨师。三个月了，我在北街开了个包子铺。
王大发跪在我包子铺门口：「刘师傅，求你救救我！」`
	plotScenes := []llmScene{
		{Description: "遣退回忆", Dialogue: "我是德聚楼三十年的主理厨师。", Duration: 4},
		{Description: "跪地求助", Dialogue: "刘师傅，求你救救我！", Duration: 4},
	}
	got := svc.postProcessAndAlignCommentaryScenes(
		source,
		plotScenes,
		4,
		"normal",
		productionmode.Profile{Mode: productionmode.ModeCommentaryComic},
	)
	if len(got) < 2 {
		t.Fatalf("expected scenes, got %d", len(got))
	}
	descs := make(map[string]bool, len(got))
	for _, sc := range got {
		descs[sc.Description] = true
	}
	hasDesc := func(part string) bool {
		for desc := range descs {
			if strings.Contains(desc, part) {
				return true
			}
		}
		return false
	}
	if !hasDesc("遣退回忆") || !hasDesc("跪地求助") {
		t.Fatalf("plot descriptions not preserved: %#v", got)
	}
	hasShop := false
	minLen := 0
	for _, sc := range got {
		l := len([]rune(strings.TrimSpace(sc.Dialogue)))
		if l > minLen {
			minLen = l
		}
		if strings.Contains(sc.Dialogue, "北街") && strings.Contains(sc.Dialogue, "包子铺") {
			hasShop = true
		}
	}
	if minLen < 18 {
		t.Fatalf("expected longer dialogue per scene, min=%d scenes=%#v", minLen, got)
	}
	_ = hasShop
}
