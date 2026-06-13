package speechtext

import "testing"

func TestExtractCharacterQuotesFromScene_InferredSpeaker(t *testing.T) {
	scene := `包子铺内，刘师傅专注揉面。王大发小心走进铺内，轻声唤："刘师傅。"刘师傅抬头。`
	got := ExtractCharacterQuotesFromScene(scene, []string{"刘师傅", "王大发"})
	if len(got) != 1 {
		t.Fatalf("expected 1 quote, got %d", len(got))
	}
	if got[0].Speaker != "王大发" || got[0].Quote != "刘师傅。" {
		t.Fatalf("unexpected quote: %+v", got[0])
	}
}

func TestExtractCharacterQuotesFromScene_SkipsVisualQuote(t *testing.T) {
	scene := `画面近景，字幕显示"环境光线柔和"。`
	got := ExtractCharacterQuotesFromScene(scene, []string{"刘师傅"})
	if len(got) != 0 {
		t.Fatalf("expected no quotes, got %+v", got)
	}
}

func TestDedupeSpeechLines(t *testing.T) {
	got := DedupeSpeechLines([]string{"一口汤一百万，不讲价", "一口汤一百万，不讲价", "违约转卖，我设下陷阱"})
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %v", got)
	}
}
