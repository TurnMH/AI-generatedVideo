package generators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DoubaoGenerator wraps the ByteDance Ark video generation API.
// Supports models: V4.0 (xingguang-3.0), doubao-seedream-4-0-250828 (doubao-seedance), etc.
type DoubaoGenerator struct {
	APIKey         string
	BaseURL        string // e.g. "https://ark.cn-beijing.volces.com"
	Model          string // e.g. "V4.0" or "doubao-seedream-4-0-250828"
	genName        string // canonical name returned by Name()
	supportsAudio  bool   // true for seedance models that support generate_audio
	supportsRatio  bool   // true for seedance models that support ratio/aspect_ratio
	client         *http.Client
}

// NewDoubaoGenerator —— 创建豆包视频生成器实例
func NewDoubaoGenerator(apiKey, baseURL, model, genName string) *DoubaoGenerator {
	if baseURL == "" {
		baseURL = "https://ark.cn-beijing.volces.com"
	}
	if model == "" {
		model = "V4.0"
	}
	if genName == "" {
		genName = "doubao"
	}
	return &DoubaoGenerator{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		genName: genName,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// NewDoubaoSeedanceGenerator —— 创建豆包 Seedance 生成器（支持宽高比和环境音频）
func NewDoubaoSeedanceGenerator(apiKey, baseURL, model, genName string) *DoubaoGenerator {
	g := NewDoubaoGenerator(apiKey, baseURL, model, genName)
	g.supportsAudio = true
	g.supportsRatio = true
	return g
}

// Name —— 返回生成器名称
func (g *DoubaoGenerator) Name() string { return g.genName }

// CloneWithModel returns a copy bound to the requested model/name.
func (g *DoubaoGenerator) CloneWithModel(model, genName string) *DoubaoGenerator {
	clone := *g
	if model != "" {
		clone.Model = model
	}
	if genName != "" {
		clone.genName = genName
	}
	return &clone
}

// IsAvailable —— 检查 API Key 是否已配置
func (g *DoubaoGenerator) IsAvailable(_ context.Context) bool { return g.APIKey != "" }

// SupportsNativeAudio —— seedance 模型支持原生环境音频（非语音）
func (g *DoubaoGenerator) SupportsNativeAudio() bool { return g.supportsAudio }

// ParamOptions —— 豆包视频支持的模型参数
func (g *DoubaoGenerator) ParamOptions() []ModelParamOption {
	opts := []ModelParamOption{
		{
			Key: "generate_mode", Label: "生成模式", Default: "img2video",
			Values: []ParamValue{
				{Value: "img2video", Label: "图生视频"},
				{Value: "startEnd2video", Label: "首尾帧生视频"},
				{Value: "reference2video", Label: "融合生视频"},
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
	}
	if g.supportsRatio {
		opts = append(opts, ModelParamOption{
			Key: "aspect_ratio", Label: "宽高比", Default: "16:9",
			Values: []ParamValue{
				{Value: "16:9", Label: "横屏 16:9"},
				{Value: "9:16", Label: "竖屏 9:16"},
				{Value: "1:1", Label: "方形 1:1"},
				{Value: "4:3", Label: "标准 4:3"},
				{Value: "3:4", Label: "竖版 3:4"},
			},
		})
		opts = append(opts, ModelParamOption{
			Key: "resolution", Label: "分辨率", Default: "720p",
			Values: []ParamValue{
				{Value: "720p", Label: "720p"},
				{Value: "1080p", Label: "1080p"},
			},
		})
	}
	if g.supportsAudio {
		opts = append(opts, ModelParamOption{
			Key: "generate_audio", Label: "原生音频", Default: "false",
			Values: []ParamValue{
				{Value: "false", Label: "无音频"},
				{Value: "true", Label: "生成音频"},
			},
		})
	}
	return opts
}

// ── API types ────────────────────────────────────────────────

type doubaoContentItem struct {
	Type     string              `json:"type"`
	Role     string              `json:"role,omitempty"`      // first_frame | last_frame | reference_image
	Text     string              `json:"text,omitempty"`
	ImageURL *doubaoImageURLItem `json:"image_url,omitempty"`
}

type doubaoImageURLItem struct {
	URL string `json:"url"`
}

type doubaoSubmitReq struct {
	Model         string              `json:"model"`
	Content       []doubaoContentItem `json:"content"`
	Ratio         string              `json:"ratio,omitempty"`
	Resolution    string              `json:"resolution,omitempty"`
	Duration      int                 `json:"duration,omitempty"`
	GenerateAudio bool                `json:"generate_audio,omitempty"`
}

type doubaoTaskResp struct {
	ID     string `json:"id"`
	Status string `json:"status"` // queued / running / succeeded / failed
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	// Content can be either an object {"video_url":"..."} or an array of typed items
	// depending on the API version. Use RawMessage to handle both.
	Content json.RawMessage `json:"content"`
}

// extractVideoURL parses the video URL from the content field which may be
// either an object {"video_url":"..."} or an array [{"type":"video_url","video_url":{"url":"..."}}].
func extractDoubaoVideoURL(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try object form: {"video_url": "https://..."}
	var obj struct {
		VideoURL string `json:"video_url"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.VideoURL != "" {
		return obj.VideoURL
	}
	// Try array form: [{"type":"video_url","video_url":{"url":"..."}}]
	var arr []struct {
		Type     string `json:"type"`
		VideoURL *struct {
			URL string `json:"url"`
		} `json:"video_url,omitempty"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, item := range arr {
			if item.Type == "video_url" && item.VideoURL != nil {
				return item.VideoURL.URL
			}
		}
	}
	return ""
}

// ── Generate ────────────────────────────────────────────────

// Generate —— 提交豆包视频任务并轮询等待完成，返回 *VideoClip
func (g *DoubaoGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	var taskID string
	err := RetrySubmit(ctx, 4, func() error {
		var e error
		taskID, e = g.submit(ctx, req)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("doubao submit: %w", err)
	}
	clip, err := g.poll(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("doubao poll %s: %w", taskID, err)
	}
	return clip, nil
}

// submit —— 提交视频生成任务，返回任务 ID
func (g *DoubaoGenerator) submit(ctx context.Context, req VideoGenerateReq) (string, error) {
	// Build content array based on generate mode.
	var content []doubaoContentItem

	switch req.GenerateMode {
	case "startEnd2video":
		// 首尾帧生视频：text prompt + first_frame + last_frame
		content = []doubaoContentItem{
			{Type: "text", Text: req.Prompt},
		}
		if req.SourceImageURL != "" {
			content = append(content, doubaoContentItem{
				Type:     "image_url",
				Role:     "first_frame",
				ImageURL: &doubaoImageURLItem{URL: req.SourceImageURL},
			})
		}
		if req.TailImageURL != "" {
			content = append(content, doubaoContentItem{
				Type:     "image_url",
				Role:     "last_frame",
				ImageURL: &doubaoImageURLItem{URL: req.TailImageURL},
			})
		}

	case "reference2video":
		// 融合生视频：text prompt + reference_image items
		content = []doubaoContentItem{
			{Type: "text", Text: req.Prompt},
		}
		for _, imgURL := range req.CharacterImageURLs {
			if imgURL != "" {
				content = append(content, doubaoContentItem{
					Type:     "image_url",
					Role:     "reference_image",
					ImageURL: &doubaoImageURLItem{URL: imgURL},
				})
			}
		}
		// Fallback to source image if no character images provided
		if len(req.CharacterImageURLs) == 0 && req.SourceImageURL != "" {
			content = append(content, doubaoContentItem{
				Type:     "image_url",
				Role:     "reference_image",
				ImageURL: &doubaoImageURLItem{URL: req.SourceImageURL},
			})
		}

	default:
		// img2video（默认）: image_url (no role) + text
		content = make([]doubaoContentItem, 0, 2)
		if req.SourceImageURL != "" {
			content = append(content, doubaoContentItem{
				Type:     "image_url",
				ImageURL: &doubaoImageURLItem{URL: req.SourceImageURL},
			})
		}
		content = append(content, doubaoContentItem{
			Type: "text",
			Text: req.Prompt,
		})
	}

	body := doubaoSubmitReq{
		Model:   g.Model,
		Content: content,
	}
	if g.supportsRatio && req.AspectRatio != "" {
		body.Ratio = req.AspectRatio
	}
	if req.Resolution != "" {
		body.Resolution = req.Resolution
	}
	if req.DurationSec > 0 {
		body.Duration = int(req.DurationSec)
	}
	// Audio: respect caller's GenerateAudio flag; for native-audio models default to true
	if g.supportsAudio {
		body.GenerateAudio = req.GenerateAudio
	}
	b, _ := json.Marshal(body)

	endpoint := g.BaseURL + "/api/v3/contents/generations/tasks"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
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
		return "", fmt.Errorf("doubao rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		body := string(respBody)
		// Surface a clear error for content-moderation rejections so callers can
		// distinguish permanent failures (no point retrying with same image).
		if strings.Contains(body, "SensitiveContentDetected") || strings.Contains(body, "PrivacyInformation") {
			return "", fmt.Errorf("doubao content moderation rejected (real-person/sensitive image): %s", body)
		}
		return "", fmt.Errorf("doubao HTTP %d: %s", resp.StatusCode, body)
	}

	var result doubaoTaskResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse submit response: %w body=%s", err, string(respBody))
	}
	if result.Error != nil && result.Error.Message != "" {
		return "", fmt.Errorf("doubao error %s: %s", result.Error.Code, result.Error.Message)
	}
	if result.ID == "" {
		return "", fmt.Errorf("doubao: no task id in response: %s", string(respBody))
	}
	return result.ID, nil
}

// poll —— 轮询任务状态直到完成、失败或超时（15分钟）
func (g *DoubaoGenerator) poll(ctx context.Context, taskID string) (*VideoClip, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("doubao: task %s timed out after 15min", taskID)
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

// queryTask —— 查询单次任务状态
func (g *DoubaoGenerator) queryTask(ctx context.Context, taskID string) (*VideoClip, bool, error) {
	endpoint := g.BaseURL + "/api/v3/contents/generations/tasks/" + taskID
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

	var result doubaoTaskResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, false, nil // transient
	}
	if result.Error != nil && result.Error.Message != "" {
		return nil, false, fmt.Errorf("doubao: %s", result.Error.Message)
	}

	switch result.Status {
	case "succeeded":
		videoURL := extractDoubaoVideoURL(result.Content)
		if videoURL == "" {
			return nil, false, fmt.Errorf("doubao: succeeded but no video_url in content: %s", string(result.Content))
		}
		return &VideoClip{
			ClipURL:     videoURL,
			DurationSec: 5,
			ModelUsed:   resolvedModelUsed(g.Model, g.genName),
		}, true, nil
	case "failed":
		return nil, false, fmt.Errorf("doubao: task %s failed", taskID)
	default: // queued, running
		return nil, false, nil
	}
}
