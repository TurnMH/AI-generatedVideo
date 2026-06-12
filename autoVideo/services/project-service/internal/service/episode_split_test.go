package service

import (
	"strings"
	"testing"

	"github.com/autovideo/project-service/internal/productionmode"
)

func TestSplitAdSemanticUnits_PreservesAdBoundaries(t *testing.T) {
	text := `【开场钩子】
3秒看懂为什么这款产品能救急。

卖点一：轻便小巧，通勤随手带走。
支持办公室、地铁、咖啡店多场景快速使用。

转场：镜头切到真实办公室桌面。
口播：早八开会前，我就靠它快速搞定。`

	units := splitAdSemanticUnits(text)
	if len(units) < 3 {
		t.Fatalf("expected >=3 semantic units, got %d", len(units))
	}

	joined := make([]string, 0, len(units))
	for _, unit := range units {
		joined = append(joined, unit.Text)
	}
	all := strings.Join(joined, "\n")
	for _, want := range []string{"开场钩子", "卖点一", "转场", "口播"} {
		if !strings.Contains(all, want) {
			t.Fatalf("semantic units missing marker %q; got %q", want, all)
		}
	}
}

func TestSimpleSplit_UsesSemanticChunksBeforeLengthFallback(t *testing.T) {
	svc := &EpisodeService{}
	text := `【开场】
今天给你看一个真实办公室里的效率提升方案。

卖点一：一键启动，操作门槛极低。
卖点二：收纳体积小，通勤包也能放下。

转场：镜头切到午休会议室。
口播：从早会到午后复盘，全程都能接上。

CTA：现在就点击领取试用资格。`

	episodes := svc.simpleSplit(text, 3, productionmode.Profile{Mode: productionmode.ModeAd})
	if len(episodes) != 3 {
		t.Fatalf("expected 3 episodes, got %d", len(episodes))
	}
	if !strings.Contains(episodes[0].Excerpt, "开场") {
		t.Fatalf("episode 1 should preserve opening hook, got %q", episodes[0].Excerpt)
	}
	if !strings.Contains(episodes[len(episodes)-1].Excerpt, "CTA") {
		t.Fatalf("last episode should preserve CTA segment, got %q", episodes[len(episodes)-1].Excerpt)
	}
}
