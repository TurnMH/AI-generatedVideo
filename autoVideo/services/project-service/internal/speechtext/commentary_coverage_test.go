package speechtext

import "testing"

func TestExtractCommentarySpeechUnits_SubtitleTags(t *testing.T) {
	source := `[字幕:我在德聚楼掌勺三十年，却被老板遣退。]
[字幕:三个月后，德聚楼陷入危机，王大发求助。]`
	got := ExtractCommentarySpeechUnits(source)
	if len(got) != 2 {
		t.Fatalf("expected 2 units, got %d: %v", len(got), got)
	}
}

func TestPackSpeechUnitsToMaxRunes(t *testing.T) {
	units := []string{
		"我是德聚楼三十年的主理厨师",
		"三个月了，我在北街开了个包子铺",
		"刘师傅，求你救救我",
	}
	got := PackSpeechUnitsToMaxRunes(units, 19)
	if len(got) < 2 {
		t.Fatalf("expected multiple packed clips, got %d: %v", len(got), got)
	}
	for i, clip := range got {
		if len([]rune(clip)) > 19 {
			t.Fatalf("clip %d exceeds max runes: %q", i, clip)
		}
	}
}

func TestCommentarySpeechRunes(t *testing.T) {
	source := "我是德聚楼三十年的主理厨师。三个月了，我在北街开了个包子铺。"
	if got := CommentarySpeechRunes(source); got < 20 {
		t.Fatalf("expected meaningful rune count, got %d", got)
	}
}
