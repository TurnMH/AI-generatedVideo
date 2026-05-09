package handler

import (
	"net/http"
	"strconv"

	"github.com/autovideo/auth-service/internal/service"
	"github.com/autovideo/auth-service/pkg/middleware"
	"github.com/autovideo/auth-service/pkg/response"
	"github.com/gin-gonic/gin"
)

type APIKeyHandler struct {
	apiKeySvc service.APIKeyService
}

type runtimeAPIKeyResponse struct {
	Provider   string `json:"provider"`
	KeyAlias   string `json:"key_alias"`
	PlainKey   string `json:"plain_key"`
	BaseURL    string `json:"base_url"`
	ModelScope string `json:"model_scope"`
	Status     string `json:"status"`
}

// NewAPIKeyHandler —— 创建 APIKeyHandler 实例，注入 API Key 服务依赖
func NewAPIKeyHandler(apiKeySvc service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{apiKeySvc: apiKeySvc}
}

type addAPIKeyRequest struct {
	Provider   string `json:"provider" binding:"required"`
	Alias      string `json:"alias"`
	Key        string `json:"key" binding:"required"`
	BaseURL    string `json:"base_url"`
	ModelScope string `json:"model_scope"`
}

// ListAPIKeysHandler —— 获取当前用户的所有 API Key 列表并返回
// ListAPIKeysHandler GET /api/v1/auth/api-keys
func (h *APIKeyHandler) ListAPIKeysHandler(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		response.Unauthorized(c, "invalid token")
		return
	}

	keys, err := h.apiKeySvc.ListAPIKeys(userID)
	if err != nil {
		response.InternalError(c, "list api keys failed")
		return
	}

	response.Success(c, keys)
}

// AddAPIKeyHandler —— 为当前用户添加一个新的 API Key
// AddAPIKeyHandler POST /api/v1/auth/api-keys
func (h *APIKeyHandler) AddAPIKeyHandler(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		response.Unauthorized(c, "invalid token")
		return
	}

	var req addAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.apiKeySvc.AddAPIKey(userID, req.Provider, req.Alias, req.Key, req.BaseURL, req.ModelScope); err != nil {
		response.Error(c, http.StatusInternalServerError, 5000, "add api key failed: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "api key added successfully"})
}

// DeleteAPIKeyHandler —— 根据 ID 删除当前用户拥有的 API Key
// DeleteAPIKeyHandler DELETE /api/v1/auth/api-keys/:id
func (h *APIKeyHandler) DeleteAPIKeyHandler(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		response.Unauthorized(c, "invalid token")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid api key id")
		return
	}

	if err := h.apiKeySvc.DeleteAPIKey(id, userID); err != nil {
		response.Error(c, http.StatusNotFound, 4004, "api key not found or not owned by you")
		return
	}

	response.Success(c, gin.H{"message": "api key deleted successfully"})
}

// ListSystemAPIKeysHandler —— 获取所有系统级 API Key 列表并返回
// ListSystemAPIKeysHandler GET /api/v1/auth/system-api-keys
func (h *APIKeyHandler) ListSystemAPIKeysHandler(c *gin.Context) {
	keys, err := h.apiKeySvc.ListSystemAPIKeys()
	if err != nil {
		response.InternalError(c, "list system api keys failed")
		return
	}
	response.Success(c, keys)
}

// ListRuntimeAPIKeysHandler —— 获取 runtime.* 前缀的系统级 API Key 明文列表，供服务间调用。
func (h *APIKeyHandler) ListRuntimeAPIKeysHandler(c *gin.Context) {
	keys, err := h.apiKeySvc.ListRuntimeAPIKeys()
	if err != nil {
		response.InternalError(c, "list runtime api keys failed")
		return
	}
	out := make([]runtimeAPIKeyResponse, 0, len(keys))
	for _, key := range keys {
		out = append(out, runtimeAPIKeyResponse{
			Provider:   key.Provider,
			KeyAlias:   key.KeyAlias,
			PlainKey:   key.PlainKey,
			BaseURL:    key.BaseURL,
			ModelScope: key.ModelScope,
			Status:     key.Status,
		})
	}
	response.Success(c, out)
}
