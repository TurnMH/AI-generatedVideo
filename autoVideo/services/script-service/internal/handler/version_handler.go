package handler

import (
	"errors"

	"github.com/autovideo/script-service/internal/service"
	"github.com/autovideo/script-service/pkg/response"
	"github.com/gin-gonic/gin"
)

// VersionHandler handles HTTP requests for script version management.
type VersionHandler struct {
	versionSvc service.VersionService
}

// NewVersionHandler —— 创建版本管理处理器实例，返回 *VersionHandler
func NewVersionHandler(versionSvc service.VersionService) *VersionHandler {
	return &VersionHandler{versionSvc: versionSvc}
}

// ListVersions —— 获取指定剧本的所有版本列表
// ListVersions GET /api/v1/scripts/:id/versions
func (h *VersionHandler) ListVersions(c *gin.Context) {
	scriptID, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid script id")
		return
	}

	versions, err := h.versionSvc.ListVersions(c.Request.Context(), scriptID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "script not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, gin.H{"versions": versions, "total": len(versions)})
}

// CreateVersion —— 为指定剧本创建新版本
// CreateVersion POST /api/v1/scripts/:id/versions
func (h *VersionHandler) CreateVersion(c *gin.Context) {
	scriptID, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid script id")
		return
	}

	var req service.CreateVersionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 4000, "invalid request body: "+err.Error())
		return
	}

	version, err := h.versionSvc.CreateVersion(c.Request.Context(), scriptID, &req)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "script not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, version)
}

// SwitchVersion —— 切换到指定的剧本版本
// SwitchVersion POST /api/v1/scripts/:id/versions/:vid/switch
func (h *VersionHandler) SwitchVersion(c *gin.Context) {
	scriptID, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid script id")
		return
	}

	versionID, err := parseID(c, "vid")
	if err != nil {
		response.Error(c, 4000, "invalid version id")
		return
	}

	version, err := h.versionSvc.SwitchVersion(c.Request.Context(), scriptID, versionID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "script or version not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "version switched", "version": version})
}
