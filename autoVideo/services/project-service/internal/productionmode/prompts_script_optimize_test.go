package productionmode

import (
	"strings"
	"testing"
)

func TestEpisodePolishSystemPrompt_CommentaryComic(t *testing.T) {
	prompt := EpisodePolishSystemPrompt(ModeCommentaryComic)
	if prompt == "" {
		t.Fatal("expected commentary polish prompt")
	}
	if !strings.Contains(prompt, "解说漫") {
		t.Fatal("commentary polish prompt should mention 解说漫")
	}
	if strings.Contains(prompt, "【内景/外景") {
		t.Fatal("commentary polish prompt should not push standard scene screenplay format")
	}
}

func TestEpisodeOptimizeSystemPrompt_CommentaryComic(t *testing.T) {
	prompt := EpisodeOptimizeSystemPrompt(ModeCommentaryComic)
	if !strings.Contains(prompt, "[字幕:") {
		t.Fatal("commentary optimize prompt should require subtitle narration markers")
	}
	if EpisodeOptimizeUserAction(ModeCommentaryComic) == EpisodeOptimizeUserAction(ModeScriptDrama) {
		t.Fatal("commentary optimize user action should differ from script drama")
	}
}

func TestEpisodeReviewSystemPrompt_CommentaryComic(t *testing.T) {
	prompt := EpisodeReviewSystemPrompt(ModeCommentaryComic)
	if !strings.Contains(prompt, "narration_gap") {
		t.Fatal("commentary review prompt should include narration-specific issue types")
	}
}

func TestHasInlineScriptAnnotations(t *testing.T) {
	if HasInlineScriptAnnotations("plain text without tags") {
		t.Fatal("expected plain text to have no inline annotations")
	}
	annotated := "开场。[字幕:夜色降临] 镜头推近。[角色:林默]"
	if !HasInlineScriptAnnotations(annotated) {
		t.Fatal("expected annotated commentary script to be detected")
	}
}

func TestShouldSkipScriptPrepAfterAutoOptimize(t *testing.T) {
	annotated := "开场。[字幕:夜色降临] 镜头推近。[角色:林默]"
	if !ShouldSkipScriptPrepAfterAutoOptimize("done", "done", annotated, ModeCommentaryComic) {
		t.Fatal("commentary should always skip script prep and use uploaded text")
	}
	if !ShouldSkipScriptPrepAfterAutoOptimize("pending", "done", annotated, ModeCommentaryComic) {
		t.Fatal("commentary should skip script prep even when optimize is not done")
	}
	if !ShouldSkipScriptPrepAfterAutoOptimize("done", "reviewing", annotated, ModeCommentaryComic) {
		t.Fatal("commentary should skip script prep even while review is in progress")
	}
	visualOnly := "开场。[场景:后厨][人物:刘师傅揉面][摄影:中景]"
	if !ShouldSkipScriptPrepAfterAutoOptimize("done", "done", visualOnly, ModeCommentaryComic) {
		t.Fatal("commentary should skip script prep and use uploaded text without [字幕:] tags")
	}
	if !ShouldSkipScriptPrepAfterAutoOptimize("done", "done", visualOnly, ModeScriptDrama) {
		t.Fatal("script drama can still skip prep with generic annotations")
	}
}
