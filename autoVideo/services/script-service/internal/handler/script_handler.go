package handler

import (
	"errors"
	"strconv"

	"github.com/autovideo/script-service/internal/service"
	"github.com/autovideo/script-service/pkg/response"
	"github.com/gin-gonic/gin"
)

type ScriptHandler struct {
	scriptSvc service.ScriptService
}

// NewScriptHandler —— 创建剧本处理器实例，返回 *ScriptHandler
func NewScriptHandler(scriptSvc service.ScriptService) *ScriptHandler {
	return &ScriptHandler{scriptSvc: scriptSvc}
}

// List —— 按项目 ID 获取剧本列表
// List GET /api/v1/scripts?project_id=123&page=1&page_size=20
func (h *ScriptHandler) List(c *gin.Context) {
	projectIDStr := c.Query("project_id")
	if projectIDStr == "" {
		response.Error(c, 4000, "project_id is required")
		return
	}

	projectID, err := strconv.ParseInt(projectIDStr, 10, 64)
	if err != nil {
		response.Error(c, 4000, "invalid project_id")
		return
	}

	page := 1
	if pageStr := c.DefaultQuery("page", "1"); pageStr != "" {
		if parsed, err := strconv.Atoi(pageStr); err == nil && parsed > 0 {
			page = parsed
		}
	}

	pageSize := 20
	if sizeStr := c.DefaultQuery("page_size", "20"); sizeStr != "" {
		if parsed, err := strconv.Atoi(sizeStr); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}

	scripts, total, err := h.scriptSvc.ListByProjectID(c.Request.Context(), projectID, page, pageSize)
	if err != nil {
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, gin.H{
		"items":     scripts,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Upload —— 处理剧本文件上传请求，读取文件内容并创建剧本记录
// Upload POST /api/v1/scripts/upload
func (h *ScriptHandler) Upload(c *gin.Context) {
	projectIDStr := c.PostForm("project_id")
	title := c.PostForm("title")
	mode := c.PostForm("mode") // "script" or "storyboard", defaults to "script" in service

	if projectIDStr == "" {
		response.Error(c, 4000, "project_id is required")
		return
	}

	projectID, err := strconv.ParseInt(projectIDStr, 10, 64)
	if err != nil {
		response.Error(c, 4000, "invalid project_id")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Error(c, 4000, "file is required")
		return
	}
	defer file.Close()

	script, err := h.scriptSvc.Upload(c.Request.Context(), &service.UploadScriptReq{
		ProjectID:  projectID,
		Title:      title,
		File:       file,
		FileHeader: header,
		Mode:       mode,
	})
	if err != nil {
		response.Error(c, 5000, "upload failed: "+err.Error())
		return
	}

	response.Success(c, gin.H{
		"script_id": script.ID,
		"title":     script.Title,
		"status":    script.ParseStatus,
	})
}

// GetByID —— 根据 ID 查询剧本详情
// GetByID GET /api/v1/scripts/:id
func (h *ScriptHandler) GetByID(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid id")
		return
	}

	script, err := h.scriptSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "script not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, script)
}

// Delete —— 级联删除剧本及其 scenes/characters/assets/split_config
// Delete DELETE /api/v1/scripts/:id
func (h *ScriptHandler) Delete(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid id")
		return
	}

	if err := h.scriptSvc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "script not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, gin.H{"deleted": true})
}

// Analyze —— 触发指定剧本的 LLM 分析，异步解析场景、角色和资产
// Analyze POST /api/v1/scripts/:id/analyze
func (h *ScriptHandler) Analyze(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid id")
		return
	}

	if err := h.scriptSvc.TriggerAnalyze(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "script not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "analysis started", "script_id": id})
}

// GetCharacters —— 获取指定剧本中提取的所有角色列表
// GetCharacters GET /api/v1/scripts/:id/characters
func (h *ScriptHandler) GetCharacters(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid id")
		return
	}

	chars, err := h.scriptSvc.GetCharacters(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "script not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, gin.H{"characters": chars, "total": len(chars)})
}

// GetAssets —— 获取指定剧本中提取的所有资产（人物/地点/道具）列表
// GetAssets GET /api/v1/scripts/:id/assets
func (h *ScriptHandler) GetAssets(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		response.Error(c, 4000, "invalid id")
		return
	}

	assets, err := h.scriptSvc.GetAssets(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, 4004, "script not found")
			return
		}
		response.Error(c, 5000, err.Error())
		return
	}

	response.Success(c, gin.H{"assets": assets, "total": len(assets)})
}

// parseID —— 从 URL 参数中解析 int64 类型的 ID
func parseID(c *gin.Context, param string) (int64, error) {
	return strconv.ParseInt(c.Param(param), 10, 64)
}
