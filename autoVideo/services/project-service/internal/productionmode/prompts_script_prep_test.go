package productionmode

import (
	"strings"
	"testing"
)

func TestScriptPrepUserContent_Commentary(t *testing.T) {
	got := ScriptPrepUserContent(ModeCommentaryComic, 3, "原文")
	if !strings.Contains(got, "解说漫") {
		t.Fatal("commentary prep user prompt should mention 解说漫")
	}
	if !strings.Contains(got, "[字幕:") {
		t.Fatal("commentary prep user prompt should require subtitle narration tags")
	}
	if strings.Contains(got, "世界观/视觉宇宙") {
		t.Fatal("commentary prep user prompt should not use drama screenplay checklist")
	}
}

func TestScriptPrepUserContent_Drama(t *testing.T) {
	got := ScriptPrepUserContent(ModeScriptDrama, 1, "原文")
	if !strings.Contains(got, "世界观/视觉宇宙") {
		t.Fatal("drama prep user prompt should keep screenplay checklist")
	}
}
