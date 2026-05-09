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

// HubagiGenerator wraps the hubagi.cn video generation API (image→video).
// Supports models: TC-GV (hubagi), voe3.1 (veo), etc.
type HubagiGenerator struct {
	APIKey  string
	BaseURL string // e.g. "https://hubagi.cn/api/v1"
	Model   string // e.g. "TC-GV" or "voe3.1"
	client  *http.Client
}

// NewHubagiGenerator —— 创建 Hubagi 视频生成器实例
func NewHubagiGenerator(apiKey, baseURL, model string) *HubagiGenerator {
	if baseURL == "" {
		baseURL = "https://hubagi.cn/api/v1"
	}
	if model == "" {
		model = "TC-GV"
	}
	return &HubagiGenerator{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Name —— 返回生成器名称，格式为 "hubagi-{模型名}"
func (g *HubagiGenerator) Name() string { return "hubagi-" + g.Model }

// IsAvailable —— 检查 API Key 是否已配置
func (g *HubagiGenerator) IsAvailable(ctx context.Context) bool {
	return g.APIKey != ""
}

// SupportsNativeAudio —— Hubagi does not embed audio in generated clips.
func (g *HubagiGenerator) SupportsNativeAudio() bool { return false }

// ParamOptions —— Hubagi 支持的模型参数（时长）
func (g *HubagiGenerator) ParamOptions() []ModelParamOption {
	return []ModelParamOption{
		{
			Key: "duration", Label: "时长", Default: "5",
			Values: []ParamValue{{Value: "3", Label: "3秒"}, {Value: "5", Label: "5秒"}, {Value: "8", Label: "8秒"}},
		},
	}
}

type hubagiSubmitReq struct {
	Model    string `json:"model"`
	ImageURL string `json:"image_url,omitempty"`
	Prompt   string `json:"prompt"`
	Duration int    `json:"duration,omitempty"`
}

type hubagiSubmitResp struct {
	ID     json.Number `json:"id"`     // API returns numeric snowflake ID, not a string
	TaskID json.Number `json:"task_id"`
	Status string      `json:"status"` // pending, processing, completed, failed
	State  string      `json:"state"`
	Error  string      `json:"error,omitempty"`
}

type hubagiPollResp struct {
	ID        json.Number `json:"id"`
	TaskID    json.Number `json:"task_id"`
	Status    string `json:"status"`
	State     string `json:"state"`
	ResultURL string `json:"result_url"`
	Creations []struct {
		URL string `json:"url"`
	} `json:"creations"`
	Error string `json:"error,omitempty"`
}

// ── Generate ────────────────────────────────────────────────

// Generate —— 提交图生视频任务并轮询等待完成，返回 *VideoClip
func (g *HubagiGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	var taskID string
	err := RetrySubmit(ctx, 4, func() error {
		var e error
		taskID, e = g.submit(ctx, req)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("hubagi submit: %w", err)
	}
	clip, err := g.poll(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("hubagi poll: %w", err)
	}
	return clip, nil
}

// submit —— 向 Hubagi API 提交视频生成请求，返回任务 ID
func (g *HubagiGenerator) submit(ctx context.Context, req VideoGenerateReq) (string, error) {
	dur := int(req.DurationSec)
	if dur <= 0 {
		dur = 5
	}

	body := hubagiSubmitReq{
		Model:    g.Model,
		ImageURL: req.SourceImageURL,
		Prompt:   req.Prompt,
		Duration: dur,
	}
	b, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.BaseURL+"/video/generations", bytes.NewReader(b))
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
		return "", fmt.Errorf("hubagi rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("hubagi HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result hubagiSubmitResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse submit response: %w body=%s", err, string(respBody))
	}
	if result.Error != "" {
		return "", fmt.Errorf("hubagi error: %s", result.Error)
	}

	id := result.ID.String()
	if id == "" {
		id = result.TaskID.String()
	}
	if id == "" {
		return "", fmt.Errorf("hubagi: no task id in response: %s", string(respBody))
	}
	return id, nil
}

// poll —— 轮询任务状态直到完成、失败或超时（10分钟）
func (g *HubagiGenerator) poll(ctx context.Context, taskID string) (*VideoClip, error) {
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	timeout := time.After(10 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("hubagi: task %s timed out after 10min", taskID)
		case <-ticker.C:
			clip, done, err := g.queryTask(ctx, taskID)
			if err != nil {
				return nil, err
			}
			if done {
				return clip, nil
			}
		}
	}
}

// queryTask —— 查询单次任务状态，返回结果和是否完成
func (g *HubagiGenerator) queryTask(ctx context.Context, taskID string) (*VideoClip, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		g.BaseURL+"/video/generations/"+taskID, nil)
	if err != nil {
		return nil, false, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.APIKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, false, nil // transient, retry
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result hubagiPollResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, false, nil // transient
	}

	switch result.Status {
	case "completed":
		videoURL := result.ResultURL
		if videoURL == "" && len(result.Creations) > 0 {
			videoURL = result.Creations[0].URL
		}
		if videoURL == "" {
			return nil, false, fmt.Errorf("hubagi: completed but no video url")
		}
		return &VideoClip{
			ClipURL:     videoURL,
			DurationSec: float64(5),
			ModelUsed:   g.Name(),
		}, true, nil
	case "failed":
		msg := "task failed"
		if result.Error != "" {
			msg = result.Error
		}
		return nil, false, fmt.Errorf("hubagi: %s", msg)
	default: // pending, processing, created
		return nil, false, nil
	}
}
