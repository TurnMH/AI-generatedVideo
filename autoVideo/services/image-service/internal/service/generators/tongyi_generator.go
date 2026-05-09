package generators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

const defaultDashScopeBase = "https://dashscope.aliyuncs.com"

type tongyiGenerator struct {
	keys          *smartKeyPool
	dashScopeBase string
	modelName     string // configurable model, e.g. "wanx-v1", "wanx2.1-t2i-turbo"
	generatorKey  string // key used to identify this generator
	client        *http.Client
	logger        *zap.Logger
}

// NewTongyiGenerator —— 创建通义万相图片生成器实例，返回 ImageGenerator 接口
func NewTongyiGenerator(apiKeys []string, dashScopeBase string, logger *zap.Logger) ImageGenerator {
	return NewTongyiGeneratorForModel(apiKeys, dashScopeBase, "wanx-v1", "tongyi", logger)
}

// NewTongyiGeneratorForModel —— 创建指定模型名称的通义万相图片生成器
func NewTongyiGeneratorForModel(apiKeys []string, dashScopeBase, modelName, generatorKey string, logger *zap.Logger) ImageGenerator {
	if dashScopeBase == "" {
		dashScopeBase = defaultDashScopeBase
	}
	return &tongyiGenerator{
		keys:          newSmartKeyPool(apiKeys),
		dashScopeBase: dashScopeBase,
		modelName:     modelName,
		generatorKey:  generatorKey,
		client:        &http.Client{Timeout: 120 * time.Second},
		logger:        logger,
	}
}

// Name —— 返回生成器名称
func (g *tongyiGenerator) Name() string { return g.generatorKey }

// IsAvailable —— 检查通义生成器是否可用（API Key 非空时返回 true）
func (g *tongyiGenerator) IsAvailable(ctx context.Context) bool {
	return g.keys.size() > 0
}

// RefCapability —— 通义万相参考图能力：
// wanx2.1-i2i / wan2.5-i2i 走图生图（强依赖），wanx-v1 走 IP-Adapter，wanx2.1-t2i-* 纯文生图。
func (g *tongyiGenerator) RefCapability() RefCapability {
	if strings.Contains(g.modelName, "i2i") {
		return RefCapability{Mode: RefModeI2I, MaxRefs: 1, StrongRef: true}
	}
	if g.modelName == "wanx-v1" {
		return RefCapability{Mode: RefModeIPAdapter, MaxRefs: 1, StrongRef: false}
	}
	// wanx2.1-t2i-* 及其他 t2i 模型不支持参考图
	return RefCapability{Mode: RefModeT2I, MaxRefs: 0, StrongRef: false}
}

// Generate —— 调用通义万相异步 API 提交图片生成任务并轮询结果，返回生成结果
func (g *tongyiGenerator) Generate(ctx context.Context, req GenerateReq) (*GenerateRes, error) {
	apiKey := g.keys.nextKey()
	if apiKey == "" {
		return nil, fmt.Errorf("tongyi: no api key configured")
	}

	size := fmt.Sprintf("%d*%d", req.Width, req.Height)

	// wanx-v1 supports the DashScope "style" parameter.
	// wanx2.1-* models do not — style guidance must be embedded in the prompt.
	supportsStyleParam := g.modelName == "wanx-v1"

	promptText := buildTongyiPrompt(req, supportsStyleParam)

	// wanx-v1 understands English; use the raw English negative prompt.
	// wanx2.1-* are Chinese-first models — use fully Chinese negative instruction
	// derived from the style preset for strongest effect.
	negativePrompt := req.NegativePrompt
	if !supportsStyleParam {
		negativePrompt = ChineseLangNegativeByStyle(req.StylePreset)
	}

	// wan2.5-i2i-preview is image-to-image: requires ref_img URL.
	// Gracefully fall back to text-to-image mode when no reference image is available
	// (e.g. character assets haven't been generated yet for this project) so that
	// storyboard generation can still proceed rather than failing silently.
	isI2I := strings.Contains(g.modelName, "i2i")
	// effectiveModel is the model name sent to the API. When i2i model has no ref image,
	// fall back to a text-to-image model so generation can still proceed.
	effectiveModel := g.modelName
	if isI2I && req.StyleReferenceURL == "" {
		// No reference image available — fall back to text-to-image mode so generation
		// can still proceed (e.g. first-pass storyboard before character images exist).
		g.logger.Warn("tongyi: i2i model has no reference image, falling back to text-to-image mode",
			zap.String("model", g.modelName),
		)
		isI2I = false
		effectiveModel = "wanx2.1-t2i-plus"
	}

	params := map[string]interface{}{
		"size": size,
		"n":    1,
	}
	if supportsStyleParam {
		params["style"] = tongyiStyle(req.StylePreset)
	}

	inputFields := map[string]interface{}{
		"prompt":          promptText,
		"negative_prompt": negativePrompt,
	}
	if isI2I {
		// wanx2.1-i2i accepts a single ref_img URL.  Use the primary character reference
		// (StyleReferenceURL) which is always the first listed character's generated image.
		inputFields["ref_img"] = req.StyleReferenceURL
		// ref_strength controls how closely the model follows the reference image.
		// 0.85 gives strong character consistency while still allowing creative composition.
		params["ref_strength"] = 0.85
		// When multiple characters exist, embed the extra references as an explicit
		// multi-character roster in the prompt so the model knows the additional people
		// that appear in the panel even though the API only accepts one image ref.
		allRefs := req.AllReferenceImageURLs()
		if len(allRefs) > 1 {
			// The extra ref URLs themselves aren't useful as prompt text, but the fact that
			// there are N characters is — the CHARACTER LOCK section already carries the
			// textual descriptions; just reinforce the count and primary reference here.
			extraCount := len(allRefs) - 1
			if extraCount == 1 {
				inputFields["prompt"] = fmt.Sprintf("%s\n[NOTE: This scene contains %d additional character(s) besides the primary reference image. Maintain consistent appearance for ALL listed characters per CHARACTER LOCK description above.]", promptText, extraCount)
			}
		}
	} else if supportsStyleParam {
		// wanx-v1 (T2I): supports IP-Adapter for subject/style reference transfer.
		// Pass the primary character reference via ip_adapter_image_url so the model
		// can anchor character appearance even without explicit i2i conditioning.
		primaryRef := req.StyleReferenceURL
		if primaryRef == "" && len(req.ReferenceImageURLs) > 0 {
			primaryRef = req.ReferenceImageURLs[0]
		}
		if primaryRef != "" {
			inputFields["ip_adapter_image_url"] = primaryRef
			// Scale 0.6: moderate influence — preserves subject likeness while allowing
			// compositional freedom for the scene layout.
			params["ip_adapter_scale"] = 0.6
		}
	}
	// wanx2.1-t2i models (non-i2i, non-wanx-v1) do not support reference images via API.
	// Character consistency relies entirely on the textual CHARACTER LOCK section in the prompt.

	payload := map[string]interface{}{
		"model":      effectiveModel,
		"input":      inputFields,
		"parameters": params,
	}
	// Tongyi only accepts seed in [0, 4294967290]
	if req.Seed >= 0 && req.Seed <= 4294967290 {
		payload["parameters"].(map[string]interface{})["seed"] = req.Seed
	}
	body, _ := json.Marshal(payload)

	submitURL := g.dashScopeBase + "/services/aigc/text2image/image-synthesis"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, submitURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("tongyi: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-DashScope-Async", "enable")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("tongyi: submit request: %w", err)
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	// Retry submit on rate limit (429) with exponential backoff + jitter to
	// prevent thundering-herd when concurrent workers hit the limit together.
	// 5 attempts: base waits 10/20/30/60/90s + up to 8s random jitter.
	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		g.keys.ReportFailure(apiKey, true)
		baseWaits := []time.Duration{10 * time.Second, 20 * time.Second, 30 * time.Second, 60 * time.Second, 90 * time.Second}
		for attempt, baseWait := range baseWaits {
			jitter := time.Duration(rand.Intn(8000)) * time.Millisecond
			wait := baseWait + jitter
			g.logger.Warn("tongyi: rate limited, retrying", zap.Int("attempt", attempt+1), zap.Duration("wait", wait))
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("tongyi: context cancelled during rate limit backoff")
			case <-time.After(wait):
			}
			httpReq2, _ := http.NewRequestWithContext(ctx, http.MethodPost, submitURL, bytes.NewReader(body))
			httpReq2.Header.Set("Authorization", "Bearer "+apiKey)
			httpReq2.Header.Set("Content-Type", "application/json")
			httpReq2.Header.Set("X-DashScope-Async", "enable")
			resp, err = g.client.Do(httpReq2)
			if err != nil {
				return nil, fmt.Errorf("tongyi: submit request: %w", err)
			}
			if resp.StatusCode == http.StatusTooManyRequests {
				resp.Body.Close()
				g.keys.ReportFailure(apiKey, true)
				continue
			}
			break
		}
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("tongyi: unexpected status %d: %s", resp.StatusCode, b)
	}

	var submitResp struct {
		Output struct {
			TaskID     string `json:"task_id"`
			TaskStatus string `json:"task_status"`
		} `json:"output"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&submitResp); err != nil {
		return nil, fmt.Errorf("tongyi: decode submit response: %w", err)
	}

	imageURL, err := g.pollTask(ctx, submitResp.Output.TaskID, apiKey)
	if err != nil {
		g.keys.ReportFailure(apiKey, false)
		return nil, err
	}

	g.keys.ReportSuccess(apiKey)
	return &GenerateRes{
		ImageURL:  imageURL,
		Width:     req.Width,
		Height:    req.Height,
		Seed:      req.Seed,
		ModelUsed: g.generatorKey,
	}, nil
}

// pollTask —— 轮询通义万相任务状态，成功时返回生成图片的 URL
func (g *tongyiGenerator) pollTask(ctx context.Context, taskID string, apiKey string) (string, error) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	url := fmt.Sprintf("%s/tasks/%s", g.dashScopeBase, taskID)

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("tongyi: polling timed out for task %s", taskID)
		case <-ticker.C:
			httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				continue
			}
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)

			resp, err := g.client.Do(httpReq)
			if err != nil {
				g.logger.Warn("tongyi: poll error", zap.String("task_id", taskID), zap.Error(err))
				continue
			}

			var result struct {
				Output struct {
					TaskStatus string `json:"task_status"`
					Results    []struct {
						URL string `json:"url"`
					} `json:"results"`
				} `json:"output"`
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			_ = json.Unmarshal(data, &result)

			switch result.Output.TaskStatus {
			case "SUCCEEDED":
				if len(result.Output.Results) == 0 {
					return "", fmt.Errorf("tongyi: succeeded but no results")
				}
				return result.Output.Results[0].URL, nil
			case "FAILED":
				return "", fmt.Errorf("tongyi: task failed: %s %s", result.Code, result.Message)
			}
		}
	}
}

// tongyiStyle —— 将风格预设名称映射为通义万相 wanx-v1 的风格标签，返回对应标签字符串。
// 仅适用于 wanx-v1 模型；wanx2.1-* 系列不支持此参数。
func tongyiStyle(preset string) string {
	styles := map[string]string{
		"anime":             "<anime>",
		"anime-2d":          "<anime>",
		"anime-3d":          "<3d cartoon>",
		"realistic":         "<photography>",
		"live-action-film":  "<photography>",
		"live-action-short": "<photography>",
		"watercolor":        "<watercolor>",
		"sketch":            "<sketch>",
		"oil":               "<oil painting>",
	}
	if s, ok := styles[preset]; ok {
		return s
	}
	return "<anime>"
}

// buildTongyiPrompt builds a Tongyi/Wanx-optimised prompt.
//
// Both wanx-v1 and wanx2.1-* are Chinese-first models. We therefore use the
// shared Chinese structured prompt builder:
//   - wanx-v1: style is partly handled by the API style param, so the prompt omits
//     duplicated style prose and focuses on subject / composition / camera / quality.
//   - wanx2.1-* : full Chinese style context is embedded directly in the prompt.
func buildTongyiPrompt(req GenerateReq, wanxV1 bool) string {
	return buildChineseStructuredPrompt(req, wanxV1)
}

// tongyiStyleContext returns a Chinese-language style description for wanx2.1 models.
// Kept as a thin alias to the shared Chinese style context for backward compatibility.
func tongyiStyleContext(stylePreset string) string {
	return ChineseLangStyleContext(stylePreset)
}
