package productionmode

import "testing"

func TestNeedsCommentaryFormatRepair_DramaMisformat(t *testing.T) {
	text := `【内景 · 德聚楼后厨 · 傍晚】
木质灶台上散落着面粉。
王大发（焦虑）
　　刘师傅。`
	if !NeedsCommentaryFormatRepair(text) {
		t.Fatal("expected drama misformat to need repair")
	}
}

func TestNeedsCommentaryFormatRepair_ValidCommentary(t *testing.T) {
	text := `[字幕:德聚楼的灶台前，刘师傅正低头揉面。] 木质台面上散落面粉。
[字幕:门口传来轻轻的脚步声，王大发站在门口，神情有些拘谨。]`
	if NeedsCommentaryFormatRepair(text) {
		t.Fatal("expected annotated commentary script to pass validation")
	}
}

func TestCommentaryFormatIssues(t *testing.T) {
	issues := CommentaryFormatIssues("【内景 · 厨房 · 夜】\n角色（怒）\n　　你出去。")
	if len(issues) == 0 {
		t.Fatal("expected format issues")
	}
	if issues[0].Type != "narration_gap" {
		t.Fatalf("unexpected issue type: %s", issues[0].Type)
	}
}
