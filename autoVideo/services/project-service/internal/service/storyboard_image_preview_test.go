package service

import (
	"strings"
	"testing"
)

func TestBuildStoryboardPromptDisplayZh_UsesChineseScene(t *testing.T) {
	got := buildStoryboardPromptDisplayZh(
		StoryboardGenerateRequest{
			SceneDescription: "王大发跪在刘师傅包子铺门口苦苦哀求。",
			Location:         "刘师傅包子铺",
			AspectRatio:      "16:9",
			Characters:       []string{"刘师傅", "王大发"},
		},
		[]string{"刘师傅", "王大发"},
		map[string]string{},
		map[string]string{
			"刘师傅": "传统工匠，面容沧桑但温和，手持旱烟袋。",
		},
		nil,
		nil,
		false,
	)

	if got == "" {
		t.Fatal("expected non-empty Chinese display prompt")
	}
	if !containsChinese(got) {
		t.Fatalf("expected Chinese display prompt, got: %q", got)
	}
	if containsChinese("Create a single 2D anime-style storyboard keyframe") && got == "Create a single 2D anime-style storyboard keyframe" {
		t.Fatalf("display prompt should not be raw English assembly, got: %q", got)
	}
}

func TestBuildStoryboardPromptDisplayZh_RawModeChinesePrompt(t *testing.T) {
	got := buildStoryboardPromptDisplayZh(
		StoryboardGenerateRequest{PromptUsed: "锁定后的中文最终提示词。"},
		nil,
		nil,
		nil,
		nil,
		nil,
		true,
	)
	if got != "锁定后的中文最终提示词。" {
		t.Fatalf("raw mode Chinese prompt_used should pass through, got %q", got)
	}
}

func TestBuildStoryboardPromptAutoSupplementsZh_IncludesStyleOpening(t *testing.T) {
	got := buildStoryboardPromptAutoSupplementsZh(storyboardAutoSupplementInput{
		StylePreset:        "anime-2d",
		ModelName:          "sdxl",
		NegativePrompt:     "text, watermark, blurry",
		ReferenceImageURLs: []string{"https://example.com/a.png"},
		HasCharacters:      true,
	})
	if !strings.Contains(got, "【风格开头】") {
		t.Fatalf("expected style opening line, got %q", got)
	}
	if !strings.Contains(got, "【模型质量词】") {
		t.Fatalf("expected quality suffix line, got %q", got)
	}
}

func TestBuildStoryboardPromptAutoSupplementsZh_RawMode(t *testing.T) {
	got := buildStoryboardPromptAutoSupplementsZh(storyboardAutoSupplementInput{RawMode: true})
	if !strings.Contains(got, "【高级锁定】") {
		t.Fatalf("expected raw mode notice, got %q", got)
	}
}
