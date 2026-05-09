package generators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

const defaultComfyWorkflowTemplate = `{
  "3": {"class_type": "KSampler", "inputs": {"cfg": __CFG_SCALE__, "denoise": 1, "latent_image": ["5", 0], "model": ["4", 0], "negative": ["7", 0], "positive": ["6", 0], "sampler_name": "dpmpp_2m_sde", "scheduler": "karras", "seed": __SEED__, "steps": __STEPS__}},
  "4": {"class_type": "CheckpointLoaderSimple", "inputs": {"ckpt_name": "sd_xl_base_1.0.safetensors"}},
  "5": {"class_type": "EmptyLatentImage", "inputs": {"batch_size": 1, "height": __HEIGHT__, "width": __WIDTH__}},
  "6": {"class_type": "CLIPTextEncode", "inputs": {"clip": ["4", 1], "text": __PROMPT_JSON__}},
  "7": {"class_type": "CLIPTextEncode", "inputs": {"clip": ["4", 1], "text": __NEGATIVE_PROMPT_JSON__}},
  "8": {"class_type": "VAEDecode", "inputs": {"samples": ["3", 0], "vae": ["4", 2]}},
  "9": {"class_type": "SaveImage", "inputs": {"filename_prefix": "autovideo", "images": ["8", 0]}}
}`

// defaultComfyImg2ImgTemplate —— 使用 VAE img2img 的 ComfyUI 工作流模板，用于参考图一致性生成
// Node 10=LoadImage(ref), 11=ImageScale(resize), 12=VAEEncode, KSampler latent→["12",0], denoise=0.7
const defaultComfyImg2ImgTemplate = `{
  "3": {"class_type": "KSampler", "inputs": {"cfg": __CFG_SCALE__, "denoise": 0.7, "latent_image": ["12", 0], "model": ["4", 0], "negative": ["7", 0], "positive": ["6", 0], "sampler_name": "dpmpp_2m_sde", "scheduler": "karras", "seed": __SEED__, "steps": __STEPS__}},
  "4": {"class_type": "CheckpointLoaderSimple", "inputs": {"ckpt_name": "sd_xl_base_1.0.safetensors"}},
  "6": {"class_type": "CLIPTextEncode", "inputs": {"clip": ["4", 1], "text": __PROMPT_JSON__}},
  "7": {"class_type": "CLIPTextEncode", "inputs": {"clip": ["4", 1], "text": __NEGATIVE_PROMPT_JSON__}},
  "8": {"class_type": "VAEDecode", "inputs": {"samples": ["3", 0], "vae": ["4", 2]}},
  "9": {"class_type": "SaveImage", "inputs": {"filename_prefix": "autovideo", "images": ["8", 0]}},
  "10": {"class_type": "LoadImage", "inputs": {"image": __REF_IMAGE_JSON__, "upload": "image"}},
  "11": {"class_type": "ImageScale", "inputs": {"image": ["10", 0], "upscale_method": "bicubic", "width": __WIDTH__, "height": __HEIGHT__, "crop": "center"}},
  "12": {"class_type": "VAEEncode", "inputs": {"pixels": ["11", 0], "vae": ["4", 2]}}
}`

type sdxlGenerator struct {
	endpoints        keyPool
	workflowTemplate string
	client           *http.Client
	logger           *zap.Logger
}

// NewSDXLGenerator —— 创建 SDXL(ComfyUI) 图片生成器实例，返回 ImageGenerator 接口
func NewSDXLGenerator(comfyURLs []string, workflowConfig string, logger *zap.Logger) ImageGenerator {
	return &sdxlGenerator{
		endpoints:        newKeyPool(comfyURLs),
		workflowTemplate: loadSDXLWorkflowTemplate(workflowConfig, logger),
		client:           &http.Client{Timeout: 30 * time.Second},
		logger:           logger,
	}
}

// Name —— 返回生成器名称 "sdxl"
func (g *sdxlGenerator) Name() string { return "sdxl" }

// IsAvailable —— 检查 ComfyUI 服务是否在线可用，返回布尔值
func (g *sdxlGenerator) IsAvailable(ctx context.Context) bool {
	for _, endpoint := range g.endpoints.keys {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/system_stats", nil)
		if err != nil {
			continue
		}
		resp, err := g.client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return true
		}
	}
	return false
}

// RefCapability —— SDXL 走 VAE img2img（有参考图），上传失败时自动降级为纯文生图。
func (g *sdxlGenerator) RefCapability() RefCapability {
	return RefCapability{Mode: RefModeI2I, MaxRefs: 1, StrongRef: false}
}

// Generate —— 向 ComfyUI 提交 SDXL 工作流生成图片并轮询结果，返回生成结果
func (g *sdxlGenerator) Generate(ctx context.Context, req GenerateReq) (*GenerateRes, error) {
	seed := req.Seed
	if seed == -1 {
		seed = rand.Int63()
	}

	prompt, negativePrompt := buildSDXLPrompts(req)

	var lastErr error
	for _, endpoint := range g.endpointCandidates() {
		res, err := g.generateOnEndpoint(ctx, endpoint, req, seed, prompt, negativePrompt)
		if err == nil {
			return res, nil
		}
		lastErr = err
		g.logger.Warn("sdxl: endpoint failed, trying next if available",
			zap.String("endpoint", endpoint),
			zap.Error(err),
		)
		if ctx.Err() != nil {
			break
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("sdxl: no comfyui endpoint configured")
}

// pollForResult —— 轮询 ComfyUI 历史记录直到任务完成，返回生成的图片文件名
func (g *sdxlGenerator) pollForResult(ctx context.Context, endpoint, promptID string) (string, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("sdxl: polling timed out for prompt %s", promptID)
		case <-ticker.C:
			filename, done, err := g.checkHistory(ctx, endpoint, promptID)
			if err != nil {
				g.logger.Warn("sdxl: check history error",
					zap.String("endpoint", endpoint),
					zap.String("prompt_id", promptID),
					zap.Error(err),
				)
				continue
			}
			if done {
				return filename, nil
			}
		}
	}
}

// checkHistory —— 查询 ComfyUI 指定任务的历史状态，返回文件名、是否完成和错误
func (g *sdxlGenerator) checkHistory(ctx context.Context, endpoint, promptID string) (string, bool, error) {
	url := fmt.Sprintf("%s/api/history/%s", endpoint, promptID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	var history map[string]struct {
		Status struct {
			StatusStr string `json:"status_str"`
		} `json:"status"`
		Outputs map[string]struct {
			Images []struct {
				Filename string `json:"filename"`
			} `json:"images"`
		} `json:"outputs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return "", false, err
	}

	entry, ok := history[promptID]
	if !ok {
		return "", false, nil // still queued
	}
	if entry.Status.StatusStr != "success" {
		if strings.Contains(entry.Status.StatusStr, "error") {
			return "", false, fmt.Errorf("sdxl: job failed with status %s", entry.Status.StatusStr)
		}
		return "", false, nil
	}

	for _, output := range entry.Outputs {
		if len(output.Images) > 0 {
			return output.Images[0].Filename, true, nil
		}
	}
	return "", false, fmt.Errorf("sdxl: no images in output")
}

func (g *sdxlGenerator) generateOnEndpoint(ctx context.Context, endpoint string, req GenerateReq, seed int64, prompt, negativePrompt string) (*GenerateRes, error) {
	// When a reference image is provided, use img2img (VAE encode) for visual consistency.
	// Upload the reference image to ComfyUI and build the img2img workflow.
	refImageFilename := ""
	refURL := req.StyleReferenceURL
	if refURL == "" {
		refURL = req.IPAdapterURL
	}
	if refURL != "" {
		if fn, err := g.uploadReferenceImage(ctx, endpoint, refURL); err == nil {
			refImageFilename = fn
		} else {
			g.logger.Warn("sdxl: reference image upload failed, using text-only generation",
				zap.String("ref_url", refURL), zap.Error(err))
		}
	}

	var workflow string
	var err error
	if refImageFilename != "" {
		workflow, err = buildSDXLImg2ImgWorkflow(g.workflowTemplate, req, seed, prompt, negativePrompt, refImageFilename)
	} else {
		workflow, err = buildSDXLWorkflow(g.workflowTemplate, req, seed, prompt, negativePrompt)
	}
	if err != nil {
		return nil, fmt.Errorf("sdxl: build request: %w", err)
	}

	payload := map[string]interface{}{"prompt": json.RawMessage(workflow)}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/api/prompt", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("sdxl: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sdxl: post prompt: %w", err)
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sdxl: unexpected status %d: %s", resp.StatusCode, b)
	}

	var promptResp struct {
		PromptID string `json:"prompt_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&promptResp); err != nil {
		return nil, fmt.Errorf("sdxl: decode prompt response: %w", err)
	}

	filename, err := g.pollForResult(ctx, endpoint, promptResp.PromptID)
	if err != nil {
		return nil, err
	}

	imageURL := fmt.Sprintf("%s/api/view?filename=%s", endpoint, filename)

	return &GenerateRes{
		ImageURL:  imageURL,
		Width:     req.Width,
		Height:    req.Height,
		Seed:      seed,
		ModelUsed: "sdxl",
	}, nil
}

func (g *sdxlGenerator) endpointCandidates() []string {
	first := g.endpoints.nextKey()
	if first == "" {
		return nil
	}

	candidates := []string{first}
	for _, endpoint := range g.endpoints.keys {
		if endpoint != first {
			candidates = append(candidates, endpoint)
		}
	}
	return candidates
}

func loadSDXLWorkflowTemplate(workflowConfig string, logger *zap.Logger) string {
	trimmed := strings.TrimSpace(workflowConfig)
	if trimmed == "" {
		return defaultComfyWorkflowTemplate
	}
	if strings.HasPrefix(trimmed, "{") {
		return trimmed
	}

	data, err := os.ReadFile(trimmed)
	if err != nil {
		logger.Warn("sdxl: failed to read workflow config, using builtin template",
			zap.String("workflow", trimmed),
			zap.Error(err),
		)
		return defaultComfyWorkflowTemplate
	}
	return string(data)
}

func buildSDXLWorkflow(template string, req GenerateReq, seed int64, prompt, negativePrompt string) (string, error) {
	promptJSON, err := json.Marshal(prompt)
	if err != nil {
		return "", err
	}
	negativeJSON, err := json.Marshal(negativePrompt)
	if err != nil {
		return "", err
	}

	replacer := strings.NewReplacer(
		"__CFG_SCALE__", fmt.Sprintf("%g", req.CfgScale),
		"__SEED__", fmt.Sprintf("%d", seed),
		"__STEPS__", fmt.Sprintf("%d", req.Steps),
		"__HEIGHT__", fmt.Sprintf("%d", req.Height),
		"__WIDTH__", fmt.Sprintf("%d", req.Width),
		"__PROMPT_JSON__", string(promptJSON),
		"__NEGATIVE_PROMPT_JSON__", string(negativeJSON),
	)
	return replacer.Replace(template), nil
}

// buildSDXLImg2ImgWorkflow —— 构建包含参考图 VAE img2img 的 ComfyUI 工作流 (denoise=0.7)
// If the user-provided workflowTemplate already contains __REF_IMAGE_JSON__, it is used as-is.
// Otherwise the built-in defaultComfyImg2ImgTemplate is used.
func buildSDXLImg2ImgWorkflow(userTemplate string, req GenerateReq, seed int64, prompt, negativePrompt, refFilename string) (string, error) {
	promptJSON, err := json.Marshal(prompt)
	if err != nil {
		return "", err
	}
	negativeJSON, err := json.Marshal(negativePrompt)
	if err != nil {
		return "", err
	}
	refJSON, err := json.Marshal(refFilename)
	if err != nil {
		return "", err
	}

	// Use the user template if it has the img2img placeholder; else use built-in.
	tpl := userTemplate
	if !strings.Contains(tpl, "__REF_IMAGE_JSON__") {
		tpl = defaultComfyImg2ImgTemplate
	}

	replacer := strings.NewReplacer(
		"__CFG_SCALE__", fmt.Sprintf("%g", req.CfgScale),
		"__SEED__", fmt.Sprintf("%d", seed),
		"__STEPS__", fmt.Sprintf("%d", req.Steps),
		"__HEIGHT__", fmt.Sprintf("%d", req.Height),
		"__WIDTH__", fmt.Sprintf("%d", req.Width),
		"__PROMPT_JSON__", string(promptJSON),
		"__NEGATIVE_PROMPT_JSON__", string(negativeJSON),
		"__REF_IMAGE_JSON__", string(refJSON),
	)
	return replacer.Replace(tpl), nil
}

// uploadReferenceImage —— 下载参考图片并通过 ComfyUI /upload/image 上传，返回文件名
// Used for img2img workflows where the reference image must be available on the ComfyUI server.
func (g *sdxlGenerator) uploadReferenceImage(ctx context.Context, endpoint, imageURL string) (string, error) {
	dlCtx, dlCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dlCancel()
	dlReq, err := http.NewRequestWithContext(dlCtx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("sdxl: build download request: %w", err)
	}
	dlResp, err := g.client.Do(dlReq)
	if err != nil {
		return "", fmt.Errorf("sdxl: download reference image: %w", err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sdxl: download reference image status %d", dlResp.StatusCode)
	}
	imgBytes, err := io.ReadAll(dlResp.Body)
	if err != nil {
		return "", fmt.Errorf("sdxl: read reference image: %w", err)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("image", "reference.png")
	if err != nil {
		return "", fmt.Errorf("sdxl: create form file: %w", err)
	}
	if _, err := fw.Write(imgBytes); err != nil {
		return "", fmt.Errorf("sdxl: write form file: %w", err)
	}
	_ = mw.WriteField("type", "input")
	_ = mw.WriteField("overwrite", "true")
	mw.Close()

	uploadReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/upload/image", &buf)
	if err != nil {
		return "", fmt.Errorf("sdxl: build upload request: %w", err)
	}
	uploadReq.Header.Set("Content-Type", mw.FormDataContentType())

	uploadResp, err := g.client.Do(uploadReq)
	if err != nil {
		return "", fmt.Errorf("sdxl: upload reference image: %w", err)
	}
	defer uploadResp.Body.Close()
	uploadBody, _ := io.ReadAll(uploadResp.Body)
	if uploadResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sdxl: upload reference image status %d: %s", uploadResp.StatusCode, uploadBody)
	}

	var uploadResult struct {
		Name      string `json:"name"`
		Subfolder string `json:"subfolder"`
	}
	if err := json.Unmarshal(uploadBody, &uploadResult); err != nil {
		return "", fmt.Errorf("sdxl: decode upload response: %w", err)
	}
	if uploadResult.Name == "" {
		return "", fmt.Errorf("sdxl: upload returned empty filename")
	}
	return uploadResult.Name, nil
}

func buildSDXLPrompts(req GenerateReq) (string, string) {
	resolvedTaskType := NormalizeImageTaskType(req.TaskType, req.Prompt)

	// guofeng and ink-poetry styles have dedicated style strings not in the shared helper.
	var prompt string
	switch req.StylePreset {
	case "guofeng", "guofeng-myth", "ink-poetry":
		styleTags := sdxlStylePrompt(req.StylePreset)
		prompt = joinPromptParts(styleTags, req.Prompt)
	default:
		// Structured SDXL prompt: style tags + task tags + primary brief + composition tags.
		// Skip prepending if the prompt already contains upstream SDXL quality prefixes.
		if hasSDXLStylePrefix(req.Prompt) {
			prompt = joinPromptParts(req.Prompt, sdxlCompositionTags(req.Width, req.Height, resolvedTaskType))
		} else {
			prompt = buildSDXLPositivePrompt(req)
		}
	}

	var negativePrompt string
	switch req.StylePreset {
	case "guofeng", "guofeng-myth", "ink-poetry":
		negativePrompt = joinPromptParts(req.NegativePrompt, sdxlStyleNegativePrompt(req.StylePreset))
	default:
		negativePrompt = buildSDXLNegativePrompt(req)
	}
	return prompt, negativePrompt
}

// hasSDXLStylePrefix reports whether the prompt already starts with SDXL-style
// quality/photorealism tokens, indicating that the upstream caller (e.g. character-service)
// has already prepended a style prefix and we should not add another one.
func hasSDXLStylePrefix(prompt string) bool {
	t := strings.TrimSpace(strings.ToLower(prompt))
	return strings.HasPrefix(t, "raw photo") ||
		strings.HasPrefix(t, "(raw photo") ||
		strings.HasPrefix(t, "(masterpiece") ||
		strings.HasPrefix(t, "(photorealistic") ||
		strings.HasPrefix(t, "(hyperrealistic")
}

func joinPromptParts(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	return strings.Join(filtered, ", ")
}

// sdxlStylePrompt and sdxlStyleNegativePrompt are superseded by the shared
// SDXLStyleTags / SDXLNegativeTags helpers in prompt_utils.go.
// Keeping the functions below only for any legacy call sites that may reference them.

func sdxlStylePrompt(style string) string {
	switch style {
	case "guofeng", "guofeng-myth":
		return "classical chinese fantasy illustration, flowing costume detail, elegant brushwork, wuxia atmosphere"
	case "ink-poetry":
		return "ink wash painting, poetic composition, soft watercolor texture, negative space"
	default:
		return SDXLStyleTags(style)
	}
}

func sdxlStyleNegativePrompt(style string) string {
	switch style {
	case "guofeng", "guofeng-myth", "ink-poetry":
		return "modern clothing, futuristic props, photorealistic face, lowres, text, watermark"
	default:
		return SDXLNegativeTags(style)
	}
}
