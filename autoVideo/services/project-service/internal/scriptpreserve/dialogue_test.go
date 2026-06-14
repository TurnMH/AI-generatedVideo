package scriptpreserve

import (
	"strings"
	"testing"
)

func TestExtractLockedDialogues_QuotedAndScreenplay(t *testing.T) {
	source := strings.TrimSpace(`
王大发清了清嗓子：“刘师傅，我老了，手也抖了。”

刘师傅（沉着）
　　你这话说得太早了。

[字幕:德聚楼的灶台前，刘师傅正低头揉面。]`)

	got := ExtractLockedDialogues(source)
	if len(got) < 3 {
		t.Fatalf("expected at least 3 locked dialogues, got %d: %+v", len(got), got)
	}
	if !containsDialogueText(got, "刘师傅，我老了，手也抖了。") {
		t.Fatalf("missing quoted dialogue: %+v", got)
	}
	if !containsDialogueText(got, "你这话说得太早了。") {
		t.Fatalf("missing screenplay dialogue: %+v", got)
	}
	if !containsDialogueText(got, "德聚楼的灶台前，刘师傅正低头揉面。") {
		t.Fatalf("missing subtitle dialogue: %+v", got)
	}
}

func TestEnforceLockedDialogues_RestoresMissingLines(t *testing.T) {
	source := `王大发说：“刘师傅，我老了。”`
	rewritten := `王大发焦虑地开口，委婉表达了自己年迈手抖、想要退居二线的想法。`

	out, restored := EnforceLockedDialogues(source, rewritten)
	if restored != 1 {
		t.Fatalf("expected 1 restored dialogue, got %d", restored)
	}
	if !strings.Contains(out, "刘师傅，我老了。") {
		t.Fatalf("restored text missing original dialogue: %q", out)
	}
}

func TestEnforceLockedDialogues_KeepsAlreadyPresentLines(t *testing.T) {
	source := `王大发说：“刘师傅。”`
	rewritten := `【内景 · 后厨 · 清晨】\n王大发（焦虑）\n　　“刘师傅。”`

	out, restored := EnforceLockedDialogues(source, rewritten)
	if restored != 0 {
		t.Fatalf("expected no restoration, got %d; out=%q", restored, out)
	}
}

func containsDialogueText(dialogues []LockedDialogue, want string) bool {
	for _, d := range dialogues {
		if d.Text == want {
			return true
		}
	}
	return false
}
