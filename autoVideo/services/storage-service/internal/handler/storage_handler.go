package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/autovideo/storage-service/internal/service"
	"github.com/autovideo/storage-service/pkg/response"
	"github.com/gin-gonic/gin"
)

// StorageHandler handles HTTP requests for storage operations.
type StorageHandler struct {
	svc *service.StorageService
}

// NewStorageHandler —— 创建存储操作 HTTP 处理器实例
// NewStorageHandler creates a StorageHandler.
func NewStorageHandler(svc *service.StorageService) *StorageHandler {
	return &StorageHandler{svc: svc}
}

// RegisterRoutes —— 将所有存储相关路由注册到指定的路由组
// RegisterRoutes wires all storage routes onto the provided router group.
func (h *StorageHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/upload", h.Upload)
	rg.GET("/url/:key", h.GetURL)
	rg.GET("/url", h.GetURLByQuery)
	rg.DELETE("/:key", h.Delete)
	rg.POST("/presign", h.Presign)
	rg.GET("/files", h.ListFiles)
}

// Upload —— 处理文件上传请求（multipart/form-data），保存文件并返回上传结果
// Upload handles POST /api/v1/storage/upload (multipart/form-data).
func (h *StorageHandler) Upload(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file field required: "+err.Error())
		return
	}
	bucket := c.PostForm("bucket")
	if bucket == "" {
		response.BadRequest(c, "bucket field required")
		return
	}
	userIDStr := c.PostForm("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid user_id")
		return
	}
	projectIDStr := c.PostForm("project_id")
	var projectID *uint64
	if projectIDStr != "" {
		parsedProjectID, parseErr := strconv.ParseUint(projectIDStr, 10, 64)
		if parseErr != nil {
			response.BadRequest(c, "invalid project_id")
			return
		}
		projectID = &parsedProjectID
	}

	f, err := fh.Open()
	if err != nil {
		response.InternalError(c, "cannot open file: "+err.Error())
		return
	}
	defer f.Close()

	req := service.UploadReq{
		UserID:      userID,
		Bucket:      bucket,
		Filename:    fh.Filename,
		ContentType: fh.Header.Get("Content-Type"),
		Size:        fh.Size,
		Reader:      f,
		ProjectID:   projectID,
		Category:    c.DefaultPostForm("category", "other"),
	}
	res, err := h.svc.Upload(c.Request.Context(), req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}

// GetURL —— 根据路径参数中的 key 生成文件访问 URL 并返回
// GetURL handles GET /api/v1/storage/url/:key
func (h *StorageHandler) GetURL(c *gin.Context) {
	key := c.Param("key")
	expiryStr := c.DefaultQuery("expiry", "3600")
	expirySec, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid expiry")
		return
	}
	u, err := h.svc.GetURL(c.Request.Context(), key, time.Duration(expirySec)*time.Second)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"url": u})
}

// GetURLByQuery —— 根据查询参数中的 key 生成文件访问 URL 并返回
// GetURLByQuery handles GET /api/v1/storage/url?key=0/20260325/uuid.txt&expiry=300
func (h *StorageHandler) GetURLByQuery(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		response.BadRequest(c, "key query parameter required")
		return
	}
	expiryStr := c.DefaultQuery("expiry", "3600")
	expirySec, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid expiry")
		return
	}
	u, err := h.svc.GetURL(c.Request.Context(), key, time.Duration(expirySec)*time.Second)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"url": u})
}

// Delete —— 删除指定 key 的文件，需校验用户权限
// Delete handles DELETE /api/v1/storage/:key
func (h *StorageHandler) Delete(c *gin.Context) {
	key := c.Param("key")
	userIDStr := c.Query("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid user_id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), key, userID); err != nil {
		if err.Error() == "permission denied" {
			response.Forbidden(c, "permission denied")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// presignReq is the request body for presigned URL generation.
type presignReq struct {
	Bucket   string `json:"bucket" binding:"required"`
	Filename string `json:"filename" binding:"required"`
	UserID   uint64 `json:"user_id" binding:"required"`
}

// Presign —— 生成预签名上传 URL，返回 presigned_url 和 object_key
// Presign handles POST /api/v1/storage/presign
func (h *StorageHandler) Presign(c *gin.Context) {
	var req presignReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	presignURL, objectKey, err := h.svc.GetPresignedPutURL(c.Request.Context(), req.Bucket, req.Filename, req.UserID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{
		"presigned_url": presignURL,
		"object_key":    objectKey,
	})
}

// ListFiles —— 查询指定用户的所有文件列表并返回
// ListFiles handles GET /api/v1/storage/files?user_id=X
func (h *StorageHandler) ListFiles(c *gin.Context) {
	userIDStr := c.Query("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid user_id")
		return
	}
	files, err := h.svc.ListFiles(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, files)
}
