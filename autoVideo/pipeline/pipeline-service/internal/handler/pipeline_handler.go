package handler

import (
	"fmt"
	"time"

	"github.com/autovideo/pipeline-service/internal/model"
	"github.com/autovideo/pipeline-service/internal/service"
	"github.com/autovideo/pipeline-service/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PipelineHandler 处理 HTTP 请求
type PipelineHandler struct {
	svc    *service.PipelineService
	logger *zap.Logger
}

// NewPipelineHandler —— 创建 PipelineHandler 实例，注入服务层和日志
func NewPipelineHandler(svc *service.PipelineService, logger *zap.Logger) *PipelineHandler {
	return &PipelineHandler{svc: svc, logger: logger}
}

// StartPipeline —— 接收启动请求，初始化流水线状态并异步执行
func (h *PipelineHandler) StartPipeline(c *gin.Context) {
	var req model.StartPipelineReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// 设置默认值
	if req.Config.MaxConcurrent <= 0 {
		req.Config.MaxConcurrent = 5
	}
	if req.Config.QualityMode == "" {
		req.Config.QualityMode = "balanced"
	}
	if req.Config.ImageModel == "" {
		req.Config.ImageModel = "auto"
	}
	if req.Config.VideoModel == "" {
		req.Config.VideoModel = "auto"
	}

	pipelineID := fmt.Sprintf("pipe_%s", uuid.New().String()[:8])

	state := &model.PipelineState{
		ID:           pipelineID,
		ProjectID:    req.ProjectID,
		ScriptID:     req.ScriptID,
		Config:       req.Config,
		Stage:        model.StageInit,
		Status:       model.StatusRunning,
		Logs:         []string{},
		SceneIDs:     []int64{},
		ImageTaskIDs: []int64{},
		VideoTaskIDs: []int64{},
		StartedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	h.svc.Start(c.Request.Context(), state)

	h.logger.Info("pipeline started",
		zap.String("pipeline_id", pipelineID),
		zap.Int64("project_id", req.ProjectID),
		zap.Int64("script_id", req.ScriptID),
	)

	response.OK(c, gin.H{"pipeline_id": pipelineID})
}

// GetStatus —— 根据 pipeline_id 查询流水线当前状态并返回
func (h *PipelineHandler) GetStatus(c *gin.Context) {
	id := c.Param("pipeline_id")
	if id == "" {
		response.BadRequest(c, "pipeline_id is required")
		return
	}

	state, err := h.svc.GetState(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.OK(c, state)
}

// PausePipeline —— 暂停指定流水线的执行
func (h *PipelineHandler) PausePipeline(c *gin.Context) {
	id := c.Param("pipeline_id")
	if err := h.svc.Pause(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"pipeline_id": id, "action": "paused"})
}

// ResumePipeline —— 恢复已暂停的流水线继续执行
func (h *PipelineHandler) ResumePipeline(c *gin.Context) {
	id := c.Param("pipeline_id")
	if err := h.svc.Resume(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"pipeline_id": id, "action": "resumed"})
}

// AbortPipeline —— 终止指定流水线的执行
func (h *PipelineHandler) AbortPipeline(c *gin.Context) {
	id := c.Param("pipeline_id")
	if err := h.svc.Abort(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"pipeline_id": id, "action": "aborted"})
}
