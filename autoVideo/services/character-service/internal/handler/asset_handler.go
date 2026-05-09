package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/autovideo/character-service/internal/model"
	"github.com/autovideo/character-service/internal/service"
	"github.com/autovideo/character-service/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// maxConcurrentEpisodeExtractions limits how many episode extractions run simultaneously
// to avoid overwhelming the LLM API with too many concurrent requests.
const maxConcurrentEpisodeExtractions = 4

type AssetHandler struct {
	svc          *service.AssetService
	extractSvc   *service.ExtractService
	logger       *zap.Logger
	extractSem   chan struct{}
	geminiBases  []string
	geminiKeys   []string
	geminiModel  string
}

// NewAssetHandler —— 创建资产处理器实例，返回 *AssetHandler
func NewAssetHandler(svc *service.AssetService, extractSvc *service.ExtractService, logger *zap.Logger) *AssetHandler {
	return &AssetHandler{svc: svc, extractSvc: extractSvc, logger: logger, extractSem: make(chan struct{}, maxConcurrentEpisodeExtractions)}
}

// SetGemini 配置 Gemini 多模态渠道
func (h *AssetHandler) SetGemini(bases, keys []string, model string) {
	h.geminiBases = bases
	h.geminiKeys = keys
	h.geminiModel = model
}

// ListAssets —— 获取项目下的资产列表，支持按类型、状态筛选和分页
// ListAssets GET /api/v1/projects/:pid/assets?type=character&status=completed&page=1&page_size=50
func (h *AssetHandler) ListAssets(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	assetType := c.Query("type")
	status := c.Query("status")
	keyword := strings.TrimSpace(c.Query("keyword"))
	var episodeID *uint64
	if raw := strings.TrimSpace(c.Query("episode_id")); raw != "" {
		if parsed, parseErr := strconv.ParseUint(raw, 10, 64); parseErr == nil {
			episodeID = &parsed
		} else {
			response.Error(c, http.StatusBadRequest, "invalid episode id")
			return
		}
	}

	// Parse pagination params
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 1000 {
		pageSize = 50
	}

	// If no pagination params given and no status filter, fallback to legacy full list for backward compatibility
	if c.Query("page") == "" && c.Query("page_size") == "" && status == "" && keyword == "" {
		assets, err := h.svc.List(pid, assetType, keyword, episodeID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.Success(c, assets)
		return
	}

	result, err := h.svc.ListPaginated(pid, assetType, status, keyword, episodeID, page, pageSize)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, result)
}

// GetAsset —— 根据 ID 获取单个资产详情
// GetAsset GET /api/v1/projects/:pid/assets/:id
func (h *AssetHandler) GetAsset(c *gin.Context) {
	id, err := parseAssetID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid asset id")
		return
	}
	asset, err := h.svc.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "asset not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, asset)
}

// CreateAsset —— 为项目创建新资产（角色/场景/道具）
// CreateAsset POST /api/v1/projects/:pid/assets
func (h *AssetHandler) CreateAsset(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	var req model.Asset
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" || req.Type == "" {
		response.Error(c, http.StatusBadRequest, "name and type are required")
		return
	}
	if req.Type != "character" && req.Type != "scene" && req.Type != "prop" && req.Type != "image" {
		response.Error(c, http.StatusBadRequest, "type must be character, scene, prop, or image")
		return
	}
	req.ProjectID = pid
	if err := h.svc.Create(&req); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessWithStatus(c, http.StatusCreated, req)
}

// UpdateAsset —— 部分更新指定资产的字段
// UpdateAsset PATCH /api/v1/projects/:pid/assets/:id
func (h *AssetHandler) UpdateAsset(c *gin.Context) {
	id, err := parseAssetID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid asset id")
		return
	}
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	asset, err := h.svc.Update(id, req)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "asset not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, asset)
}

// DeleteAsset —— 删除指定资产，锁定资产不可删除
// DeleteAsset DELETE /api/v1/projects/:pid/assets/:id
func (h *AssetHandler) DeleteAsset(c *gin.Context) {
	id, err := parseAssetID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid asset id")
		return
	}
	if err := h.svc.Delete(id); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "asset not found")
			return
		}
		if errors.Is(err, service.ErrAssetLocked) {
			response.Error(c, http.StatusConflict, "asset is locked and cannot be deleted")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// DeleteAllAssets —— 删除项目下所有未锁定的资产
// DeleteAllAssets DELETE /api/v1/projects/:pid/assets
// Removes all unlocked assets for the project (including extracting sentinels).
func (h *AssetHandler) DeleteAllAssets(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	if err := h.svc.DeleteByProjectID(pid); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.logger.Info("all unlocked assets deleted", zap.Uint64("project_id", pid))
	response.Success(c, gin.H{"deleted": true})
}

// GenerateAsset —— 触发单个资产的图像生成
// GenerateAsset POST /api/v1/projects/:pid/assets/:id/generate
func (h *AssetHandler) GenerateAsset(c *gin.Context) {
	id, err := parseAssetID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid asset id")
		return
	}
	var req struct {
		ModelName    string `json:"model_name"`
		PromptSuffix string `json:"prompt_suffix"`
		StylePreset  string `json:"style_preset"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
	}
	asset, err := h.svc.Generate(id, req.ModelName, req.PromptSuffix, req.StylePreset)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "asset not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, asset)
}

// GenerateAllAssets —— 批量触发项目下所有待处理资产的图像生成
// GenerateAllAssets POST /api/v1/projects/:pid/assets/generate-all
func (h *AssetHandler) GenerateAllAssets(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	var episodeID *uint64
	if raw := strings.TrimSpace(c.Query("episode_id")); raw != "" {
		if parsed, parseErr := strconv.ParseUint(raw, 10, 64); parseErr == nil {
			episodeID = &parsed
		} else {
			response.Error(c, http.StatusBadRequest, "invalid episode id")
			return
		}
	}
	var body struct {
		ModelName      string            `json:"model_name"`
		ModelNames     []string          `json:"model_names"`
		PromptSuffix   string            `json:"prompt_suffix"`
		PromptSuffixes map[string]string `json:"prompt_suffixes"`
		Force          bool              `json:"force"`
	}
	_ = c.ShouldBindJSON(&body)
	count, err := h.svc.GenerateAll(pid, episodeID, body.ModelName, body.ModelNames, body.PromptSuffix, body.PromptSuffixes, body.Force)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	payload := gin.H{"triggered": count}
	if episodeID != nil {
		payload["episode_id"] = *episodeID
	}
	response.Success(c, payload)
}

// GenerateBatchAssets —— 批量触发指定 ID 列表的资产图像生成
// GenerateBatchAssets POST /api/v1/projects/:pid/assets/generate-batch
func (h *AssetHandler) GenerateBatchAssets(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	var body struct {
		AssetIDs       []uint64          `json:"asset_ids"`
		ModelName      string            `json:"model_name"`
		ModelNames     []string          `json:"model_names"`
		PromptSuffix   string            `json:"prompt_suffix"`
		PromptSuffixes map[string]string `json:"prompt_suffixes"`
		Force          bool              `json:"force"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(body.AssetIDs) == 0 {
		response.Error(c, http.StatusBadRequest, "asset_ids must not be empty")
		return
	}
	count, err := h.svc.GenerateBatch(pid, body.AssetIDs, body.ModelName, body.ModelNames, body.PromptSuffix, body.PromptSuffixes, body.Force)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, gin.H{"triggered": count, "requested": len(body.AssetIDs)})
}

// PauseAssetGeneration —— 暂停项目下资源图像生成
// PauseAssetGeneration POST /api/v1/projects/:pid/assets/pause-generation
func (h *AssetHandler) PauseAssetGeneration(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	count, err := h.svc.PauseProjectGeneration(pid)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{"paused": count, "status": "paused"})
}

// ResumeAssetGeneration —— 继续项目下已暂停的资源图像生成
// ResumeAssetGeneration POST /api/v1/projects/:pid/assets/resume-generation
func (h *AssetHandler) ResumeAssetGeneration(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	count, err := h.svc.ResumeProjectGeneration(pid)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{"triggered": count, "status": "resumed"})
}

// RetryAsset —— 重试失败资产的图像生成，可指定模型
// ResetAsset —— 清除已生成图片，将资产重置为待生成状态
// ResetAsset POST /api/v1/projects/:pid/assets/:id/reset
func (h *AssetHandler) ResetAsset(c *gin.Context) {
	id, err := parseAssetID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid asset id")
		return
	}
	asset, err := h.svc.ResetAsset(id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "asset not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, asset)
}

// RetryAsset POST /api/v1/projects/:pid/assets/:id/retry
func (h *AssetHandler) RetryAsset(c *gin.Context) {
	id, err := parseAssetID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid asset id")
		return
	}
	var body struct {
		ModelName string `json:"model_name"`
	}
	_ = c.ShouldBindJSON(&body) // optional body
	asset, err := h.svc.RetryOne(id, body.ModelName)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "asset not found")
			return
		}
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, asset)
}

// RegenPanel —— 角色资产重绘单栏（closeup/front/side/back），复用原 seed，重新拼接 composite。
// RegenPanel POST /api/v1/projects/:pid/assets/:id/regen-panel
// body: { "panel": "closeup|front|side|back", "prompt_override": "...optional...", "model_name": "...optional..." }
func (h *AssetHandler) RegenPanel(c *gin.Context) {
	id, err := parseAssetID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid asset id")
		return
	}
	var body struct {
		Panel          string `json:"panel"`
		PromptOverride string `json:"prompt_override"`
		ModelName      string `json:"model_name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(body.Panel) == "" {
		response.Error(c, http.StatusBadRequest, "panel is required")
		return
	}
	// 立即返回 202，后端异步执行（单栏耗时 ~30s~3min，视模型而定）。
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if err := h.svc.RegenPanel(ctx, id, body.Panel, body.PromptOverride, body.ModelName); err != nil {
			h.logger.Error("regen panel failed",
				zap.Uint64("asset_id", id),
				zap.String("panel", body.Panel),
				zap.Error(err))
		}
	}()
	response.Success(c, gin.H{
		"asset_id": id,
		"panel":    body.Panel,
		"status":   "accepted",
		"message":  "panel regeneration queued; poll GET /assets/:id for progress",
	})
}

// RecompositePanels —— 用已有 panel_images 重新拼接 composite，不重新生成任何图片。
// POST /api/v1/projects/:pid/assets/:id/recomposite
func (h *AssetHandler) RecompositePanels(c *gin.Context) {
	id, err := parseAssetID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid asset id")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	compositeURL, err := h.svc.RecompositePanels(ctx, id)
	if err != nil {
		h.logger.Error("recomposite panels failed", zap.Uint64("asset_id", id), zap.Error(err))
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{
		"asset_id":      id,
		"composite_url": compositeURL,
	})
}

// RetryFailedAssets —— 批量重试项目下所有失败资源，可按集和模型筛选
// RetryFailedAssets POST /api/v1/projects/:pid/assets/retry-failed
func (h *AssetHandler) RetryFailedAssets(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	var episodeID *uint64
	if raw := strings.TrimSpace(c.Query("episode_id")); raw != "" {
		if parsed, parseErr := strconv.ParseUint(raw, 10, 64); parseErr == nil {
			episodeID = &parsed
		} else {
			response.Error(c, http.StatusBadRequest, "invalid episode id")
			return
		}
	}
	var body struct {
		ModelName string `json:"model_name"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
	}
	count, err := h.svc.RetryFailed(pid, episodeID, body.ModelName)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	payload := gin.H{"retried": count}
	if episodeID != nil {
		payload["episode_id"] = *episodeID
	}
	if body.ModelName != "" {
		payload["model_name"] = body.ModelName
	}
	response.Success(c, payload)
}

// UploadAsset —— 手动上传资产图片文件
// UploadAsset POST /api/v1/projects/:pid/assets/:id/upload
func (h *AssetHandler) UploadAsset(c *gin.Context) {
	id, err := parseAssetID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid asset id")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	asset, err := h.svc.Upload(id, header.Filename, file)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "asset not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, asset)
}

// ChatAsset —— 向资产追加对话消息并更新描述
// ChatAsset POST /api/v1/projects/:pid/assets/:id/chat
func (h *AssetHandler) ChatAsset(c *gin.Context) {
	id, err := parseAssetID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid asset id")
		return
	}
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	asset, err := h.svc.Chat(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "asset not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, asset)
}

// ChatFree —— 无资产上下文的自由聊天（左侧参考助手面板）
// ChatFree POST /api/v1/chat
func (h *AssetHandler) ChatFree(c *gin.Context) {
	var req struct {
		Messages  []map[string]string `json:"messages" binding:"required"`
		ModelName string              `json:"model_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Messages) == 0 {
		response.Error(c, http.StatusBadRequest, "messages required")
		return
	}
	reply, err := h.svc.ChatFree(c.Request.Context(), req.Messages, req.ModelName)
	if err != nil {
		errMsg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) ||
			strings.Contains(errMsg, "deadline exceeded") ||
			strings.Contains(errMsg, "context canceled") ||
			strings.Contains(errMsg, "timeout") {
			response.Error(c, http.StatusGatewayTimeout, "AI 模型响应超时，请稍后重试")
			return
		}
		response.Error(c, http.StatusInternalServerError, errMsg)
		return
	}
	response.Success(c, gin.H{"reply": reply})
}

// ChatGemini —— 调用 Gemini 多模态接口，支持同时输出文本和图片（TEXT+IMAGE）
// ChatGemini POST /api/v1/chat/gemini
func (h *AssetHandler) ChatGemini(c *gin.Context) {
	if len(h.geminiBases) == 0 || len(h.geminiKeys) == 0 {
		response.Error(c, http.StatusServiceUnavailable, "Gemini 渠道未配置")
		return
	}

	var req struct {
		Messages []struct {
			Role    string `json:"role"`    // "user" | "model"
			Content string `json:"content"`
		} `json:"messages" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Messages) == 0 {
		response.Error(c, http.StatusBadRequest, "messages required")
		return
	}

	// Pick first available base+key
	base := strings.TrimRight(h.geminiBases[0], "/")
	apiKey := h.geminiKeys[0]
	model := h.geminiModel
	if model == "" {
		model = "gemini-2.0-flash-exp"
	}

	// Build Gemini generateContent request
	type geminiPart struct {
		Text string `json:"text"`
	}
	type geminiContent struct {
		Role  string       `json:"role"`
		Parts []geminiPart `json:"parts"`
	}
	type generationConfig struct {
		ResponseModalities []string `json:"responseModalities"`
	}
	type geminiReq struct {
		Contents         []geminiContent  `json:"contents"`
		GenerationConfig generationConfig `json:"generationConfig"`
	}

	var contents []geminiContent
	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		if role != "user" && role != "model" {
			continue // skip system for now
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	body, err := json.Marshal(geminiReq{
		Contents: contents,
		GenerationConfig: generationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
		},
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "marshal request failed")
		return
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", base, model, apiKey)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "deadline exceeded") {
			response.Error(c, http.StatusGatewayTimeout, "Gemini 响应超时，请稍后重试")
			return
		}
		response.Error(c, http.StatusInternalServerError, fmt.Sprintf("gemini call: %s", err.Error()))
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		response.Error(c, http.StatusBadGateway, fmt.Sprintf("gemini error %d: %s", resp.StatusCode, string(respBody)))
		return
	}

	// Parse response — parts can be text or inlineData (base64 image)
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"` // base64
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		response.Error(c, http.StatusInternalServerError, "parse gemini response failed")
		return
	}
	if geminiResp.Error != nil {
		response.Error(c, http.StatusBadGateway, geminiResp.Error.Message)
		return
	}
	if len(geminiResp.Candidates) == 0 {
		response.Error(c, http.StatusBadGateway, "gemini returned no candidates")
		return
	}

	type OutputPart struct {
		Type     string `json:"type"`               // "text" | "image"
		Text     string `json:"text,omitempty"`
		MimeType string `json:"mime_type,omitempty"`
		Data     string `json:"data,omitempty"`     // base64
	}
	var parts []OutputPart
	for _, p := range geminiResp.Candidates[0].Content.Parts {
		if p.InlineData != nil {
			parts = append(parts, OutputPart{Type: "image", MimeType: p.InlineData.MimeType, Data: p.InlineData.Data})
		} else if p.Text != "" {
			parts = append(parts, OutputPart{Type: "text", Text: p.Text})
		}
	}
	response.Success(c, gin.H{"parts": parts})
}

// UpdateConsistencyConfig —— 更新项目的一致性配置参数
// UpdateConsistencyConfig PATCH /api/v1/projects/:pid/consistency-config
func (h *AssetHandler) UpdateConsistencyConfig(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	var req struct {
		Strength float64 `json:"strength"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, gin.H{
		"project_id":           pid,
		"consistency_strength": req.Strength,
	})
}

// parseProjectID —— 从路由参数中解析项目 ID，返回 uint64
func parseProjectID(c *gin.Context) (uint64, error) {
	return strconv.ParseUint(c.Param("pid"), 10, 64)
}

// parseAssetID —— 从路由参数中解析资产 ID，返回 uint64
func parseAssetID(c *gin.Context) (uint64, error) {
	return strconv.ParseUint(c.Param("id"), 10, 64)
}

// ExtractEpisodeAssets —— 异步提取单集剧本中的资源，立即返回 202
// ExtractEpisodeAssets POST /api/v1/projects/:pid/assets/extract-episode/:eid
// Extracts assets from a single episode's content using LLM.
// Returns 202 immediately and processes extraction asynchronously.
func (h *AssetHandler) ExtractEpisodeAssets(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	eid, err := strconv.ParseUint(c.Param("eid"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid episode id")
		return
	}

	jwtToken := ""
	authHeader := c.GetHeader("Authorization")
	if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 {
		jwtToken = parts[1]
	}

	go func() {
		h.extractSem <- struct{}{}
		defer func() { <-h.extractSem }()
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("panic in episode extraction",
					zap.Uint64("project_id", pid),
					zap.Uint64("episode_id", eid),
					zap.Any("panic", r),
				)
			}
		}()
		if _, err := h.extractSvc.ExtractFromEpisode(context.Background(), pid, eid, jwtToken); err != nil {
			h.logger.Error("async episode extraction failed",
				zap.Uint64("project_id", pid),
				zap.Uint64("episode_id", eid),
				zap.Error(err),
			)
		}
	}()

	response.SuccessWithStatus(c, http.StatusAccepted, gin.H{
		"message":    "episode asset extraction started",
		"status":     "processing",
		"episode_id": eid,
	})
}

// ExtractAssets —— 异步从项目剧本中提取所有资源
// Extracts assets from the project's script text using LLM.
// Existing unlocked assets are deleted before each project-wide extraction.
// Returns 202 immediately and processes extraction asynchronously.
func (h *AssetHandler) ExtractAssets(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	// Forward the caller's JWT token for project-service auth
	jwtToken := ""
	authHeader := c.GetHeader("Authorization")
	if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 {
		jwtToken = parts[1]
	}

	// Run extraction asynchronously
	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("panic in asset extraction",
					zap.Uint64("project_id", pid),
					zap.Any("panic", r),
				)
			}
		}()
		if err := h.svc.DeleteByProjectID(pid); err != nil {
			h.logger.Error("delete existing assets before extraction failed",
				zap.Uint64("project_id", pid),
				zap.Error(err),
			)
			return
		}

		if _, err := h.extractSvc.ExtractFromProject(context.Background(), pid, jwtToken); err != nil {
			h.logger.Error("async asset extraction failed",
				zap.Uint64("project_id", pid),
				zap.Error(err),
			)
		}
	}()

	response.SuccessWithStatus(c, http.StatusAccepted, gin.H{
		"message": "asset extraction started",
		"status":  "processing",
	})
}

// BackfillEpisodeIDs —— 回填资产关联的剧集 ID，返回更新数量
// BackfillEpisodeIDs POST /api/v1/projects/:pid/assets/backfill-episodes
func (h *AssetHandler) BackfillEpisodeIDs(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	jwtToken := ""
	authHeader := c.GetHeader("Authorization")
	if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 {
		jwtToken = parts[1]
	}
	updated, err := h.extractSvc.BackfillEpisodeIDs(c.Request.Context(), pid, jwtToken)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{"updated": updated})
}

// AutoMatchVoices —— 为项目下尚未绑定音色的人物资源自动匹配音色
// AutoMatchVoices POST /api/v1/projects/:pid/assets/auto-match-voices
func (h *AssetHandler) AutoMatchVoices(c *gin.Context) {
	pid, err := parseProjectID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	count, err := h.svc.AutoMatchVoices(pid)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{"updated": count})
}
