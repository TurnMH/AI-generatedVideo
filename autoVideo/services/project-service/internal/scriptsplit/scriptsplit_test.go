package scriptsplit

import (
	"strings"
	"testing"
)

func TestNormalizeForEpisodeSplit_StripsSynopsisKeepsNarrativePrologue(t *testing.T) {
	script := strings.TrimSpace(`
【简介】
这是一本关于包子铺逆袭的小说，最终拿到五百万投资。

【导语】
天还没亮，包子铺里已经飘出麦香。王大发揉着面团，门外有人轻声唤他：“刘师傅。”

01

王大发把塑料桶往地上一墩，陈大鹏从后厨探出头。`)

	got := NormalizeForEpisodeSplit(script)
	if got.StrippedSynopsis == "" || !strings.Contains(got.StrippedSynopsis, "五百万") {
		t.Fatalf("expected synopsis from 简介, got %q", got.StrippedSynopsis)
	}
	if strings.Contains(got.Text, "【简介】") {
		t.Fatalf("简介 block should be removed from split text")
	}
	if !strings.Contains(got.Text, "刘师傅") {
		t.Fatalf("narrative prologue should remain, got %q", got.Text)
	}
	if !strings.Contains(got.Text, "01") {
		t.Fatalf("chapter marker should remain, got %q", got.Text)
	}
}

func TestSplitByChapters_NumericMarkersAndPrologueMerge(t *testing.T) {
	script := strings.TrimSpace(`
【导语】
天还没亮，包子铺里已经飘出麦香。王大发揉着面团，门外有人轻声唤他：“刘师傅。”

01

第一段正文，王大发把塑料桶往地上一墩，陈大鹏从后厨探出头，两人开始准备开门营业。

02

第二段正文，门外排队的人渐渐多了起来，包子铺的蒸汽弥漫整条街。`)

	episodes := SplitByChapters(script)
	if len(episodes) != 2 {
		t.Fatalf("expected 2 numeric chapter episodes, got %d", len(episodes))
	}
	if !strings.Contains(episodes[0].Excerpt, "刘师傅") {
		t.Fatalf("prologue should merge into first chapter episode")
	}
	if !strings.Contains(episodes[0].Excerpt, "第一段正文") {
		t.Fatalf("first chapter body missing")
	}
}

func TestRepairSplit_MergesSummaryTrailerEpisode(t *testing.T) {
	episodes := []DraftEpisode{
		{
			Title:   "预告",
			Summary: "简介",
			Excerpt: "这是一家濒临倒闭的包子铺，最终拿到五百万投资，完成逆袭。",
		},
		{
			Title:   "01",
			Summary: "正文",
			Excerpt: strings.Repeat("王大发在包子铺里揉面，陈大鹏在后厨忙活。", 20),
		},
	}

	repaired, actions := RepairSplit(episodes)
	if len(repaired) != 1 {
		t.Fatalf("expected merged episodes, got %d", len(repaired))
	}
	if len(actions) == 0 {
		t.Fatalf("expected repair actions")
	}
	if !strings.Contains(repaired[0].Excerpt, "五百万") || !strings.Contains(repaired[0].Excerpt, "王大发") {
		t.Fatalf("merged excerpt missing both parts: %q", repaired[0].Excerpt)
	}
}

func TestNeedsStructuralReview_DetectsTrailerEpisode(t *testing.T) {
	episodes := []DraftEpisode{
		{Title: "第1集", Excerpt: "最终拿到五百万，完成逆袭。"},
		{Title: "第2集", Excerpt: strings.Repeat("王大发在包子铺里揉面。", 30)},
	}
	if !NeedsStructuralReview(episodes) {
		t.Fatal("expected structural review for summary trailer episode")
	}
}
