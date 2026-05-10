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

// SuannengGenerator wraps the SophNet (算能) Seedance video generation API.
// The SophNet API is protocol-compatible with ByteDance Ark (doubao ARK):
//   - Submit: POST {BaseURL} with doubao ARK content[] body
//   - Response: {"id": "..."} (field name is "id", not "task_id")
//   - Query:   GET {BaseURL}/{taskID}  → doubao ARK status response
//   - Status values: "succeeded" / "running" / "failed" (same as doubao ARK)
//
// SuannengBase should be the full task creation URL:
// https://www.sophnet.com/api/open-apis/projects/easyllms/videogenerator/volces/tasks
type SuannengGenerator struct {
	APIKey  string
	BaseURL string // full path to tasks endpoint (without trailing slash)
	Model   string // actual doubao model name, e.g. "doubao-seedance-1-5-pro-251215"
	client  *http.Client
}

// NewSuannengGenerator —— 创建算能视频生成器实例
func NewSuannengGenerator(apiKey, baseURL, model string) *SuannengGenerator {
	if baseURL == "" {
		baseURL = "https://www.sophnet.com/api/open-apis/projects/easyllms/videogenerator/volces/tasks"
	}
	if model == "" {
		model = "doubao-seedance-1-5-pro-251215"
	}
	return &SuannengGenerator{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Name —— 返回生成器名称
func (g *SuannengGenerator) Name() string { return "suanneng" }

// CloneWithModel returns a copy bound to the requested model.
func (g *SuannengGenerator) CloneWithModel(model string) *SuannengGenerator {
	clone := *g
	if model != "" {
		clone.Model = model
	}
	return &clone
}

// IsAvailable —— 检查 API Key 是否已配置
func (g *SuannengGenerator) IsAvailable(_ context.Context) bool { return g.APIKey != "" }

// SupportsNativeAudio —— 算能使用 doubao ARK 协议，支持 generate_audio 参数
func (g *SuannengGenerator) SupportsNativeAudio() bool { return true }

// ParamOptions —— 算能无前端可配参数（通过 content 文本内嵌 --duration/--resolution 控制）
func (g *SuannengGenerator) ParamOptions() []ModelParamOption {
	return []ModelParamOption{
		{
			Key: "generate_mode", Label: "生成模式", Default: "img2video",
			Values: []ParamValue{
				{Value: "img2video", Label: "图生视频"},
				{Value: "startEnd2video", Label: "首尾帧生视频"},
				{Value: "reference2video", Label: "融合生视频"},
			},
		},
		{
			Key: "generate_audio", Label: "原生音频", Default: "false",
			Values: []ParamValue{
				{Value: "false", Label: "无音频"},
				{Value: "true", Label: "生成音频"},
			},
		},
		{
			Key: "duration", Label: "时长", Default: "5",
			Values: []ParamValue{
				{Value: "5", Label: "5秒"},
				{Value: "8", Label: "8秒"},
				{Value: "10", Label: "10秒"},
			},
		},
		{
			Key: "aspect_ratio", Label: "宽高比", Default: "16:9",
			Values: []ParamValue{
				{Value: "16:9", Label: "横屏 16:9"},
				{Value: "9:16", Label: "竖屏 9:16"},
				{Value: "1:1", Label: "方形 1:1"},
			},
		},
		{
			Key: "resolution", Label: "分辨率", Default: "720p",
			Values: []ParamValue{
				{Value: "720p", Label: "720p"},
				{Value: "1080p", Label: "1080p"},
			},
		},
	}
}

// ── API types (doubao ARK protocol) ──────────────────────────

// suannengSubmitReq mirrors the doubao ARK contents/generations/tasks request body.
type suannengSubmitReq struct {
	Model         string              `json:"model"`
	Content       []doubaoContentItem `json:"content"`
	GenerateAudio bool                `json:"generate_audio"`
	Ratio         string              `json:"ratio,omitempty"`
	Resolution    string              `json:"resolution,omitempty"`
	Duration      int                 `json:"duration,omitempty"`
}

// suannengSubmitResp is the task creation response — field "id" (not "task_id").
type suannengSubmitResp struct {
	ID    string `json:"id"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// suannengStatusResp mirrors doubao ARK task query response.
// Content uses RawMessage because the API may return either an object
// {"video_url":"..."} or an array [{type, video_url:{url}}].
type suannengStatusResp struct {
	ID     string `json:"id"`
	Status string `json:"status"` // queued / running / succeeded / failed
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Content json.RawMessage `json:"content"`
}

// ── Generate ────────────────────────────────────────────────

// Generate —— 提交算能视频任务并轮询等待完成，返回 *VideoClip
func (g *SuannengGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	var taskID string
	err := RetrySubmit(ctx, 4, func() error {
		var e error
		taskID, e = g.submit(ctx, req)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("suanneng submit: %w", err)
	}
	clip, err := g.poll(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("suanneng poll %s: %w", taskID, err)
	}
	return clip, nil
}

// submit —— 使用 doubao ARK content[] 格式提交视频生成任务，返回任务 ID
func (g *SuannengGenerator) submit(ctx context.Context, req VideoGenerateReq) (string, error) {
	dur := int(req.DurationSec)
	if dur <= 0 {
		dur = 5
	}
	resolution := req.Resolution
	if resolution == "" {
		resolution = "720p"
	}
	ratio := req.AspectRatio
	if ratio == "" {
		ratio = "16:9"
	}

	// Build content array based on generate mode.
	var content []doubaoContentItem
	switch req.GenerateMode {
	case "startEnd2video":
		content = []doubaoContentItem{
			{Type: "text", Text: req.Prompt},
		}
		if req.SourceImageURL != "" {
			content = append(content, doubaoContentItem{
				Type: "image_url", Role: "first_frame",
				ImageURL: &doubaoImageURLItem{URL: req.SourceImageURL},
			})
		}
		if req.TailImageURL != "" {
			content = append(content, doubaoContentItem{
				Type: "image_url", Role: "last_frame",
				ImageURL: &doubaoImageURLItem{URL: req.TailImageURL},
			})
		}

	case "reference2video":
		content = []doubaoContentItem{
			{Type: "text", Text: req.Prompt},
		}
		for _, imgURL := range req.CharacterImageURLs {
			if imgURL != "" {
				content = append(content, doubaoContentItem{
					Type: "image_url", Role: "reference_image",
					ImageURL: &doubaoImageURLItem{URL: imgURL},
				})
			}
		}
		if len(req.CharacterImageURLs) == 0 && req.SourceImageURL != "" {
			content = append(content, doubaoContentItem{
				Type: "image_url", Role: "reference_image",
				ImageURL: &doubaoImageURLItem{URL: req.SourceImageURL},
			})
		}

	default:
		content = []doubaoContentItem{
			{Type: "text", Text: req.Prompt},
		}
		if req.SourceImageURL != "" {
			content = append(content, doubaoContentItem{
				Type:     "image_url",
				ImageURL: &doubaoImageURLItem{URL: req.SourceImageURL},
			})
		}
	}

	body := suannengSubmitReq{
		Model:         g.Model,
		Content:       content,
		GenerateAudio: req.GenerateAudio,
		Ratio:         ratio,
		Resolution:    resolution,
		Duration:      dur,
	}
	b, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL, bytes.NewReader(b))
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
		return "", fmt.Errorf("suanneng rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("suanneng HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result suannengSubmitResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse submit response: %w body=%s", err, string(respBody))
	}
	if result.Error != nil && result.Error.Message != "" {
		return "", fmt.Errorf("suanneng: %s: %s", result.Error.Code, result.Error.Message)
	}
	if result.ID == "" {
		return "", fmt.Errorf("suanneng: no id in response: %s", string(respBody))
	}
	return result.ID, nil
}

// poll —— 轮询任务状态直到完成、失败或超时（15分钟）
func (g *SuannengGenerator) poll(ctx context.Context, taskID string) (*VideoClip, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("suanneng: task %s timed out after 15min", taskID)
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

// queryTask —— 查询单次任务状态（doubao ARK 兼容响应格式）
func (g *SuannengGenerator) queryTask(ctx context.Context, taskID string) (*VideoClip, bool, error) {
	url := g.BaseURL + "/" + taskID
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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

	var result suannengStatusResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, false, nil // transient parse error
	}
	if result.Error != nil && result.Error.Message != "" {
		return nil, false, fmt.Errorf("suanneng: %s", result.Error.Message)
	}

	switch result.Status {
	case "succeeded":
		videoURL := extractDoubaoVideoURL(result.Content)
		if videoURL == "" {
			return nil, false, fmt.Errorf("suanneng: succeeded but no video_url in content: %s", string(result.Content))
		}
		return &VideoClip{
			ClipURL:     videoURL,
			DurationSec: 5,
			ModelUsed:   resolvedModelUsed(g.Model, g.Name()),
		}, true, nil
	case "failed":
		return nil, false, fmt.Errorf("suanneng: task %s failed", taskID)
	default: // queued, running
		return nil, false, nil
	}
}
