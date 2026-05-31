package generators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// MinMaxGenerator is a minimal MiniMax/Hailuo video generator.
// Defaults target WaveSpeed's public MiniMax Hailuo 02 endpoints:
//
//	POST /api/v3/minimax/hailuo-02/{standard,pro,fast}
//	GET  /api/v3/predictions/{requestId}/result
//
// It also accepts MiniMax/fengxi compatibility fields in responses.
type MinMaxGenerator struct {
	APIKey                string
	BaseURL               string
	Model                 string
	Img2VideoEndpointStd  string
	Img2VideoEndpointPro  string
	Img2VideoEndpointFast string
	QueryEndpoint         string
	FileRetrieveEndpoint  string
	FastPretreatment      string
	PromptOptimizer       string
	client                *http.Client
}

type MinMaxOption func(*MinMaxGenerator)

func WithMinMaxEndpoints(std, pro, fast, query, fileRetrieve string) MinMaxOption {
	return func(g *MinMaxGenerator) {
		if std != "" {
			g.Img2VideoEndpointStd = normalizeMinMaxEndpoint(std)
		}
		if pro != "" {
			g.Img2VideoEndpointPro = normalizeMinMaxEndpoint(pro)
		}
		if fast != "" {
			g.Img2VideoEndpointFast = normalizeMinMaxEndpoint(fast)
		}
		if query != "" {
			g.QueryEndpoint = query
		}
		if fileRetrieve != "" {
			g.FileRetrieveEndpoint = fileRetrieve
		}
	}
}

func WithMinMaxFlags(fastPretreatment, promptOptimizer string) MinMaxOption {
	return func(g *MinMaxGenerator) {
		g.FastPretreatment = fastPretreatment
		g.PromptOptimizer = promptOptimizer
	}
}

func NewMinMaxGenerator(apiKey, baseURL, model string, opts ...MinMaxOption) *MinMaxGenerator {
	if baseURL == "" {
		baseURL = "https://api.wavespeed.ai"
	}
	if model == "" {
		model = "MiniMax-Hailuo-02"
	}
	g := &MinMaxGenerator{
		APIKey:                apiKey,
		BaseURL:               baseURL,
		Model:                 model,
		Img2VideoEndpointStd:  "/api/v3/minimax/hailuo-02/standard",
		Img2VideoEndpointPro:  "/api/v3/minimax/hailuo-02/pro",
		Img2VideoEndpointFast: "/api/v3/minimax/hailuo-02/fast",
		QueryEndpoint:         "/api/v3/predictions/{requestId}/result",
		FileRetrieveEndpoint:  "/v1/files/retrieve",
		client:                &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(g)
	}
	g.applyModelEndpointDefaults()
	return g
}

func (g *MinMaxGenerator) Name() string { return "minmax" }

func (g *MinMaxGenerator) CloneWithModel(model string) *MinMaxGenerator {
	clone := *g
	if model != "" {
		clone.Model = model
	}
	clone.applyModelEndpointDefaults()
	return &clone
}

func (g *MinMaxGenerator) IsAvailable(_ context.Context) bool { return g.APIKey != "" }

func (g *MinMaxGenerator) SupportsNativeAudio() bool { return false }

func (g *MinMaxGenerator) ParamOptions() []ModelParamOption {
	return []ModelParamOption{
		{Key: "duration", Label: "时长", Default: "6", Values: []ParamValue{{Value: "6", Label: "6秒"}, {Value: "10", Label: "10秒"}}},
		{Key: "resolution", Label: "分辨率", Default: "768P", Values: []ParamValue{{Value: "512P", Label: "512P/快速"}, {Value: "768P", Label: "768P/标准"}, {Value: "1080P", Label: "1080P/专业"}}},
		{Key: "aspect_ratio", Label: "画面比例", Default: "16:9", Values: []ParamValue{{Value: "16:9", Label: "横屏 16:9"}, {Value: "9:16", Label: "竖屏 9:16"}, {Value: "1:1", Label: "方形 1:1"}}},
	}
}

type minMaxSubmitReq struct {
	Model                 string `json:"model,omitempty"`
	Prompt                string `json:"prompt,omitempty"`
	FirstFrameImage       string `json:"first_frame_image,omitempty"`
	LastFrameImage        string `json:"last_frame_image,omitempty"`
	Image                 string `json:"image,omitempty"`
	EndImage              string `json:"end_image,omitempty"`
	Duration              int    `json:"duration"`
	Resolution            string `json:"resolution,omitempty"`
	AspectRatio           string `json:"aspect_ratio,omitempty"`
	FastPretreatment      string `json:"fast_pretreatment,omitempty"`
	PromptOptimizer       string `json:"prompt_optimizer,omitempty"`
	EnablePromptExpansion *bool  `json:"enable_prompt_expansion,omitempty"`
	GoFast                *bool  `json:"go_fast,omitempty"`
}

type minMaxResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    *struct {
		ID      string   `json:"id"`
		Status  string   `json:"status"`
		Outputs []string `json:"outputs"`
		URLs    []string `json:"urls"`
		Error   string   `json:"error"`
	} `json:"data"`
	TaskID   string `json:"task_id"`
	BaseResp *struct {
		StatusCode string `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
	TaskIDAlt        string   `json:"taskId"`
	RequestID        string   `json:"request_id"`
	ID               string   `json:"id"`
	Status           string   `json:"status"`
	VideoDownLoadURL string   `json:"videoDownLoadUrl"`
	Output           string   `json:"output"`
	Outputs          []string `json:"outputs"`
}

func (g *MinMaxGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	var taskID string
	err := RetrySubmit(ctx, 4, func() error {
		var e error
		taskID, e = g.submit(ctx, req)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("minmax submit: %w", err)
	}
	clip, err := g.poll(ctx, taskID, req.DurationSec)
	if err != nil {
		return nil, fmt.Errorf("minmax poll %s: %w", taskID, err)
	}
	return clip, nil
}

func (g *MinMaxGenerator) submit(ctx context.Context, req VideoGenerateReq) (string, error) {
	dur := int(req.DurationSec)
	if dur <= 0 {
		dur = 6
	}
	resolution := strings.ToUpper(firstNonEmpty(req.Resolution, "768P"))
	if resolution == "720P" {
		resolution = "768P"
	}
	endpoint := g.Img2VideoEndpointStd
	goFast := false
	if strings.Contains(resolution, "512") || strings.EqualFold(req.VideoMode, "fast") {
		endpoint = g.Img2VideoEndpointFast
		goFast = true
	} else if strings.Contains(resolution, "1080") || strings.EqualFold(req.VideoMode, "pro") {
		endpoint = g.Img2VideoEndpointPro
	}
	if dur == 10 && endpoint == g.Img2VideoEndpointFast {
		// Fast endpoint supports both 6s and 10s, but keep the caller's duration explicit.
	}
	body := minMaxSubmitReq{
		Model:            g.Model,
		Prompt:           req.Prompt,
		FirstFrameImage:  req.SourceImageURL,
		LastFrameImage:   req.TailImageURL,
		Duration:         dur,
		Resolution:       resolution,
		AspectRatio:      req.AspectRatio,
		FastPretreatment: g.FastPretreatment,
		PromptOptimizer:  g.PromptOptimizer,
	}
	if strings.EqualFold(req.VideoMode, "standard") {
		endpoint = g.Img2VideoEndpointStd
	}
	if body.FirstFrameImage == "" && len(req.CharacterImageURLs) > 0 {
		body.FirstFrameImage = req.CharacterImageURLs[0]
	}
	// Compatibility aliases for providers still expecting fengxi's older field names.
	body.Image = body.FirstFrameImage
	body.EndImage = body.LastFrameImage
	if goFast {
		body.GoFast = boolPtr(true)
	}
	if body.FastPretreatment == "" && g.FastPretreatment != "" {
		body.FastPretreatment = g.FastPretreatment
	}
	if body.PromptOptimizer == "" && g.PromptOptimizer != "" {
		body.PromptOptimizer = g.PromptOptimizer
	}

	b, _ := json.Marshal(body)
	url := joinURL(g.BaseURL, endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", bearerValue(g.APIKey))
	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 429 {
		return "", fmt.Errorf("minmax rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("minmax HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var out minMaxResp
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("parse submit response: %w body=%s", err, string(respBody))
	}
	if out.BaseResp != nil && out.BaseResp.StatusCode != "" && out.BaseResp.StatusCode != "0" {
		return "", fmt.Errorf("minmax error %s: %s", out.BaseResp.StatusCode, out.BaseResp.StatusMsg)
	}
	if out.Code >= 400 {
		return "", fmt.Errorf("minmax error %d: %s", out.Code, out.Message)
	}
	taskID := firstNonEmpty(out.TaskID, out.TaskIDAlt)
	if taskID == "" && out.Data != nil {
		taskID = out.Data.ID
	}
	if taskID == "" {
		return "", fmt.Errorf("minmax: no task id in response: %s", string(respBody))
	}
	return taskID, nil
}

func (g *MinMaxGenerator) poll(ctx context.Context, taskID string, requestedDuration float64) (*VideoClip, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	timeout := time.After(15 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("minmax: task %s timed out after 15min", taskID)
		case <-ticker.C:
			clip, done, err := g.queryTask(ctx, taskID, requestedDuration)
			if err != nil {
				return nil, err
			}
			if done {
				return clip, nil
			}
		}
	}
}

func (g *MinMaxGenerator) queryTask(ctx context.Context, taskID string, requestedDuration float64) (*VideoClip, bool, error) {
	u := joinURL(g.BaseURL, g.QueryEndpoint)
	if strings.Contains(g.QueryEndpoint, "{requestId}") {
		u = joinURL(g.BaseURL, strings.ReplaceAll(g.QueryEndpoint, "{requestId}", url.PathEscape(taskID)))
	} else {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u = u + sep + "taskId=" + url.QueryEscape(taskID)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	httpReq.Header.Set("Authorization", bearerValue(g.APIKey))
	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, false, nil
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 500 {
		return nil, false, nil
	}
	if resp.StatusCode >= 400 {
		return nil, false, fmt.Errorf("minmax query HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var out minMaxResp
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, false, nil
	}
	status := strings.ToLower(strings.TrimSpace(out.Status))
	videoURL := firstNonEmpty(out.VideoDownLoadURL, out.Output)
	if videoURL == "" && len(out.Outputs) > 0 {
		videoURL = out.Outputs[0]
	}
	if out.Data != nil {
		status = strings.ToLower(strings.TrimSpace(firstNonEmpty(status, out.Data.Status)))
		if videoURL == "" {
			if len(out.Data.Outputs) > 0 {
				videoURL = out.Data.Outputs[0]
			} else if len(out.Data.URLs) > 0 {
				videoURL = out.Data.URLs[0]
			}
		}
		if strings.EqualFold(status, "failed") && out.Data.Error != "" {
			return nil, false, fmt.Errorf("minmax: task %s failed — %s", taskID, out.Data.Error)
		}
	}
	if isMinMaxSuccess(status) || strings.Contains(strings.ToLower(string(respBody)), `"status":"succeeded"`) {
		if videoURL == "" {
			return nil, false, fmt.Errorf("minmax: succeeded but no video url in response: %s", string(respBody))
		}
		return &VideoClip{ClipURL: videoURL, DurationSec: resolvedDurationSec(0, requestedDuration), ModelUsed: g.Model}, true, nil
	}
	if isMinMaxFailure(status) {
		return nil, false, fmt.Errorf("minmax: task %s failed", taskID)
	}
	return nil, false, nil
}

func isMinMaxSuccess(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return s == "success" || s == "succeeded" || s == "completed"
}

func isMinMaxFailure(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return s == "fail" || s == "failed" || s == "error"
}

func bearerValue(apiKey string) string {
	if strings.HasPrefix(strings.ToLower(apiKey), "bearer ") {
		return apiKey
	}
	return "Bearer " + apiKey
}

func normalizeMinMaxEndpoint(endpoint string) string {
	switch strings.TrimSpace(endpoint) {
	case "/api/v3/minimax/hailuo-02/i2v-standard":
		return "/api/v3/minimax/hailuo-02/standard"
	case "/api/v3/minimax/hailuo-02/i2v-pro":
		return "/api/v3/minimax/hailuo-02/pro"
	case "/api/v3/minimax/hailuo-02/i2v-fast":
		return "/api/v3/minimax/hailuo-02/fast"
	case "/api/v3/minimax/hailuo-2.3/i2v-standard":
		return "/api/v3/minimax/hailuo-2.3/standard"
	case "/api/v3/minimax/hailuo-2.3/i2v-pro":
		return "/api/v3/minimax/hailuo-2.3/pro"
	case "/api/v3/minimax/hailuo-2.3/i2v-fast":
		return "/api/v3/minimax/hailuo-2.3/fast"
	default:
		return endpoint
	}
}

func (g *MinMaxGenerator) applyModelEndpointDefaults() {
	family := minMaxEndpointFamily(g.Model)
	if family == "" {
		family = "hailuo-02"
	}
	g.Img2VideoEndpointStd = defaultMinMaxFamilyEndpoint(g.Img2VideoEndpointStd, family, "standard")
	g.Img2VideoEndpointPro = defaultMinMaxFamilyEndpoint(g.Img2VideoEndpointPro, family, "pro")
	g.Img2VideoEndpointFast = defaultMinMaxFamilyEndpoint(g.Img2VideoEndpointFast, family, "fast")
}

func minMaxEndpointFamily(model string) string {
	s := strings.ToLower(strings.TrimSpace(model))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, "--", "-")
	s = strings.Trim(s, "-")
	s = strings.ReplaceAll(s, "minimax-", "")
	s = strings.ReplaceAll(s, "hailuo2", "hailuo-02")
	s = strings.ReplaceAll(s, "hailuo-2-3-fast", "hailuo-2-3")
	s = strings.ReplaceAll(s, "hailuo-2-3", "hailuo-2.3")
	if strings.Contains(s, "hailuo-2.3") {
		return "hailuo-2.3"
	}
	if strings.Contains(s, "hailuo-02") || strings.Contains(s, "hailuo") {
		return "hailuo-02"
	}
	return ""
}

func defaultMinMaxFamilyEndpoint(current, family, mode string) string {
	current = normalizeMinMaxEndpoint(current)
	if current == "" || strings.Contains(current, "/hailuo-02/") || strings.Contains(current, "/hailuo-2.3/") {
		return "/api/v3/minimax/" + family + "/" + mode
	}
	return current
}

func boolPtr(v bool) *bool { return &v }

func joinURL(base, elem string) string {
	if strings.HasPrefix(elem, "http://") || strings.HasPrefix(elem, "https://") {
		return elem
	}
	if base == "" {
		return elem
	}
	if elem == "" {
		return strings.TrimRight(base, "/")
	}
	if parsed, err := url.Parse(base); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.Path = path.Join(parsed.Path, elem)
		return parsed.String()
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(elem, "/")
}
