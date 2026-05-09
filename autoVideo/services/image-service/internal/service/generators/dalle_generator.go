package generators

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

const defaultOpenAIBase = "https://api.openai.com"

type dalleGenerator struct {
	keys         *smartKeyPool
	baseURL      string
	imagesURL    string // full URL for the /images/generations endpoint
	modelName    string // configurable model name, e.g. "gpt-image-1.5", "dall-e-3"
	generatorKey string // key used to identify this generator
	maxPromptLen int    // 0 = unlimited; >0 = hard rune cap on prompt before sending
	client       *http.Client
	logger       *zap.Logger
}

// NewDalleGenerator —— 创建 DALL-E 图片生成器实例，返回 ImageGenerator 接口
func NewDalleGenerator(apiKeys []string, baseURL string, logger *zap.Logger) ImageGenerator {
	return NewDalleGeneratorForModel(apiKeys, baseURL, "gpt-image-1.5", "dalle", logger)
}

// NewDalleGeneratorForModel —— 创建指定模型名称的 OpenAI 兼容图片生成器
func NewDalleGeneratorForModel(apiKeys []string, baseURL, modelName, generatorKey string, logger *zap.Logger) ImageGenerator {
	if baseURL == "" {
		baseURL = defaultOpenAIBase
	}
	// Strip trailing slash and version suffix to get a clean root URL.
	baseURL = strings.TrimRight(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")

	// CogView (ZhipuAI) uses /v4/images/generations — the base URL in config
	// already contains /v4 (e.g. https://open.bigmodel.cn/api/paas/v4), so strip
	// it here and re-add it as part of the explicit path to avoid /v4/v1/… duplication.
	var imagesURL string
	if isCogViewModel(modelName) {
		cleanBase := strings.TrimSuffix(baseURL, "/v4")
		imagesURL = cleanBase + "/v4/images/generations"
	} else {
		imagesURL = baseURL + "/v1/images/generations"
	}

	return &dalleGenerator{
		keys:         newSmartKeyPool(apiKeys),
		baseURL:      baseURL,
		imagesURL:    imagesURL,
		modelName:    modelName,
		generatorKey: generatorKey,
		client:       &http.Client{Timeout: 120 * time.Second},
		logger:       logger,
	}
}

// NewDalleGeneratorWithEndpoint —— 创建 OpenAI 兼容图片生成器，支持自定义完整 endpoint URL。
// 用于豆包 Ark (ark.cn-beijing.volces.com/api/v3/images/generations)、千帆等非标 v1 路径的渠道。
// maxPromptLen: 0 = 不限制；>0 = prompt 超出时截断（千帆等有严格字符上限的渠道需要设置）。
func NewDalleGeneratorWithEndpoint(apiKeys []string, fullEndpoint, modelName, generatorKey string, logger *zap.Logger) ImageGenerator {
	return NewDalleGeneratorWithEndpointAndCap(apiKeys, fullEndpoint, modelName, generatorKey, 0, logger)
}

// NewDalleGeneratorWithEndpointAndCap —— 同 NewDalleGeneratorWithEndpoint，额外支持 prompt 长度上限。
func NewDalleGeneratorWithEndpointAndCap(apiKeys []string, fullEndpoint, modelName, generatorKey string, maxPromptLen int, logger *zap.Logger) ImageGenerator {
	return &dalleGenerator{
		keys:         newSmartKeyPool(apiKeys),
		baseURL:      fullEndpoint,
		imagesURL:    fullEndpoint,
		modelName:    modelName,
		generatorKey: generatorKey,
		maxPromptLen: maxPromptLen,
		client:       &http.Client{Timeout: 120 * time.Second},
		logger:       logger,
	}
}

// Name —— 返回生成器名称
func (g *dalleGenerator) Name() string { return g.generatorKey }

// IsAvailable —— 检查 DALL-E 生成器是否可用（API Key 非空时返回 true）
func (g *dalleGenerator) IsAvailable(ctx context.Context) bool {
	return g.keys.size() > 0
}

// RefCapability —— OpenAI 兼容模型参考图能力：
// gpt-image-* 走 edits 多图融合；Doubao/CogView 走单图 ip-adapter；其余（DALL-E 3 等）纯文生图。
func (g *dalleGenerator) RefCapability() RefCapability {
	if isGPTImageModel(g.modelName) {
		return RefCapability{Mode: RefModeFusion, MaxRefs: 10, StrongRef: false}
	}
	if isDoubaoEndpoint(g.imagesURL) || isCogViewModel(g.modelName) {
		return RefCapability{Mode: RefModeIPAdapter, MaxRefs: 1, StrongRef: false}
	}
	return RefCapability{Mode: RefModeT2I, MaxRefs: 0, StrongRef: false}
}

// Generate —— 调用 DALL-E API 生成图片，支持重试和内容审核处理，返回生成结果
func (g *dalleGenerator) Generate(ctx context.Context, req GenerateReq) (*GenerateRes, error) {
	size := fmt.Sprintf("%dx%d", req.Width, req.Height)
	prompt := sanitizePrompt(buildDallePrompt(req, g.modelName))
	if g.maxPromptLen > 0 {
		if r := []rune(prompt); len(r) > g.maxPromptLen {
			prompt = string(r[:g.maxPromptLen])
			g.logger.Debug("dalle: prompt truncated", zap.String("generator", g.generatorKey), zap.Int("cap", g.maxPromptLen))
		}
	}
	// Qianfan API only accepts English prompts; strip CJK characters to avoid 400 errors.
	if isQianfanEndpoint(g.imagesURL) {
		prompt = stripCJKCharacters(prompt)
		if prompt == "" {
			prompt = "a beautiful illustration, high quality"
		}
	}

	apiKey := g.keys.nextKey()
	if apiKey == "" {
		return nil, fmt.Errorf("dalle: no api key configured")
	}

	// GPT-image models support /images/edits for reference-guided generation.
	// When reference images are provided, attempt the edits endpoint first for
	// character/style consistency, then fall back to normal generation on error.
	if isGPTImageModel(g.modelName) {
		refs := req.AllReferenceImageURLs()
		if len(refs) > 0 {
			if res, err := g.generateWithEdit(ctx, refs, prompt, size, apiKey); err == nil {
				return res, nil
			} else {
				g.logger.Warn("gpt-image: edit endpoint failed, falling back to text generation",
					zap.String("model", g.modelName), zap.Error(err))
			}
		}
	}

	payload := map[string]interface{}{
		"model":  g.modelName,
		"prompt": prompt,
		"n":      1,
		"size":   size,
	}
	// Inject reference image for models/APIs that support character/style consistency.
	// Doubao/Ark seedream models support reference_image_url for face & costume consistency.
	// CogView-4 uses ref_image_url for the same purpose.
	if req.StyleReferenceURL != "" {
		if isDoubaoEndpoint(g.imagesURL) {
			payload["reference_image_url"] = req.StyleReferenceURL
		} else if isCogViewModel(g.modelName) {
			payload["ref_image_url"] = req.StyleReferenceURL
		}
	}
	// Seed透传：Doubao/Ark seedream 以及 CogView 支持 seed 参数锁定随机种子，
	// 用于四视图一致性（同 seed 相邻调用能显著降低人物变形概率）。
	// OpenAI 官方 DALL-E/gpt-image 不支持 seed，跳过。
	if req.Seed > 0 {
		if isDoubaoEndpoint(g.imagesURL) || isCogViewModel(g.modelName) {
			payload["seed"] = req.Seed
		}
	}
	body, _ := json.Marshal(payload)

	var lastErr error
	// 5 attempts: base waits 0/15/30/60/90s + up to 8s random jitter to prevent
	// thundering-herd when multiple workers hit 429 simultaneously.
	baseBackoffs := []time.Duration{0, 15 * time.Second, 30 * time.Second, 60 * time.Second, 90 * time.Second}

	for attempt, baseWait := range baseBackoffs {
		// Pick a fresh key each attempt so we rotate away from rate-limited keys.
		apiKey = g.keys.nextKey()
		if apiKey == "" {
			return nil, fmt.Errorf("dalle: no api key configured")
		}
		if attempt > 0 {
			jitter := time.Duration(rand.Intn(8000)) * time.Millisecond
			wait := baseWait + jitter
			g.logger.Warn("retrying after rate limit",
				zap.Int("attempt", attempt+1),
				zap.Duration("backoff", wait),
			)
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("dalle: context cancelled during backoff: %w", ctx.Err())
			case <-time.After(wait):
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
			g.imagesURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("dalle: build request: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := g.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("dalle: request failed: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			// Distinguish content moderation blocks from real rate limits.
			if strings.Contains(string(b), "moderation_blocked") || strings.Contains(string(b), "safety_violations") {
				violation := extractSafetyViolation(b)
				g.logger.Warn("dalle: prompt blocked by content moderation",
					zap.String("violation", violation),
					zap.String("prompt_prefix", truncate(prompt, 100)),
				)
				return nil, fmt.Errorf("dalle: content moderation blocked (%s), prompt needs revision", violation)
			}

			g.keys.ReportFailure(apiKey, true)
			lastErr = fmt.Errorf("dalle: rate limited (429): %s", b)
			continue
		}

		if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusBadGateway {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			g.keys.ReportFailure(apiKey, false)
			lastErr = fmt.Errorf("dalle: unexpected status %d: %s", resp.StatusCode, b)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("dalle: unexpected status %d: %s", resp.StatusCode, b)
		}

		var result struct {
			Data []struct {
				URL           string `json:"url"`
				B64JSON       string `json:"b64_json"`
				RevisedPrompt string `json:"revised_prompt"`
			} `json:"data"`
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("dalle: decode response: %w", err)
		}

		if len(result.Data) == 0 {
			return nil, fmt.Errorf("dalle: empty data in response")
		}

		imageURL := result.Data[0].URL
		if imageURL == "" && result.Data[0].B64JSON != "" {
			imageURL = "data:image/png;base64," + result.Data[0].B64JSON
		}
		if imageURL == "" {
			return nil, fmt.Errorf("dalle: no url or b64_json in response")
		}

		g.keys.ReportSuccess(apiKey)
		return &GenerateRes{
			ImageURL:  imageURL,
			Width:     req.Width,
			Height:    req.Height,
			Seed:      -1,
			ModelUsed: g.generatorKey,
		}, nil
	}

	return nil, lastErr
}

// generateWithEdit —— 使用 /images/edits 端点，以参考图为基础生成风格一致的图片 (GPT-image系列专用)
// Downloads refURLs, uploads each as a separate image[] multipart field so
// gpt-image-1 can fuse multiple references (every storyboard character) for
// identity consistency. Falls back gracefully if individual downloads fail.
func (g *dalleGenerator) generateWithEdit(ctx context.Context, refURLs []string, prompt, size, apiKey string) (*GenerateRes, error) {
	if len(refURLs) == 0 {
		return nil, fmt.Errorf("gpt-image-edit: no reference images")
	}
	// Cap at 4 references: gpt-image-1 accepts multiple files but larger
	// multipart bodies slow uploads and multiply cost with little gain.
	const maxRefs = 4
	if len(refURLs) > maxRefs {
		refURLs = refURLs[:maxRefs]
	}

	// Build multipart form for /images/edits.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("model", g.modelName)
	_ = mw.WriteField("prompt", prompt)
	_ = mw.WriteField("n", "1")
	_ = mw.WriteField("size", size)

	attached := 0
	for i, refURL := range refURLs {
		refURL = strings.TrimSpace(refURL)
		if refURL == "" {
			continue
		}
		// Download reference image (10s timeout — usually an internal CDN URL).
		dlCtx, dlCancel := context.WithTimeout(ctx, 10*time.Second)
		dlReq, err := http.NewRequestWithContext(dlCtx, http.MethodGet, refURL, nil)
		if err != nil {
			dlCancel()
			g.logger.Warn("gpt-image-edit: build download request; skipping", zap.String("url", refURL), zap.Error(err))
			continue
		}
		dlResp, err := g.client.Do(dlReq)
		if err != nil {
			dlCancel()
			g.logger.Warn("gpt-image-edit: download reference image; skipping", zap.String("url", refURL), zap.Error(err))
			continue
		}
		if dlResp.StatusCode != http.StatusOK {
			dlResp.Body.Close()
			dlCancel()
			g.logger.Warn("gpt-image-edit: download status not ok; skipping", zap.String("url", refURL), zap.Int("status", dlResp.StatusCode))
			continue
		}
		imgBytes, err := io.ReadAll(dlResp.Body)
		dlResp.Body.Close()
		dlCancel()
		if err != nil {
			g.logger.Warn("gpt-image-edit: read reference image; skipping", zap.String("url", refURL), zap.Error(err))
			continue
		}
		// gpt-image-1 accepts image[] as an array param — append one file per ref.
		fw, err := mw.CreateFormFile("image[]", fmt.Sprintf("reference_%d.png", i+1))
		if err != nil {
			return nil, fmt.Errorf("gpt-image-edit: create form file: %w", err)
		}
		if _, err := fw.Write(imgBytes); err != nil {
			return nil, fmt.Errorf("gpt-image-edit: write form file: %w", err)
		}
		attached++
	}
	if attached == 0 {
		mw.Close()
		return nil, fmt.Errorf("gpt-image-edit: failed to attach any reference image")
	}
	mw.Close()

	// Derive edits URL from the generations URL.
	editsURL := strings.Replace(g.imagesURL, "/images/generations", "/images/edits", 1)
	editReq, err := http.NewRequestWithContext(ctx, http.MethodPost, editsURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("gpt-image-edit: build edit request: %w", err)
	}
	editReq.Header.Set("Authorization", "Bearer "+apiKey)
	editReq.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := g.client.Do(editReq)
	if err != nil {
		return nil, fmt.Errorf("gpt-image-edit: request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gpt-image-edit: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("gpt-image-edit: decode response: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("gpt-image-edit: empty data in response")
	}
	imageURL := result.Data[0].URL
	if imageURL == "" && result.Data[0].B64JSON != "" {
		imageURL = "data:image/png;base64," + result.Data[0].B64JSON
	}
	if imageURL == "" {
		return nil, fmt.Errorf("gpt-image-edit: no url or b64_json in response")
	}
	return &GenerateRes{
		ImageURL:  imageURL,
		Width:     parseSizeWidth(size),
		Height:    parseSizeHeight(size),
		Seed:      -1,
		ModelUsed: g.generatorKey,
	}, nil
}

func parseSizeWidth(size string) int {
	w, _ := parseDimensions(size)
	return w
}

func parseSizeHeight(size string) int {
	_, h := parseDimensions(size)
	return h
}

func extractSafetyViolation(body []byte) string {
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		msg := parsed.Error.Message
		if idx := strings.Index(msg, "safety_violations=["); idx >= 0 {
			start := idx + len("safety_violations=[")
			if end := strings.Index(msg[start:], "]"); end >= 0 {
				return msg[start : start+end]
			}
		}
	}
	return "unknown"
}

// sanitizePrompt —— 替换提示词中可能触发内容审核的敏感词汇，返回清洗后的提示词
func sanitizePrompt(prompt string) string {
	replacements := map[string]string{
		"斩首":   "defeated",
		"人头":   "mask trophy",
		"砍头":   "vanquished foe",
		"吊死":   "trapped by rope",
		"白绫":   "white silk ribbon",
		"上吊":   "hanging silk",
		"吊绑":   "bound with rope",
		"裸":    "lightly clothed",
		"赤身":   "warrior in light armor",
		"赤裸":   "lightly armored",
		"自刎":   "dramatic farewell",
		"自尽":   "dramatic parting",
		"血泊":   "fallen warrior scene",
		"鲜血淋漓": "battle-scarred",
	}
	for old, replacement := range replacements {
		prompt = strings.ReplaceAll(prompt, old, replacement)
	}
	return prompt
}

// truncate —— 将字符串截断到指定最大长度，超出部分用省略号替代
func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "…"
}

// buildDallePrompt —— 根据请求参数拼接 DALL-E / GPT-4o-image / CogView 的完整提示词。
//
// The prompt is now generated from a structured prompt spec so different model
// families receive the same semantic content expressed in their preferred form:
//   - GPT-image / DALL-E / Doubao-image → English natural language blocks
//   - CogView → Chinese structured blocks
func buildDallePrompt(req GenerateReq, modelName string) string {
	if isCogViewModel(modelName) {
		return buildCogViewPrompt(req)
	}
	return buildNaturalLanguagePrompt(req)
}

// isCogViewModel returns true for ZhipuAI CogView image generation models.
func isCogViewModel(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(modelName), "cogview")
}

// isGPTImageModel returns true for OpenAI gpt-image models that support /images/edits.
func isGPTImageModel(modelName string) bool {
	lm := strings.ToLower(modelName)
	return strings.HasPrefix(lm, "gpt-image")
}

// isDoubaoEndpoint returns true when the endpoint URL is a Volcengine Ark (豆包) endpoint.
// Doubao seedream models support reference_image_url for character/face consistency.
func isDoubaoEndpoint(url string) bool {
	return strings.Contains(url, "volces.com") || strings.Contains(url, "volcengine")
}

// isQianfanEndpoint returns true when the endpoint URL is a Baidu Qianfan endpoint.
// Qianfan API only accepts English prompts.
func isQianfanEndpoint(url string) bool {
	return strings.Contains(url, "baidubce.com") || strings.Contains(url, "qianfan")
}

// stripCJKCharacters removes CJK (Chinese/Japanese/Korean) characters and surrounding
// punctuation from a string, returning only the remaining ASCII/Latin content.
func stripCJKCharacters(s string) string {
	var b strings.Builder
	for _, r := range s {
		// Keep ASCII printable, common punctuation, and basic Latin
		if r < 0x2E80 {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	// Collapse multiple spaces
	result := strings.Join(strings.Fields(b.String()), " ")
	return strings.TrimSpace(result)
}

// buildCogViewPrompt builds a Chinese-optimised prompt for ZhipuAI CogView models.
//
// CogView-3-plus and CogView-4 respond best to structured Chinese instructions.
// We preserve the user's prompt as the core scene brief and add explicit Chinese
// blocks for composition, camera, quality, consistency, and negative guidance.
func buildCogViewPrompt(req GenerateReq) string {
	spec := buildImagePromptSpec(req)
	negative := ChineseLangNegativeByStyle(req.StylePreset)
	if spec.NegativeZH != "" {
		negative = joinChineseSentences(negative, spec.NegativeZH)
	}
	return joinChineseSentences(
		buildChineseStructuredPrompt(req, false),
		negative,
	)
}

// DecodeBase64Image —— 从 data URI 中解码 Base64 图片数据，非 data URI 时返回 nil
func DecodeBase64Image(dataURI string) []byte {
	if !strings.HasPrefix(dataURI, "data:") {
		return nil
	}
	idx := strings.Index(dataURI, ",")
	if idx < 0 {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(dataURI[idx+1:])
	if err != nil {
		return nil
	}
	return decoded
}
