package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/autovideo/model-service/internal/model"
	"github.com/autovideo/model-service/internal/repository"
	"github.com/autovideo/model-service/internal/service"
	"github.com/autovideo/model-service/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ModelWithTemplates wraps a Model with its associated prompt templates (级联查询响应).
type ModelWithTemplates struct {
	*model.Model
	Templates []service.PromptTemplate `json:"templates"`
}

// ModelHandler groups all HTTP handlers for the model resource.
type ModelHandler struct {
	modelSvc  *service.ModelService
	healthSvc *service.HealthService
	usageRepo *repository.UsageRepo
	logger    *zap.Logger
}

// NewModelHandler —— 创建 ModelHandler 实例，注入所需依赖
// NewModelHandler constructs a ModelHandler.
func NewModelHandler(
	modelSvc *service.ModelService,
	healthSvc *service.HealthService,
	usageRepo *repository.UsageRepo,
	logger *zap.Logger,
) *ModelHandler {
	return &ModelHandler{
		modelSvc:  modelSvc,
		healthSvc: healthSvc,
		usageRepo: usageRepo,
		logger:    logger,
	}
}

// RegisterRoutes —— 将所有模型相关的 HTTP 路由注册到 Gin 引擎
// RegisterRoutes attaches all model routes to the engine.
func (h *ModelHandler) RegisterRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	{
		models := v1.Group("/models")
		{
			models.GET("", h.ListModels)
			models.POST("", h.CreateModel)
			models.GET("/health", h.GetHealthStatus) // must be before /:id
			models.GET("/:id", h.GetModel)
			models.PUT("/:id", h.UpdateModel)
			models.DELETE("/:id", h.DeleteModel)
			models.PATCH("/:id/toggle", h.ToggleModel)
			models.PATCH("/:id/default", h.SetDefaultModel)
			models.POST("/:id/test", h.TestModel)
		}
		v1.GET("/usage", h.GetUsage)
	}
}

// ListModels —— 处理 GET /models，按条件筛选并返回模型列表
// ListModels godoc
// GET /api/v1/models?type=image&types=image,video&provider=openai&speed_rating=fast&enabled=true&sort_by=input_price_asc&include_templates=true
// types 参数（逗号分隔）优先于 type，支持 mutilModelList 级联查询
// include_templates=true 时每个 model 附带关联的 prompt_templates（级联）
func (h *ModelHandler) ListModels(c *gin.Context) {
	f := model.ListFilter{
		Type:        c.Query("type"),
		Provider:    c.Query("provider"),
		SpeedRating: c.Query("speed_rating"),
		SortBy:      c.Query("sort_by"),
	}

	// mutilModelList: parse comma-separated types (e.g. types=image,video,llm)
	if typesParam := c.Query("types"); typesParam != "" {
		for _, t := range strings.Split(typesParam, ",") {
			if t = strings.TrimSpace(t); t != "" {
				f.Types = append(f.Types, t)
			}
		}
	}

	if enabled := c.Query("enabled"); enabled != "" {
		b := enabled == "true"
		f.Enabled = &b
	}

	// fetch model list
	var models []*model.Model
	var err error
	if f.Provider != "" || f.SpeedRating != "" || f.Enabled != nil || f.SortBy != "" || len(f.Types) > 0 {
		models, err = h.modelSvc.ListFiltered(c.Request.Context(), f)
	} else {
		models, err = h.modelSvc.List(c.Request.Context(), f.Type)
	}
	if err != nil {
		h.logger.Error("list models", zap.Error(err))
		response.InternalError(c, "failed to list models")
		return
	}

	// 级联查询: include_templates=true 时附带每个模型关联的 prompt templates
	if c.Query("include_templates") == "true" {
		byBinding, fetchErr := service.FetchTemplates(c.Request.Context())
		if fetchErr != nil {
			h.logger.Warn("fetch templates failed, returning models without templates", zap.Error(fetchErr))
			// 降级：仍然返回模型列表，templates 为空
		}
		result := make([]ModelWithTemplates, 0, len(models))
		universalTemplates := byBinding[""] // model_binding="" 的通用模板
		for _, m := range models {
			tpls := make([]service.PromptTemplate, 0)
			if byBinding != nil {
				// 先加该 model_key 专属模板，再加通用模板
				tpls = append(tpls, byBinding[m.ModelKey]...)
				tpls = append(tpls, universalTemplates...)
			}
			result = append(result, ModelWithTemplates{Model: m, Templates: tpls})
		}
		response.Success(c, result)
		return
	}

	response.Success(c, models)
}

// GetModel —— 处理 GET /models/:id，根据 ID 返回单个模型详情
// GetModel godoc
// GET /api/v1/models/:id
func (h *ModelHandler) GetModel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "invalid model id")
		return
	}
	m, err := h.modelSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "model not found")
		return
	}
	response.Success(c, m)
}

// CreateModel —— 处理 POST /models，创建新模型并返回创建结果
// CreateModel godoc
// POST /api/v1/models  (admin)
func (h *ModelHandler) CreateModel(c *gin.Context) {
	var m model.Model
	if err := c.ShouldBindJSON(&m); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.modelSvc.Create(c.Request.Context(), &m); err != nil {
		h.logger.Error("create model", zap.Error(err))
		response.InternalError(c, "failed to create model")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "created", "data": m})
}

// UpdateModel —— 处理 PUT /models/:id，更新指定模型的全部字段
// UpdateModel godoc
// PUT /api/v1/models/:id
func (h *ModelHandler) UpdateModel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "invalid model id")
		return
	}
	m, err := h.modelSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "model not found")
		return
	}
	if err := c.ShouldBindJSON(m); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	m.ID = id // prevent client from changing the PK
	if err := h.modelSvc.Update(c.Request.Context(), m); err != nil {
		h.logger.Error("update model", zap.Uint64("id", id), zap.Error(err))
		response.InternalError(c, "failed to update model")
		return
	}
	response.Success(c, m)
}

// DeleteModel —— 处理 DELETE /models/:id，删除指定模型
// DeleteModel godoc
// DELETE /api/v1/models/:id
func (h *ModelHandler) DeleteModel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "invalid model id")
		return
	}
	if _, err := h.modelSvc.GetByID(c.Request.Context(), id); err != nil {
		response.NotFound(c, "model not found")
		return
	}
	if err := h.modelSvc.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("delete model", zap.Uint64("id", id), zap.Error(err))
		response.InternalError(c, "failed to delete model")
		return
	}
	response.Success(c, gin.H{"id": id, "deleted": true})
}

// ToggleModel —— 处理 PATCH /models/:id/toggle，切换模型的启用/禁用状态
// ToggleModel godoc
// PATCH /api/v1/models/:id/toggle
func (h *ModelHandler) ToggleModel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "invalid model id")
		return
	}
	m, err := h.modelSvc.ToggleActive(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "model not found")
		return
	}
	response.Success(c, m)
}

// SetDefaultModel —— 处理 PATCH /models/:id/default，将指定模型设为同类型的默认模型
// SetDefaultModel godoc
// PATCH /api/v1/models/:id/default
func (h *ModelHandler) SetDefaultModel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "invalid model id")
		return
	}
	m, err := h.modelSvc.SetDefault(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "model not found")
		return
	}
	response.Success(c, m)
}

// GetHealthStatus —— 处理 GET /models/health，返回所有活跃模型的最新健康状态
// GetHealthStatus godoc
// GET /api/v1/models/health
func (h *ModelHandler) GetHealthStatus(c *gin.Context) {
	healths, err := h.modelSvc.GetHealthStatus(c.Request.Context())
	if err != nil {
		h.logger.Error("get health status", zap.Error(err))
		response.InternalError(c, "failed to get health status")
		return
	}
	response.Success(c, healths)
}

// TestModel —— 处理 POST /models/:id/test，对指定模型执行一次手动健康探测并返回结果
// TestModel godoc
// POST /api/v1/models/:id/test — trigger an on-demand health probe
func (h *ModelHandler) TestModel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.BadRequest(c, "invalid model id")
		return
	}
	m, err := h.modelSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "model not found")
		return
	}
	if err := h.healthSvc.CheckModel(c.Request.Context(), m); err != nil {
		h.logger.Error("manual health check", zap.Uint64("id", id), zap.Error(err))
		response.InternalError(c, "health check failed")
		return
	}
	b := h.healthSvc.GetBreaker(id)
	response.Success(c, gin.H{
		"model_id":       id,
		"breaker_state":  b.StateString(),
		"checked_at":     time.Now().UTC(),
	})
}

// GetUsage —— 处理 GET /usage，按用户和时间范围查询模型使用记录
// GetUsage godoc
// GET /api/v1/usage?user_id=123&start=2024-01-01T00:00:00Z&end=2024-12-31T23:59:59Z
func (h *ModelHandler) GetUsage(c *gin.Context) {
	var userID uint64
	if uid := c.Query("user_id"); uid != "" {
		userID, _ = strconv.ParseUint(uid, 10, 64)
	}

	var start, end time.Time
	if s := c.Query("start"); s != "" {
		start, _ = time.Parse(time.RFC3339, s)
	}
	if e := c.Query("end"); e != "" {
		end, _ = time.Parse(time.RFC3339, e)
	}

	records, err := h.usageRepo.Query(c.Request.Context(), userID, start, end)
	if err != nil {
		h.logger.Error("get usage", zap.Error(err))
		response.InternalError(c, "failed to get usage")
		return
	}
	response.Success(c, records)
}

// parseID —— 从 Gin 路由参数中解析名为 "id" 的 uint64 值
// parseID extracts a uint64 path parameter named "id".
func parseID(c *gin.Context) (uint64, error) {
	return strconv.ParseUint(c.Param("id"), 10, 64)
}
