package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/autovideo/image-service/pkg/metrics"

	"github.com/autovideo/image-service/internal/model"
	"github.com/autovideo/image-service/internal/repository"
	"github.com/autovideo/image-service/internal/service/generators"
	"github.com/autovideo/image-service/internal/stylepreset"
	"github.com/autovideo/image-service/pkg/config"
	"go.uber.org/zap"
)

type GenerateRequest struct {
	SceneID           *int64  `json:"scene_id"`
	ProjectID         int64   `json:"project_id"`
	UserID            int64   `json:"user_id"`
	Prompt            string  `json:"prompt"`
	NegativePrompt    string  `json:"negative_prompt"`
	TaskType          string  `json:"task_type"`
	StylePreset       string  `json:"style_preset"`
	StyleReferenceURL string  `json:"style_reference_url"`
	// ReferenceImageURLs carries additional character/scene references for
	// multi-image aware generators. StyleReferenceURL is still used as the
	// primary reference for single-ref generators (SDXL/Tongyi/Flux).
	ReferenceImageURLs []string `json:"reference_image_urls"`
	// IsCharacterSheet marks any attached reference as a 4-panel character
	// turnaround so downstream generators inject explicit "SAME person,
	// 4 views" guidance.
	IsCharacterSheet bool    `json:"is_character_sheet"`
	ModelName         string  `json:"model_name"`
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	Steps             int     `json:"steps"`
	CfgScale          float64 `json:"cfg_scale"`
	Seed              int64   `json:"seed"`
}

type ImageService interface {
	Generate(ctx context.Context, req GenerateRequest) (*model.ImageTask, error)
	GetTask(ctx context.Context, id int64) (*model.ImageTask, error)
	ListTasks(ctx context.Context, filter repository.ListFilter) ([]*model.ImageTask, int64, error)
	RetryTask(ctx context.Context, id int64) (*model.ImageTask, error)
	CancelTask(ctx context.Context, id int64) error
	DeleteProjectData(ctx context.Context, projectID int64) error
	RunGeneration(ctx context.Context, task *model.ImageTask)
	ResumeOrphanedTasks(ctx context.Context)
	StartStaleCleanup(ctx context.Context, interval time.Duration)
	ModelStatus(ctx context.Context) []ModelStatusItem
	ModelCapabilities(ctx context.Context) []ModelCapabilityItem
	TaskTypeCapabilities(ctx context.Context) []TaskTypeCapabilityItem
	GeminiChannels(ctx context.Context) []GeminiChannelStatus
}

// GeminiChannelStatus holds runtime validation info for a single (base, key) Gemini proxy channel.
type GeminiChannelStatus struct {
	Index   int    `json:"index"`
	Base    string `json:"base"`
	KeyMask string `json:"key_mask"` // first 8 + last 4 chars
	Valid   bool   `json:"valid"`
	Error   string `json:"error,omitempty"`
}

type ModelStatusItem struct {
	Key       string `json:"key"`
	Available bool   `json:"available"`
}

type ModelCapabilityItem struct {
	Key              string   `json:"key"`
	Available        bool     `json:"available"`
	ProviderKey      string   `json:"provider_key"`
	SizeMode         string   `json:"size_mode"`
	Verification     string   `json:"verification"`
	AllowedSizes     []string `json:"allowed_sizes,omitempty"`
	DefaultSquare    string   `json:"default_square,omitempty"`
	DefaultPortrait  string   `json:"default_portrait,omitempty"`
	DefaultLandscape string   `json:"default_landscape,omitempty"`
	MinWidth         int      `json:"min_width,omitempty"`
	MaxWidth         int      `json:"max_width,omitempty"`
	MinHeight        int      `json:"min_height,omitempty"`
	MaxHeight        int      `json:"max_height,omitempty"`
	RequireMultiple  int      `json:"require_multiple,omitempty"`
	Notes            string   `json:"notes,omitempty"`
	// 参考图能力声明：分镁生成和模型选择时可用此判断该模型是否支持图生图 / 融合模式。
	RefMode   string `json:"ref_mode"`            // 参考图模式：t2i | ip-adapter | i2i | fusion
	MaxRefs   int    `json:"max_refs"`             // 最多接受多少张参考图（0=不支持）
	StrongRef bool   `json:"strong_ref"`           // 无参考图时输出质量明显下降
}

type TaskTypeCapabilityItem struct {
	TaskType    string `json:"task_type"`
	RatioIntent string `json:"ratio_intent"`
}

type imageService struct {
	repo       repository.ImageRepo
	generators map[string]generators.ImageGenerator
	cfg        *config.Config
	logger     *zap.Logger
	httpClient *http.Client
	genSems    map[string]chan struct{} // per-generator concurrency limiter
	localSem   chan struct{}            // dedicated slots for self-hosted local image generation
	globalSem  chan struct{}            // overall cap to protect the system
}

// NewImageService —— 创建图片服务实例，初始化 per-generator 并发控制信号量，返回 ImageService 接口
func NewImageService(
	repo repository.ImageRepo,
	gens map[string]generators.ImageGenerator,
	cfg *config.Config,
	logger *zap.Logger,
) ImageService {
	maxWorkers := config.RecommendedMaxWorkers(cfg)
	localSlots := config.RecommendedLocalSlots(cfg)
	totalGens := len(gens)

	// Build per-generator semaphores so channels don't block each other.
	genSems := make(map[string]chan struct{}, totalGens)
	for key := range gens {
		slots := config.PerGeneratorSlots(cfg, key, totalGens)
		genSems[key] = make(chan struct{}, slots)
		logger.Info("generator concurrency configured",
			zap.String("generator", key),
			zap.Int("slots", slots),
		)
	}

	return &imageService{
		repo:       repo,
		generators: gens,
		cfg:        cfg,
		logger:     logger,
		httpClient: newOptimizedHTTPClient(),
		genSems:    genSems,
		localSem:   make(chan struct{}, localSlots),
		globalSem:  make(chan struct{}, maxWorkers),
	}
}

// Generate —— 创建图片生成任务并异步调度执行，返回创建的任务信息
func (s *imageService) Generate(ctx context.Context, req GenerateRequest) (*model.ImageTask, error) {
	if req.ModelName == "" {
		req.ModelName = "sdxl"
	}
	if req.Steps == 0 {
		req.Steps = 20
	}
	if req.CfgScale == 0 {
		req.CfgScale = 7.0
	}
	if req.Seed == 0 {
		req.Seed = -1
	}
	if req.StylePreset == "" {
		req.StylePreset = stylepreset.Default
	}
	req.TaskType = generators.NormalizeImageTaskType(req.TaskType, req.Prompt)
	sizeResult := generators.NormalizeGenerateSize(req.ModelName, req.TaskType, req.Width, req.Height)
	if sizeResult.Changed {
		s.logger.Info("normalized image request size",
			zap.String("model", req.ModelName),
			zap.String("task_type", req.TaskType),
			zap.Int("requested_width", req.Width),
			zap.Int("requested_height", req.Height),
			zap.Int("normalized_width", sizeResult.Width),
			zap.Int("normalized_height", sizeResult.Height),
			zap.String("policy_mode", string(sizeResult.Policy.Mode)),
			zap.String("verification", string(sizeResult.Policy.Verification)),
			zap.String("reason", sizeResult.Reason),
		)
	}
	req.Width = sizeResult.Width
	req.Height = sizeResult.Height

	task := &model.ImageTask{
		SceneID:           req.SceneID,
		ProjectID:         req.ProjectID,
		UserID:            req.UserID,
		Prompt:            req.Prompt,
		NegativePrompt:    req.NegativePrompt,
		TaskType:          req.TaskType,
		StylePreset:       req.StylePreset,
		StyleReferenceURL: req.StyleReferenceURL,
		IsCharacterSheet:  req.IsCharacterSheet,
		ModelName:         req.ModelName,
		Width:             req.Width,
		Height:            req.Height,
		Steps:             req.Steps,
		CfgScale:          req.CfgScale,
		Seed:              req.Seed,
		Status:            model.StatusPending,
	}
	if len(req.ReferenceImageURLs) > 0 {
		if raw, err := json.Marshal(req.ReferenceImageURLs); err == nil {
			task.ReferenceImageURLs = raw
		}
	}

	if err := s.repo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("create image task: %w", err)
	}

	go s.runGenerationWithSem(context.Background(), task)

	return task, nil
}

// RunGeneration —— 使用 per-generator 信号量调度图片生成任务
// RunGeneration dispatches a task using the per-generator semaphore.
func (s *imageService) RunGeneration(ctx context.Context, task *model.ImageTask) {
	s.runGenerationWithSem(ctx, task)
}

// runGenerationWithSem —— 在 per-generator 信号量控制下执行图片生成，包含内容审核回退和通义降级逻辑
func (s *imageService) runGenerationWithSem(ctx context.Context, task *model.ImageTask) {
	genStart := time.Now()

	// Global cap to protect overall system resources.
	s.globalSem <- struct{}{}
	defer func() { <-s.globalSem }()

	// Per-generator semaphore so different channels don't block each other.
	genKey := task.ModelName
	if usesLocalModel(genKey) {
		s.localSem <- struct{}{}
		defer func() { <-s.localSem }()
	} else if sem, ok := s.genSems[genKey]; ok {
		metrics.QueueDepth.WithLabelValues(genKey).Inc()
		sem <- struct{}{}
		defer func() {
			<-sem
			metrics.QueueDepth.WithLabelValues(genKey).Dec()
		}()
	}

	// Record generation duration and outcome when done.
	defer func() {
		duration := time.Since(genStart).Seconds()
		status := "success"
		if task.Status == "failed" {
			status = "failure"
		}
		metrics.GenerationDuration.WithLabelValues(genKey, status).Observe(duration)
		metrics.GenerationTotal.WithLabelValues(genKey, status).Inc()
	}()

	// Tongyi uses async API (submit+poll) and needs longer timeout
	timeout := 6 * time.Minute
	if task.ModelName == "tongyi" {
		timeout = 15 * time.Minute
	} else if usesLocalModel(task.ModelName) {
		timeout = 10 * time.Minute
	}
	genCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_ = s.repo.UpdateStatus(genCtx, task.ID, model.StatusRunning, "", "", "")

	gen := s.selectGenerator(task.ModelName)
	req := generators.GenerateReq{
		Prompt:            task.Prompt,
		NegativePrompt:    task.NegativePrompt,
		TaskType:          task.TaskType,
		StylePreset:       task.StylePreset,
		StyleReferenceURL: task.StyleReferenceURL,
		IsCharacterSheet:  task.IsCharacterSheet,
		Width:             task.Width,
		Height:            task.Height,
		Steps:             task.Steps,
		CfgScale:          task.CfgScale,
		Seed:              task.Seed,
	}
	if len(task.ReferenceImageURLs) > 0 {
		var urls []string
		if err := json.Unmarshal(task.ReferenceImageURLs, &urls); err == nil {
			req.ReferenceImageURLs = urls
		}
	}

	res, err := gen.Generate(genCtx, req)

	// Fallback chain: moderation blocked → LLM rewrite → retry → fallback to tongyi
	if err != nil && strings.Contains(err.Error(), "content moderation blocked") {
		s.logger.Warn("moderation blocked, attempting LLM prompt rewrite",
			zap.Int64("task_id", task.ID),
			zap.String("original_prompt", truncateStr(task.Prompt, 120)),
		)

		rewritten := s.rewritePromptForSafety(genCtx, task.Prompt)
		if rewritten != "" && rewritten != task.Prompt {
			req.Prompt = rewritten

			// Retry with rewritten prompt on same generator
			res, err = gen.Generate(genCtx, req)
		}

		// Still blocked or failed → try tongyi as fallback
		if err != nil {
			if fallback, ok := s.generators["tongyi"]; ok && fallback.IsAvailable(genCtx) {
				s.logger.Info("falling back to tongyi generator",
					zap.Int64("task_id", task.ID),
				)
				req.Prompt = rewritten
				if req.Prompt == "" {
					req.Prompt = task.Prompt
				}
				res, err = fallback.Generate(genCtx, req)
				if err == nil {
					s.logger.Info("tongyi fallback succeeded", zap.Int64("task_id", task.ID))
				}
			}
		}
	}

	if err != nil {
		s.logger.Error("generation failed",
			zap.Int64("task_id", task.ID),
			zap.String("model", task.ModelName),
			zap.Error(err),
		)
		_ = s.repo.UpdateStatus(ctx, task.ID, model.StatusFailed, "", "", err.Error())
		return
	}

	resultURL, err := s.uploadToStorage(ctx, task.ID, task.ProjectID, res.ImageURL)
	if err != nil {
		s.logger.Error("upload failed", zap.Int64("task_id", task.ID), zap.Error(err))
		_ = s.repo.UpdateStatus(ctx, task.ID, model.StatusFailed, "", "", err.Error())
		return
	}

	_ = s.repo.UpdateStatus(ctx, task.ID, model.StatusSucceeded, resultURL, "", "")
	s.logger.Info("generation succeeded",
		zap.Int64("task_id", task.ID),
		zap.String("url", resultURL),
		zap.String("model", res.ModelUsed),
	)
}

// rewritePromptForSafety —— 调用 LLM 改写被内容审核拦截的提示词，失败时返回空字符串
func (s *imageService) rewritePromptForSafety(ctx context.Context, originalPrompt string) string {
	llmKey := s.cfg.Models.OpenAIKey
	if llmKey == "" && len(s.cfg.Models.OpenAIKeys) > 0 {
		llmKey = s.cfg.Models.OpenAIKeys[0]
	}
	llmBase := s.cfg.Models.OpenAIBase
	if llmKey == "" {
		return ""
	}

	systemMsg := `You are a prompt rewriter for AI image generation. The user's prompt was blocked by content moderation.
Rewrite it to be safe while preserving the artistic intent. Rules:
- Replace violent imagery (severed heads, hanging, stabbing) with symbolic alternatives (trophies, dramatic poses, flowing fabric)
- Replace sexual/nude descriptions with elegant clothed alternatives
- Replace self-harm references with dramatic/poetic alternatives
- Keep the Chinese classical literature artistic style
- Wrap the scene in "classical Chinese ink painting" or "anime illustration" framing
- Output ONLY the rewritten English prompt, nothing else.`

	reqBody := map[string]interface{}{
		"model": "gpt-5.4-mini",
		"messages": []map[string]string{
			{"role": "system", "content": systemMsg},
			{"role": "user", "content": "Rewrite this blocked prompt:\n" + originalPrompt},
		},
		"temperature": 0.4,
		"max_tokens":  512,
	}
	data, _ := json.Marshal(reqBody)

	llmCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	base := strings.TrimRight(llmBase, "/")
	base = strings.TrimSuffix(base, "/v1")
	httpReq, err := http.NewRequestWithContext(llmCtx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return ""
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+llmKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		s.logger.Warn("LLM prompt rewrite failed", zap.Error(err))
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		s.logger.Warn("LLM prompt rewrite non-200", zap.Int("status", resp.StatusCode))
		return ""
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return ""
	}

	rewritten := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	s.logger.Info("prompt rewritten for safety",
		zap.String("original", truncateStr(originalPrompt, 80)),
		zap.String("rewritten", truncateStr(rewritten, 80)),
	)
	return rewritten
}

// ResumeOrphanedTasks —— 启动时恢复上次崩溃遗留的 running/pending 任务并重新调度
// It resets running→pending, then re-dispatches pending tasks in batches.
func (s *imageService) ResumeOrphanedTasks(ctx context.Context) {
	// Step 1: Reset stale running tasks to pending
	n, err := s.repo.ResetStaleRunning(ctx)
	if err != nil {
		s.logger.Error("failed to reset stale running tasks", zap.Error(err))
	} else if n > 0 {
		s.logger.Info("reset stale running tasks on startup", zap.Int64("count", n))
	}

	// Step 2: Re-dispatch pending tasks in batches (use lastID cursor to avoid re-querying same tasks)
	const batchSize = 50
	total := int64(0)
	lastID := int64(0)
	for {
		tasks, err := s.repo.FindPendingAfterID(ctx, lastID, batchSize)
		if err != nil {
			s.logger.Error("failed to find pending tasks for resume", zap.Error(err))
			break
		}
		if len(tasks) == 0 {
			break
		}
		for _, task := range tasks {
			go s.RunGeneration(ctx, task)
			total++
			if task.ID > lastID {
				lastID = task.ID
			}
		}
		s.logger.Info("resumed pending batch",
			zap.Int("batch_size", len(tasks)),
			zap.Int64("total_resumed", total),
			zap.Int64("last_id", lastID),
		)
		// Small pause between batches to avoid flooding
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
	if total > 0 {
		s.logger.Info("orphaned task resume complete", zap.Int64("total", total))
	}
}

// StartStaleCleanup —— 定时清理卡在 running 超时的任务，重置为 pending 并重新调度
func (s *imageService) StartStaleCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.repo.ResetStaleRunningOlderThan(ctx, 8*time.Minute)
			if err != nil {
				s.logger.Error("stale cleanup: reset failed", zap.Error(err))
				continue
			}
			if n > 0 {
				s.logger.Info("stale cleanup: reset stuck running tasks", zap.Int64("count", n))
				tasks, err := s.repo.FindPendingAfterID(ctx, 0, int(n)+10)
				if err == nil {
					for _, t := range tasks {
						go s.RunGeneration(ctx, t)
					}
				}
			}
		}
	}
}

// truncateStr —— 将字符串截断到指定字符数，超出部分用省略号替代
func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// newOptimizedHTTPClient creates an HTTP client with connection pooling tuned for
// high-concurrency image upload/download workloads. Keep-alive connections are
// reused across requests, TLS handshakes are amortised, and idle connections are
// retained longer to avoid repeated TCP setup during burst traffic.
func newOptimizedHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   20,
			MaxConnsPerHost:       50,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ForceAttemptHTTP2:     true,
		},
	}
}

func usesLocalModel(modelName string) bool {
	return modelName == "" || modelName == "sdxl"
}

// modelDisplayNameAliases maps Chinese UI display names to their API key registered in generators.
// This allows frontend model labels (model.name from DB) to resolve even when the API key differs.
var modelDisplayNameAliases = map[string]string{
	"CogView-4 (智谱AI)":            "cogview-4",
	"CogView-3-Plus (智谱AI)":       "cogview-3-plus",
	"CogView (智谱AI)":               "cogview",
	"通义 Wanx2.1-Plus (阿里云)":       "wanx2.1-t2i-plus",
	"通义 Wanx2.1-Turbo (阿里云)":      "wanx2.1-t2i-turbo",
	"通义 Wan2.5-I2I (阿里云)":         "wan2.5-i2i-preview",
	"通义 (阿里云)":                     "tongyi",
	"香蕉2.1 (Gemini Flash)":         "banana2.1",
	"香蕉2.0 (Gemini Pro)":           "banana2.0",
	"星融2.5 (Gemini Flash)":         "xingrong2.5",
	"GPT-Image-1 (OpenAI via ppai)": "gpt-image-1",
	"GPT-Image-1.5 (OpenAI via ppai)": "gpt-image-1.5",
	"豆包 Seedream 4.0 (ByteDance)":  "doubao-seedream-4-0-250828",
	"豆包 Seedream 3.0-t2i (ByteDance)": "doubao-seedream-3-0-t2i",
	"千帆 Flux.1-Schnell":            "qianfan-flux.1-schnell",
	"千帆 Stable Diffusion XL":       "qianfan-stable-diffusion-xl",
	"Gemini Flash Image":            "gemini-3.1-flash-image",
	"Gemini Pro Image":              "gemini-3-pro-image",
	"Flux.1-Schnell":                "flux.1-schnell",
	"Stable Diffusion XL":          "stable-diffusion-xl",
	"百度 Image (文心一格)":              "baidu-img",
}

// selectGenerator —— 根据模型名称选择对应的图片生成器，默认返回 SDXL
func (s *imageService) selectGenerator(modelName string) generators.ImageGenerator {
	if gen, ok := s.generators[modelName]; ok {
		return gen
	}
	// Try Chinese display name alias mapping
	if apiKey, ok := modelDisplayNameAliases[modelName]; ok {
		if gen, ok := s.generators[apiKey]; ok {
			return gen
		}
	}
	s.logger.Warn("unknown model, falling back to sdxl",
		zap.String("model", modelName),
		zap.Strings("available", func() []string {
			names := make([]string, 0, len(s.generators))
			for k := range s.generators {
				names = append(names, k)
			}
			return names
		}()),
	)
	return s.generators["sdxl"]
}

// uploadToStorage —— 下载生成的图片并上传到存储服务（最多重试 3 次），返回存储 URL
func (s *imageService) uploadToStorage(ctx context.Context, taskID, projectID int64, sourceURL string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		url, err := s.doUpload(ctx, taskID, projectID, sourceURL)
		if err == nil {
			return url, nil
		}
		lastErr = err
		s.logger.Warn("upload attempt failed",
			zap.Int64("task_id", taskID),
			zap.Int("attempt", attempt+1),
			zap.Error(err),
		)
	}
	return "", fmt.Errorf("upload failed after 3 attempts: %w", lastErr)
}

// doUpload —— 执行单次图片下载并上传到存储服务的操作，返回存储 URL
func (s *imageService) doUpload(ctx context.Context, taskID, projectID int64, sourceURL string) (string, error) {
	var imageBytes []byte

	if strings.HasPrefix(sourceURL, "data:") {
		// Handle base64 data URI from generators (e.g., gpt-image-1.5)
		imageBytes = generators.DecodeBase64Image(sourceURL)
		if imageBytes == nil {
			return "", fmt.Errorf("invalid data URI")
		}
	} else {
		// Download image from URL
		dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
		if err != nil {
			return "", fmt.Errorf("build download request: %w", err)
		}
		dlResp, err := s.httpClient.Do(dlReq)
		if err != nil {
			return "", fmt.Errorf("download image: %w", err)
		}
		var readErr error
		imageBytes, readErr = io.ReadAll(dlResp.Body)
		dlResp.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("read image bytes: %w", readErr)
		}
	}

	// Upload to storage-service via multipart POST /api/v1/storage/upload
	uploadURL := fmt.Sprintf("%s/api/v1/storage/upload", s.cfg.Storage.BaseURL)
	filename := fmt.Sprintf("task_%d.png", taskID)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("bucket", "images")
	_ = writer.WriteField("user_id", "1")
	_ = writer.WriteField("project_id", fmt.Sprintf("%d", projectID))
	_ = writer.WriteField("category", "image")
	// Detect real image MIME type so CDN and third-party APIs receive the correct
	// Content-Type (e.g. image/png or image/jpeg) instead of application/octet-stream.
	imgContentType := http.DetectContentType(imageBytes)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	h.Set("Content-Type", imgContentType)
	part, err := writer.CreatePart(h)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(imageBytes); err != nil {
		return "", fmt.Errorf("write image bytes: %w", err)
	}
	writer.Close()

	upReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return "", fmt.Errorf("build upload request: %w", err)
	}
	upReq.Header.Set("Content-Type", writer.FormDataContentType())

	upResp, err := s.httpClient.Do(upReq)
	if err != nil {
		return "", fmt.Errorf("upload request: %w", err)
	}
	defer upResp.Body.Close()

	if upResp.StatusCode < 200 || upResp.StatusCode >= 300 {
		b, _ := io.ReadAll(upResp.Body)
		return "", fmt.Errorf("storage returned %d: %s", upResp.StatusCode, b)
	}

	// Parse storage response to get CDN URL
	var storageResp struct {
		Data struct {
			ObjectKey string `json:"object_key"`
			CdnURL    string `json:"cdn_url"`
			FileID    uint64 `json:"file_id"`
		} `json:"data"`
	}
	respBody, _ := io.ReadAll(upResp.Body)
	if err := json.Unmarshal(respBody, &storageResp); err == nil && storageResp.Data.CdnURL != "" {
		return storageResp.Data.CdnURL, nil
	}
	if storageResp.Data.ObjectKey != "" {
		return fmt.Sprintf("%s/api/v1/storage/url/%s", s.cfg.Storage.BaseURL, storageResp.Data.ObjectKey), nil
	}
	return fmt.Sprintf("%s/images/%d.png", s.cfg.Storage.BaseURL, taskID), nil
}

// GetTask —— 根据 ID 查询图片生成任务，返回任务详情
func (s *imageService) GetTask(ctx context.Context, id int64) (*model.ImageTask, error) {
	return s.repo.GetByID(ctx, id)
}

// ListTasks —— 按筛选条件分页查询图片任务列表，返回任务列表和总数
func (s *imageService) ListTasks(ctx context.Context, filter repository.ListFilter) ([]*model.ImageTask, int64, error) {
	return s.repo.List(ctx, filter)
}

// RetryTask —— 重试失败或已取消的任务，重置状态并重新调度，返回更新后的任务
func (s *imageService) RetryTask(ctx context.Context, id int64) (*model.ImageTask, error) {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if task.Status != model.StatusFailed && task.Status != model.StatusCancelled {
		return nil, fmt.Errorf("task %d cannot be retried (status: %s)", id, task.Status)
	}

	fields := map[string]interface{}{
		"status":    model.StatusPending,
		"error_msg": "",
	}
	if err := s.repo.UpdateFields(ctx, id, fields); err != nil {
		return nil, err
	}
	task.Status = model.StatusPending

	go s.RunGeneration(context.Background(), task)
	return task, nil
}

// CancelTask —— 取消状态为 pending 的图片生成任务，返回错误信息
func (s *imageService) CancelTask(ctx context.Context, id int64) error {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if task.Status != model.StatusPending {
		return fmt.Errorf("task %d cannot be cancelled (status: %s)", id, task.Status)
	}
	return s.repo.UpdateStatus(ctx, id, model.StatusCancelled, "", "", "")
}

func (s *imageService) DeleteProjectData(ctx context.Context, projectID int64) error {
	return s.repo.DeleteByProjectID(ctx, projectID)
}

// ModelStatus —— 检查所有注册生成器的可用状态，返回各模型 key 及其可用性
func (s *imageService) ModelStatus(ctx context.Context) []ModelStatusItem {
	// Primary ranked order
	primary := []string{"dalle", "dall-e-3", "gpt-4o-image", "flux", "wanx2.1-t2i-plus", "tongyi", "wanx2.1-t2i-turbo", "sdxl"}
	seen := map[string]bool{}
	items := make([]ModelStatusItem, 0, len(s.generators))
	for _, key := range primary {
		if gen, ok := s.generators[key]; ok {
			seen[key] = true
			items = append(items, ModelStatusItem{Key: key, Available: gen.IsAvailable(ctx)})
		}
	}
	// Append any remaining registered generators not in the primary list
	for key, gen := range s.generators {
		if !seen[key] {
			items = append(items, ModelStatusItem{Key: key, Available: gen.IsAvailable(ctx)})
		}
	}
	return items
}

// ModelCapabilities —— 返回各模型的当前能力声明、尺寸策略和验证级别，供前端/网关展示与预校验。
func (s *imageService) ModelCapabilities(ctx context.Context) []ModelCapabilityItem {
	statuses := s.ModelStatus(ctx)
	items := make([]ModelCapabilityItem, 0, len(statuses))
	for _, status := range statuses {
		policy := generators.ResolveImageSizePolicy(status.Key)
		item := ModelCapabilityItem{
			Key:              status.Key,
			Available:        status.Available,
			ProviderKey:      policy.ProviderKey,
			SizeMode:         string(policy.Mode),
			Verification:     string(policy.Verification),
			AllowedSizes:     policy.AllowedSizes,
			DefaultSquare:    policy.DefaultSquare,
			DefaultPortrait:  policy.DefaultPortrait,
			DefaultLandscape: policy.DefaultLandscape,
			MinWidth:         policy.MinWidth,
			MaxWidth:         policy.MaxWidth,
			MinHeight:        policy.MinHeight,
			MaxHeight:        policy.MaxHeight,
			RequireMultiple:  policy.RequireMultiple,
			Notes:            policy.Notes,
		}
		// 填充参考图能力声明
		if gen, ok := s.generators[status.Key]; ok {
			rc := gen.RefCapability()
			item.RefMode = string(rc.Mode)
			item.MaxRefs = rc.MaxRefs
			item.StrongRef = rc.StrongRef
		}
		items = append(items, item)
	}
	return items
}

// TaskTypeCapabilities —— 返回 task_type 与目标画幅意图的映射，供前端决定默认比例与 UI 提示。
func (s *imageService) TaskTypeCapabilities(ctx context.Context) []TaskTypeCapabilityItem {
	defaults := generators.TaskTypeRatioDefaults()
	order := []string{"portrait", "character-sheet", "poster", "storyboard", "scene-concept", "general"}
	items := make([]TaskTypeCapabilityItem, 0, len(order))
	for _, taskType := range order {
		items = append(items, TaskTypeCapabilityItem{
			TaskType:    taskType,
			RatioIntent: string(defaults[taskType]),
		})
	}
	return items
}

// GeminiChannels —— 并发探测每个 Gemini 代理渠道（GET {base}/v1/models），返回实时有效性。
// 超时设为 8s，适合 UI 快速刷新场景。
func (s *imageService) GeminiChannels(ctx context.Context) []GeminiChannelStatus {
	bases := s.cfg.Models.GeminiBases
	keys := s.cfg.Models.GeminiKeys
	if len(keys) == 0 {
		return []GeminiChannelStatus{}
	}

	maskKey := func(k string) string {
		if len(k) < 16 {
			return "••••••••"
		}
		return k[:8] + "••••••••" + k[len(k)-4:]
	}

	type result struct {
		idx int
		ok  bool
		err string
	}
	ch := make(chan result, len(keys))

	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	for i, key := range keys {
		base := ""
		if i < len(bases) {
			base = bases[i]
		} else if len(bases) > 0 {
			base = bases[len(bases)-1]
		}
		if base == "" {
			base = "https://generativelanguage.googleapis.com"
		}
		go func(idx int, b, k string) {
			url := strings.TrimRight(b, "/") + "/v1/models"
			req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, url, nil)
			if err != nil {
				ch <- result{idx, false, err.Error()}
				return
			}
			req.Header.Set("Authorization", "Bearer "+k)
			resp, err := s.httpClient.Do(req)
			if err != nil {
				ch <- result{idx, false, err.Error()}
				return
			}
			defer resp.Body.Close()
			io.Copy(io.Discard, resp.Body) //nolint:errcheck
			if resp.StatusCode == http.StatusOK {
				ch <- result{idx, true, ""}
			} else {
				ch <- result{idx, false, fmt.Sprintf("HTTP %d", resp.StatusCode)}
			}
		}(i, base, key)
	}

	items := make([]GeminiChannelStatus, len(keys))
	for i, key := range keys {
		base := ""
		if i < len(bases) {
			base = bases[i]
		} else if len(bases) > 0 {
			base = bases[len(bases)-1]
		}
		if base == "" {
			base = "https://generativelanguage.googleapis.com"
		}
		items[i] = GeminiChannelStatus{
			Index:   i,
			Base:    strings.TrimRight(base, "/"),
			KeyMask: maskKey(key),
		}
	}

	for range keys {
		r := <-ch
		items[r.idx].Valid = r.ok
		items[r.idx].Error = r.err
	}
	return items
}
