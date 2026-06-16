package service

import (
	"strings"
	"testing"
)

func TestSanitizeCharacterDescriptionForPanelStripsMultiViewLayout(t *testing.T) {
	raw := "林夏，九头身比例。画面左侧三分之一为头肩肖像，右侧三分之二为全身正面、侧面、背面三视图并排。"
	got := sanitizeCharacterDescriptionForPanel(raw)
	for _, banned := range []string{"三视图", "左侧三分之一", "并排", "右侧三分之二"} {
		if strings.Contains(got, banned) {
			t.Fatalf("sanitized description still contains %q: %q", banned, got)
		}
	}
	if !strings.Contains(got, "林夏") {
		t.Fatalf("sanitized description should keep character name, got %q", got)
	}
}

func TestCharacterFrontPanelPromptForShortDescription(t *testing.T) {
	prompt := composeCharacterPanelPrompt("林夏", "年轻女子。", "", false, CharacterPanelFront)
	for _, want := range []string{
		"SINGLE SUBJECT ONLY",
		"exactly one person in this image",
		"九头身比例",
		"9-head body proportion",
		"画面中只有这一个人物",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("front panel prompt missing %q in %q", want, prompt)
		}
	}
	for _, banned := range []string{"three views", "side by side", "三视图", "并排"} {
		if strings.Contains(strings.ToLower(prompt), strings.ToLower(banned)) {
			t.Fatalf("front panel prompt should not contain layout phrase %q", banned)
		}
	}
}
