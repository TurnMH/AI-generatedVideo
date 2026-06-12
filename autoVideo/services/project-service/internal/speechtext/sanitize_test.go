package speechtext

import (
	"strings"
	"testing"
)

func TestSanitizeForSpeech_StripsStoryboardDescriptionNoise(t *testing.T) {
	drop := []string{
		"德聚楼后厨，傍晚，空气中弥漫着淡淡的面香，刘师傅正专注地揉着面团。",
		"刘师傅。",
		"刘师傅缓缓抬头。",
		"三个月前。",
	}
	for _, line := range drop {
		if out := SanitizeForSpeech(line); out != "" {
			t.Fatalf("expected empty for %q, got %q", line, out)
		}
	}
	keep := []string{
		"刘师傅，你干了三十年，机器炒菜比你稳得多，我老了，手也抖了，得换人。",
		"这桶底料，是我三十年心血的结晶，没有了它，德聚楼的味道就不在了。",
	}
	for _, line := range keep {
		if out := SanitizeForSpeech(line); out != line {
			t.Fatalf("expected keep %q, got %q", line, out)
		}
	}
}

func TestSanitizeForSpeech_StripsSceneHeadingsAndActions(t *testing.T) {
	in := `【内景 · 德聚楼后厨 · 傍晚】
刘师傅（沉稳）将手放在案板上
刘师傅（沉稳）：今天这刀，得磨亮了。
[摄影:推镜]
旁白：后厨里，刀光映着夕阳。`
	out := SanitizeForSpeech(in)
	if !strings.Contains(out, "刘师傅：今天这刀，得磨亮了。") {
		t.Fatalf("missing dialogue: %q", out)
	}
	if !strings.Contains(out, "旁白：后厨里，刀光映着夕阳。") {
		t.Fatalf("missing narration: %q", out)
	}
	for _, bad := range []string{"内景", "德聚楼", "将手放在案板上", "推镜"} {
		if strings.Contains(out, bad) {
			t.Fatalf("leaked %q in %q", bad, out)
		}
	}
}
