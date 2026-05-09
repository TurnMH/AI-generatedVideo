package handler

import (
	"net/http"

	"github.com/autovideo/character-service/internal/service"
	"github.com/autovideo/character-service/pkg/response"
	"github.com/gin-gonic/gin"
)

type StyleHandler struct {
	svc *service.CharacterService
}

// NewStyleHandler —— 创建风格处理器实例，返回 *StyleHandler
func NewStyleHandler(svc *service.CharacterService) *StyleHandler {
	return &StyleHandler{svc: svc}
}

// ListPresets —— 获取所有系统预置风格列表
// ListPresets GET /api/v1/style-presets
func (h *StyleHandler) ListPresets(c *gin.Context) {
	// reuse StyleRepo via service
	presets, err := h.svc.ListStylePresets()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, presets)
}
