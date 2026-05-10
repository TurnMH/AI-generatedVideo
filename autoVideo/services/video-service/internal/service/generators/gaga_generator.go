package generators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"
)

// GagaGenerator wraps the Gaga video generation API (xingdian2.0).
// Correct 2-step flow (confirmed from fengxi docs):
//  Step 1 — upload image asset via multipart → get Long asset ID
//  Step 2 — POST /v1/generations with source.content = asset ID + chunks[]
//
// API: https://api.gaga.art
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
		client:  &http.Client{Timeout: 120 * time.Second},
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

// gagaAssetResp — Step 1: POST /v1/assets response (ID is a Long int64)
type gagaAssetResp struct {
	ID int64 `json:"id"`
}

// gagaSubmitReq — Step 2: POST /v1/generations request body
type gagaSubmitReq struct {
	Model       string           `json:"model"`
	Resolution  string           `json:"resolution"`
	AspectRatio string           `json:"aspectRatio"`
	Source      gagaSource       `json:"source"`
	Chunks      []gagaChunk      `json:"chunks"`
}

type gagaSource struct {
	Type    string `json:"type"`
	Content int64  `json:"content"` // Long asset ID
}

type gagaChunk struct {
	Duration               int              `json:"duration"`
	Conditions             []gagaCondition  `json:"conditions"`
	EnablePromptEnhancement bool            `json:"enablePromptEnhancement"`
}

type gagaCondition struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// gagaGenerationResp — Step 2 response and polling response (ID is also Long)
type gagaGenerationResp struct {
	ID       int64  `json:"id"`
	Status   string `json:"status"`   // pending / completed / failed
	VideoURL string `json:"url"`
	Creations []struct {
		URL string `json:"url"`
	} `json:"creations"`
}

// ── Generate ────────────────────────────────────────────────

// Generate —— 上传图片资产 → 提交视频生成任务 → 轮询等待完成
func (g *GagaGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	// Step 1: upload image to get asset ID
	assetID, err := g.uploadAsset(ctx, req.SourceImageURL)
	if err != nil {
		return nil, fmt.Errorf("gaga upload asset: %w", err)
	}

	// Step 2: submit generation
	var genID int64
	err = RetrySubmit(ctx, 3, func() error {
		var e error
		genID, e = g.submit(ctx, req, assetID)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("gaga submit: %w", err)
	}

	// Step 3: poll for completion
	clip, err := g.poll(ctx, genID)
	if err != nil {
		return nil, fmt.Errorf("gaga poll %d: %w", genID, err)
	}
	return clip, nil
}

// uploadAsset —— 下载源图片并通过 multipart/form-data 上传到 gaga /v1/assets
func (g *GagaGenerator) uploadAsset(ctx context.Context, imageURL string) (int64, error) {
	// Download image
	imgReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return 0, fmt.Errorf("build image download request: %w", err)
	}
	imgResp, err := g.client.Do(imgReq)
	if err != nil {
		return 0, fmt.Errorf("download image: %w", err)
	}
	defer imgResp.Body.Close()
	imgData, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return 0, fmt.Errorf("read image: %w", err)
	}

	// Detect filename/extension from URL
	filename := path.Base(imageURL)
	if filename == "" || filename == "/" || !strings.Contains(filename, ".") {
		filename = "image.jpg"
	}
	// Strip query params
	if idx := strings.Index(filename, "?"); idx != -1 {
		filename = filename[:idx]
	}

	// Build multipart body
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return 0, fmt.Errorf("create form file: %w", err)
	}
	if _, err := fw.Write(imgData); err != nil {
		return 0, fmt.Errorf("write image data: %w", err)
	}
	mw.Close()

	uploadReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.BaseURL+"/v1/assets", &buf)
	if err != nil {
		return 0, err
	}
	uploadReq.Header.Set("Content-Type", mw.FormDataContentType())
	uploadReq.Header.Set("Authorization", "Bearer "+g.APIKey)

	resp, err := g.client.Do(uploadReq)
	if err != nil {
		return 0, fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("upload HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var asset gagaAssetResp
	if err := json.Unmarshal(respBody, &asset); err != nil {
		return 0, fmt.Errorf("parse asset response: %w body=%s", err, string(respBody))
	}
	if asset.ID == 0 {
		return 0, fmt.Errorf("gaga: no asset id in response: %s", string(respBody))
	}
	return asset.ID, nil
}

// submit —— 提交视频生成任务，返回生成任务 ID (Long)
func (g *GagaGenerator) submit(ctx context.Context, req VideoGenerateReq, assetID int64) (int64, error) {
	dur := int(req.DurationSec)
	if dur <= 0 {
		dur = 5
	}

	aspectRatio := "16:9"
	if req.AspectRatio != "" {
		aspectRatio = req.AspectRatio
	}

	body := gagaSubmitReq{
		Model:       g.Model,
		Resolution:  "720p",
		AspectRatio: aspectRatio,
		Source: gagaSource{
			Type:    "image",
			Content: assetID,
		},
		Chunks: []gagaChunk{
			{
				Duration: dur,
				Conditions: []gagaCondition{
					{Type: "text", Content: req.Prompt},
				},
				EnablePromptEnhancement: true,
			},
		},
	}
	b, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.BaseURL+"/v1/generations", bytes.NewReader(b))
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.APIKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 429 {
		return 0, fmt.Errorf("gaga rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("gaga HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result gagaGenerationResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("parse submit response: %w body=%s", err, string(respBody))
	}
	if result.ID == 0 {
		return 0, fmt.Errorf("gaga: no id in response: %s", string(respBody))
	}
	return result.ID, nil
}

// poll —— 轮询任务状态直到完成、失败或超时（15分钟）
func (g *GagaGenerator) poll(ctx context.Context, genID int64) (*VideoClip, error) {
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("gaga: generation %d timed out after 15min", genID)
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
func (g *GagaGenerator) queryTask(ctx context.Context, genID int64) (*VideoClip, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/generations/%d", g.BaseURL, genID), nil)
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
	case "completed", "success":
		// Try creations array first, then top-level url
		videoURL := result.VideoURL
		if videoURL == "" && len(result.Creations) > 0 {
			videoURL = result.Creations[0].URL
		}
		if videoURL == "" {
			return nil, false, fmt.Errorf("gaga: completed but no video url in response: %s", string(respBody))
		}
		return &VideoClip{
			ClipURL:     videoURL,
			DurationSec: 5,
			ModelUsed:   g.Name(),
		}, true, nil
	case "failed":
		return nil, false, fmt.Errorf("gaga: task %d failed", genID)
	default: // pending, processing, running
		return nil, false, nil
	}
}

