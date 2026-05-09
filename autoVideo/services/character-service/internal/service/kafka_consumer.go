package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/autovideo/character-service/internal/model"
	"github.com/autovideo/character-service/internal/stylepreset"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// defaultMaxConcurrent limits how many asset image-generation goroutines run in
// parallel.  5 concurrent submissions keeps image-service within standard
// single-API-key rate limits.  Tune via concurrency.max_generations in config.
const defaultMaxConcurrent = 5

type KafkaConsumer struct {
	reader         *kafka.Reader
	resultWriter   *kafka.Writer
	assetSvc       *AssetService
	imageBaseURL   string
	projectBaseURL string
	jwtSecret      string
	logger         *zap.Logger
	sem            chan struct{} // concurrency limiter
	httpClient     *http.Client  // reused across all requests for connection pooling
}

// NewKafkaConsumer —— 创建 Kafka 消费者实例，用于消费图像生成任务
func NewKafkaConsumer(
	brokers []string,
	group, topic, resultTopic, imageBaseURL, projectBaseURL, jwtSecret string,
	assetSvc *AssetService,
	logger *zap.Logger,
	maxConcurrent int,
) *KafkaConsumer {
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrent
	}
	return &KafkaConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			GroupID:  group,
			Topic:    topic,
			MinBytes: 1,
			MaxBytes: 10e6,
		}),
		resultWriter: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    resultTopic,
			Balancer: &kafka.LeastBytes{},
		},
		assetSvc:       assetSvc,
		imageBaseURL:   imageBaseURL,
		projectBaseURL: strings.TrimRight(projectBaseURL, "/"),
		jwtSecret:      jwtSecret,
		logger:         logger,
		sem:            make(chan struct{}, maxConcurrent),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        maxConcurrent + 10,
				MaxIdleConnsPerHost: maxConcurrent + 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Start —— 启动消费循环，持续读取 Kafka 消息并并发处理图像生成任务
func (c *KafkaConsumer) Start(ctx context.Context) {
	c.logger.Info("asset kafka consumer started", zap.Int("max_concurrent", cap(c.sem)))
	go c.recoverPendingAssets(ctx)
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logger.Error("read message", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		// Acquire semaphore before spawning — blocks if at concurrency limit.
		// The goroutine releases it early (after submitting to image-service) so the
		// slot is not held during the polling phase.
		c.sem <- struct{}{}
		go func(m kafka.Message) {
			c.handle(ctx, m, func() { <-c.sem })
		}(msg)
	}
}

// handle —— 处理单条 Kafka 消息：运行 LLM 提示词流水线、提交图像任务，并在提交后立即释放
// 并发信号量槽位，使轮询阶段与后续流水线任务并行执行。
func (c *KafkaConsumer) handle(ctx context.Context, msg kafka.Message, releaseSem func()) {
	// Release the semaphore exactly once — either right after image-service submission
	// (happy path) or via defer (error/early-return path).
	var once sync.Once
	release := func() { once.Do(releaseSem) }
	defer release()
	var req AssetGenerateRequest
	if err := json.Unmarshal(msg.Value, &req); err != nil {
		c.logger.Error("unmarshal asset request", zap.Error(err))
		return
	}
	currentAsset, err := c.assetSvc.GetByID(req.AssetID)
	if err != nil {
		c.logger.Warn("asset generation skipped: asset missing", zap.Uint64("asset_id", req.AssetID), zap.Error(err))
		return
	}
	if c.assetSvc.isProjectGenerationPaused(req.ProjectID) || currentAsset.Status == "paused" {
		if currentAsset.Status != "paused" {
			_ = c.assetSvc.markAssetPaused(currentAsset)
		}
		c.logger.Info("asset generation paused before processing", zap.Uint64("asset_id", req.AssetID), zap.Uint64("project_id", req.ProjectID))
		return
	}
	c.logger.Info("processing asset generation", zap.Uint64("asset_id", req.AssetID))
	_ = c.assetSvc.UpdateGenerationProgress(req.AssetID, buildAssetGenerationProgress("submitting", 14, "正在提交到图片服务", "", 0, req.ModelName))

	stylePreset := stylepreset.Default
	negativePrompt := buildProjectImageNegativePrompt(nil, req.Type)
	var profile *projectVisualProfile

	// Fetch visual profile FIRST so prompt composition is style-aware.
	if token, tokenErr := c.buildServiceToken(); tokenErr == nil {
		if p, profileErr := fetchProjectVisualProfile(ctx, c.projectBaseURL, req.ProjectID, token); profileErr == nil {
			profile = p
			stylePreset = projectImageStylePreset(profile)
			negativePrompt = buildProjectImageNegativePrompt(profile, req.Type)
		} else {
			c.logger.Warn("fetch project visual profile for asset generation", zap.Uint64("project_id", req.ProjectID), zap.Error(profileErr))
		}
	} else {
		c.logger.Warn("build project visual token for asset generation", zap.Uint64("project_id", req.ProjectID), zap.Error(tokenErr))
	}

	// Request-level style_preset overrides project profile (e.g. from /images page style picker).
	if strings.TrimSpace(req.StylePreset) != "" {
		stylePreset = strings.TrimSpace(req.StylePreset)
	}

	prompt := req.Prompt
	if prompt == "" {
		// LLM-refine the description before prompt composition (translates Chinese, injects skill hints).
		refinedDesc := c.assetSvc.refineAssetDescriptionForGeneration(
			ctx, req.ProjectID, req.Type, req.Name, req.Description,
			stylePreset, buildProjectVisualHintForAsset(profile, req.Type))
		// Compose a style-aware prompt now that stylePreset is known.
		prompt = composeAssetImagePrompt(req.Type, req.Name, refinedDesc, req.PromptSuffix, stylePreset)
	}

	if profile != nil {
		prompt = appendProjectVisualHintInline(prompt, buildProjectVisualHintForAsset(profile, req.Type))
	}

	// Final LLM polish: merge the mechanically assembled prompt (mixed Chinese/English
	// templates + appended visual hints) into a single coherent, semantic prompt.
	prompt = c.assetSvc.polishAssetImagePrompt(ctx, prompt, req.Type, stylePreset)

	// 传递明确的 task_type 避免 image-service 依靠提示词推断出错误尺寸。
	singleImageTaskType := assetTypeToTaskType(req.Type)
	taskID, err := c.submitImageTask(ctx, req.AssetID, req.ProjectID, prompt, req.ModelName, stylePreset, negativePrompt, req.Type, singleImageTaskType, 0, 0, 0, "")

	result := AssetGenerateResult{AssetID: req.AssetID}
	if err != nil {
		c.logger.Error("image task submit failed", zap.Uint64("asset_id", req.AssetID), zap.Error(err))
		result.Status = "failed"
		result.ErrorMsg = err.Error()
		_ = c.assetSvc.UpdateStatus(req.AssetID, "failed", err.Error())
		c.publishResult(ctx, result)
		return // defer release() fires here
	}

	// Submission succeeded — release semaphore early so the slot is free for the
	// next pipeline task while this goroutine polls the image-service.
	release()

	imageURL, err := c.pollImageTask(ctx, req.AssetID, taskID, req.ModelName)

	result = AssetGenerateResult{AssetID: req.AssetID}
	if err != nil {
		c.logger.Error("image generation failed", zap.Uint64("asset_id", req.AssetID), zap.Error(err))
		result.Status = "failed"
		result.ErrorMsg = err.Error()
		_ = c.assetSvc.UpdateStatus(req.AssetID, "failed", err.Error())
	} else {
		result.Status = "completed"
		result.ImageURL = imageURL
		_ = c.assetSvc.UpdateGenerated(req.AssetID, imageURL, prompt, req.ModelName)
	}

	c.publishResult(ctx, result)
}

// assetTypeToTaskType maps an asset type string to the image-service task_type value.
// This ensures size-normalization policy picks the correct aspect ratio defaults.
func assetTypeToTaskType(assetType string) string {
	switch strings.TrimSpace(strings.ToLower(assetType)) {
	case "character", "角色", "人物":
		return "portrait"
	case "scene", "场景", "地点":
		return "scene-concept"
	case "prop", "道具", "item", "物品":
		return "general"
	default:
		return ""
	}
}

// assetImageDimensions returns width and height for image generation based on asset type.
// Character three-view sheets need landscape orientation; scenes also benefit from wide framing.
// Props are best rendered in portrait (product/hero shot).
func assetImageDimensions(assetType string) (width, height int) {
	switch strings.TrimSpace(strings.ToLower(assetType)) {
	case "character", "角色", "人物":
		return 512, 768 // portrait — 3:4 竖幅，纯白背景单人全身
	case "scene", "场景", "地点":
		return 896, 512 // wide landscape — cinematic scene concept
	case "prop", "道具", "item", "物品":
		return 512, 768 // portrait — product / hero shot
	default:
		return 512, 768
	}
}

// submitImageTask —— 向图像服务提交生成任务并返回任务 ID，不轮询结果。
// 调用方应在释放信号量后再调用 pollImageTask 轮询。
// seed <= 0 表示由图像服务随机，>0 表示强制使用该 seed（四视图分栏共享同一 seed 以保证一致性）。
// referenceImageURL 非空时透传给 image-service，底层 generator 会把它作为参考图（Doubao/Seedream 支持）。
// taskType 透传给 image-service 的 task_type 字段（"portrait"/"character-sheet" 等），影响尺寸归一化策略；空串由 image-service 自行推断。
func (c *KafkaConsumer) submitImageTask(ctx context.Context, assetID, projectID uint64, prompt, modelName, stylePreset, negativePrompt, assetType, taskType string, width, height int, seed int64, referenceImageURL string) (int64, error) {
	if modelName == "" {
		modelName = "dalle"
	}
	if strings.TrimSpace(stylePreset) == "" {
		stylePreset = stylepreset.Default
	}
	if width <= 0 || height <= 0 {
		width, height = assetImageDimensions(assetType)
	}
	reqBody := map[string]interface{}{
		"project_id":      projectID,
		"prompt":          prompt,
		"negative_prompt": negativePrompt,
		"style_preset":    stylePreset,
		"model_name":      modelName,
		"user_id":         1,
		"width":           width,
		"height":          height,
	}
	if strings.TrimSpace(taskType) != "" {
		reqBody["task_type"] = taskType
	}
	if seed > 0 {
		reqBody["seed"] = seed
	}
	if strings.TrimSpace(referenceImageURL) != "" {
		reqBody["style_reference_url"] = referenceImageURL
	}
	data, _ := json.Marshal(reqBody)

	token, err := c.buildServiceToken()
	if err != nil {
		return 0, fmt.Errorf("build service token: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.imageBaseURL+"/api/v1/images/generate", bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("image-service call: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var submitResult struct {
		Code int `json:"code"`
		Data struct {
			ID     int64  `json:"id"`
			TaskID int64  `json:"task_id"`
			Status string `json:"status"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &submitResult); err != nil {
		return 0, fmt.Errorf("parse response: %w (body: %s)", err, string(body[:min(200, len(body))]))
	}
	if submitResult.Code != 200 && submitResult.Code != 0 {
		return 0, fmt.Errorf("image-service error: %s", submitResult.Message)
	}

	taskID := submitResult.Data.TaskID
	if taskID == 0 {
		taskID = submitResult.Data.ID
	}
	if taskID == 0 {
		return 0, fmt.Errorf("no task ID returned")
	}
	_ = c.assetSvc.UpdateGenerationProgress(assetID, buildAssetGenerationProgress("submitted", 24, "图片任务已创建", "等待图片服务开始处理", taskID, modelName))
	return taskID, nil
}

// callImageService —— 提交图像任务并同步等待结果（包装 submitImageTask + pollImageTask）。
// Kept for backward-compatibility; new call sites should use submitImageTask + pollImageTask directly.
func (c *KafkaConsumer) callImageService(ctx context.Context, assetID, projectID uint64, prompt, modelName, stylePreset, negativePrompt, assetType string) (string, error) {
	taskID, err := c.submitImageTask(ctx, assetID, projectID, prompt, modelName, stylePreset, negativePrompt, assetType, "", 0, 0, 0, "")
	if err != nil {
		return "", err
	}
	return c.pollImageTask(ctx, assetID, taskID, modelName)
}

func (c *KafkaConsumer) recoverPendingAssets(ctx context.Context) {
	assets, err := c.assetSvc.ListRecoverablePendingAssets(cap(c.sem) * 20)
	if err != nil {
		c.logger.Error("load recoverable pending assets", zap.Error(err))
		return
	}
	if len(assets) == 0 {
		return
	}
	c.logger.Info("recovering pending asset tasks", zap.Int("count", len(assets)))
	for i := range assets {
		select {
		case <-ctx.Done():
			return
		case c.sem <- struct{}{}:
		}
		go func(asset model.Asset) {
			c.recoverPendingAsset(ctx, &asset, func() { <-c.sem })
		}(assets[i])
	}
}

func (c *KafkaConsumer) recoverPendingAsset(ctx context.Context, asset *model.Asset, releaseSem func()) {
	// Release semaphore exactly once — before polling so the slot is free for new tasks.
	var once sync.Once
	release := func() { once.Do(releaseSem) }
	defer release() // safety net for early returns

	// 角色资产使用四视图流程（四个独立任务）。服务重启后只能从 generation_progress 中恢复
	// closeup 的单个 task_id，无法重建完整四栏 + 合成图。直接标为 failed，用户可在 UI 重试。
	if isCharacterAssetType(asset.Type) {
		c.logger.Warn("character panel asset cannot be auto-recovered after restart; marking failed for manual retry",
			zap.Uint64("asset_id", asset.ID))
		_ = c.assetSvc.UpdateStatus(asset.ID, "failed", "服务重启中断四视图生成，请重新触发生成")
		c.publishResult(ctx, AssetGenerateResult{
			AssetID:  asset.ID,
			Status:   "failed",
			ErrorMsg: "服务重启中断四视图生成，请重新触发生成",
		})
		return
	}

	metadata, err := parseAssetMetadata(asset.Metadata)
	if err != nil {
		c.logger.Warn("skip pending asset recovery: parse metadata failed", zap.Uint64("asset_id", asset.ID), zap.Error(err))
		return
	}
	raw, ok := metadata["generation_progress"].(map[string]interface{})
	if !ok {
		return
	}
	taskID := int64(numberValue(raw["task_id"]))
	if taskID == 0 {
		return
	}
	modelName := strings.TrimSpace(stringValue(raw["model_name"]))
	if modelName == "" {
		modelName = "dalle"
	}
	if c.assetSvc.isProjectGenerationPaused(asset.ProjectID) || asset.Status == "paused" {
		return
	}
	if err := c.assetSvc.repo.UpdateStatusOnly(asset.ID, "generating", ""); err != nil {
		c.logger.Warn("mark asset generating for recovery failed", zap.Uint64("asset_id", asset.ID), zap.Error(err))
	}

	// --- Re-build prompt using the full pipeline (same as initial generation) ---
	// This ensures recovered assets receive the same quality as newly generated ones.
	prompt := asset.PromptUsed
	stylePreset := stylepreset.Default

	if strings.TrimSpace(prompt) == "" {
		// Fetch the project visual profile so we get the correct style preset and hints.
		var profile *projectVisualProfile
		if token, tokenErr := c.buildServiceToken(); tokenErr == nil {
			if p, profileErr := fetchProjectVisualProfile(ctx, c.projectBaseURL, asset.ProjectID, token); profileErr == nil {
				profile = p
				stylePreset = projectImageStylePreset(profile)
			}
		}

		// Step 1: filter character references from scene/prop descriptions.
		description := asset.Description
		if isPropOrSceneAssetType(asset.Type) {
			description = stripCharacterMentions(asset.Type, description)
		}

		// Step 2: compose the style-aware prompt.
		prompt = composeAssetImagePrompt(asset.Type, asset.Name, description, asset.PromptUsed, stylePreset)

		// Step 3: append project visual hint (asset-type-filtered).
		if profile != nil {
			prompt = appendProjectVisualHintInline(prompt, buildProjectVisualHintForAsset(profile, asset.Type))
		}

		// Step 4: final LLM polish pass.
		prompt = c.assetSvc.polishAssetImagePrompt(ctx, prompt, asset.Type, stylePreset)
	}

	// LLM pipeline complete — release semaphore before the polling loop so the
	// slot is available for new pipeline tasks.
	release()

	imageURL, recoverErr := c.pollImageTask(ctx, asset.ID, taskID, modelName)
	result := AssetGenerateResult{AssetID: asset.ID}
	if recoverErr != nil {
		c.logger.Error("recover asset generation failed", zap.Uint64("asset_id", asset.ID), zap.Int64("task_id", taskID), zap.Error(recoverErr))
		result.Status = "failed"
		result.ErrorMsg = recoverErr.Error()
		_ = c.assetSvc.UpdateStatus(asset.ID, "failed", recoverErr.Error())
	} else {
		result.Status = "completed"
		result.ImageURL = imageURL
		_ = c.assetSvc.UpdateGenerated(asset.ID, imageURL, prompt, modelName)
	}
	c.publishResult(ctx, result)
}

func (c *KafkaConsumer) pollImageTask(ctx context.Context, assetID uint64, taskID int64, modelName string) (string, error) {
	token, err := c.buildServiceToken()
	if err != nil {
		return "", fmt.Errorf("build service token: %w", err)
	}
	pollURL := fmt.Sprintf("%s/api/v1/images/tasks/%d", c.imageBaseURL, taskID)
	deadline := time.Now().Add(20 * time.Minute)
	lastProgressKey := ""
	attempt := 0
	for time.Now().Before(deadline) {
		attempt++
		time.Sleep(2 * time.Second)
		pollReq, _ := http.NewRequestWithContext(ctx, "GET", pollURL, nil)
		pollReq.Header.Set("Authorization", "Bearer "+token)
		pollResp, err := c.httpClient.Do(pollReq)
		if err != nil {
			c.logger.Warn("poll image task failed", zap.Int64("task_id", taskID), zap.Error(err))
			continue
		}
		pollBody, _ := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()

		var pollResult struct {
			Code int `json:"code"`
			Data struct {
				Status    string `json:"status"`
				ResultURL string `json:"result_url"`
				ErrorMsg  string `json:"error_msg"`
			} `json:"data"`
		}
		if err := json.Unmarshal(pollBody, &pollResult); err != nil {
			continue
		}
		progress := progressFromTaskStatus(pollResult.Data.Status, attempt, taskID, modelName)
		progressKey := fmt.Sprintf("%s-%d", progress.Stage, progress.Percent)
		if progressKey != lastProgressKey {
			lastProgressKey = progressKey
			_ = c.assetSvc.UpdateGenerationProgress(assetID, progress)
		}
		switch pollResult.Data.Status {
		case "succeeded", "completed":
			if pollResult.Data.ResultURL != "" {
				return pollResult.Data.ResultURL, nil
			}
			return "", fmt.Errorf("task completed but no result URL")
		case "failed":
			if pollResult.Data.ErrorMsg != "" {
				return "", fmt.Errorf("%s", pollResult.Data.ErrorMsg)
			}
			return "", fmt.Errorf("image generation task %d failed", taskID)
		}
	}
	return "", fmt.Errorf("image generation task %d timed out after 20 minutes", taskID)
}

func progressFromTaskStatus(status string, attempt int, taskID int64, modelName string) assetGenerationProgress {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "queued", "pending", "":
		return buildAssetGenerationProgress("queued", min(38, 24+attempt*2), "图片服务排队中", "任务已提交，等待分配算力", taskID, modelName)
	case "running", "processing", "in_progress":
		return buildAssetGenerationProgress("rendering", min(92, 42+attempt*3), "AI 正在生成图片", "模型已经开始渲染，请稍候", taskID, modelName)
	case "succeeded", "completed":
		return buildAssetGenerationProgress("completed", 100, "图片生成完成", "正在回传最新结果", taskID, modelName)
	case "failed":
		return buildAssetGenerationProgress("failed", 100, "图片生成失败", "图片服务返回失败状态", taskID, modelName)
	default:
		return buildAssetGenerationProgress("processing", min(88, 36+attempt*3), "正在处理图片任务", normalized, taskID, modelName)
	}
}

// buildServiceToken —— 构建服务间调用的最小 JWT Token
// buildServiceToken creates a minimal JWT for service-to-service calls.
func (c *KafkaConsumer) buildServiceToken() (string, error) {
	header := base64url(`{"alg":"HS256","typ":"JWT"}`)
	now := time.Now().Unix()
	claims := fmt.Sprintf(`{"user_id":1,"role":"service","iss":"autovideo-character","exp":%d,"iat":%d}`, now+300, now)
	payload := base64url(claims)
	sigInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(c.jwtSecret))
	mac.Write([]byte(sigInput))
	sig := strings.TrimRight(base64.URLEncoding.EncodeToString(mac.Sum(nil)), "=")
	return sigInput + "." + sig, nil
}

// base64url —— 将字符串进行 Base64 URL 编码（去除填充符）
func base64url(s string) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString([]byte(s)), "=")
}

// publishResult —— 将图像生成结果发布到 Kafka 结果主题
func (c *KafkaConsumer) publishResult(ctx context.Context, result AssetGenerateResult) {
	data, err := json.Marshal(result)
	if err != nil {
		c.logger.Error("marshal result", zap.Error(err))
		return
	}
	if err := c.resultWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(fmt.Sprintf("asset-%d", result.AssetID)),
		Value: data,
	}); err != nil {
		c.logger.Error("publish result", zap.Error(err))
	}
}

// Close —— 关闭 Kafka 消费者和结果写入器的连接
func (c *KafkaConsumer) Close() error {
	c.reader.Close()
	return c.resultWriter.Close()
}

// modelFamily —— 粗粒度归类图像模型家族，用于 generator 分支（Doubao 支持串行参考图，其他模型并行）。
// 返回值： "doubao" | "sdxl" | "flux" | "tongyi" | "openai" | "gemini" | "baidu" | "other"
func modelFamily(modelName string) string {
	n := strings.ToLower(strings.TrimSpace(modelName))
	switch {
	case n == "":
		return "other"
	case strings.Contains(n, "doubao") || strings.Contains(n, "seedream") || strings.Contains(n, "seedance"):
		return "doubao"
	case strings.Contains(n, "sdxl") || strings.Contains(n, "stable-diffusion") || strings.Contains(n, "sd-xl"):
		return "sdxl"
	case strings.Contains(n, "flux"):
		return "flux"
	case strings.Contains(n, "tongyi") || strings.Contains(n, "wanx") || strings.Contains(n, "qwen"):
		return "tongyi"
	case strings.Contains(n, "dalle") || strings.Contains(n, "dall-e") || strings.Contains(n, "gpt-image") || strings.Contains(n, "openai"):
		return "openai"
	case strings.Contains(n, "gemini") || strings.Contains(n, "imagen"):
		return "gemini"
	case strings.Contains(n, "baidu") || strings.Contains(n, "ernie"):
		return "baidu"
	case strings.Contains(n, "cogview") || strings.Contains(n, "zhipu"):
		return "cogview"
	default:
		return "other"
	}
}

// generateCharacterPanels —— 角色资产的四视图生成编排核心。
// 流程：
//  1. 生成唯一 seed（全 4 栏共享，提升跨栏一致性——对支持 seed 的后端）。
//  2. 按家族选择并发/串行：Doubao 串行（先生成 closeup 作为面部参考，再并行生成 front/side/back，面部一致性更好），其他全并行。
//  3. 每栏独立 composeCharacterPanelPromptWithStyle + 前缀样式 + profile 视觉提示 + LLM polish。
//  4. 收集 4 个 URL → composeHorizontalPanels 拼接 → 上传 storage-service → UpdateGeneratedPanels 落库。
//
// release —— handle 外层传入的信号量释放函数；在 4 个 submit 完成后释放（比单图路径稍晚，但避免并发爆仓）。
func (c *KafkaConsumer) generateCharacterPanels(
	ctx context.Context,
	req AssetGenerateRequest,
	profile *projectVisualProfile,
	stylePreset string,
	negativePrompt string,
	refinedDesc string,
	release func(),
) {
	assetID := req.AssetID
	result := AssetGenerateResult{AssetID: assetID}

	// 1. 同一 seed：对支持 seed 的 generator（SDXL / Flux / Doubao / Tongyi / CogView）锁定种子
	//    提升 4 栏一致性；OpenAI / Gemini 不支持 seed —— 由描述冗余 + 参考图补偿。
	seed := time.Now().UnixNano()&0x7fffffff + 1 // >0，避免被误认为"随机"

	panels := CharacterPanelOrder() // [front, closeup, side, back]
	panelURLs := make([]string, len(panels))
	panelPrompts := make([]string, len(panels))

	visualHint := buildProjectVisualHintForAsset(profile, req.Type)
	stylePrefix := buildCharacterStylePrefix(stylePreset)

	// buildPanelPrompt —— 构造某一栏的最终提示词。
	buildPanelPrompt := func(panel CharacterPanel) string {
		p := composeCharacterPanelPromptWithStyle(req.Name, refinedDesc, req.PromptSuffix, stylePreset, panel)
		if stylePrefix != "" {
			p = stylePrefix + ", " + strings.TrimLeft(p, ", ")
		}
		if visualHint != "" {
			p = appendProjectVisualHintInline(p, visualHint)
		}
		// Per-panel polish —— 保持 LLM 不破坏"single subject"约束：polishAssetImagePrompt 已内置 character guardrails。
		p = c.assetSvc.polishAssetImagePrompt(ctx, p, req.Type, stylePreset)
		return p
	}

	submitPanel := func(panel CharacterPanel, refURL string) (int64, string, error) {
		prompt := buildPanelPrompt(panel)
		w, h := panelImageAspect(panel)
		panelNeg := appendPanelSpecificNegatives(negativePrompt, panel)
		// closeup 使用 portrait 任务类型以优先选取竖幅尺寸（如 Doubao 960×1280 而非 1024×1024）；
		// 其余全身视图使用 character-view（单张全身人物视图语义，避免 character-sheet 多栏布局暗示影响生成）。
		panelTaskType := "character-view"
		if panel == CharacterPanelCloseup {
			panelTaskType = "portrait"
		}
		taskID, err := c.submitImageTask(
			ctx, assetID, req.ProjectID, prompt, req.ModelName, stylePreset,
			panelNeg, req.Type, panelTaskType, w, h, seed, refURL,
		)
		return taskID, prompt, err
	}

	pollPanel := func(panel CharacterPanel, taskID int64) (string, error) {
		return c.pollImageTask(ctx, assetID, taskID, req.ModelName)
	}

	_ = c.assetSvc.UpdateGenerationProgress(assetID, buildAssetGenerationProgress(
		"submitting", 15, "四视图流程 · 优先生成特写 (共4栏)", "", 0, req.ModelName))

	if modelFamily(req.ModelName) == "doubao" {
		// ── 串行路径（Doubao/Seedream）：
		// 先生成 closeup（panels[1]）获取清晰面部参考图，
		// 再把 closeup URL 作为 reference_image_url 传给其余三栏，
		// 使 front/side/back 与 closeup 面部保持一致。
		closeupIdx := -1
		for i, p := range panels {
			if p == CharacterPanelCloseup {
				closeupIdx = i
				break
			}
		}
		if closeupIdx < 0 {
			closeupIdx = 1 // 安全兜底
		}

		tidCloseup, promptCloseup, err := submitPanel(panels[closeupIdx], "")
		if err != nil {
			c.failPanelsRun(ctx, assetID, result, fmt.Errorf("submit closeup: %w", err), release)
			return
		}
		panelPrompts[closeupIdx] = promptCloseup
		urlCloseup, err := pollPanel(panels[closeupIdx], tidCloseup)
		if err != nil || urlCloseup == "" {
			c.failPanelsRun(ctx, assetID, result, fmt.Errorf("poll closeup: %w", err), release)
			return
		}
		panelURLs[closeupIdx] = urlCloseup

		// 剩余 3 栏并行，用 closeup URL 作为 reference_image_url 提升面部一致性
		type panelResult struct {
			idx int
			url string
			err error
			p   string
		}
		ch := make(chan panelResult, 3)
		for i := range panels {
			if i == closeupIdx {
				continue
			}
			go func(i int) {
				tid, prompt, err := submitPanel(panels[i], urlCloseup)
				if err != nil {
					ch <- panelResult{idx: i, err: err}
					return
				}
				u, err := pollPanel(panels[i], tid)
				ch <- panelResult{idx: i, url: u, err: err, p: prompt}
			}(i)
		}
		for range make([]struct{}, 3) {
			r := <-ch
			if r.err != nil {
				c.logger.Warn("panel generation failed", zap.Int("idx", r.idx), zap.Error(r.err))
			} else {
				panelURLs[r.idx] = r.url
				panelPrompts[r.idx] = r.p
			}
		}
	} else {
		// ── 并行路径（OpenAI / Tongyi / SDXL / Flux 等）──
		// 这些模型不使用 reference_image_url，4 栏同时提交可节省时间。
		type panelResult struct {
			idx int
			url string
			err error
			p   string
		}
		ch := make(chan panelResult, 4)
		for i, p := range panels {
			go func(i int, panel CharacterPanel) {
				tid, prompt, err := submitPanel(panel, "")
				if err != nil {
					ch <- panelResult{idx: i, err: err}
					return
				}
				u, err := pollPanel(panel, tid)
				ch <- panelResult{idx: i, url: u, err: err, p: prompt}
			}(i, p)
		}
		for i := 0; i < 4; i++ {
			r := <-ch
			if r.err != nil {
				c.logger.Warn("panel generation failed", zap.Int("idx", r.idx), zap.Error(r.err))
			} else {
				panelURLs[r.idx] = r.url
				panelPrompts[r.idx] = r.p
			}
		}
	}

	// 提交阶段全部走完，信号量已无用 —— 释放
	release()

	// 至少需要一个 URL 才能继续，否则整体失败
	validCount := 0
	for _, u := range panelURLs {
		if strings.TrimSpace(u) != "" {
			validCount++
		}
	}
	if validCount == 0 {
		result.Status = "failed"
		result.ErrorMsg = "all 4 panels failed"
		_ = c.assetSvc.UpdateStatus(assetID, "failed", result.ErrorMsg)
		c.publishResult(ctx, result)
		return
	}

	// 2. 拼接
	_ = c.assetSvc.UpdateGenerationProgress(assetID, buildAssetGenerationProgress(
		"compositing", 85, fmt.Sprintf("拼接四视图 (有效 %d/4 栏)", validCount), "", 0, req.ModelName))

	// 只把有效 URL 送入 compositor；若不足 4 个，仍按现有顺序拼接（空位跳过）
	nonEmpty := make([]string, 0, 4)
	for _, u := range panelURLs {
		if strings.TrimSpace(u) != "" {
			nonEmpty = append(nonEmpty, u)
		}
	}
	compositeBytes, err := composeHorizontalPanels(ctx, c.httpClient, nonEmpty)
	compositeURL := ""
	if err != nil {
		c.logger.Error("composite panels failed", zap.Uint64("asset_id", assetID), zap.Error(err))
	} else if storage := c.assetSvc.Storage(); storage != nil {
		url, uploadErr := storage.Upload(compositeFileName(assetID), bytes.NewReader(compositeBytes))
		if uploadErr != nil {
			c.logger.Error("upload composite failed", zap.Uint64("asset_id", assetID), zap.Error(uploadErr))
		} else {
			compositeURL = url
		}
	}
	// 兜底：没拼成功时把第一个有效 panel 视作 composite，保证老前端有图可显
	if compositeURL == "" && len(nonEmpty) > 0 {
		compositeURL = nonEmpty[0]
	}

	// 3. 落库：把 4 栏 URL + composite + 使用的 seed 写入 panel_images / composite_image_url / seed 字段
	// 使用首个非空 prompt 作为代表性 prompt 写 prompt_used 字段
	primaryPrompt := ""
	for _, p := range panelPrompts {
		if strings.TrimSpace(p) != "" {
			primaryPrompt = p
			break
		}
	}
	if err := c.assetSvc.UpdateGeneratedPanels(assetID, panelURLs, compositeURL, primaryPrompt, req.ModelName, seed); err != nil {
		c.logger.Error("persist panels failed", zap.Uint64("asset_id", assetID), zap.Error(err))
		result.Status = "failed"
		result.ErrorMsg = err.Error()
		_ = c.assetSvc.UpdateStatus(assetID, "failed", err.Error())
		c.publishResult(ctx, result)
		return
	}

	result.Status = "completed"
	if validCount < 4 {
		result.Status = "partial"
	}
	// 主资源图用正面视图（front，在新顺序中为 panelURLs[0]），拼接图仅存 composite_image_url。
	primaryURL := ""
	if len(panelURLs) > 0 && strings.TrimSpace(panelURLs[0]) != "" {
		primaryURL = panelURLs[0]
	} else {
		for _, u := range panelURLs {
			if strings.TrimSpace(u) != "" {
				primaryURL = u
				break
			}
		}
	}
	if primaryURL == "" {
		primaryURL = compositeURL
	}
	result.ImageURL = primaryURL
	c.publishResult(ctx, result)

	// 4. AI 视觉质检 + 自动重试（异步，不阻塞主流程）
	go func() {
		bgCtx := context.Background()

		// ── 第一轮质检 ──────────────────────────────────────────────────
		vCtx, vCancel := context.WithTimeout(bgCtx, 60*time.Second)
		report, err := c.assetSvc.VisionValidatePanels(vCtx, panelURLs, req.Name, req.Description)
		vCancel()
		if err != nil {
			c.logger.Warn("vision QA skipped (round-1)", zap.Uint64("asset_id", assetID), zap.Error(err))
			return
		}
		_ = c.assetSvc.UpdatePanelValidation(assetID, report)
		c.logger.Info("vision QA round-1", zap.Uint64("asset_id", assetID), zap.Bool("pass", report.OverallPass))
		if report.OverallPass {
			return
		}

		// ── 收集失败栏位 ─────────────────────────────────────────────────
		var failedPanels []string
		for _, p := range report.Panels {
			if !p.Pass {
				failedPanels = append(failedPanels, p.Panel)
			}
		}
		if len(failedPanels) == 0 {
			failedPanels = []string{"closeup", "front", "side", "back"}
		}
		regenModel := strings.TrimSpace(req.ModelName)
		if regenModel == "" {
			regenModel = "dalle"
		}
		c.logger.Info("vision QA failed, auto-retrying panels",
			zap.Uint64("asset_id", assetID), zap.Strings("panels", failedPanels))

		// ── 逐栏自动重绘 ─────────────────────────────────────────────────
		for _, panel := range failedPanels {
			regenCtx, regenCancel := context.WithTimeout(bgCtx, 150*time.Second)
			regenErr := c.RegenPanel(regenCtx, assetID, panel, "", regenModel)
			regenCancel()
			if regenErr != nil {
				c.logger.Warn("auto-regen panel failed",
					zap.Uint64("asset_id", assetID), zap.String("panel", panel), zap.Error(regenErr))
			}
		}

		// ── 第二轮质检 ──────────────────────────────────────────────────
		freshAsset, fetchErr := c.assetSvc.GetByID(assetID)
		if fetchErr != nil {
			c.logger.Warn("fetch asset for re-QA failed", zap.Uint64("asset_id", assetID), zap.Error(fetchErr))
			return // 保持 completed 状态
		}
		newURLs := make([]string, 4)
		for i, u := range freshAsset.PanelImages {
			if i < 4 {
				newURLs[i] = u
			}
		}
		vCtx2, vCancel2 := context.WithTimeout(bgCtx, 60*time.Second)
		report2, err2 := c.assetSvc.VisionValidatePanels(vCtx2, newURLs, req.Name, req.Description)
		vCancel2()
		if err2 != nil {
			c.logger.Warn("vision QA skipped (round-2)", zap.Uint64("asset_id", assetID), zap.Error(err2))
			return
		}
		_ = c.assetSvc.UpdatePanelValidation(assetID, report2)
		c.logger.Info("vision QA round-2", zap.Uint64("asset_id", assetID), zap.Bool("pass", report2.OverallPass))
		if !report2.OverallPass {
			_ = c.assetSvc.UpdateStatus(assetID, "qa_failed", report2.Summary)
			c.logger.Warn("vision QA still failing after retry, marked qa_failed",
				zap.Uint64("asset_id", assetID), zap.String("summary", report2.Summary))
		}
	}()
}

// failPanelsRun —— 四视图早期失败的公共退出路径。
func (c *KafkaConsumer) failPanelsRun(ctx context.Context, assetID uint64, result AssetGenerateResult, err error, release func()) {
	release()
	c.logger.Error("character panels run failed", zap.Uint64("asset_id", assetID), zap.Error(err))
	result.Status = "failed"
	result.ErrorMsg = err.Error()
	_ = c.assetSvc.UpdateStatus(assetID, "failed", err.Error())
	c.publishResult(ctx, result)
}

// RegenPanel —— 重绘角色资产的某一栏（closeup/front/side/back），复用原 seed，替换对应索引的 URL，
// 重新拼接 composite 并落库。promptOverride 非空时直接当最终 prompt 使用（跳过模板/polish）。
// modelName 空串时沿用资产现有模型；panel 必须是 4 栏之一。
func (c *KafkaConsumer) RegenPanel(ctx context.Context, assetID uint64, panelStr, promptOverride, modelName string) error {
	asset, err := c.assetSvc.GetByID(assetID)
	if err != nil {
		return fmt.Errorf("get asset: %w", err)
	}
	if !isCharacterAssetType(asset.Type) {
		return fmt.Errorf("regen-panel only supported for character assets (got %q)", asset.Type)
	}
	panel := parseCharacterPanel(panelStr)
	if panel == "" {
		return fmt.Errorf("invalid panel %q (must be one of closeup/front/side/back)", panelStr)
	}
	if strings.TrimSpace(modelName) == "" {
		// Asset model 本身不存 model_name（pipeline 层透传），老资产无处可取，直接用默认。
		modelName = "dalle"
	}

	// 定位目标栏索引
	order := CharacterPanelOrder()
	idx := -1
	for i, p := range order {
		if p == panel {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("unknown panel index for %q", panel)
	}

	// 拉取 visual profile（风格 / 负向提示词 / 视觉提示）
	stylePreset := stylepreset.Default
	negativePrompt := buildProjectImageNegativePrompt(nil, asset.Type)
	var profile *projectVisualProfile
	if token, tokenErr := c.buildServiceToken(); tokenErr == nil {
		if p, profileErr := fetchProjectVisualProfile(ctx, c.projectBaseURL, asset.ProjectID, token); profileErr == nil {
			profile = p
			stylePreset = projectImageStylePreset(profile)
			negativePrompt = buildProjectImageNegativePrompt(profile, asset.Type)
		}
	}

	// 组装 prompt
	var prompt string
	if strings.TrimSpace(promptOverride) != "" {
		prompt = strings.TrimSpace(promptOverride)
	} else {
		refined := strings.TrimSpace(asset.Description)
		if refined == "" && strings.TrimSpace(asset.PromptUsed) != "" {
			refined = strings.TrimSpace(asset.PromptUsed)
		}
		prompt = composeCharacterPanelPromptWithStyle(asset.Name, refined, asset.PromptUsed, stylePreset, panel)
		if prefix := buildCharacterStylePrefix(stylePreset); prefix != "" {
			prompt = prefix + ", " + strings.TrimLeft(prompt, ", ")
		}
		if hint := buildProjectVisualHintForAsset(profile, asset.Type); hint != "" {
			prompt = appendProjectVisualHintInline(prompt, hint)
		}
		prompt = c.assetSvc.polishAssetImagePrompt(ctx, prompt, asset.Type, stylePreset)
	}

	// seed：沿用资产现有 seed；若为 0 则生成新 seed（老数据兼容）
	seed := asset.Seed
	if seed <= 0 {
		seed = time.Now().UnixNano()&0x7fffffff + 1
	}

	// Doubao：非 closeup 栏使用现有 closeup URL 作为 reference，提升面部一致性
	// closeup 的索引由 CharacterPanelOrder() 动态确定，不依赖硬编码 [0]。
	refURL := ""
	if panel != CharacterPanelCloseup {
		for i, p := range CharacterPanelOrder() {
			if p == CharacterPanelCloseup && i < len(asset.PanelImages) {
				if cu := strings.TrimSpace(asset.PanelImages[i]); cu != "" {
					refURL = cu
				}
				break
			}
		}
	}

	w, h := panelImageAspect(panel)
	panelNeg := appendPanelSpecificNegatives(negativePrompt, panel)

	_ = c.assetSvc.UpdateGenerationProgress(assetID, buildAssetGenerationProgress(
		"submitting", 20, fmt.Sprintf("单栏重绘 · %s", string(panel)), "", 0, modelName))

	// 重绘时同样传递 panel 专属的 task_type，确保尺寸归一化与首次生成一致。
	regenPanelTaskType := "character-view"
	if panel == CharacterPanelCloseup {
		regenPanelTaskType = "portrait"
	}
	taskID, err := c.submitImageTask(ctx, assetID, asset.ProjectID, prompt, modelName, stylePreset, panelNeg, asset.Type, regenPanelTaskType, w, h, seed, refURL)
	if err != nil {
		_ = c.assetSvc.UpdateStatus(assetID, "failed", err.Error())
		return fmt.Errorf("submit panel: %w", err)
	}
	newURL, err := c.pollImageTask(ctx, assetID, taskID, modelName)
	if err != nil || strings.TrimSpace(newURL) == "" {
		_ = c.assetSvc.UpdateStatus(assetID, "failed", fmt.Sprintf("poll panel: %v", err))
		if err == nil {
			err = fmt.Errorf("empty url")
		}
		return fmt.Errorf("poll panel: %w", err)
	}

	// 合并到现有 PanelImages（老资产可能为空，按 4 栏补齐）
	panelURLs := make([]string, 4)
	for i, u := range asset.PanelImages {
		if i < 4 {
			panelURLs[i] = u
		}
	}
	panelURLs[idx] = newURL

	// 重新拼接
	_ = c.assetSvc.UpdateGenerationProgress(assetID, buildAssetGenerationProgress(
		"compositing", 85, "重绘完成 · 拼接", "", 0, modelName))
	nonEmpty := make([]string, 0, 4)
	for _, u := range panelURLs {
		if strings.TrimSpace(u) != "" {
			nonEmpty = append(nonEmpty, u)
		}
	}
	compositeURL := asset.CompositeImageURL
	if composite, cerr := composeHorizontalPanels(ctx, c.httpClient, nonEmpty); cerr == nil {
		if storage := c.assetSvc.Storage(); storage != nil {
			if url, upErr := storage.Upload(compositeFileName(assetID), bytes.NewReader(composite)); upErr == nil {
				compositeURL = url
			} else {
				c.logger.Warn("regen-panel upload composite failed", zap.Error(upErr))
			}
		}
	} else {
		c.logger.Warn("regen-panel composite failed", zap.Error(cerr))
	}
	if compositeURL == "" {
		compositeURL = newURL
	}

	if err := c.assetSvc.UpdateGeneratedPanels(assetID, panelURLs, compositeURL, prompt, modelName, seed); err != nil {
		return fmt.Errorf("persist panels: %w", err)
	}
	// 主资源图用正面视图（front，在新顺序中为 panelURLs[0]），拼接图仅存 composite_image_url。
	regenPrimaryURL := ""
	if len(panelURLs) > 0 && strings.TrimSpace(panelURLs[0]) != "" {
		regenPrimaryURL = panelURLs[0]
	} else {
		for _, u := range panelURLs {
			if strings.TrimSpace(u) != "" {
				regenPrimaryURL = u
				break
			}
		}
	}
	if regenPrimaryURL == "" {
		regenPrimaryURL = compositeURL
	}
	c.publishResult(ctx, AssetGenerateResult{AssetID: assetID, Status: "completed", ImageURL: regenPrimaryURL})
	return nil
}
