package generators

import (
	"strings"
	"testing"
)

func TestBuildSDXLWorkflowReplacesTemplateTokens(t *testing.T) {
	t.Parallel()

	workflow, err := buildSDXLWorkflow(
		defaultComfyWorkflowTemplate,
		GenerateReq{Width: 512, Height: 768, Steps: 24, CfgScale: 6.5},
		12345,
		"anime hero",
		"lowres",
	)
	if err != nil {
		t.Fatalf("buildSDXLWorkflow() error = %v", err)
	}
	if strings.Contains(workflow, "__PROMPT_JSON__") || strings.Contains(workflow, "__CFG_SCALE__") {
		t.Fatalf("workflow placeholders were not fully replaced: %s", workflow)
	}
	if !strings.Contains(workflow, "\"anime hero\"") {
		t.Fatalf("workflow does not contain prompt: %s", workflow)
	}
}

func TestBuildSDXLPromptsAddsStyleHints(t *testing.T) {
	t.Parallel()

	prompt, negative := buildSDXLPrompts(GenerateReq{
		Prompt:         "young swordsman under moonlight",
		NegativePrompt: "extra limbs",
		StylePreset:    "guofeng-myth",
	})

	if !strings.Contains(prompt, "classical chinese fantasy illustration") {
		t.Fatalf("prompt missing style prefix: %q", prompt)
	}
	if !strings.Contains(negative, "modern clothing") {
		t.Fatalf("negative prompt missing style guard: %q", negative)
	}
}

func TestBuildSDXLPromptsUsesExplicitTaskType(t *testing.T) {
	t.Parallel()

	prompt, negative := buildSDXLPrompts(GenerateReq{
		Prompt:         "a hero in armor",
		NegativePrompt: "extra limbs",
		TaskType:       "storyboard",
		StylePreset:    "anime-2d",
		Width:          1280,
		Height:         720,
	})

	if !strings.Contains(prompt, "storyboard frame") {
		t.Fatalf("prompt missing storyboard task tags: %q", prompt)
	}
	if !strings.Contains(negative, "unclear action") {
		t.Fatalf("negative prompt missing storyboard guardrails: %q", negative)
	}
	if strings.Contains(prompt, "character portrait") {
		t.Fatalf("prompt should respect explicit task type over keyword inference: %q", prompt)
	}
}

func TestNormalizeImageTaskTypeAliases(t *testing.T) {
	t.Parallel()

	if got := NormalizeImageTaskType("character_sheet", ""); got != "character-sheet" {
		t.Fatalf("NormalizeImageTaskType(character_sheet) = %q", got)
	}
	if got := NormalizeImageTaskType("", "角色立绘，红衣剑客"); got != "portrait" {
		t.Fatalf("NormalizeImageTaskType(infer portrait) = %q", got)
	}
}
