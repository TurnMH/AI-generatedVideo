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

// ViduGenerator wraps the Vidu Enterprise v2 video generation API.
// Supports models: viduq3-pro (xingcheng-2.6), viduq3-mix (xingchen-3.1).
type ViduGenerator struct {
	APIKey  string
	BaseURL string // e.g. "https://api.vidu.cn/ent/v2"
	Model   string // e.g. "viduq3-pro" or "viduq3-mix"
	genName string // canonical name returned by Name()
	client  *http.Client
}

// NewViduGenerator —— 创建 Vidu 视频生成器实例
func NewViduGenerator(apiKey, baseURL, model, genName string) *ViduGenerator {
	if baseURL == "" {
		baseURL = "https://api.vidu.cn/ent/v2"
	}
	if model == "" {
		model = "viduq3-pro"
	}
	if genName == "" {
		genName = "vidu"
	}
	return &ViduGenerator{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		genName: genName,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Name —— 返回生成器名称
func (g *ViduGenerator) Name() string { return g.genName }

// IsAvailable —— 检查 API Key 是否已配置
func (g *ViduGenerator) IsAvailable(_ context.Context) bool { return g.APIKey != "" }

// SupportsNativeAudio —— Vidu 不支持原生音频嵌入
func (g *ViduGenerator) SupportsNativeAudio() bool { return false }

// ParamOptions —— Vidu 支持的模型参数（时长、分辨率、宽高比、运动幅度）
func (g *ViduGenerator) ParamOptions() []ModelParamOption {
	return []ModelParamOption{
		{
			Key: "duration", Label: "时长", Default: "5",
			Values: []ParamValue{{Value: "4", Label: "4秒"}, {Value: "8", Label: "8秒"}},
		},
		{
			Key: "resolution", Label: "分辨率", Default: "1080p",
			Values: []ParamValue{
				{Value: "360p", Label: "360p"}, {Value: "720p", Label: "720p"}, {Value: "1080p", Label: "1080p"},
			},
		},
		{
			Key: "aspect_ratio", Label: "画面比例", Default: "16:9",
			Values: []ParamValue{
				{Value: "16:9", Label: "横屏 16:9"}, {Value: "9:16", Label: "竖屏 9:16"}, {Value: "1:1", Label: "方形 1:1"},
			},
		},
		{
			Key: "motion_amplitude", Label: "运动幅度", Default: "auto",
			Values: []ParamValue{
				{Value: "auto", Label: "自动"}, {Value: "small", Label: "小"}, {Value: "medium", Label: "中"}, {Value: "large", Label: "大"},
			},
		},
	}
}

// ── API types ────────────────────────────────────────────────

type viduSubmitReq struct {
	Model             string   `json:"model"`
	Images            []string `json:"images,omitempty"`
	SubjectReferences []string `json:"subject_references,omitempty"` // character reference images for subject consistency
	Prompt            string   `json:"prompt"`
	Duration          int      `json:"duration"`
	Resolution        string   `json:"resolution"`
	MovementAmplitude string   `json:"movement_amplitude"`
}

type viduSubmitResp struct {
	TaskID string `json:"task_id"` // new API returns task_id
}

type viduCreationResp struct {
	ID    string `json:"id"`
	State string `json:"state"` // queueing / processing / success / failed
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Creations []struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	} `json:"creations"`
}

// ── Generate ────────────────────────────────────────────────

// Generate —— 提交 Vidu 视频任务并轮询等待完成，返回 *VideoClip
func (g *ViduGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	var creationID string
	err := RetrySubmit(ctx, 4, func() error {
		var e error
		creationID, e = g.submit(ctx, req)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("vidu submit: %w", err)
	}
	clip, err := g.poll(ctx, creationID)
	if err != nil {
		return nil, fmt.Errorf("vidu poll %s: %w", creationID, err)
	}
	return clip, nil
}

// submit —— 提交视频生成任务，返回 creation ID
func (g *ViduGenerator) submit(ctx context.Context, req VideoGenerateReq) (string, error) {
	dur := int(req.DurationSec)
	if dur <= 0 {
		dur = 5
	}

	body := viduSubmitReq{
		Model:             g.Model,
		Prompt:            req.Prompt,
		Duration:          dur,
		Resolution:        firstNonEmpty(req.Resolution, "1080p"),
		MovementAmplitude: firstNonEmpty(req.MotionAmplitude, "auto"),
	}

	// Endpoint dispatch by model family.
	// Vidu Enterprise v2 uses three distinct endpoints; sending a model to the
	// wrong endpoint returns HTTP 400 "model is not supported":
	//   - /reference2video  — reference-to-video (viduq3-mix, q1-mix ...)
	//   - /img2video        — image-to-video (viduq3-pro, viduq3-turbo, vidu2.0 ...)
	//   - /text2video       — pure text-to-video
	isMixModel := strings.Contains(strings.ToLower(g.Model), "mix")
	hasRefs := len(req.CharacterImageURLs) > 0
	hasSource := req.SourceImageURL != ""

	var submitPath string
	switch {
	case isMixModel:
		submitPath = "/reference2video"
		refs := req.CharacterImageURLs
		if len(refs) == 0 && hasSource {
			refs = []string{req.SourceImageURL}
		}
		body.Images = refs
	case hasSource:
		submitPath = "/img2video"
		body.Images = []string{req.SourceImageURL}
		if hasRefs {
			body.SubjectReferences = req.CharacterImageURLs
		}
	default:
		submitPath = "/text2video"
	}

	b, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.BaseURL+submitPath, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Token "+g.APIKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 429 {
		return "", fmt.Errorf("vidu rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("vidu HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result viduSubmitResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse submit response: %w body=%s", err, string(respBody))
	}
	if result.TaskID == "" {
		return "", fmt.Errorf("vidu: no task_id in response: %s", string(respBody))
	}
	return result.TaskID, nil
}

// poll —— 轮询任务状态直到完成、失败或超时（15分钟）
func (g *ViduGenerator) poll(ctx context.Context, creationID string) (*VideoClip, error) {
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("vidu: creation %s timed out after 15min", creationID)
		case <-ticker.C:
			clip, done, err := g.queryTask(ctx, creationID)
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
func (g *ViduGenerator) queryTask(ctx context.Context, creationID string) (*VideoClip, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		g.BaseURL+"/tasks/"+creationID+"/creations", nil)
	if err != nil {
		return nil, false, err
	}
	httpReq.Header.Set("Authorization", "Token "+g.APIKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, false, nil // transient, retry
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result viduCreationResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, false, nil
	}

	switch result.State {
	case "success":
		videoURL := ""
		if len(result.Creations) > 0 {
			videoURL = result.Creations[0].URL
		}
		if videoURL == "" {
			return nil, false, fmt.Errorf("vidu: success but no video url")
		}
		return &VideoClip{
			ClipURL:     videoURL,
			DurationSec: 5,
			ModelUsed:   g.genName,
		}, true, nil
	case "failed":
		msg := "task failed"
		if result.Error != nil && result.Error.Message != "" {
			msg = result.Error.Message
		}
		return nil, false, fmt.Errorf("vidu: %s", msg)
	default: // queueing, processing
		return nil, false, nil
	}
}
