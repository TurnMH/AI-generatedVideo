package service

import (
	"strings"
	"testing"
)

func TestCharacterCloseupPanelPromptEmphasizesHDPortrait(t *testing.T) {
	prompt := composeCharacterPanelPrompt("晨袍女人", "清晨刚醒，披着晨袍，神情放松。", "保留生活化气质", false, CharacterPanelCloseup)

	for _, want := range []string{
		"人物形象高清特写",
		"extra high-definition facial close-up portrait",
		"identity-focused hero portrait",
		"face fills roughly 65% to 75% of the frame height",
		"ultra sharp focus",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("closeup prompt %q does not contain %q", prompt, want)
		}
	}
}
