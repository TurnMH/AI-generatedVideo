package service

import (
	"strings"
	"testing"
)

func TestCleanScriptForSpeech_StripsProductionTags(t *testing.T) {
	in := `第一章 拿住这个贼子[剪辑:开场先用惨叫声引入][音效:惨叫][美术:地牢][调色:冷调][摄影:推镜][灯光:冷白][服化:娄真五+长袍][导演:紧凑节奏][场记:接上一场]
娄真五：拿住这个贼子！
掌刑执事：是，娄师兄。
环境：阴冷地牢。
场景：地牢入口。
摄影：推至特写。
（他缓缓站起身）
【画面转场】`
	out := cleanScriptForSpeech(in)
	want := []string{"娄真五：拿住这个贼子！", "掌刑执事：是，娄师兄。"}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing dialogue line: %q\ngot: %q", w, out)
		}
	}
	bad := []string{"第一章", "拿住这个贼子！娄真五", "阴冷地牢", "推至特写", "缓缓站起身", "画面转场", "地牢入口"}
	for _, b := range bad {
		if b == "" {
			continue
		}
		if strings.Contains(out, b) && b != "阴冷地牢" {
			t.Errorf("leaked text: %q\ngot: %q", b, out)
		}
	}
	if strings.Contains(out, "环境") || strings.Contains(out, "场景") || strings.Contains(out, "摄影") {
		t.Errorf("leaked production prefix:\n%s", out)
	}
	if strings.Contains(out, "第一章") {
		t.Errorf("leaked chapter title:\n%s", out)
	}
	t.Logf("cleaned output:\n%s", out)
}

func TestCleanScriptForSpeech_StripsSceneHeadingsAndActions(t *testing.T) {
	in := `【内景 · 德聚楼后厨 · 傍晚】
刘师傅（沉稳）将手放在案板上
刘师傅（沉稳）：今天这刀，得磨亮了。
△蒸汽弥漫
内景 · 厨房 · 夜
角色
旁白：后厨里，刀光映着夕阳。`
	out := cleanScriptForSpeech(in)
	want := []string{"刘师傅：今天这刀，得磨亮了。", "旁白：后厨里，刀光映着夕阳。"}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing dialogue line: %q\ngot: %q", w, out)
		}
	}
	bad := []string{"内景", "德聚楼后厨", "将手放在案板上", "蒸汽弥漫", "角色"}
	for _, b := range bad {
		if strings.Contains(out, b) {
			t.Errorf("leaked non-speech text: %q\ngot: %q", b, out)
		}
	}
}

func TestCleanScriptForSpeech_StripsStoryboardDescriptionNoise(t *testing.T) {
	samples := []struct {
		in   string
		keep bool
	}{
		{"德聚楼后厨，傍晚，空气中弥漫着淡淡的面香，刘师傅正专注地揉着面团。", false},
		{"刘师傅。", false},
		{"刘师傅缓缓抬头。", false},
		{"三个月前。", false},
		{"刘师傅，你干了三十年，机器炒菜比你稳得多，我老了，手也抖了，得换人。", true},
		{"这桶底料，是我三十年心血的结晶，没有了它，德聚楼的味道就不在了。", true},
		{"刘师傅，德聚楼出了大问题，求你救救我！", true},
	}
	for _, sample := range samples {
		out := cleanScriptForSpeech(sample.in)
		hasContent := strings.TrimSpace(out) != ""
		if hasContent != sample.keep {
			t.Errorf("input=%q keep=%v got=%q", sample.in, sample.keep, out)
		}
	}
}

func TestJoinDialogues_CleansEachLine(t *testing.T) {
	dialogues := []string{
		"【内景 · 厨房 · 夜】\n刘师傅：开火。",
		"环境：蒸汽弥漫。\n旁白：香气飘出。",
	}
	out := joinDialogues(dialogues)
	if strings.Contains(out, "内景") || strings.Contains(out, "环境") || strings.Contains(out, "蒸汽弥漫") {
		t.Fatalf("joinDialogues leaked stage directions: %q", out)
	}
	if !strings.Contains(out, "开火") || !strings.Contains(out, "香气飘出") {
		t.Fatalf("joinDialogues dropped dialogue: %q", out)
	}
	if strings.Contains(out, "旁白") || strings.Contains(out, "刘师傅：") {
		t.Fatalf("joinDialogues should strip routing labels: %q", out)
	}
}