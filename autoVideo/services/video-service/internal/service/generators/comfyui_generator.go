package generators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ComfyUIVideoGenerator drives a local ComfyUI workflow for image-to-video generation.
type ComfyUIVideoGenerator struct {
	Endpoint       string
	WorkflowConfig string
	// char-c5: IP-Adapter model name for character reference; injected as __IPADAPTER_MODEL_JSON__ in workflow template
	IPAdapterModel string
	// char-c6: LoRA model name for character style binding; injected as __LORA_MODEL_JSON__ in workflow template
	LoRAModel  string
	LoRAWeight float64 // default 0.8
	Client     *http.Client
}

type comfyUIViewAsset struct {
	Filename  string `json:"filename"`
	Subfolder string `json:"subfolder"`
	Type      string `json:"type"`
}

type comfyUIHistoryEntry struct {
	Status struct {
		StatusStr string `json:"status_str"`
		Completed bool   `json:"completed"`
	} `json:"status"`
	Outputs map[string]struct {
		Images []comfyUIViewAsset `json:"images"`
		Gifs   []comfyUIViewAsset `json:"gifs"`
		Videos []comfyUIViewAsset `json:"videos"`
	} `json:"outputs"`
}

const comfyUIMaxFrames = 16

func NewComfyUIVideoGenerator(endpoint, workflowConfig string) *ComfyUIVideoGenerator {
	return &ComfyUIVideoGenerator{
		Endpoint:       strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		WorkflowConfig: strings.TrimSpace(workflowConfig),
		LoRAWeight:     0.8,
		Client:         &http.Client{Timeout: 60 * time.Second},
	}
}

// WithIPAdapter sets the IP-Adapter model for character reference images (char-c5).
func (g *ComfyUIVideoGenerator) WithIPAdapter(modelName string) *ComfyUIVideoGenerator {
	g.IPAdapterModel = modelName
	return g
}

// WithLoRA sets the LoRA model and weight for character style binding (char-c6).
func (g *ComfyUIVideoGenerator) WithLoRA(modelName string, weight float64) *ComfyUIVideoGenerator {
	g.LoRAModel = modelName
	if weight > 0 {
		g.LoRAWeight = weight
	}
	return g
}

func (g *ComfyUIVideoGenerator) Name() string { return "comfyui-video" }

func (g *ComfyUIVideoGenerator) IsAvailable(ctx context.Context) bool {
	workflowTemplate := g.workflowTemplate()
	if strings.TrimSpace(g.Endpoint) == "" || strings.TrimSpace(workflowTemplate) == "" {
		return false
	}
	_, err := g.doRequestWithFallback(ctx, http.MethodGet, "/api/system_stats", "/system_stats", nil, "")
	return err == nil
}

// SupportsNativeAudio —— ComfyUI video does not embed audio in generated clips.
func (g *ComfyUIVideoGenerator) SupportsNativeAudio() bool { return false }

// ParamOptions —— ComfyUI 由工作流决定，无固定可配参数
func (g *ComfyUIVideoGenerator) ParamOptions() []ModelParamOption { return nil }

func (g *ComfyUIVideoGenerator) Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error) {
	if strings.TrimSpace(g.Endpoint) == "" {
		return nil, fmt.Errorf("comfyui-video: endpoint not configured")
	}
	workflowTemplate := g.workflowTemplate()
	if strings.TrimSpace(workflowTemplate) == "" {
		return nil, fmt.Errorf("comfyui-video: workflow not configured")
	}

	imageName, err := g.uploadInputImage(ctx, req.SourceImageURL)
	if err != nil {
		return nil, err
	}

	seed := time.Now().UnixNano()
	frames, fps := videoFramePlan(req.DurationSec)
	workflow, err := buildComfyUIVideoWorkflowWithConfig(workflowTemplate, imageName, req, seed, frames, fps, g.IPAdapterModel, g.LoRAModel, g.LoRAWeight)
	if err != nil {
		return nil, fmt.Errorf("comfyui-video: build workflow: %w", err)
	}

	promptID, err := g.submitWorkflow(ctx, workflow)
	if err != nil {
		return nil, err
	}

	asset, err := g.pollForAsset(ctx, promptID)
	if err != nil {
		return nil, err
	}

	return &VideoClip{
		ClipURL:     g.buildViewURL(asset),
		DurationSec: req.DurationSec,
		ModelUsed:   g.Name(),
	}, nil
}

func (g *ComfyUIVideoGenerator) workflowTemplate() string {
	return loadComfyUIVideoWorkflowTemplate(g.WorkflowConfig)
}

func (g *ComfyUIVideoGenerator) uploadInputImage(ctx context.Context, sourceURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("comfyui-video: build source image request: %w", err)
	}
	resp, err := g.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("comfyui-video: download source image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("comfyui-video: source image HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("comfyui-video: read source image: %w", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", sourceFileName(sourceURL))
	if err != nil {
		return "", fmt.Errorf("comfyui-video: create upload part: %w", err)
	}
	if _, err := part.Write(imageBytes); err != nil {
		return "", fmt.Errorf("comfyui-video: write upload body: %w", err)
	}
	_ = writer.WriteField("type", "input")
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("comfyui-video: finalize upload body: %w", err)
	}

	respBody, err := g.doRequestWithFallback(ctx, http.MethodPost, "/api/upload/image", "/upload/image", body.Bytes(), writer.FormDataContentType())
	if err != nil {
		return "", err
	}
	var result struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("comfyui-video: decode upload response: %w", err)
	}
	if strings.TrimSpace(result.Name) == "" {
		return "", fmt.Errorf("comfyui-video: upload response missing image name")
	}
	return result.Name, nil
}

func (g *ComfyUIVideoGenerator) submitWorkflow(ctx context.Context, workflow string) (string, error) {
	payload := map[string]json.RawMessage{
		"prompt": json.RawMessage(workflow),
	}
	body, _ := json.Marshal(payload)
	respBody, err := g.doRequestWithFallback(ctx, http.MethodPost, "/api/prompt", "/prompt", body, "application/json")
	if err != nil {
		return "", err
	}
	var result struct {
		PromptID string `json:"prompt_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("comfyui-video: decode prompt response: %w", err)
	}
	if strings.TrimSpace(result.PromptID) == "" {
		return "", fmt.Errorf("comfyui-video: missing prompt_id")
	}
	return result.PromptID, nil
}

func (g *ComfyUIVideoGenerator) pollForAsset(ctx context.Context, promptID string) (*comfyUIViewAsset, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.NewTimer(60 * time.Minute)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout.C:
			return nil, fmt.Errorf("comfyui-video: prompt %s timed out", promptID)
		case <-ticker.C:
			asset, done, err := g.checkHistory(ctx, promptID)
			if err != nil {
				return nil, err
			}
			if done {
				return asset, nil
			}
		}
	}
}

func (g *ComfyUIVideoGenerator) checkHistory(ctx context.Context, promptID string) (*comfyUIViewAsset, bool, error) {
	respBody, err := g.doRequestWithFallback(ctx, http.MethodGet, "/api/history/"+promptID, "/history/"+promptID, nil, "")
	if err != nil {
		return nil, false, err
	}

	var history map[string]comfyUIHistoryEntry
	if err := json.Unmarshal(respBody, &history); err != nil {
		return nil, false, fmt.Errorf("comfyui-video: decode history: %w", err)
	}
	entry, ok := history[promptID]
	if !ok {
		return nil, false, nil
	}

	if strings.Contains(strings.ToLower(entry.Status.StatusStr), "error") {
		return nil, false, fmt.Errorf("comfyui-video: workflow failed with status %s", entry.Status.StatusStr)
	}

	asset := firstComfyUIOutput(entry)
	if asset != nil {
		return asset, true, nil
	}

	if entry.Status.Completed || strings.EqualFold(entry.Status.StatusStr, "success") {
		return nil, false, fmt.Errorf("comfyui-video: workflow completed without output asset")
	}
	return nil, false, nil
}

func firstComfyUIOutput(entry comfyUIHistoryEntry) *comfyUIViewAsset {
	for _, output := range entry.Outputs {
		for _, items := range [][]comfyUIViewAsset{output.Videos, output.Gifs, output.Images} {
			if len(items) == 0 {
				continue
			}
			asset := items[0]
			if strings.TrimSpace(asset.Type) == "" {
				asset.Type = "output"
			}
			return &asset
		}
	}
	return nil
}

func (g *ComfyUIVideoGenerator) buildViewURL(asset *comfyUIViewAsset) string {
	query := url.Values{}
	query.Set("filename", asset.Filename)
	query.Set("type", defaultString(asset.Type, "output"))
	if strings.TrimSpace(asset.Subfolder) != "" {
		query.Set("subfolder", asset.Subfolder)
	}
	return fmt.Sprintf("%s/view?%s", g.Endpoint, query.Encode())
}

func (g *ComfyUIVideoGenerator) doRequestWithFallback(ctx context.Context, method, apiPath, legacyPath string, body []byte, contentType string) ([]byte, error) {
	respBody, status, err := g.doRequest(ctx, method, apiPath, body, contentType)
	if status != http.StatusNotFound || legacyPath == "" {
		return respBody, err
	}
	respBody, _, err = g.doRequest(ctx, method, legacyPath, body, contentType)
	return respBody, err
}

func (g *ComfyUIVideoGenerator) doRequest(ctx context.Context, method, path string, body []byte, contentType string) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.Endpoint+path, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("comfyui-video: build %s request: %w", path, err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("comfyui-video: request %s: %w", path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return respBody, resp.StatusCode, fmt.Errorf("comfyui-video: %s returned %d: %s", path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, resp.StatusCode, nil
}

func loadComfyUIVideoWorkflowTemplate(workflowConfig string) string {
	trimmed := strings.TrimSpace(workflowConfig)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "{") {
		return trimmed
	}
	data, err := os.ReadFile(trimmed)
	if err != nil {
		return ""
	}
	return string(data)
}

func buildComfyUIVideoWorkflow(template, imageName string, req VideoGenerateReq, seed int64, frames, fps int) (string, error) {
	return buildComfyUIVideoWorkflowWithConfig(template, imageName, req, seed, frames, fps, "", "", 0.8)
}

// buildComfyUIVideoWorkflowWithConfig builds the workflow JSON with optional IP-Adapter and LoRA placeholders (char-c5/c6).
// The workflow template may reference __IPADAPTER_MODEL_JSON__, __LORA_MODEL_JSON__, __LORA_WEIGHT__ for these features.
func buildComfyUIVideoWorkflowWithConfig(template, imageName string, req VideoGenerateReq, seed int64, frames, fps int, ipAdapterModel, loraModel string, loraWeight float64) (string, error) {
	if strings.TrimSpace(template) == "" {
		return "", fmt.Errorf("empty workflow template")
	}
	imageJSON, err := json.Marshal(imageName)
	if err != nil {
		return "", err
	}
	promptJSON, err := json.Marshal(strings.TrimSpace(req.Prompt))
	if err != nil {
		return "", err
	}
	negativeJSON, err := json.Marshal(strings.TrimSpace(req.NegativePrompt))
	if err != nil {
		return "", err
	}
	ipAdapterJSON, _ := json.Marshal(ipAdapterModel)
	loraJSON, _ := json.Marshal(loraModel)
	if loraWeight <= 0 {
		loraWeight = 0.8
	}

	replacer := strings.NewReplacer(
		"__IMAGE_JSON__", string(imageJSON),
		"__PROMPT_JSON__", string(promptJSON),
		"__NEGATIVE_PROMPT_JSON__", string(negativeJSON),
		"__SEED__", strconv.FormatInt(seed, 10),
		"__FRAMES__", strconv.Itoa(frames),
		"__FPS__", strconv.Itoa(fps),
		"__IPADAPTER_MODEL_JSON__", string(ipAdapterJSON), // char-c5
		"__LORA_MODEL_JSON__", string(loraJSON),           // char-c6
		"__LORA_WEIGHT__", strconv.FormatFloat(loraWeight, 'f', 2, 64), // char-c6
	)
	return replacer.Replace(template), nil
}

func videoFramePlan(durationSec float64) (frames int, fps int) {
	fps = videoFPS(durationSec)
	if durationSec > 0 {
		maxFPS := int(float64(comfyUIMaxFrames) / durationSec)
		if maxFPS > 0 && fps > maxFPS {
			fps = maxFPS
		}
	}
	if fps < 1 {
		fps = 1
	}
	frames = int(durationSec * float64(fps))
	if frames < fps {
		frames = fps
	}
	if frames > comfyUIMaxFrames {
		frames = comfyUIMaxFrames
	}
	return frames, fps
}

func videoFPS(durationSec float64) int {
	if durationSec >= 8 {
		return 10
	}
	return 8
}

func sourceFileName(sourceURL string) string {
	if parsed, err := url.Parse(sourceURL); err == nil {
		if base := filepath.Base(parsed.Path); base != "." && base != "/" && base != "" {
			return base
		}
	}
	return "frame.png"
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
