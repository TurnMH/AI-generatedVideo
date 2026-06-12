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
	if !ShouldSkipScriptPrepAfterAutoOptimize("done", "done", annotated) {
		t.Fatal("expected to skip script prep when optimize output is already annotated")
	}
	if ShouldSkipScriptPrepAfterAutoOptimize("pending", "done", annotated) {
		t.Fatal("should not skip when optimize is not done")
	}
	if ShouldSkipScriptPrepAfterAutoOptimize("done", "reviewing", annotated) {
		t.Fatal("should not skip while review is still in progress")
	}
}
