package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/autovideo/character-service/internal/model"
	"github.com/autovideo/character-service/internal/service"
	"github.com/autovideo/character-service/pkg/response"
	"github.com/gin-gonic/gin"
)

type CharacterHandler struct {
	svc *service.CharacterService
}

// NewCharacterHandler —— 创建角色处理器实例，返回 *CharacterHandler
func NewCharacterHandler(svc *service.CharacterService) *CharacterHandler {
	return &CharacterHandler{svc: svc}
}

// List —— 按项目 ID 查询角色列表
// List GET /api/v1/characters?project_id=xxx
func (h *CharacterHandler) List(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Query("project_id"), 10, 64)
	if err != nil || projectID <= 0 {
		response.Error(c, http.StatusBadRequest, "project_id is required")
		return
	}
	list, err := h.svc.ListByProject(projectID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, list)
}

// Create —— 创建新角色
// Create POST /api/v1/characters
func (h *CharacterHandler) Create(c *gin.Context) {
	var req model.Character
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.ProjectID == 0 || req.Name == "" {
		response.Error(c, http.StatusBadRequest, "project_id and name are required")
		return
	}
	if err := h.svc.Create(&req); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessWithStatus(c, http.StatusCreated, req)
}

// Get —— 根据 ID 获取单个角色详情
// Get GET /api/v1/characters/:id
func (h *CharacterHandler) Get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	ch, err := h.svc.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "character not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, ch)
}

// Update —— 更新指定角色信息
// Update PUT /api/v1/characters/:id
func (h *CharacterHandler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	var req model.Character
	if err = c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ch, err := h.svc.Update(id, &req)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "character not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, ch)
}

// Delete —— 删除指定角色
// Delete DELETE /api/v1/characters/:id
func (h *CharacterHandler) Delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err = h.svc.Delete(id); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "character not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// UploadReference —— 上传角色参考图片
// UploadReference POST /api/v1/characters/:id/reference
func (h *CharacterHandler) UploadReference(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	cdnURL, err := h.svc.UploadReference(id, header.Filename, file)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "character not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{"reference_image_url": cdnURL})
}

// SetStyle —— 设置角色的风格预设和风格参考图
// SetStyle POST /api/v1/characters/:id/style
func (h *CharacterHandler) SetStyle(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		StylePreset       string `json:"style_preset" binding:"required"`
		StyleReferenceURL string `json:"style_reference_url"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ch, err := h.svc.SetStyle(id, req.StylePreset, req.StyleReferenceURL)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			response.Error(c, http.StatusNotFound, "character not found")
		case errors.Is(err, service.ErrInvalidStylePreset):
			response.Error(c, http.StatusBadRequest, "invalid style preset")
		default:
			response.Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	response.Success(c, ch)
}

// parseID —— 从路由参数中解析角色 ID，返回 int64
func parseID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}
