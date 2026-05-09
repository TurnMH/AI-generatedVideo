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

// Sora2Generator wraps the OpenAI-compatible Sora-2 video generation API (image→video).
type Sora2Generator struct {
	APIKey  string
	BaseURL string // e.g. "https://api.easyart.cc/v1"
	client  *http.Client
}

// NewSora2Generator —— 创建 Sora2 视频生成器实例
func NewSora2Generator(apiKey, baseURL string) *Sora2Generator {
	if baseURL == "" {
		baseURL = "https://api.easyart.cc/v1"
	}
	return &Sora2Generator{
		APIKey:  apiKey,
		BaseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Name —— 返回生成器名称 "sora2"
func (g *Sora2Generator) Name() string { return "sora2" }

// IsAvailable —— 检查 API Key 是否已配置
func (g *Sora2Generator) IsAvailable(ctx context.Context) bool {
	return g.APIKey != ""
}

// SupportsNativeAudio —— Sora2 does not embed audio in generated clips.
func (g *Sora2Generator) SupportsNativeAudio() bool { return false }

// ParamOptions —— Sora2 支持的模型参数（时长、宽高比、分辨率、生成数量）
func (g *Sora2Generator) ParamOptions() []ModelParamOption {
	return []ModelParamOption{
		{
			Key: "duration", Label: "时长", Default: "10",
			Values: []ParamValue{{Value: "5", Label: "5秒"}, {Value: "10", Label: "10秒"}, {Value: "20", Label: "20秒"}},
		},
		{
			Key: "aspect_ratio", Label: "画面比例", Default: "16:9",
			Values: []ParamValue{
				{Value: "16:9", Label: "横屏 16:9"}, {Value: "9:16", Label: "竖屏 9:16"}, {Value: "1:1", Label: "方形 1:1"},
			},
		},
		{
			Key: "resolution", Label: "分辨率", Default: "1080p",
			Values: []ParamValue{
				{Value: "480p", Label: "480p"}, {Value: "720p", Label: "720p"}, {Value: "1080p", Label: "1080p"},
			},
		},
		{
			Key: "count", Label: "生成数量", Default: "1",
			Values: []ParamValue{{Value: "1", Label: "1个"}, {Value: "2", Label: "2个"}, {Value: "4", Label: "4个"}},
		},
	}
}

type sora2SubmitReq struct {
	Model       string `json:"model"`
	ImageURL    string `json:"image_url,omitempty"`
	Prompt      string `json:"prompt"`
	N           int    `json:"n"`
	Duration    int    `json:"duration,omitempty"`
	AspectRatio string `json:"aspect_ratio,omitempty"` // "16:9" "9:16" "1:1"
	Resolution  string `json:"resolution,omitempty"`   // "480p" "720p" "1080p"
}

type sora2SubmitResp struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`
	Status string `json:"status"` // queued, in_progress, completed, failed
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type sora2PollResp struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	VideoURL string `json:"video_url"`
	Error    *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// ── Generate ────────────────────────────────────────────────

// Generate —— 提交视频生成任务并轮询等待完成，返回 *VideoClip
func (g *Sora2Generator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	var taskID string
	err := RetrySubmit(ctx, 4, func() error {
		var e error
		taskID, e = g.submit(ctx, req)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("sora2 submit: %w", err)
	}
	clip, err := g.poll(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("sora2 poll: %w", err)
	}
	return clip, nil
}

// submit —— 向 Sora2 API 提交视频生成请求，返回任务 ID
func (g *Sora2Generator) submit(ctx context.Context, req VideoGenerateReq) (string, error) {
	dur := int(req.DurationSec)
	if dur <= 0 {
		dur = 10
	}
	n := req.Count
	if n <= 0 {
		n = 1
	}

	body := sora2SubmitReq{
		Model:       "sora-2",
		ImageURL:    req.SourceImageURL,
		Prompt:      req.Prompt,
		N:           n,
		Duration:    dur,
		AspectRatio: req.AspectRatio,                          // omitted when empty → API default
		Resolution:  firstNonEmpty(req.Resolution, "1080p"),
	}
	b, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.BaseURL+"/videos", bytes.NewReader(b))
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
		return "", fmt.Errorf("sora2 rate limited (429)")
	}

	var result sora2SubmitResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse submit response: %w body=%s", err, string(respBody))
	}
	if result.Error != nil && result.Error.Message != "" {
		return "", fmt.Errorf("sora2 error: %s", result.Error.Message)
	}
	id := result.ID
	if id == "" {
		id = result.TaskID
	}
	if id == "" {
		return "", fmt.Errorf("sora2: no task id in response: %s", string(respBody))
	}
	return id, nil
}

// poll —— 轮询 Sora2 任务状态直到完成、失败或超时（15分钟）
func (g *Sora2Generator) poll(ctx context.Context, taskID string) (*VideoClip, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("sora2: task %s timed out after 15min", taskID)
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

// queryTask —— 查询 Sora2 任务状态，返回结果和是否完成
func (g *Sora2Generator) queryTask(ctx context.Context, taskID string) (*VideoClip, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		g.BaseURL+"/videos/"+taskID, nil)
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

	var result sora2PollResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, false, nil
	}

	switch result.Status {
	case "completed":
		if result.VideoURL == "" {
			return nil, false, fmt.Errorf("sora2: completed but no video_url")
		}
		return &VideoClip{
			ClipURL:     result.VideoURL,
			DurationSec: 10,
			ModelUsed:   "sora2",
		}, true, nil
	case "failed":
		msg := "task failed"
		if result.Error != nil && result.Error.Message != "" {
			msg = result.Error.Message
		}
		return nil, false, fmt.Errorf("sora2: %s", msg)
	default: // queued, in_progress
		return nil, false, nil
	}
}
