package handler

import (
	"net/http"
	"strconv"

	"github.com/autovideo/image-service/internal/repository"
	"github.com/autovideo/image-service/internal/service"
	"github.com/autovideo/image-service/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ImageHandler struct {
	svc    service.ImageService
	logger *zap.Logger
}

// NewImageHandler —— 创建 ImageHandler 实例，返回 *ImageHandler
func NewImageHandler(svc service.ImageService, logger *zap.Logger) *ImageHandler {
	return &ImageHandler{svc: svc, logger: logger}
}

type generateReq struct {
	SceneID           *int64  `json:"scene_id"`
	ProjectID         int64   `json:"project_id"          binding:"required"`
	UserID            int64   `json:"user_id"              binding:"required"`
	Prompt            string  `json:"prompt"               binding:"required"`
	NegativePrompt    string  `json:"negative_prompt"`
	TaskType          string  `json:"task_type"`
	StylePreset       string  `json:"style_preset"`
	StyleReferenceURL string  `json:"style_reference_url"`
	// ReferenceImageURLs lets the caller attach extra character/scene references
	// for multi-image aware models (Gemini, Qwen-Image-Edit, Seedream, gpt-image-1).
	ReferenceImageURLs []string `json:"reference_image_urls"`
	// IsCharacterSheet marks any attached reference as a 4-panel character
	// turnaround so image-service can inject explicit "SAME person, 4 views"
	// guidance into every generator that accepts a reference image.
	IsCharacterSheet bool   `json:"is_character_sheet"`
	ModelName         string  `json:"model_name"`
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	Steps             int     `json:"steps"`
	CfgScale          float64 `json:"cfg_scale"`
	Seed              int64   `json:"seed"`
}

// Generate —— 处理图片生成请求，解析参数并调用服务层异步生成，返回任务信息
func (h *ImageHandler) Generate(c *gin.Context) {
	var req generateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	task, err := h.svc.Generate(c.Request.Context(), service.GenerateRequest{
		SceneID:           req.SceneID,
		ProjectID:         req.ProjectID,
		UserID:            req.UserID,
		Prompt:            req.Prompt,
		NegativePrompt:    req.NegativePrompt,
		TaskType:          req.TaskType,
		StylePreset:       req.StylePreset,
		StyleReferenceURL: req.StyleReferenceURL,
		ReferenceImageURLs: req.ReferenceImageURLs,
		IsCharacterSheet:  req.IsCharacterSheet,
		ModelName:         req.ModelName,
		Width:             req.Width,
		Height:            req.Height,
		Steps:             req.Steps,
		CfgScale:          req.CfgScale,
		Seed:              req.Seed,
	})
	if err != nil {
		h.logger.Error("generate failed", zap.Error(err))
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, task)
}

// GetTask —— 根据任务 ID 查询图片生成任务详情并返回
func (h *ImageHandler) GetTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	task, err := h.svc.GetTask(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "task not found")
		return
	}

	response.Success(c, task)
}

// ListTasks —— 分页查询图片生成任务列表，支持按项目、场景、任务类型和状态筛选
func (h *ImageHandler) ListTasks(c *gin.Context) {
	filter := repository.ListFilter{
		Page:     parseInt(c.Query("page"), 1),
		PageSize: parseInt(c.Query("page_size"), 20),
	}
	if v := c.Query("project_id"); v != "" {
		filter.ProjectID, _ = strconv.ParseInt(v, 10, 64)
	}
	if v := c.Query("scene_id"); v != "" {
		filter.SceneID, _ = strconv.ParseInt(v, 10, 64)
	}
	filter.TaskType = c.Query("task_type")
	filter.Status = c.Query("status")

	tasks, total, err := h.svc.ListTasks(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("list tasks failed", zap.Error(err))
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, gin.H{
		"total": total,
		"items": tasks,
		"page":  filter.Page,
		"size":  filter.PageSize,
	})
}

// RetryTask —— 重试指定的失败图片生成任务，返回更新后的任务信息
func (h *ImageHandler) RetryTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	task, err := h.svc.RetryTask(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, task)
}

// CancelTask —— 取消指定的待处理图片生成任务
func (h *ImageHandler) CancelTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid task id")
		return
	}

	if err := h.svc.CancelTask(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, nil)
}

// DeleteProjectData —— 删除项目下所有图片任务（幂等：项目无任务时 GORM 删 0 行不报错，返回 200）
func (h *ImageHandler) DeleteProjectData(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("pid"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}

	if err := h.svc.DeleteProjectData(c.Request.Context(), projectID); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil)
}

// parseInt —— 将字符串解析为整数，解析失败或值小于等于 0 时返回默认值
func parseInt(s string, defaultVal int) int {
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}

// ModelStatus —— 返回所有图片生成模型的可用状态
func (h *ImageHandler) ModelStatus(c *gin.Context) {
	items := h.svc.ModelStatus(c.Request.Context())
	c.JSON(200, gin.H{"models": items})
}

// ModelCapabilities —— 返回所有图片模型的能力声明、task_type 默认比例与尺寸标准化策略
func (h *ImageHandler) ModelCapabilities(c *gin.Context) {
	response.Success(c, gin.H{
		"models":                 h.svc.ModelCapabilities(c.Request.Context()),
		"task_type_capabilities": h.svc.TaskTypeCapabilities(c.Request.Context()),
	})
}

// GeminiChannels —— 并发探测每个 Gemini 代理渠道有效性，返回实时状态供管理界面展示
func (h *ImageHandler) GeminiChannels(c *gin.Context) {
	items := h.svc.GeminiChannels(c.Request.Context())
	response.Success(c, items)
}
