package speechtext

import "testing"

func TestCompactClipDialogue_DedupesRepeatedSentences(t *testing.T) {
	text := `刘师傅。 我在德聚楼干了三十年，从洗碗学徒熬到主厨，但机器炒菜分秒不差，我老了手抖了，不如机器好用。 我在德聚楼干了三十年，从洗碗学徒熬到主厨，但机器炒菜分秒不差，我老了手抖了，不如机器好用。 我在德聚楼干了三十年，从洗碗学徒熬到主厨，但机器炒菜分秒不差，我老了手抖了，不如机器好用。`
	got := CompactClipDialogue(text, 180)
	if got == "" {
		t.Fatal("expected compact dialogue")
	}
	if count := stringsCount(got, "我在德聚楼干了三十年"); count != 1 {
		t.Fatalf("expected one retained sentence, got %d in %q", count, got)
	}
}

func TestCompactClipDialogue_PrefersFirstSubtitleTag(t *testing.T) {
	text := `[字幕:德聚楼的灶台前，刘师傅正低头揉面。] 后厨灯光昏黄。 [字幕:三个月前，正是这个声音在德聚楼后厨当众宣布了解雇。]`
	got := CompactClipDialogue(text, 180)
	want := "德聚楼的灶台前，刘师傅正低头揉面。"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSplitSpeechUnitsPreservingPunctuation(t *testing.T) {
	text := `"王总，你说个数吧。"`
	got := splitSpeechUnitsPreservingPunctuation(text)
	if len(got) != 1 || got[0] != `"王总，你说个数吧。"` {
		t.Fatalf("expected [\"王总，你说个数吧。\"], got %v", got)
	}

	text2 := `就差了点时间……"
我低头点上了烟，没理他。
"王总，你说个数吧。"`
	got2 := splitSpeechUnitsPreservingPunctuation(text2)
	if len(got2) != 3 {
		t.Fatalf("expected 3 units, got %d: %v", len(got2), got2)
	}
	if got2[0] != `就差了点时间……"` || got2[1] != `我低头点上了烟，没理他。` || got2[2] != `"王总，你说个数吧。"` {
		t.Fatalf("unexpected units: %v", got2)
	}
}

func TestCompactCommentaryDialogue_VerbatimAndPunctuation(t *testing.T) {
	text := `就差了点时间……" 我低头点上了烟，没理他。 "王总，你说个数吧。"`
	got := CompactCommentaryDialogue(text, 100)
	if got != text {
		t.Fatalf("expected verbatim text preserved, got %q", got)
	}
}

func stringsCount(text, needle string) int {
	count := 0
	for {
		idx := indexOfString(text, needle)
		if idx < 0 {
			return count
		}
		count++
		text = text[idx+len(needle):]
	}
}

func indexOfString(text, needle string) int {
	for i := 0; i+len(needle) <= len(text); i++ {
		if text[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
