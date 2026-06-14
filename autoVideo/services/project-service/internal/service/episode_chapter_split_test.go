package service

import (
	"strings"
	"testing"

	"github.com/autovideo/project-service/internal/scriptsplit"
)

func TestResolveStructuralEpisodeSplit_PrefersChaptersOverKeywords(t *testing.T) {
	chapterBody := strings.Repeat("这是第一章的正文内容，人物开始行动。", 8)
	chapterBody2 := strings.Repeat("这是第二章的正文内容，剧情继续推进。", 8)
	script := "前言内容。\n第一章 开端\n" + chapterBody + "\n第二章 转折\n" + chapterBody2
	keywords := []string{"第一章 开端", "第二章 转折"}

	episodes, method := resolveStructuralEpisodeSplit(script, script, keywords)
	if method != "chapters" {
		t.Fatalf("expected chapter split, got method=%q", method)
	}
	if len(episodes) != 2 {
		t.Fatalf("expected 2 chapter episodes, got %d", len(episodes))
	}
}

func TestResolveStructuralEpisodeSplit_PrefersOriginalChaptersOverOptimized(t *testing.T) {
	chapterBody := strings.Repeat("这是第一章的正文内容，人物开始行动。", 8)
	chapterBody2 := strings.Repeat("这是第二章的正文内容，剧情继续推进。", 8)
	original := "第一章 开端\n" + chapterBody + "\n第二章 转折\n" + chapterBody2
	optimized := "第一章 开端\n" + strings.Repeat("优化后改写的第一章内容。", 20) + "\n第二章 转折\n" + strings.Repeat("优化后改写的第二章内容。", 20)

	episodes, method := resolveStructuralEpisodeSplit(optimized, original, nil)
	if method != "chapters_original" {
		t.Fatalf("expected chapters_original, got method=%q", method)
	}
	if len(episodes) != 2 {
		t.Fatalf("expected 2 chapter episodes from original script, got %d", len(episodes))
	}
	if !strings.Contains(episodes[0].Excerpt, "人物开始行动") {
		t.Fatalf("expected original chapter body, got %q", episodes[0].Excerpt)
	}
}

func TestResolveStructuralEpisodeSplit_FallsBackToOriginalScriptChapters(t *testing.T) {
	chapterBody := strings.Repeat("这是第一章的正文内容，人物开始行动。", 8)
	chapterBody2 := strings.Repeat("这是第二章的正文内容，剧情继续推进。", 8)
	original := "第一章 开端\n" + chapterBody + "\n第二章 转折\n" + chapterBody2
	optimized := "第一段。\n第二段。"

	episodes, method := resolveStructuralEpisodeSplit(optimized, original, nil)
	if method != "chapters_original" {
		t.Fatalf("expected chapters_original, got method=%q", method)
	}
	if len(episodes) != 2 {
		t.Fatalf("expected 2 chapter episodes from original script, got %d", len(episodes))
	}
}

func TestResolveStructuralEpisodeSplit_UsesKeywordsWhenNoChapters(t *testing.T) {
	keywords := []string{"【段落A】", "【段落B】"}
	optimized := "开场白。\n【段落A】\n正文一。\n【段落B】\n正文二。"

	episodes, method := resolveStructuralEpisodeSplit(optimized, optimized, keywords)
	if method != "user_keywords" {
		t.Fatalf("expected user_keywords fallback, got method=%q", method)
	}
	if len(episodes) < 2 {
		t.Fatalf("expected keyword split episodes, got %d", len(episodes))
	}
}

func TestResolveStructuralEpisodeSplit_NumericChapters(t *testing.T) {
	script := scriptsplit.NormalizeForEpisodeSplit(strings.TrimSpace(`
【简介】
这是一本关于包子铺逆袭的小说。

【导语】
天还没亮，包子铺里已经飘出麦香。王大发揉着面团，门外有人轻声唤他：“刘师傅。”

01

第一段正文，王大发把塑料桶往地上一墩，陈大鹏从后厨探出头，两人开始准备开门营业。

02

第二段正文，门外排队的人渐渐多了起来，包子铺的蒸汽弥漫整条街。`)).Text

	episodes, method := resolveStructuralEpisodeSplit(script, script, nil)
	if method != "chapters" {
		t.Fatalf("expected chapters split for numeric markers, got method=%q", method)
	}
	if len(episodes) != 2 {
		t.Fatalf("expected 2 chapter episodes, got %d", len(episodes))
	}
	if !strings.Contains(episodes[0].Excerpt, "刘师傅") {
		t.Fatalf("prologue should merge into first episode")
	}
}
