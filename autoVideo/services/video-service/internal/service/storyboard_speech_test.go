package service

import (
	"strings"
	"testing"
)

func TestExtractStoryboardSpeechText_StripsVisualDescriptions(t *testing.T) {
	samples := []struct {
		in   string
		want string
	}{
		{
			in:   "旁白：包子铺内部，刘师傅神情沉着冷静，指向旁边一个黑色塑料桶，桶壁有明显油渍和底料印记。王大发面露震惊和难以置信，画面近景突出两人表情和桶的细节，环境光线柔和。",
			want: "",
		},
		{
			in:   "旁白：我在德聚楼掌勺三十年，却被老板用两千块遣退",
			want: "旁白：我在德聚楼掌勺三十年，却被老板用两千块遣退",
		},
		{
			in:   "旁白：三个月后，订单退款，王大发跪求",
			want: "旁白：三个月后，订单退款，王大发跪求",
		},
		{
			in:   "我在德聚楼掌勺三十年，却被老板用两千块遣退",
			want: "我在德聚楼掌勺三十年，却被老板用两千块遣退",
		},
		{
			in:   "画面推近。[字幕:德聚楼的灶台前，刘师傅正低头揉面。] 后厨灯光昏黄。",
			want: "德聚楼的灶台前，刘师傅正低头揉面。",
		},
	}
	for _, sample := range samples {
		got := extractStoryboardSpeechText(sample.in)
		if got != sample.want {
			t.Errorf("input=%q\ngot=%q\nwant=%q", sample.in, got, sample.want)
		}
	}
}

func TestCleanScriptForSpeech_SpeakerPrefixedVisualDescription(t *testing.T) {
	samples := []struct {
		in   string
		keep bool
	}{
		{"旁白：包子铺内部，刘师傅神情沉着冷静，指向旁边一个黑色塑料桶，桶壁有明显油渍和底料印记。王大发面露震惊和难以置信，画面近景突出两人表情和桶的细节，环境光线柔和。", false},
		{"旁白：我在德聚楼掌勺三十年，却被老板用两千块遣退", true},
		{"旁白：三个月后，订单退款，王大发跪求", true},
	}
	for _, sample := range samples {
		extracted := extractStoryboardSpeechText(sample.in)
		out := cleanScriptForSpeech(extracted)
		hasContent := strings.TrimSpace(out) != ""
		if hasContent != sample.keep {
			t.Errorf("input=%q keep=%v got=%q", sample.in, sample.keep, out)
		}
	}
}

func TestLooksLikeSpeakerVisualStaging_Dialogue(t *testing.T) {
	if looksLikeSpeakerVisualStaging("拿住这个贼子！") {
		t.Fatal("dialogue should not be visual staging")
	}
	if !looksLikeSpeakerVisualStaging("包子铺内部，刘师傅神情沉着冷静，画面近景突出两人表情，环境光线柔和。") {
		t.Fatal("expected visual staging")
	}
}

func TestCleanPerClipDialogue_StripsStageDirections(t *testing.T) {
	got := cleanPerClipDialogue("环境：蒸汽弥漫。\n旁白：香气飘出。")
	if strings.Contains(got, "环境") || strings.Contains(got, "蒸汽弥漫") {
		t.Fatalf("leaked stage direction: %q", got)
	}
	if !strings.Contains(got, "旁白：香气飘出。") {
		t.Fatalf("dropped dialogue: %q", got)
	}
}

func TestFormatStoryboardDubbingFromFields_SplitsNarrationAndCharacter(t *testing.T) {
	dialogue := "三个月后，德聚楼陷入危机，王大发求助\n一口汤一百万，不讲价\n一口汤一百万，不讲价"
	scene := `包子铺内，刘师傅专注揉面。王大发小心走进铺内，轻声唤："刘师傅。"`
	got := formatStoryboardDubbingFromFields(dialogue, scene, []string{"刘师傅", "王大发"}, true)
	if !strings.Contains(got, "旁白：三个月后，德聚楼陷入危机，王大发求助") {
		t.Fatalf("missing narration: %q", got)
	}
	if !strings.Contains(got, "王大发：刘师傅。") {
		t.Fatalf("missing character quote: %q", got)
	}
	if strings.Count(got, "一口汤一百万，不讲价") > 1 {
		t.Fatalf("expected deduped narration, got %q", got)
	}
}

func TestCreateStoryboardTaskFlow_EmptyVisualOnly(t *testing.T) {
	visual := "旁白：包子铺内部，刘师傅神情沉着冷静，画面近景突出两人表情，环境光线柔和。"
	extracted := extractStoryboardSpeechText(ensureSpeakerLabelsForStoryboardDubbing(visual))
	cleaned := strings.TrimSpace(cleanScriptForSpeech(extracted))
	if cleaned != "" {
		t.Fatalf("expected empty cleaned text for visual-only storyboard, got %q", cleaned)
	}
}

func TestEnsureCommentaryNarratorLabels(t *testing.T) {
	got := ensureCommentaryNarratorLabels("我在德聚楼掌勺三十年，却被老板用两千块遣退")
	if got != "旁白：我在德聚楼掌勺三十年，却被老板用两千块遣退" {
		t.Fatalf("got %q", got)
	}
}

func TestCleanPerClipDialogueForMode_Commentary(t *testing.T) {
	got := cleanPerClipDialogueForMode("刘师傅：刘师傅正低头揉面，面粉散落在木质台面上。", true)
	if !strings.Contains(got, "旁白：") {
		t.Fatalf("expected narrator label, got %q", got)
	}
}

func TestNormalizeMislabeledNarrationSpeakers(t *testing.T) {
	in := "刘师傅：刘师傅正低头揉面，面粉散落在木质台面上。"
	got := normalizeMislabeledNarrationSpeakers(in)
	want := "旁白：刘师傅正低头揉面，面粉散落在木质台面上。"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCleanPerClipDialogue_RelabelsCharacterNarration(t *testing.T) {
	got := cleanPerClipDialogue("刘师傅：刘师傅正低头揉面，面粉散落在木质台面上。")
	if !strings.Contains(got, "旁白：") {
		t.Fatalf("expected narrator relabel, got %q", got)
	}
	if strings.HasPrefix(got, "刘师傅：") {
		t.Fatalf("character speaker leaked: %q", got)
	}
}

func TestEnsureSpeakerLabels_MultiLineWithNarrator(t *testing.T) {
	in := "环境：蒸汽弥漫。\n旁白：香气飘出。"
	got := ensureSpeakerLabelsForStoryboardDubbing(in)
	if strings.HasPrefix(got, "旁白：环境：") {
		t.Fatalf("unexpected blanket narrator prefix: %q", got)
	}
}
