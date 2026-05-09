package handler

import (
	"strconv"

	"github.com/autovideo/storage-service/internal/service"
	"github.com/autovideo/storage-service/pkg/response"
	"github.com/gin-gonic/gin"
)

// ProjectStorageHandler handles HTTP requests for project storage management.
type ProjectStorageHandler struct {
	svc *service.ProjectStorageService
}

// NewProjectStorageHandler —— 创建项目存储 HTTP 处理器实例
// NewProjectStorageHandler creates a ProjectStorageHandler.
func NewProjectStorageHandler(svc *service.ProjectStorageService) *ProjectStorageHandler {
	return &ProjectStorageHandler{svc: svc}
}

// GetDetails —— 查询指定项目的存储详情（按分类汇总），返回 JSON 响应
// GetDetails GET /api/v1/projects/:pid/storage
func (h *ProjectStorageHandler) GetDetails(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("pid"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid project_id")
		return
	}

	category := c.Query("category")

	details, err := h.svc.GetStorageDetails(c.Request.Context(), projectID, category)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, details)
}

// deleteFilesReq is the request body for deleting specific files.
type deleteFilesReq struct {
	FileIDs []uint64 `json:"file_ids" binding:"required"`
}

// GetBulkTotals —— 批量查询多个项目的存储占用（GET /api/v1/storage/projects/totals?ids=1,2,3）
// GetBulkTotals GET /api/v1/storage/projects/totals
func (h *ProjectStorageHandler) GetBulkTotals(c *gin.Context) {
	raw := c.Query("ids")
	if raw == "" {
		response.BadRequest(c, "ids query param required")
		return
	}

	var ids []uint64
	for _, s := range splitCSV(raw) {
		id, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			response.BadRequest(c, "invalid id: "+s)
			return
		}
		ids = append(ids, id)
	}

	totals, err := h.svc.GetBulkTotals(c.Request.Context(), ids)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, totals)
}

// splitCSV splits a comma-separated string and trims whitespace.
func splitCSV(s string) []string {
	var out []string
	for _, part := range splitString(s, ',') {
		if t := trimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func splitString(s string, sep rune) []string {
	var result []string
	start := 0
	for i, r := range s {
		if r == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func (h *ProjectStorageHandler) DeleteFiles(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("pid"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid project_id")
		return
	}

	var req deleteFilesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	deletedBytes, err := h.svc.DeleteFiles(c.Request.Context(), projectID, req.FileIDs)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"deleted_count": len(req.FileIDs),
		"freed_bytes":   deletedBytes,
	})
}

// CleanHistory —— 清理项目中所有历史版本文件，返回释放的字节数
// CleanHistory POST /api/v1/projects/:pid/storage/clean-history
func (h *ProjectStorageHandler) CleanHistory(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("pid"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid project_id")
		return
	}

	deletedBytes, err := h.svc.CleanHistory(c.Request.Context(), projectID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"freed_bytes": deletedBytes,
		"message":     "history files cleaned",
	})
}

// DeleteProjectFiles —— 删除项目下全部文件
// DeleteProjectFiles DELETE /api/v1/projects/:pid/storage
func (h *ProjectStorageHandler) DeleteProjectFiles(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("pid"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid project_id")
		return
	}

	deletedBytes, deletedCount, err := h.svc.DeleteProjectFiles(c.Request.Context(), projectID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"deleted_count": deletedCount,
		"freed_bytes":   deletedBytes,
	})
}
