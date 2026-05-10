package generators

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/autovideo/video-service/pkg/keyrotator"
)

// klingJWTToken generates a signed JWT for the Kling official API (api.klingai.com).
// key format: "accessKey:secretKey" — accessKey becomes JWT iss, secretKey signs with HS256.
// Returns the raw apiKey unchanged for proxy services that accept Bearer tokens directly.
func klingJWTToken(apiKey string) string {
	idx := strings.Index(apiKey, ":")
	if idx <= 0 {
		return apiKey // not a key pair — use as-is (proxy / legacy)
	}
	accessKey := apiKey[:idx]
	secretKey := apiKey[idx+1:]

	now := time.Now().Unix()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]interface{}{
		"iss": accessKey,
		"exp": now + 1800,
		"nbf": now - 5,
	})
	payloadEnc := base64.RawURLEncoding.EncodeToString(payload)
	sigData := header + "." + payloadEnc
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(sigData))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return sigData + "." + sig
}

// KlingGenerator wraps the Kuaishou Kling image-to-video API.
type KlingGenerator struct {
	APIKey      string
	rotator     *keyrotator.Rotator
	BaseURL     string // https://api.klingai.com
	ModelName   string // e.g. "kling-v1-6" or "kling-v3"
	OmniModel   string // e.g. "kling-v3-omni" for fusion video
	nameOverride string // optional registry name override (e.g. "aiping")
	client      *http.Client
}

// NewKlingGenerator —— 创建可灵视频生成器实例
// Provide a single apiKey or use NewKlingGeneratorWithKeys for multiple key rotation.
func NewKlingGenerator(apiKey, baseURL string) *KlingGenerator {
	return NewKlingGeneratorWithKeys(baseURL, apiKey)
}

// NewKlingGeneratorWithKeys creates a KlingGenerator that round-robins across
// multiple API keys to avoid rate limits.
func NewKlingGeneratorWithKeys(baseURL string, keys ...string) *KlingGenerator {
	if baseURL == "" {
		baseURL = "https://api.klingai.com"
	}
	r := keyrotator.New(keys...)
	primary := ""
	if r.Len() > 0 {
		primary = r.Next()
		// Reset counter so Next() starts from key[0] on first Generate() call.
	}
	return &KlingGenerator{
		APIKey:    primary,
		rotator:   keyrotator.New(keys...),
		BaseURL:   baseURL,
		ModelName: "kling-v1-6",
		OmniModel: "kling-v1-6",
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// nextToken returns the authorization token for the next request.
// If the key is in "accessKey:secretKey" format it produces a fresh JWT; otherwise returns the raw key.
func (g *KlingGenerator) nextToken() string {
	key := g.APIKey
	if g.rotator != nil && g.rotator.Len() > 0 {
		key = g.rotator.Next()
	}
	return klingJWTToken(key)
}

// Name —— 返回生成器名称 "kling" (or nameOverride if set)
func (g *KlingGenerator) Name() string {
	if g.nameOverride != "" {
		return g.nameOverride
	}
	return "kling"
}

// WithName overrides the registry name for this generator instance.
func (g *KlingGenerator) WithName(name string) *KlingGenerator {
	g.nameOverride = name
	return g
}

// WithModel sets the model name for video generation (e.g. "kling-v3" for Kling 3.0).
func (g *KlingGenerator) WithModel(model string) *KlingGenerator {
	if model != "" {
		g.ModelName = model
	}
	return g
}

// WithOmniModel sets the omni model name for fusion video generation.
func (g *KlingGenerator) WithOmniModel(model string) *KlingGenerator {
	if model != "" {
		g.OmniModel = model
	}
	return g
}

// Clone returns a shallow copy so per-request model selection does not mutate
// the shared generator instance kept in VideoService.
func (g *KlingGenerator) Clone() *KlingGenerator {
	clone := *g
	return &clone
}

// IsAvailable —— 检查 API Key 是否已配置
func (g *KlingGenerator) IsAvailable(ctx context.Context) bool {
	if g.rotator != nil && g.rotator.Len() > 0 {
		return true
	}
	return g.APIKey != ""
}

// SupportsNativeAudio —— Kling does not embed audio in generated clips.
func (g *KlingGenerator) SupportsNativeAudio() bool { return false }

// ParamOptions —— 可灵支持的模型参数（时长、画质模式、宽高比）
func (g *KlingGenerator) ParamOptions() []ModelParamOption {
	return []ModelParamOption{
		{
			Key: "duration", Label: "时长", Default: "5",
			Values: []ParamValue{{Value: "5", Label: "5秒"}, {Value: "10", Label: "10秒"}},
		},
		{
			Key: "video_mode", Label: "生成模式", Default: "std",
			Values: []ParamValue{{Value: "std", Label: "标准"}, {Value: "pro", Label: "专业"}},
		},
		{
			Key: "aspect_ratio", Label: "画面比例", Default: "16:9",
			Values: []ParamValue{
				{Value: "16:9", Label: "横屏 16:9"}, {Value: "9:16", Label: "竖屏 9:16"}, {Value: "1:1", Label: "方形 1:1"},
			},
		},
	}
}

type klingElement struct {
	ImageURL    string `json:"image_url"`
	Description string `json:"description,omitempty"`
}

type klingCreateReq struct {
	ModelName    string         `json:"model_name"`
	ImageURL     string         `json:"image_url"`
	TailImageURL string         `json:"tail_image_url,omitempty"`
	Prompt       string         `json:"prompt"`
	Duration     int            `json:"duration"`
	Mode         string         `json:"mode"`
	AspectRatio  string         `json:"aspect_ratio,omitempty"` // "16:9" "9:16" "1:1"
	Elements     []klingElement `json:"elements,omitempty"`
}

type klingCreateResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskID string `json:"task_id"`
	} `json:"data"`
}

type klingPollResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskStatus    string `json:"task_status"`     // processing / succeed / failed
		TaskStatusMsg string `json:"task_status_msg"` // failure reason when status=failed
		TaskResult    struct {
			Videos []struct {
				URL      string  `json:"url"`
				Duration float64 `json:"duration"`
			} `json:"videos"`
		} `json:"task_result"`
	} `json:"data"`
}

// Generate —— 提交图生视频任务并轮询等待完成，返回 *VideoClip
func (g *KlingGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	var taskID, token string
	err := RetrySubmit(ctx, 4, func() error {
		var e error
		taskID, token, e = g.submit(ctx, req)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("kling submit: %w", err)
	}

	// Reuse the same API token (same account) for all poll requests so that
	// multi-account key rotation does not cause "task not found" errors.
	clip, err := g.poll(ctx, taskID, token)
	if err != nil {
		return nil, fmt.Errorf("kling poll: %w", err)
	}
	return clip, nil
}

// submit —— 向可灵 API 提交图生视频请求，返回任务 ID 和本次使用的 token
func (g *KlingGenerator) submit(ctx context.Context, req VideoGenerateReq) (taskID string, token string, err error) {
	dur := int(req.DurationSec)
	if dur <= 0 {
		dur = 5
	}
	body := klingCreateReq{
		ModelName:    g.ModelName,
		ImageURL:     req.SourceImageURL,
		TailImageURL: req.TailImageURL,
		Prompt:       req.Prompt,
		Duration:     dur,
		Mode:         firstNonEmpty(req.VideoMode, "std"),
		AspectRatio:  req.AspectRatio, // omitted when empty → API uses its default (16:9)
	}
	// Attach character reference images for subject consistency when available.
	for i, charURL := range req.CharacterImageURLs {
		if i >= 3 { // Kling supports up to 3 elements
			break
		}
		body.Elements = append(body.Elements, klingElement{ImageURL: charURL})
	}

	b, _ := json.Marshal(body)
	httpReq, newErr := http.NewRequestWithContext(ctx, http.MethodPost,
		g.BaseURL+"/v1/videos/image2video", bytes.NewReader(b))
	if newErr != nil {
		return "", "", newErr
	}
	token = g.nextToken()
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, newErr := g.client.Do(httpReq)
	if newErr != nil {
		return "", "", newErr
	}
	defer resp.Body.Close()

	var result klingCreateResp
	if newErr = json.NewDecoder(resp.Body).Decode(&result); newErr != nil {
		return "", "", newErr
	}
	if result.Code != 0 {
		return "", "", fmt.Errorf("kling error %d: %s", result.Code, result.Message)
	}
	return result.Data.TaskID, token, nil
}

// poll —— 轮询可灵任务状态直到完成或失败（15分钟超时）
func (g *KlingGenerator) poll(ctx context.Context, taskID string, token string) (*VideoClip, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	timeout := time.After(15 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("kling poll timeout after 15 minutes (task %s)", taskID)
		case <-ticker.C:
			clip, done, err := g.queryTask(ctx, taskID, token)
			if err != nil {
				return nil, err
			}
			if done {
				return clip, nil
			}
		}
	}
}

// queryTask —— 查询可灵任务状态，返回结果和是否完成
// token must be the same token used in submit() to avoid cross-account lookup failures.
func (g *KlingGenerator) queryTask(ctx context.Context, taskID string, token string) (*VideoClip, bool, error) {
	url := fmt.Sprintf("%s/v1/videos/tasks/%s", g.BaseURL, taskID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	var result klingPollResp
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, false, err
	}
	if result.Code != 0 {
		return nil, false, fmt.Errorf("kling poll error %d: %s", result.Code, result.Message)
	}

	switch result.Data.TaskStatus {
	case "succeed":
		videos := result.Data.TaskResult.Videos
		if len(videos) == 0 {
			return nil, false, errors.New("kling: no video in result")
		}
		return &VideoClip{
			ClipURL:     videos[0].URL,
			DurationSec: videos[0].Duration,
			ModelUsed:   resolvedModelUsed(g.ModelName, g.Name()),
		}, true, nil
	case "failed":
		reason := strings.TrimSpace(result.Data.TaskStatusMsg)
		if reason == "" {
			reason = strings.TrimSpace(result.Message)
		}
		if reason == "" {
			reason = "no details"
		}
		return nil, false, fmt.Errorf("kling: task %s failed — %s", taskID, reason)
	default:
		// still processing
		return nil, false, nil
	}
}
