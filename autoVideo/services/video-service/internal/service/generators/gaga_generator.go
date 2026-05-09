package generators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GagaGenerator wraps the Gaga video generation API.
// Endpoint: POST/GET {GagaBase}/v1/generations
type GagaGenerator struct {
	APIKey  string
	BaseURL string // e.g. "https://api.gaga.art"
	Model   string // e.g. "gaga-1"
	client  *http.Client
}

// NewGagaGenerator —— 创建 Gaga 视频生成器实例
func NewGagaGenerator(apiKey, baseURL, model string) *GagaGenerator {
	if baseURL == "" {
		baseURL = "https://api.gaga.art"
	}
	if model == "" {
		model = "gaga-1"
	}
	return &GagaGenerator{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Name —— 返回生成器名称
func (g *GagaGenerator) Name() string { return "gaga" }

// IsAvailable —— 检查 API Key 是否已配置
func (g *GagaGenerator) IsAvailable(_ context.Context) bool { return g.APIKey != "" }

// SupportsNativeAudio —— Gaga 不支持原生音频嵌入
func (g *GagaGenerator) SupportsNativeAudio() bool { return false }

// ParamOptions —— Gaga 支持的模型参数（时长）
func (g *GagaGenerator) ParamOptions() []ModelParamOption {
	return []ModelParamOption{
		{
			Key: "duration", Label: "时长", Default: "5",
			Values: []ParamValue{{Value: "3", Label: "3秒"}, {Value: "5", Label: "5秒"}, {Value: "10", Label: "10秒"}},
		},
	}
}

// ── API types ────────────────────────────────────────────────

type gagaSubmitReq struct {
	Model    string `json:"model"`
	ImageURL string `json:"image_url,omitempty"`
	Prompt   string `json:"prompt"`
	Duration int    `json:"duration"`
}

type gagaGenerationResp struct {
	ID       string `json:"id"`
	Status   string `json:"status"`   // pending / completed / failed
	VideoURL string `json:"video_url"`
	Error    *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ── Generate ────────────────────────────────────────────────

// Generate —— 提交 Gaga 视频任务并轮询等待完成，返回 *VideoClip
func (g *GagaGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	var genID string
	err := RetrySubmit(ctx, 4, func() error {
		var e error
		genID, e = g.submit(ctx, req)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("gaga submit: %w", err)
	}
	clip, err := g.poll(ctx, genID)
	if err != nil {
		return nil, fmt.Errorf("gaga poll %s: %w", genID, err)
	}
	return clip, nil
}

// submit —— 提交视频生成任务，返回生成 ID
func (g *GagaGenerator) submit(ctx context.Context, req VideoGenerateReq) (string, error) {
	dur := int(req.DurationSec)
	if dur <= 0 {
		dur = 5
	}

	body := gagaSubmitReq{
		Model:    g.Model,
		ImageURL: req.SourceImageURL,
		Prompt:   req.Prompt,
		Duration: dur,
	}
	b, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.BaseURL+"/v1/generations", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.APIKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 429 {
		return "", fmt.Errorf("gaga rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("gaga HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result gagaGenerationResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse submit response: %w body=%s", err, string(respBody))
	}
	if result.Error != nil && result.Error.Message != "" {
		return "", fmt.Errorf("gaga: %s", result.Error.Message)
	}
	if result.ID == "" {
		return "", fmt.Errorf("gaga: no id in response: %s", string(respBody))
	}
	return result.ID, nil
}

// poll —— 轮询任务状态直到完成、失败或超时（15分钟）
func (g *GagaGenerator) poll(ctx context.Context, genID string) (*VideoClip, error) {
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("gaga: generation %s timed out after 15min", genID)
		case <-ticker.C:
			clip, done, err := g.queryTask(ctx, genID)
			if err != nil {
				return nil, err
			}
			if done {
				return clip, nil
			}
		}
	}
}

// queryTask —— 查询单次任务状态
func (g *GagaGenerator) queryTask(ctx context.Context, genID string) (*VideoClip, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		g.BaseURL+"/v1/generations/"+genID, nil)
	if err != nil {
		return nil, false, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.APIKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, false, nil // transient
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result gagaGenerationResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, false, nil
	}

	switch result.Status {
	case "completed":
		if result.VideoURL == "" {
			return nil, false, fmt.Errorf("gaga: completed but no video_url")
		}
		return &VideoClip{
			ClipURL:     result.VideoURL,
			DurationSec: 5,
			ModelUsed:   g.Name(),
		}, true, nil
	case "failed":
		msg := "task failed"
		if result.Error != nil && result.Error.Message != "" {
			msg = result.Error.Message
		}
		return nil, false, fmt.Errorf("gaga: %s", msg)
	default: // pending, processing
		return nil, false, nil
	}
}
