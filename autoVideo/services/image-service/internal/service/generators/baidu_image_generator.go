package generators

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
)

// baiduImageGenerator calls the Baidu BCE VOD aigc image API.
//
// API: POST https://vod.bj.baidubce.com/v3/aigc/image
//   - Auth: "Authorization: Bearer {bce-v3/ALTAK-...}" for task creation
//   - Request: {"model":"NB","messages":[{"role":"user","content":[{"type":"text","text":"..."}]}]}
//   - Response (create): {"taskId":"tsk-..."}
//   - Response (poll): {"taskId":"...","status":"SUCCESS","imageUrl":"..."}
//
// Task polling uses BCE-AUTH-V1 HMAC-SHA256 signed GET:
//   GET /v3/aigc/image/tasks/{taskId}
//
// Available models: NB, NBP, NB2, I4YG1, I4FG1, I4G1
type baiduImageGenerator struct {
	bearerToken  string // full "bce-v3/ALTAK-..." token used as Bearer for task creation
	accessKey    string // AK for BCE-AUTH-V1 signing (task polling); derived from token if empty
	secretKey    string // SK for BCE-AUTH-V1 signing (task polling)
	endpoint     string // e.g. "https://vod.bj.baidubce.com/v3/aigc/image"
	model        string // e.g. "NB", "NBP", "I4G1"
	genKey       string // registry key
	pollInterval time.Duration // how often to poll task status; defaults to baiduImagePollInterval
	client       *http.Client
	logger       *zap.Logger
}

const (
	baiduImageDefaultBase   = "https://vod.bj.baidubce.com/v3/aigc/image"
	baiduImageDefaultModel  = "NB"
	baiduImagePollInterval  = 5 * time.Second
	baiduImagePollTimeout   = 120 * time.Second
)

// NewBaiduImageGenerator creates a Baidu BCE image generator.
//
//   - bearerToken: full "bce-v3/ALTAK-..." token (used with Bearer prefix for task creation)
//   - accessKey / secretKey: BCE-AUTH-V1 credentials for task polling; if both empty,
//     the generator will attempt to split the bearerToken as "bce-v3/{AK}/{SK}"
//   - baseURL: API endpoint, defaults to https://vod.bj.baidubce.com/v3/aigc/image
//   - model: one of NB/NBP/NB2/I4YG1/I4FG1/I4G1, defaults to NB
func NewBaiduImageGenerator(bearerToken, baseURL, model, genKey string, logger *zap.Logger) ImageGenerator {
	return newBaiduImageGeneratorFull(bearerToken, "", "", baseURL, model, genKey, logger)
}

// NewBaiduImageGeneratorWithCredentials is like NewBaiduImageGenerator but also
// accepts explicit AK/SK for BCE-AUTH-V1 signing used when polling task status.
func NewBaiduImageGeneratorWithCredentials(bearerToken, ak, sk, baseURL, model, genKey string, logger *zap.Logger) ImageGenerator {
	return newBaiduImageGeneratorFull(bearerToken, ak, sk, baseURL, model, genKey, logger)
}

func newBaiduImageGeneratorFull(bearerToken, ak, sk, baseURL, model, genKey string, logger *zap.Logger) ImageGenerator {
	if bearerToken == "" {
		return &baiduImageGenerator{logger: logger}
	}
	if baseURL == "" {
		baseURL = baiduImageDefaultBase
	}
	if model == "" {
		model = baiduImageDefaultModel
	}
	if genKey == "" {
		genKey = "baidu-img"
	}
	// Attempt to derive AK/SK from "bce-v3/{AK}/{SK}" bearer token format if not provided.
	if ak == "" || sk == "" {
		parts := strings.SplitN(bearerToken, "/", 3)
		if len(parts) == 3 {
			ak = parts[1]
			sk = parts[2]
		}
	}
	return &baiduImageGenerator{
		bearerToken:  bearerToken,
		accessKey:    ak,
		secretKey:    sk,
		endpoint:     strings.TrimRight(baseURL, "/"),
		model:        model,
		genKey:       genKey,
		pollInterval: baiduImagePollInterval,
		client:       &http.Client{Timeout: 150 * time.Second},
		logger:       logger,
	}
}

func (g *baiduImageGenerator) Name() string { return g.genKey }
func (g *baiduImageGenerator) IsAvailable(_ context.Context) bool {
	return g.bearerToken != ""
}

// RefCapability —— 百度图片生成器走多图融合模式（messages.content[] inline base64）。
func (g *baiduImageGenerator) RefCapability() RefCapability {
	return RefCapability{Mode: RefModeFusion, MaxRefs: 6, StrongRef: false}
}

// baiduImageCreateReq mirrors the Baidu image task creation body.
type baiduImageCreateReq struct {
	Model    string                `json:"model"`
	Messages []baiduImageMessage   `json:"messages"`
}

type baiduImageMessage struct {
	Role    string               `json:"role"`
	Content []baiduImageContent  `json:"content"`
}

type baiduImageContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Data     string `json:"data,omitempty"`     // base64 inline image
	MIMEType string `json:"mime_type,omitempty"`
}

type baiduImageCreateResp struct {
	TaskID  string `json:"taskId"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type baiduImagePollResp struct {
	TaskID   string `json:"taskId"`
	Status   string `json:"status"` // PENDING / RUNNING / SUCCESS / FAILED
	ImageURL string `json:"imageUrl"`
	// Inline image data (some models return base64 directly)
	ImageData string `json:"imageData"`
	MIMEType  string `json:"mimeType"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// Generate submits a text-to-image request to the Baidu BCE image API and polls for the result.
func (g *baiduImageGenerator) Generate(ctx context.Context, req GenerateReq) (*GenerateRes, error) {
	if g.bearerToken == "" {
		return nil, fmt.Errorf("baidu image: no API key configured")
	}

	// Baidu ERNIE-iRAG / Nano-Banana aligned image endpoints respond best to
	// the Chinese structured prompt block (same layer DALL-E uses in
	// English). Routing through the shared builder also folds in the
	// 4-panel character-sheet explanation; previously req.Prompt was used
	// verbatim and it was lost.
	prompt := buildChineseStructuredPrompt(req, false)
	if strings.TrimSpace(prompt) == "" {
		prompt = strings.TrimSpace(req.Prompt)
	}
	// buildChineseStructuredPrompt only includes task-level negatives; the
	// caller-supplied NegativePrompt still needs to be appended explicitly
	// so ERNIE sees both (mirrors how CogView's builder adds it separately).
	if userNeg := strings.TrimSpace(ChineseLangNegativeInstruction(req.NegativePrompt)); userNeg != "" {
		if prompt != "" {
			prompt = prompt + userNeg
		} else {
			prompt = userNeg
		}
	}
	if prompt == "" {
		prompt = "Generate an image."
	}

	// Build messages array
	contents := []baiduImageContent{{Type: "text", Text: prompt}}

	// Attach reference images as inline base64 if provided. Iterate over every
	// reference URL (character + scene assets) so ERNIE-iRAG / Baidu image-edit
	// can see all relevant subjects, not just the primary character.
	var helperGen = &geminiImageGenerator{client: g.client, logger: g.logger}
	refs := req.AllReferenceImageURLs()
	const maxRefs = 4
	if len(refs) > maxRefs {
		refs = refs[:maxRefs]
	}
	for _, imgURL := range refs {
		if imgURL == "" {
			continue
		}
		inlineData, err := helperGen.fetchAndEncode(ctx, imgURL)
		if err != nil {
			g.logger.Warn("baidu image: failed to fetch reference; skipping",
				zap.String("url", imgURL), zap.Error(err))
			continue
		}
		contents = append(contents, baiduImageContent{
			Type:     "image",
			Data:     inlineData.Data,
			MIMEType: inlineData.MimeType,
		})
	}

	payload := baiduImageCreateReq{
		Model:    g.model,
		Messages: []baiduImageMessage{{Role: "user", Content: contents}},
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// Task creation uses "Authorization: Bearer {bce-v3/ALTAK-...}"
	httpReq.Header.Set("Authorization", "Bearer "+g.bearerToken)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("create task http %d: %s", resp.StatusCode, string(raw))
	}

	var createResp baiduImageCreateResp
	if err := json.Unmarshal(raw, &createResp); err != nil {
		return nil, fmt.Errorf("decode create response: %w", err)
	}
	if createResp.Code != "" && createResp.Code != "200" {
		return nil, fmt.Errorf("create task error %s: %s", createResp.Code, createResp.Message)
	}
	if createResp.TaskID == "" {
		return nil, fmt.Errorf("no taskId in response: %s", string(raw))
	}

	g.logger.Info("baidu image: task created, polling...", zap.String("taskId", createResp.TaskID))
	return g.pollTask(ctx, createResp.TaskID)
}

// pollTask polls GET /v3/aigc/image/tasks/{taskId} with BCE-AUTH-V1 signing until the task completes.
func (g *baiduImageGenerator) pollTask(ctx context.Context, taskID string) (*GenerateRes, error) {
	deadline := time.Now().Add(baiduImagePollTimeout)
	taskPath := "/v3/aigc/image/tasks/" + taskID

	// Build the task URL: same host as endpoint, but with the tasks path.
	parsed, _ := url.Parse(g.endpoint)
	taskURL := parsed.Scheme + "://" + parsed.Host + taskPath

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("baidu image: polling timed out for task %s", taskID)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(g.pollInterval):
		}

		result, done, err := g.queryTask(ctx, taskURL, taskPath)
		if done {
			// Task reached terminal state (SUCCESS or FAILED)
			return result, err
		}
		if err != nil {
			// Transient poll error (network, auth, etc.) — retry
			g.logger.Warn("baidu image: poll error, retrying", zap.String("taskId", taskID), zap.Error(err))
		}
	}
}

// queryTask fetches task status once. Returns (result, done, err).
// done=true means either SUCCESS or FAILED; done=false means still running.
func (g *baiduImageGenerator) queryTask(ctx context.Context, taskURL, taskPath string) (*GenerateRes, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, taskURL, nil)
	if err != nil {
		return nil, false, err
	}

	// Try Bearer token first for GET (same as POST creation).
	// BCE-AUTH-V1 HMAC signing is only used as fallback when explicit non-ALTAK AK/SK are provided
	// (bce-v3/ALTAK-... tokens cannot be split into valid AK/SK for HMAC).
	useHMAC := g.accessKey != "" && g.secretKey != "" && !strings.HasPrefix(g.accessKey, "ALTAK-")
	if useHMAC {
		ts := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		auth := g.bceSign("GET", taskPath, ts)
		httpReq.Header.Set("Authorization", auth)
		httpReq.Header.Set("x-bce-date", ts)
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+g.bearerToken)
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, false, fmt.Errorf("poll http: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	// 4xx = client error (auth, not found, etc.) → terminal failure, don't retry
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, true, fmt.Errorf("poll http %d: %s", resp.StatusCode, string(raw))
	}
	// 5xx = server error → retryable
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("poll http %d: %s", resp.StatusCode, string(raw))
	}

	var pollResp baiduImagePollResp
	if err := json.Unmarshal(raw, &pollResp); err != nil {
		return nil, false, fmt.Errorf("decode poll response: %w", err)
	}

	switch pollResp.Status {
	case "SUCCESS":
		if pollResp.ImageURL != "" {
			return &GenerateRes{ImageURL: pollResp.ImageURL, ModelUsed: g.genKey}, true, nil
		}
		if pollResp.ImageData != "" {
			mimeType := pollResp.MIMEType
			if mimeType == "" {
				mimeType = "image/png"
			}
			// Validate it's valid base64
			if _, decErr := base64.StdEncoding.DecodeString(pollResp.ImageData[:min64(len(pollResp.ImageData), 4)]); decErr == nil {
				dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, pollResp.ImageData)
				return &GenerateRes{ImageURL: dataURI, ModelUsed: g.genKey}, true, nil
			}
		}
		return nil, true, fmt.Errorf("task %s succeeded but no image data", pollResp.TaskID)
	case "FAILED":
		return nil, true, fmt.Errorf("task %s failed: %s", pollResp.TaskID, pollResp.Message)
	case "PENDING", "RUNNING", "":
		return nil, false, nil
	default:
		return nil, false, nil
	}
}

func min64(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// bceSign returns a BCE-AUTH-V1 Authorization header value using HMAC-SHA256.
func (g *baiduImageGenerator) bceSign(method, path, ts string) string {
	expiry := 1800
	authPrefix := fmt.Sprintf("BCE-AUTH-V1/%s/%s/%d", g.accessKey, ts, expiry)
	signingKey := baiduHMACSHA256(g.secretKey, authPrefix)

	// Parse host from endpoint
	parsed, _ := url.Parse(g.endpoint)
	host := parsed.Hostname()

	signHeaders := map[string]string{
		"host":       host,
		"x-bce-date": ts,
	}
	signHeaderKeys := make([]string, 0, len(signHeaders))
	for k := range signHeaders {
		signHeaderKeys = append(signHeaderKeys, k)
	}
	sort.Strings(signHeaderKeys)

	canonicalHeaderParts := make([]string, 0, len(signHeaderKeys))
	for _, k := range signHeaderKeys {
		canonicalHeaderParts = append(canonicalHeaderParts, url.QueryEscape(k)+":"+url.QueryEscape(signHeaders[k]))
	}
	canonicalHeaders := strings.Join(canonicalHeaderParts, "\n")
	signedHeaders := strings.Join(signHeaderKeys, ";")

	canonicalURI := baiduPercentEncodeURI(path)
	canonicalRequest := strings.Join([]string{strings.ToUpper(method), canonicalURI, "", canonicalHeaders}, "\n")
	signature := baiduHMACSHA256(signingKey, canonicalRequest)
	return fmt.Sprintf("%s/%s/%s", authPrefix, signedHeaders, signature)
}

func baiduHMACSHA256(key, data string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func baiduPercentEncodeURI(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.QueryEscape(p)
	}
	return strings.Join(parts, "/")
}
