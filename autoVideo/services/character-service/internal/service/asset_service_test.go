package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/autovideo/character-service/internal/model"
)

func TestMergeAssetDescription(t *testing.T) {
	t.Run("appends instruction to existing description", func(t *testing.T) {
		got := mergeAssetDescription("红衣女侠，正面站立，电影感光影。", "把披风改成黑色，并增加金属肩甲。")
		want := "红衣女侠，正面站立，电影感光影。\n\n补充要求：\n把披风改成黑色，并增加金属肩甲。"
		if got != want {
			t.Fatalf("mergeAssetDescription() = %q, want %q", got, want)
		}
	})

	t.Run("does not duplicate repeated instruction", func(t *testing.T) {
		current := "红衣女侠。\n\n补充要求：\n把披风改成黑色，并增加金属肩甲。"
		got := mergeAssetDescription(current, "把披风改成黑色，并增加金属肩甲。")
		if got != current {
			t.Fatalf("mergeAssetDescription() duplicated instruction: got %q, want %q", got, current)
		}
	})
}

func TestBuildAssetFallbackChatResult(t *testing.T) {
	result := buildAssetFallbackChatResult(&model.Asset{
		Status:      "pending",
		Description: "古风少年，长发，竹林背景。",
	}, map[string]interface{}{
		"content": "增加一把折扇，表情更从容。",
	})

	if result == nil {
		t.Fatal("buildAssetFallbackChatResult() returned nil")
	}
	if result.AssistantReply == "" {
		t.Fatal("buildAssetFallbackChatResult() returned empty assistant reply")
	}
	wantDescription := "古风少年，长发，竹林背景。\n\n补充要求：\n增加一把折扇，表情更从容。"
	if result.UpdatedDescription != wantDescription {
		t.Fatalf("buildAssetFallbackChatResult() description = %q, want %q", result.UpdatedDescription, wantDescription)
	}
}

func TestComposeAssetImagePromptIncludesTypeSpecificGuidance(t *testing.T) {
	t.Run("character asset", func(t *testing.T) {
		prompt := composeAssetImagePrompt("character", "晨袍女人", "清晨刚醒，披着晨袍，神情放松。", "保留生活化气质", "")
		for _, want := range []string{
			"晨袍女人",
			"character reference sheet",
			"head and shoulder portrait on the left third of the image",
			"head and neck fully visible in portrait section",
			"full body front view, full body side view, full body back view arranged side by side",
			"清晨刚醒，披着晨袍，神情放松。",
			"保留生活化气质",
			"pure white background",
			"(no text:2.0)",
			"(masterpiece:1.5)",
		} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("prompt %q does not contain %q", prompt, want)
			}
		}
		// Must NOT be an instruction-style prompt for characters.
		for _, bad := range []string{"影视前期设定", "突出角色身份"} {
			if strings.Contains(prompt, bad) {
				t.Fatalf("character prompt should not contain instruction text %q", bad)
			}
		}
	})

	t.Run("scene asset", func(t *testing.T) {
		prompt := composeAssetImagePrompt("scene", "清晨浴室", "柔和灯光、镜面水汽、暖色氛围。", "", "")
		// Anime path uses English no-person tags
		if !strings.Contains(prompt, "no people") || !strings.Contains(prompt, "no humans") {
			t.Fatalf("scene prompt missing strict no-person constraint: %q", prompt)
		}
	})
}

func TestResolveAssetSuccessfulImageURL(t *testing.T) {
	t.Run("prefers selected generated image when present", func(t *testing.T) {
		metadata := map[string]interface{}{}
		raw, err := json.Marshal([]map[string]string{{
			"url":        "https://cdn.example.com/a.png",
			"model_name": "flux",
		}})
		if err != nil {
			t.Fatalf("marshal versions: %v", err)
		}
		var generatedImages interface{}
		if err := json.Unmarshal(raw, &generatedImages); err != nil {
			t.Fatalf("unmarshal versions: %v", err)
		}
		metadata["generated_images"] = generatedImages
		metadata["selected_generated_image_url"] = "https://cdn.example.com/a.png"

		got := resolveAssetSuccessfulImageURL(&model.Asset{}, metadata)
		if got != "https://cdn.example.com/a.png" {
			t.Fatalf("resolveAssetSuccessfulImageURL() = %q, want selected image", got)
		}
	})

	t.Run("falls back to primary image url", func(t *testing.T) {
		got := resolveAssetSuccessfulImageURL(&model.Asset{ImageURL: "https://cdn.example.com/primary.png"}, map[string]interface{}{})
		if got != "https://cdn.example.com/primary.png" {
			t.Fatalf("resolveAssetSuccessfulImageURL() = %q, want primary image", got)
		}
	})

	t.Run("falls back to first generated version when no selection", func(t *testing.T) {
		metadata := map[string]interface{}{}
		raw, err := json.Marshal([]map[string]string{{
			"url":        "https://cdn.example.com/b.png",
			"model_name": "dalle",
		}})
		if err != nil {
			t.Fatalf("marshal versions: %v", err)
		}
		var generatedImages interface{}
		if err := json.Unmarshal(raw, &generatedImages); err != nil {
			t.Fatalf("unmarshal versions: %v", err)
		}
		metadata["generated_images"] = generatedImages

		got := resolveAssetSuccessfulImageURL(&model.Asset{}, metadata)
		if got != "https://cdn.example.com/b.png" {
			t.Fatalf("resolveAssetSuccessfulImageURL() = %q, want generated image", got)
		}
	})
}
