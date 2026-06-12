package service

import "testing"

func TestResolveStructuralEpisodeSplit_PrefersChaptersOverKeywords(t *testing.T) {
	script := "前言内容。\n第一章 开端\n第一段。\n第二章 转折\n第二段。"
	keywords := []string{"第一章 开端", "第二章 转折"}

	episodes, method := resolveStructuralEpisodeSplit(script, script, keywords)
	if method != "chapters" {
		t.Fatalf("expected chapter split, got method=%q", method)
	}
	if len(episodes) != 2 {
		t.Fatalf("expected 2 chapter episodes, got %d", len(episodes))
	}
}

func TestResolveStructuralEpisodeSplit_FallsBackToOriginalScriptChapters(t *testing.T) {
	original := "第一章 开端\n第一段。\n第二章 转折\n第二段。"
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
