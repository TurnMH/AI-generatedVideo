package generators

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// BaiduBCEGenerator wraps the Baidu BCE VOD image-to-video API.
// Auth: BCE-AUTH-V1 (HMAC-SHA256 signed requests).
// API docs: https://cloud.baidu.com/doc/VDB/index.html
//   - Create: POST https://vod.bj.baidubce.com/v2/aigc/image_to_video
//   - Query:  GET  https://vod.bj.baidubce.com/v2/tasks/{taskId}
//   - Status: PENDING / RUNNING / SUCCESS / FAILED
//   - Models: V10 / V15 / V20 (720p, recommended) / VQ1 (high quality)
type BaiduBCEGenerator struct {
	AccessKey string
	SecretKey string
	Model     string // e.g. "V20"
	client    *http.Client
}

const (
	bceHost     = "vod.bj.baidubce.com"
	bceCreatePath = "/v2/aigc/image_to_video"
	bceQueryPath  = "/v2/tasks/"
	bceExpiry   = 1800 // seconds
)

// NewBaiduBCEGenerator —— 创建百度 BCE 视频生成器实例
func NewBaiduBCEGenerator(accessKey, secretKey, model string) *BaiduBCEGenerator {
	if model == "" {
		model = "V20"
	}
	return &BaiduBCEGenerator{
		AccessKey: accessKey,
		SecretKey: secretKey,
		Model:     model,
		client:    &http.Client{Timeout: 60 * time.Second},
	}
}

// Name —— 返回生成器名称
func (g *BaiduBCEGenerator) Name() string { return "baidu-bce" }

// IsAvailable —— 检查 API 凭证是否已配置
func (g *BaiduBCEGenerator) IsAvailable(_ context.Context) bool {
	return g.AccessKey != "" && g.SecretKey != ""
}

// SupportsNativeAudio —— 百度 BCE 视频不支持原生音频嵌入
func (g *BaiduBCEGenerator) SupportsNativeAudio() bool { return false }

// ParamOptions —— 百度 BCE 支持的参数
func (g *BaiduBCEGenerator) ParamOptions() []ModelParamOption {
	return []ModelParamOption{
		{
			Key: "duration", Label: "时长", Default: "4",
			Values: []ParamValue{{Value: "4", Label: "4秒"}, {Value: "8", Label: "8秒"}},
		},
		{
			Key: "aspect_ratio", Label: "画面比例", Default: "16:9",
			Values: []ParamValue{
				{Value: "16:9", Label: "横屏 16:9"}, {Value: "9:16", Label: "竖屏 9:16"}, {Value: "1:1", Label: "方形 1:1"},
			},
		},
	}
}

// ── BCE-AUTH-V1 signing ──────────────────────────────────────

// bceSign returns a BCE-AUTH-V1 Authorization header value.
// signHeaders must be a subset of the actual request headers (sorted, lowercased keys).
func (g *BaiduBCEGenerator) bceSign(method, path, queryStr string, headers map[string]string) string {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	authPrefix := fmt.Sprintf("BCE-AUTH-V1/%s/%s/%d", g.AccessKey, timestamp, bceExpiry)

	// Derive signing key
	signingKey := bceHMACSHA256(g.SecretKey, authPrefix)

	// Canonical URI (percent-encode everything except '/')
	canonicalURI := percentEncodeURI(path)

	// Canonical query string (sorted)
	canonicalQuery := ""
	if queryStr != "" {
		params, _ := url.ParseQuery(queryStr)
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(params.Get(k)))
		}
		canonicalQuery = strings.Join(parts, "&")
	}

	// Canonical headers (sorted by lowercased key)
	signHeaderKeys := make([]string, 0, len(headers))
	for k := range headers {
		signHeaderKeys = append(signHeaderKeys, strings.ToLower(k))
	}
	sort.Strings(signHeaderKeys)

	// Build a lookup map of lowercased key → value
	lowerHeaders := make(map[string]string, len(headers))
	for k, v := range headers {
		lowerHeaders[strings.ToLower(k)] = strings.TrimSpace(v)
	}

	canonicalHeaderParts := make([]string, 0, len(signHeaderKeys))
	for _, k := range signHeaderKeys {
		canonicalHeaderParts = append(canonicalHeaderParts, url.QueryEscape(k)+":"+url.QueryEscape(lowerHeaders[k]))
	}
	canonicalHeaders := strings.Join(canonicalHeaderParts, "\n")
	signedHeaders := strings.Join(signHeaderKeys, ";")

	canonicalRequest := strings.Join([]string{
		strings.ToUpper(method),
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
	}, "\n")

	signature := bceHMACSHA256(signingKey, canonicalRequest)
	return fmt.Sprintf("%s/%s/%s", authPrefix, signedHeaders, signature)
}

func bceHMACSHA256(key, data string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// percentEncodeURI encodes a URI path, preserving '/' separators.
func percentEncodeURI(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.QueryEscape(p)
	}
	return strings.Join(parts, "/")
}

// ── API types ────────────────────────────────────────────────

type bceCreateResp struct {
	TaskID  string `json:"taskId"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type bceQueryResp struct {
	TaskID string `json:"taskId"`
	Status string `json:"status"` // PENDING / RUNNING / SUCCESS / FAILED
	Result *struct {
		Output *struct {
			VideoURL string `json:"videoUrl"`
		} `json:"output"`
	} `json:"result"`
	// Some API versions flatten the output
	Output *struct {
		VideoURL string `json:"videoUrl"`
	} `json:"output"`
	Message string `json:"message"`
}

// ── Generate ────────────────────────────────────────────────

// Generate —— 提交百度 BCE 图生视频任务并轮询等待完成，返回 *VideoClip
func (g *BaiduBCEGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	taskID, err := g.submit(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("baidu-bce submit: %w", err)
	}
	clip, err := g.poll(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("baidu-bce poll %s: %w", taskID, err)
	}
	return clip, nil
}

// submit —— 提交视频生成任务，返回任务 ID
func (g *BaiduBCEGenerator) submit(ctx context.Context, req VideoGenerateReq) (string, error) {
	dur := int(req.DurationSec)
	if dur <= 0 {
		dur = 4
	}
	ratio := req.AspectRatio
	if ratio == "" {
		ratio = "16:9"
	}

	// Request body: model field + model{Name}TaskInput
	taskInput := map[string]interface{}{
		"image":             map[string]string{"imageUrl": req.SourceImageURL},
		"prompt":            req.Prompt,
		"duration":          dur,
		"resolution":        "720P",
		"aspectRatio":       ratio,
		"movementAmplitude": "auto",
	}
	inputKey := "model" + g.Model + "TaskInput"
	bodyMap := map[string]interface{}{
		"model": g.Model,
		inputKey: taskInput,
	}
	b, _ := json.Marshal(bodyMap)

	endpoint := "https://" + bceHost + bceCreatePath
	xBceDate := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	signHeaders := map[string]string{
		"content-type": "application/json",
		"host":         bceHost,
		"x-bce-date":   xBceDate,
	}
	auth := g.bceSign(http.MethodPost, bceCreatePath, "", signHeaders)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Host", bceHost)
	httpReq.Header.Set("x-bce-date", xBceDate)
	httpReq.Header.Set("Authorization", auth)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("baidu-bce HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result bceCreateResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse create response: %w body=%s", err, string(respBody))
	}
	if result.TaskID == "" {
		return "", fmt.Errorf("baidu-bce: no taskId in response: %s", string(respBody))
	}
	return result.TaskID, nil
}

// poll —— 轮询任务状态直到完成、失败或超时（15分钟）
func (g *BaiduBCEGenerator) poll(ctx context.Context, taskID string) (*VideoClip, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	timeout := time.After(15 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("baidu-bce: task %s timed out after 15min", taskID)
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
func (g *BaiduBCEGenerator) queryTask(ctx context.Context, taskID string) (*VideoClip, bool, error) {
	path := bceQueryPath + taskID
	endpoint := "https://" + bceHost + path
	xBceDate := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	signHeaders := map[string]string{
		"host":       bceHost,
		"x-bce-date": xBceDate,
	}
	auth := g.bceSign(http.MethodGet, path, "", signHeaders)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false, err
	}
	httpReq.Header.Set("Host", bceHost)
	httpReq.Header.Set("x-bce-date", xBceDate)
	httpReq.Header.Set("Authorization", auth)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, false, nil // transient
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result bceQueryResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, false, nil // transient
	}

	switch result.Status {
	case "SUCCESS":
		// Extract videoUrl from either result.output or top-level output
		videoURL := ""
		if result.Result != nil && result.Result.Output != nil {
			videoURL = result.Result.Output.VideoURL
		} else if result.Output != nil {
			videoURL = result.Output.VideoURL
		}
		if videoURL == "" {
			return nil, false, fmt.Errorf("baidu-bce: succeeded but no videoUrl in response")
		}
		return &VideoClip{
			ClipURL:     videoURL,
			DurationSec: 4,
			ModelUsed:   g.Name(),
		}, true, nil
	case "FAILED":
		return nil, false, fmt.Errorf("baidu-bce: task %s failed: %s", taskID, result.Message)
	default: // PENDING, RUNNING
		return nil, false, nil
	}
}
