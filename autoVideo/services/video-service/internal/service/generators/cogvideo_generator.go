package generators

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// CogVideoGenerator wraps the CogVideoX model via Replicate.
type CogVideoGenerator struct {
	APIKey string
	client *http.Client
}

// NewCogVideoGenerator —— 创建 CogVideoX 生成器实例
func NewCogVideoGenerator(apiKey string) *CogVideoGenerator {
	return &CogVideoGenerator{
		APIKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Name —— 返回生成器名称 "cogvideo"
func (g *CogVideoGenerator) Name() string { return "cogvideo" }

// IsAvailable —— 检查 API Key 是否已配置
func (g *CogVideoGenerator) IsAvailable(ctx context.Context) bool {
	return g.APIKey != ""
}

// SupportsNativeAudio —— CogVideo does not embed audio in generated clips.
func (g *CogVideoGenerator) SupportsNativeAudio() bool { return false }

// ParamOptions —— CogVideo 固定 5 秒单段，无可配参数
func (g *CogVideoGenerator) ParamOptions() []ModelParamOption {
	return []ModelParamOption{
		{
			Key: "duration", Label: "时长", Default: "5",
			Values: []ParamValue{{Value: "5", Label: "5秒（固定）"}},
		},
	}
}

type replicateCreateReq struct {
	Version string         `json:"version,omitempty"`
	Input   map[string]any `json:"input"`
}

type replicatePrediction struct {
	ID     string `json:"id"`
	Status string `json:"status"` // starting / processing / succeeded / failed / canceled
	Output any    `json:"output"` // string or []string
	Error  string `json:"error"`
}

const cogVideoModel = "thudm/cogvideox-5b"
const replicateBase = "https://api.replicate.com/v1"

// Generate —— 提交视频生成任务并轮询等待完成，返回 *VideoClip
func (g *CogVideoGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	predID, err := g.submit(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("cogvideo submit: %w", err)
	}
	clip, err := g.poll(ctx, predID)
	if err != nil {
		return nil, fmt.Errorf("cogvideo poll: %w", err)
	}
	return clip, nil
}

// submit —— 向 Replicate API 提交预测请求，返回 prediction ID
func (g *CogVideoGenerator) submit(ctx context.Context, req VideoGenerateReq) (string, error) {
	body := replicateCreateReq{
		Input: map[string]any{
			"image":  req.SourceImageURL,
			"prompt": req.Prompt,
		},
	}
	b, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/models/%s/predictions", replicateBase, cogVideoModel)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Token "+g.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var pred replicatePrediction
	if err := json.NewDecoder(resp.Body).Decode(&pred); err != nil {
		return "", err
	}
	if pred.ID == "" {
		return "", errors.New("cogvideo: empty prediction id")
	}
	return pred.ID, nil
}

// poll —— 轮询 Replicate 预测结果直到完成或失败（15分钟超时）
func (g *CogVideoGenerator) poll(ctx context.Context, predID string) (*VideoClip, error) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	timeout := time.After(15 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("cogvideo poll timeout after 15 minutes (prediction %s)", predID)
		case <-ticker.C:
			clip, done, err := g.queryPrediction(ctx, predID)
			if err != nil {
				return nil, err
			}
			if done {
				return clip, nil
			}
		}
	}
}

// queryPrediction —— 查询单次预测状态，返回结果和是否完成
func (g *CogVideoGenerator) queryPrediction(ctx context.Context, predID string) (*VideoClip, bool, error) {
	url := fmt.Sprintf("%s/predictions/%s", replicateBase, predID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	httpReq.Header.Set("Authorization", "Token "+g.APIKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	var pred replicatePrediction
	if err := json.NewDecoder(resp.Body).Decode(&pred); err != nil {
		return nil, false, err
	}

	switch pred.Status {
	case "succeeded":
		videoURL, err := extractFirstString(pred.Output)
		if err != nil {
			return nil, false, fmt.Errorf("cogvideo: %w", err)
		}
		// CogVideoX default output is ~6 seconds
		dur := 6.0
		if d, ok := pred.Output.(map[string]any); ok {
			if v, exists := d["duration"]; exists {
				if f, ok := v.(float64); ok && f > 0 {
					dur = f
				}
			}
		}
		return &VideoClip{
			ClipURL:     videoURL,
			DurationSec: dur,
			ModelUsed:   g.Name(),
		}, true, nil
	case "failed", "canceled":
		return nil, false, fmt.Errorf("cogvideo: prediction %s — %s", pred.Status, pred.Error)
	default:
		return nil, false, nil
	}
}

// extractFirstString —— 从接口值中提取第一个字符串（支持 string 或 []any 类型）
func extractFirstString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case []any:
		if len(x) > 0 {
			if s, ok := x[0].(string); ok {
				return s, nil
			}
		}
	}
	return "", errors.New("unexpected output format")
}
