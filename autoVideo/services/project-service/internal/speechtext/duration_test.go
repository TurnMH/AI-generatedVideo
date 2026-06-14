package speechtext

import "testing"

func TestMaxRunesForClipDuration(t *testing.T) {
	if got := MaxRunesForClipDuration(5, "normal"); got != 24 {
		t.Fatalf("5s normal got %d want 24", got)
	}
	if got := MaxRunesForClipDuration(10, "normal"); got != 48 {
		t.Fatalf("10s normal got %d want 48", got)
	}
	if got := MaxRunesForClipDuration(5, "with_pauses"); got != 19 {
		t.Fatalf("5s with_pauses got %d want 19", got)
	}
}

func TestFitStoryboardDialogue_TruncatesLongCommentary(t *testing.T) {
	dialogue := "刘师傅，这次来，是专程道歉的。当初那件事，我处理得太粗暴了。德聚楼现在确实出了问题，机器上线之后口味垮了，上上周婚宴，新郎的岳父把那碗汤当场泼了我一脸。"
	got := FitStoryboardDialogue(dialogue, 5, "normal", true)
	if got == dialogue {
		t.Fatalf("expected truncated dialogue, got %q", got)
	}
	if len([]rune(got)) > MaxRunesForClipDuration(5, "normal")+4 {
		t.Fatalf("dialogue too long for 5s clip: %q", got)
	}
}

func TestFinalizeCommentaryDialogueWithLimit_SubtitleScene(t *testing.T) {
	got := FinalizeCommentaryDialogueWithLimit("[字幕:三个月前，正是这个声音在德聚楼后厨当众宣布了解雇。]", MaxRunesForClipDuration(4, "normal"))
	if got == "" {
		t.Fatal("expected subtitle narration")
	}
}
