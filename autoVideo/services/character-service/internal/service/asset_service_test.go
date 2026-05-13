package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/autovideo/character-service/internal/model"
	"go.uber.org/zap"
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
			"strict model-sheet consistency",
			"costume layers, accessories, hairstyle silhouette, and footwear clearly readable",
			"neutral presentation pose, orthographic design-sheet readability",
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
		for _, want := range []string{
			"no people",
			"no humans",
			"location design sheet",
			"建筑结构、陈设关系、材质与光源方向必须可读",
			"no silhouette",
		} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("scene prompt %q does not contain %q", prompt, want)
			}
		}
	})

	t.Run("prop asset", func(t *testing.T) {
		prompt := composeAssetImagePrompt("prop", "青铜匕首", "青铜刀身，木质握柄，边缘有磨损。", "强调纹饰", "")
		for _, want := range []string{
			"prop design sheet",
			"青铜匕首",
			"强调纹饰",
			"prop turnaround reference",
			"强调正三维轮廓、连接结构、接缝、刻纹、边缘磨损与功能构造可读性",
			"front or three-quarter readable presentation",
		} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("prop prompt %q does not contain %q", prompt, want)
			}
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

func TestResolveChatFreeRouteUsesRuntimeModelMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":[{"model_key":"gpt-5.4","name":"GPT 5.4","provider":"openai","api_endpoint":"https://precision.example/v1","api_key_ref":"runtime.character.llm","is_active":true}]}`))
	}))
	defer server.Close()

	svc := NewAssetService(
		nil, nil, zap.NewNop(),
		"https://fallback.example/v1", "fallback-key", "gpt-4.1", "", time.Second,
		"https://claude.example/v1", "claude-key",
		"https://qwen.example/v1", "qwen-key",
		"https://zhipu.example/v1", "zhipu-key",
		"https://gemini.example/v1", "gemini-key",
		server.URL,
	)

	baseURL, apiKey := svc.resolveChatFreeRoute(context.Background(), "gpt-5.4")
	if baseURL != "https://precision.example/v1" {
		t.Fatalf("resolveChatFreeRoute() baseURL = %q, want precision endpoint", baseURL)
	}
	if apiKey != "fallback-key" {
		t.Fatalf("resolveChatFreeRoute() apiKey = %q, want default runtime key", apiKey)
	}
}

func TestResolveChatFreeRoutePrefersEndpointProviderOverEasyartRef(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":[{"model_key":"glm-5","name":"GLM-5","provider":"zhipu","api_endpoint":"https://open.bigmodel.cn/api/paas/v4/chat/completions","api_key_ref":"easyart","is_active":true}]}`))
	}))
	defer server.Close()

	svc := NewAssetService(
		nil, nil, zap.NewNop(),
		"https://fallback.example/v1", "fallback-key", "gpt-4.1", "", time.Second,
		"https://claude.example/v1", "easyart-key",
		"https://qwen.example/v1", "qwen-key",
		"https://zhipu.example/v1", "zhipu-key",
		"https://gemini.example/v1", "gemini-key",
		server.URL,
	)

	baseURL, apiKey := svc.resolveChatFreeRoute(context.Background(), "glm-5")
	if baseURL != "https://open.bigmodel.cn/api/paas/v4/chat/completions" {
		t.Fatalf("resolveChatFreeRoute() baseURL = %q, want runtime endpoint", baseURL)
	}
	if apiKey != "zhipu-key" {
		t.Fatalf("resolveChatFreeRoute() apiKey = %q, want endpoint-matched zhipu key", apiKey)
	}
}

func TestResolveChatFreeRoutePrefersGeminiFamilyOverGenericProxyEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":[{"model_key":"gemini-3.1-pro-preview","name":"gemini-3.1-pro-preview","provider":"google","api_endpoint":"https://poloai.top/v1/chat/completions","api_key_ref":"easyart","is_active":true}]}`))
	}))
	defer server.Close()

	svc := NewAssetService(
		nil, nil, zap.NewNop(),
		"https://fallback.example/v1", "fallback-key", "gpt-4.1", "", time.Second,
		"https://claude.example/v1", "easyart-key",
		"https://qwen.example/v1", "qwen-key",
		"https://zhipu.example/v1", "zhipu-key",
		"https://gemini.example/v1", "gemini-key",
		server.URL,
	)

	baseURL, apiKey := svc.resolveChatFreeRoute(context.Background(), "gemini-3.1-pro-preview")
	if baseURL != "https://gemini.example/v1" {
		t.Fatalf("resolveChatFreeRoute() baseURL = %q, want gemini family endpoint", baseURL)
	}
	if apiKey != "gemini-key" {
		t.Fatalf("resolveChatFreeRoute() apiKey = %q, want gemini family key", apiKey)
	}
}

func TestResolveChatFreeRouteFallsBackToFamilyRouting(t *testing.T) {
	svc := NewAssetService(
		nil, nil, zap.NewNop(),
		"https://fallback.example/v1", "fallback-key", "gpt-4.1", "", time.Second,
		"https://claude.example/v1", "claude-key",
		"https://qwen.example/v1", "qwen-key",
		"https://zhipu.example/v1", "zhipu-key",
		"https://gemini.example/v1", "gemini-key",
		"",
	)

	baseURL, apiKey := svc.resolveChatFreeRoute(context.Background(), "claude-3-7-sonnet")
	if baseURL != "https://claude.example/v1" {
		t.Fatalf("resolveChatFreeRoute() baseURL = %q, want claude endpoint", baseURL)
	}
	if apiKey != "claude-key" {
		t.Fatalf("resolveChatFreeRoute() apiKey = %q, want claude key", apiKey)
	}
}

func TestChatFreeUsesFullRuntimeEndpointWithoutDuplicatingPath(t *testing.T) {
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected llm path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Gemini。"}}]}`))
	}))
	defer llmServer.Close()

	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"code":0,"message":"success","data":[{"model_key":"gemini-2.5-flash","name":"gemini-2.5-flash","provider":"google","api_endpoint":"%s/v1/chat/completions","api_key_ref":"easyart","is_active":true}]}`,
			llmServer.URL,
		)))
	}))
	defer modelServer.Close()

	svc := NewAssetService(
		nil, nil, zap.NewNop(),
		"https://fallback.example/v1", "fallback-key", "gpt-4.1", "", time.Second,
		"https://claude.example/v1", "claude-key",
		"https://qwen.example/v1", "qwen-key",
		"https://zhipu.example/v1", "zhipu-key",
		"https://gemini.example/v1", "gemini-key",
		modelServer.URL,
	)

	reply, err := svc.ChatFree(context.Background(), []map[string]string{{
		"role":    "user",
		"content": "test",
	}}, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("ChatFree() error = %v", err)
	}
	if reply != "Gemini。" {
		t.Fatalf("ChatFree() reply = %q, want Gemini。", reply)
	}
}
