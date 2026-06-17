package service

import (
	"strings"
	"testing"
)

func TestIsCommentaryDialoguePollutedDescription(t *testing.T) {
	t.Parallel()

	cases := []struct {
		desc   string
		pollut bool
	}{
		{`眼。我拉开躺椅坐下，点上。"说吧。"。王大发…`, true},
		{`北街包子铺门口，刘师傅坐躺椅抽旱烟；王大发站对面。`, false},
		{`解说镜头 3：我抽着旱烟。`, false},
	}
	for _, tc := range cases {
		if got := isCommentaryDialoguePollutedDescription(tc.desc); got != tc.pollut {
			t.Fatalf("isCommentaryDialoguePollutedDescription(%q) = %v, want %v", tc.desc, got, tc.pollut)
		}
	}
}

func TestNormalizeCommentaryFirstPerson(t *testing.T) {
	t.Parallel()

	got := normalizeCommentaryFirstPerson("我拉开躺椅坐下 | 王大发站对面", "刘师傅")
	want := "刘师傅拉开躺椅坐下 | 王大发站对面"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExtractCommentaryVisualDescriptionFromSource(t *testing.T) {
	t.Parallel()

	source := `说："换新的，老古董碍事。"没看我一眼。
我拉开躺椅坐下，装好旱烟，点上。"说吧。"
"刘师傅，这次来，是专程道歉的。当初那件事，我处理得太粗暴了。"
王大发的脸上还挂着那副圆润的笑，但眼神已经不一样了。`
	unit := `我拉开躺椅坐下，装好旱烟，点上。"说吧。"`
	got := extractCommentaryVisualDescriptionFromSource(source, unit, nil, "刘师傅")
	if got == "" {
		t.Fatal("expected visual description")
	}
	if strings.HasPrefix(got, "眼") || strings.Contains(got, `"`) || strings.Contains(got, `「`) {
		t.Fatalf("expected semantic visual beat without dialogue pollution, got %q", got)
	}
	if !strings.Contains(got, "刘师傅") || !strings.Contains(got, "躺椅") {
		t.Fatalf("expected POV character and action staging, got %q", got)
	}
}

func TestExcerptCommentaryVisualHint_NoHardTruncation(t *testing.T) {
	t.Parallel()

	source := `没看我一眼。我拉开躺椅坐下，装好旱烟，点上。"说吧。"王大发脸上挂着笑。`
	unit := `我拉开躺椅坐下，装好旱烟，点上。"说吧。"`
	got := excerptCommentaryVisualHint(source, unit)
	if got == "" {
		t.Fatal("expected hint")
	}
	if strings.HasPrefix(got, "眼") {
		t.Fatalf("expected sentence-based extraction, got polluted prefix %q", got)
	}
}

func TestBuildMultiCharacterBlockingCueResolvesNarrator(t *testing.T) {
	t.Parallel()

	got := buildMultiCharacterBlockingCue(
		[]string{"王大发", "刘师傅"},
		`我拉开躺椅坐下，装好旱烟。王大发脸上挂着笑。`,
		"我拉开躺椅坐下",
	)
	if got == "" {
		t.Fatal("expected blocking cue")
	}
	if !strings.Contains(got, "刘师傅:") || !strings.Contains(got, "王大发:") {
		t.Fatalf("expected both characters in blocking cue, got %q", got)
	}
}
