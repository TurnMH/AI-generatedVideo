package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/autovideo/video-service/internal/model"
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

func TestMaxRunesForClipDurationSec(t *testing.T) {
	if got := maxRunesForClipDurationSec(5, "normal"); got != 24 {
		t.Fatalf("5s normal got %d want 24", got)
	}
}

func TestCleanPerClipDialogueWithFields_SplitsMixedSpeech(t *testing.T) {
	dialogue := "刘师傅，这次来，是专程道歉的。当初那件事，我处理得太粗暴了。"
	got := cleanPerClipDialogueWithFields(dialogue, "", []string{"王大发"}, true, 180)
	if !strings.Contains(got, "王大发：") {
		t.Fatalf("expected character line, got %q", got)
	}
}

func TestExtractDialogues_UsesSceneContext(t *testing.T) {
	rc := model.RenderConfig{
		"dialogues":          []interface{}{"刘师傅，这次来，是专程道歉的。"},
		"scene_descriptions": []interface{}{""},
		"scene_characters":   []interface{}{[]interface{}{"王大发"}},
		"production_mode":      "commentary",
		"durations":            []interface{}{5.0},
	}
	got := extractDialogues(rc, 1)
	if len(got) == 0 || !strings.Contains(got[0], "王大发：") {
		t.Fatalf("extractDialogues should split character speech: %#v", got)
	}
}

func TestCleanPerClipDialogue_StripsStageDirections(t *testing.T) {
	got := cleanPerClipDialogueForMode("环境：蒸汽弥漫。\n旁白：香气飘出。", false, 180)
	if strings.Contains(got, "环境") || strings.Contains(got, "蒸汽弥漫") {
		t.Fatalf("leaked stage direction: %q", got)
	}
	if got == "" || !strings.HasPrefix(strings.TrimSpace(got), "旁白") {
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
	if strings.Count(got, "一口汤一百万，不讲价") > 2 {
		t.Fatalf("expected deduped narration, got %q", got)
	}
}

func TestFormatStoryboardDubbingFromFields_SplitsEmbeddedQuoteFromNarration(t *testing.T) {
	dialogue := `就是他，我走那天，他顺手把我用了二十年的老铁勺扔进垃圾桶，说："换新的，老古董碍事。"`
	got := formatStoryboardDubbingFromFields(dialogue, "", []string{"王大发", "陈大鹏", "刘师傅"}, true)
	if !strings.Contains(got, "旁白：") || strings.Contains(got, "换新的，老古董碍事") && strings.Count(got, "换新的，老古董碍事") != 1 {
		t.Fatalf("expected narration without duplicated quote, got %q", got)
	}
	if !strings.Contains(got, "：换新的，老古董碍事。") {
		t.Fatalf("expected separate character quote line, got %q", got)
	}
}

func TestFormatStoryboardDubbingFromFields_DirectAddressCharacterSpeech(t *testing.T) {
	dialogue := "刘师傅，这次来，是专程道歉的。当初那件事，我处理得太粗暴了。"
	got := formatStoryboardDubbingFromFields(dialogue, "", []string{"刘师傅", "王大发"}, true)
	if strings.Contains(got, "旁白：刘师傅，这次来") {
		t.Fatalf("direct address should not stay in narration: %q", got)
	}
	if !strings.Contains(got, "王大发：这次来，是专程道歉的。当初那件事，我处理得太粗暴了。") {
		t.Fatalf("expected王大发 character line, got %q", got)
	}
}

func TestFormatStoryboardDubbingFromFields_DirectAddressWithSingleListedCharacter(t *testing.T) {
	dialogue := "刘师傅，这次来，是专程道歉的。当初那件事，我处理得太粗暴了。"
	got := formatStoryboardDubbingFromFields(dialogue, "", []string{"王大发"}, true)
	if strings.Contains(got, "旁白：刘师傅，这次来") {
		t.Fatalf("direct address should not stay in narration: %q", got)
	}
	if !strings.Contains(got, "王大发：这次来，是专程道歉的。当初那件事，我处理得太粗暴了。") {
		t.Fatalf("expected王大发 character line, got %q", got)
	}
}

func TestCompactClipDialogue_PreservesSpeakerLines(t *testing.T) {
	in := "旁白：就是他，我走那天，他顺手把我用了二十年的老铁勺扔进垃圾桶。\n王大发：换新的，老古董碍事。"
	got := compactClipDialogue(in, 180)
	if !strings.Contains(got, "\n") {
		t.Fatalf("expected multi-line output, got %q", got)
	}
	if !strings.Contains(got, "王大发：换新的，老古董碍事") {
		t.Fatalf("missing character line: %q", got)
	}
}

func TestCompactSingleSpeechBody_TruncatesByRunesNotBytes(t *testing.T) {
	in := strings.Repeat("我在德聚楼掌勺三十年，却被老板用两千块遣退。", 3)
	// Must not panic when maxRunes is smaller than byte length of CJK text.
	got := compactSingleSpeechBody(in, 28)
	if got == "" {
		t.Fatal("expected truncated text")
	}
	if utf8.RuneCountInString(strings.TrimSuffix(got, "。")) > 28 {
		t.Fatalf("expected <= 28 runes before suffix, got %q", got)
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
	got := cleanPerClipDialogueForMode("刘师傅：刘师傅正低头揉面，面粉散落在木质台面上。", true, 0)
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
