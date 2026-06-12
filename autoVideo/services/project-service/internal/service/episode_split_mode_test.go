package service

import (
	"strings"
	"testing"

	"github.com/autovideo/project-service/internal/productionmode"
)

func TestSimpleSplit_ScriptDramaUsesLengthFallback(t *testing.T) {
	svc := &EpisodeService{}
	text := strings.Repeat("这是一段用于测试剧本模式分集兜底的叙事内容。", 40)
	episodes := svc.simpleSplit(text, 4, productionmode.Profile{Mode: productionmode.ModeScriptDrama})
	if len(episodes) != 4 {
		t.Fatalf("expected 4 episodes, got %d", len(episodes))
	}
}

func TestSimpleSplit_CommentaryUsesLengthFallback(t *testing.T) {
	svc := &EpisodeService{}
	text := strings.Repeat("解说漫旁白内容用于测试分集兜底。", 50)
	episodes := svc.simpleSplit(text, 3, productionmode.Profile{Mode: productionmode.ModeCommentaryComic})
	if len(episodes) != 3 {
		t.Fatalf("expected 3 episodes, got %d", len(episodes))
	}
}
