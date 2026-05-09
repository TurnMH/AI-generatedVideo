package handler

import (
	"errors"

	"github.com/autovideo/script-service/internal/service"
	"github.com/autovideo/script-service/pkg/response"
	"github.com/gin-gonic/gin"
)

// SplitHandler handles HTTP requests for split configuration management.
type SplitHandler struct {
	scriptSvc service.ScriptService
}

// NewSplitHandler —— 创建拆分配置处理器实例，返回 *SplitHandler
func NewSplitHandler(scriptSvc service.ScriptService) *SplitHandler {
	return &SplitHandler{scriptSvc: scriptSvc}
}

// GetSplitConfig —— 获取指定剧本的拆分配置
// GetSplitConfig GET /api/v1/scripts/:id/split-config
func (h *SplitHandler) GetSplitConfig(c *gin.Context) {
	scriptID, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid script id")
		return
	}

	config, err := h.scriptSvc.GetSplitConfig(c.Request.Context(), scriptID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "split config not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, config)
}

// UpdateSplitConfig —— 更新指定剧本的拆分配置参数
// UpdateSplitConfig PUT /api/v1/scripts/:id/split-config
func (h *SplitHandler) UpdateSplitConfig(c *gin.Context) {
	scriptID, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid script id")
		return
	}

	var req service.UpdateSplitConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 4000, "invalid request body: "+err.Error())
		return
	}

	config, err := h.scriptSvc.UpdateSplitConfig(c.Request.Context(), scriptID, &req)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "script not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, config)
}

// ReSplit —— 使用当前拆分配置重新触发剧本分析
// ReSplit POST /api/v1/scripts/:id/re-split
func (h *SplitHandler) ReSplit(c *gin.Context) {
	scriptID, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid script id")
		return
	}

	if err := h.scriptSvc.ReSplit(c.Request.Context(), scriptID); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "script not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "re-split triggered", "script_id": scriptID})
}
