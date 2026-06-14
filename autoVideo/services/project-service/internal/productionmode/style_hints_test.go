package productionmode

import (
	"strings"
	"testing"

	"github.com/autovideo/project-service/internal/stylepreset"
)

func TestDefaultSpeechPace(t *testing.T) {
	if got := DefaultSpeechPace(stylepreset.Anime3D); got != "slightly_fast" {
		t.Fatalf("anime-3d pace = %q", got)
	}
	if got := DefaultSpeechPace(stylepreset.LiveActionShort); got != "with_pauses" {
		t.Fatalf("live-action-short pace = %q", got)
	}
}

func TestStyleSplitVisualHint_Anime3D(t *testing.T) {
	got := StyleSplitVisualHint(stylepreset.Anime3D, "dynamic")
	if !strings.Contains(got, "三维动漫") {
		t.Fatalf("missing 3d hint: %q", got)
	}
	if strings.Contains(got, "二维线稿") && strings.Contains(got, "不要写成纯二维") == false {
		t.Fatalf("should discourage flat 2d: %q", got)
	}
}

func TestSceneSplitStyleBlock(t *testing.T) {
	block := SceneSplitStyleBlock(SceneSplitParams{
		StyleHint: StyleSplitVisualHint(stylepreset.Anime2D, ""),
	})
	if block == "" || !strings.Contains(block, "视觉风格约束") {
		t.Fatalf("unexpected block: %q", block)
	}
}

func TestScriptPrepRuntimeContextIncludesDuration(t *testing.T) {
	got := ScriptPrepRuntimeContext(stylepreset.Anime3D, "gentle", 5, "slightly_fast")
	if !strings.Contains(got, "5 秒") || !strings.Contains(got, "三维动漫") {
		t.Fatalf("missing runtime context: %q", got)
	}
}
