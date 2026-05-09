package handler

import (
	"errors"

	"github.com/autovideo/script-service/internal/service"
	"github.com/autovideo/script-service/pkg/response"
	"github.com/gin-gonic/gin"
)

type SceneHandler struct {
	scriptSvc service.ScriptService
}

// NewSceneHandler —— 创建场景处理器实例，返回 *SceneHandler
func NewSceneHandler(scriptSvc service.ScriptService) *SceneHandler {
	return &SceneHandler{scriptSvc: scriptSvc}
}

// GetScenes —— 获取指定剧本的所有场景列表
// GetScenes GET /api/v1/scripts/:id/scenes
func (h *SceneHandler) GetScenes(c *gin.Context) {
	scriptID, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid script id")
		return
	}

	scenes, err := h.scriptSvc.GetScenes(c.Request.Context(), scriptID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "script not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, gin.H{"scenes": scenes, "total": len(scenes)})
}

// UpdateScene —— 更新指定剧本下某个场景的描述、提示词或状态
// UpdateScene PUT /api/v1/scripts/:id/scenes/:sid
func (h *SceneHandler) UpdateScene(c *gin.Context) {
	scriptID, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid script id")
		return
	}

	sceneID, err := parseID(c, "sid")
	if err != nil {
		response.Error(c, 4000, "invalid scene id")
		return
	}

	var req service.UpdateSceneReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 4000, "invalid request body: "+err.Error())
		return
	}

	scene, err := h.scriptSvc.UpdateScene(c.Request.Context(), scriptID, sceneID, &req)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "scene not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, scene)
}

// GenerateImage —— 触发指定场景的 AI 图片生成
// GenerateImage POST /api/v1/scenes/:id/generate-image
func (h *SceneHandler) GenerateImage(c *gin.Context) {
	sceneID, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid scene id")
		return
	}

	scene, err := h.scriptSvc.GenerateSceneImage(c.Request.Context(), sceneID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "scene not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, scene)
}

type batchGenerateReq struct {
	SceneIDs []int64 `json:"scene_ids" binding:"required"`
}

// BatchGenerate —— 批量触发多个场景的 AI 图片生成，返回成功触发的数量
// BatchGenerate POST /api/v1/scenes/batch-generate
func (h *SceneHandler) BatchGenerate(c *gin.Context) {
	var req batchGenerateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 4000, "invalid request: "+err.Error())
		return
	}

	count, err := h.scriptSvc.BatchGenerateSceneImages(c.Request.Context(), req.SceneIDs)
	if err != nil {
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, gin.H{"triggered": count, "scene_ids": req.SceneIDs})
}
