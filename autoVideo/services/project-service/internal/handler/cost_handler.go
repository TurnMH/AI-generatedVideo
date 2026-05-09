package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/autovideo/project-service/internal/service"
	"github.com/autovideo/project-service/pkg/middleware"
	"github.com/autovideo/project-service/pkg/response"
)

// CostHandler exposes the cost estimation endpoint.
type CostHandler struct {
	costSvc    *service.CostService
	projectSvc *service.ProjectService
}

// NewCostHandler —— 创建成本预估处理器实例
// NewCostHandler creates a new CostHandler.
func NewCostHandler(costSvc *service.CostService, projectSvc *service.ProjectService) *CostHandler {
	return &CostHandler{costSvc: costSvc, projectSvc: projectSvc}
}

// GetCostEstimate —— 处理成本预估请求，返回项目的预估费用
// GetCostEstimate godoc
// GET /api/v1/projects/:id/cost-estimate
func (h *CostHandler) GetCostEstimate(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	project, err := h.projectSvc.Get(id, userID)
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "project not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	estimate, err := h.costSvc.EstimateCost(project)
	if err != nil {
		response.InternalError(c, "failed to estimate cost: "+err.Error())
		return
	}

	response.OK(c, estimate)
}
