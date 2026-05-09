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

const replicateBaseURL = "https://api.replicate.com/v1"
const fluxModel = "black-forest-labs/flux-1.1-pro"

type fluxGenerator struct {
	keys   *smartKeyPool
	client *http.Client
	logger *zap.Logger
}

// NewFluxGenerator —— 创建 Flux 图片生成器实例，返回 ImageGenerator 接口
func NewFluxGenerator(apiKeys []string, logger *zap.Logger) ImageGenerator {
	return &fluxGenerator{
		keys:   newSmartKeyPool(apiKeys),
		client: &http.Client{Timeout: 30 * time.Second},
		logger: logger,
	}
}

// Name —— 返回生成器名称 "flux"
func (g *fluxGenerator) Name() string { return "flux" }

// IsAvailable —— 检查 Flux 生成器是否可用（API Key 非空时返回 true）
func (g *fluxGenerator) IsAvailable(ctx context.Context) bool {
	return g.keys.size() > 0
}

// RefCapability —— Flux 通过单张 image 输入进行 IP-Adapter 风格迁移。
func (g *fluxGenerator) RefCapability() RefCapability {
	return RefCapability{Mode: RefModeIPAdapter, MaxRefs: 1, StrongRef: false}
}

// Generate —— 通过 Replicate API 提交 Flux 图片生成任务并轮询结果，返回生成结果
func (g *fluxGenerator) Generate(ctx context.Context, req GenerateReq) (*GenerateRes, error) {
	apiKey := g.keys.nextKey()
	if apiKey == "" {
		return nil, fmt.Errorf("flux: no api key configured")
	}

	input := map[string]interface{}{
		"prompt":           buildFluxPrompt(req), // style-aware prompt
		"width":            req.Width,
		"height":           req.Height,
		"num_outputs":      1,
		"output_format":    "webp",
		"output_quality":   90,
		"safety_tolerance": 5,
	}
	if req.Seed != -1 {
		input["seed"] = req.Seed
	}
	// Inject reference image for visual consistency when a character or scene asset is available.
	// flux-1.1-pro accepts an "image" URL input for img2img conditioning, and "image_strength"
	// (0.0 = fully follow reference, 1.0 = fully follow prompt). We default to 0.85 so the
	// prompt drives the scene composition while the reference constrains character appearance.
	if refURL := req.AllReferenceImageURLs(); len(refURL) > 0 && strings.TrimSpace(refURL[0]) != "" {
		input["image"] = strings.TrimSpace(refURL[0])
		input["image_strength"] = 0.85
	} else if strings.TrimSpace(req.StyleReferenceURL) != "" {
		input["image"] = strings.TrimSpace(req.StyleReferenceURL)
		input["image_strength"] = 0.85
	}

	// Replicate new-style API for public models: POST /v1/models/{owner}/{name}/predictions
	// with just {"input": {...}}. Using "version" with a model slug is incorrect — that field
	// expects a version SHA hash. The model-scoped endpoint is the correct approach for flux-1.1-pro.
	body, _ := json.Marshal(map[string]interface{}{
		"input": input,
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		replicateBaseURL+"/models/"+fluxModel+"/predictions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("flux: build request: %w", err)
	}
	g.setHeaders(httpReq, apiKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("flux: create prediction: %w", err)
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	// Retry on rate limit (429) with jitter to avoid thundering herd.
	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		g.keys.ReportFailure(apiKey, true)
		baseWaits := []time.Duration{15 * time.Second, 30 * time.Second, 60 * time.Second, 90 * time.Second}
		for _, baseWait := range baseWaits {
			jitter := time.Duration(rand.Intn(8000)) * time.Millisecond
			wait := baseWait + jitter
			g.logger.Warn("flux: rate limited, retrying", zap.Duration("wait", wait))
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("flux: context cancelled during rate limit backoff")
			case <-time.After(wait):
			}
			retryReq, _ := http.NewRequestWithContext(ctx, http.MethodPost,
				replicateBaseURL+"/predictions", bytes.NewReader(body))
			g.setHeaders(retryReq, apiKey)
			resp, err = g.client.Do(retryReq)
			if err != nil {
				return nil, fmt.Errorf("flux: create prediction: %w", err)
			}
			if resp.StatusCode == http.StatusTooManyRequests {
				resp.Body.Close()
				g.keys.ReportFailure(apiKey, true)
				continue
			}
			break
		}
	}

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("flux: unexpected status %d: %s", resp.StatusCode, b)
	}

	var prediction struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prediction); err != nil {
		return nil, fmt.Errorf("flux: decode prediction: %w", err)
	}

	imageURL, seed, err := g.pollPrediction(ctx, prediction.ID, apiKey)
	if err != nil {
		g.keys.ReportFailure(apiKey, false)
		return nil, err
	}

	g.keys.ReportSuccess(apiKey)
	return &GenerateRes{
		ImageURL:  imageURL,
		Width:     req.Width,
		Height:    req.Height,
		Seed:      seed,
		ModelUsed: "flux",
	}, nil
}

// pollPrediction —— 轮询 Replicate 预测任务状态，成功时返回图片 URL 和 seed
func (g *fluxGenerator) pollPrediction(ctx context.Context, predID string, apiKey string) (string, int64, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	url := fmt.Sprintf("%s/predictions/%s", replicateBaseURL, predID)

	for {
		select {
		case <-ctx.Done():
			return "", 0, fmt.Errorf("flux: polling timed out for prediction %s", predID)
		case <-ticker.C:
			httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				continue
			}
			g.setHeaders(httpReq, apiKey)

			resp, err := g.client.Do(httpReq)
			if err != nil {
				g.logger.Warn("flux: poll error", zap.String("pred_id", predID), zap.Error(err))
				continue
			}

			var result struct {
				Status  string   `json:"status"`
				Output  []string `json:"output"`
				Error   string   `json:"error"`
				Metrics struct {
					Seed int64 `json:"seed"`
				} `json:"metrics"`
			}
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			_ = json.Unmarshal(data, &result)

			switch result.Status {
			case "succeeded":
				if len(result.Output) == 0 {
					return "", 0, fmt.Errorf("flux: succeeded but no output")
				}
				return result.Output[0], result.Metrics.Seed, nil
			case "failed", "canceled":
				return "", 0, fmt.Errorf("flux: prediction %s — %s", result.Status, result.Error)
			}
		}
	}
}

// setHeaders —— 为 HTTP 请求设置 Replicate API 认证和内容类型头
func (g *fluxGenerator) setHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Token "+apiKey)
	req.Header.Set("Content-Type", "application/json")
}

// buildFluxPrompt constructs a Flux-optimised prompt.
//
// Flux follows the same structured natural-language family as GPT-image, but it
// is especially sensitive to prompt bloat.  The shared builder keeps the content
// explicit while avoiding repeated style synonyms and preserving a clear order:
// quality → style → subject/scene → composition/camera → lighting → constraints.
func buildFluxPrompt(req GenerateReq) string {
	return buildNaturalLanguagePrompt(req)
}
