package handler

import (
	"net/http"
	"strconv"

	"github.com/autovideo/character-service/internal/model"
	"github.com/autovideo/character-service/internal/repository"
	"github.com/autovideo/character-service/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CharacterGroupHandler handles CRUD for character groups (视频串行流程 多造型管理).
type CharacterGroupHandler struct {
	repo   *repository.CharacterGroupRepo
	logger *zap.Logger
}

func NewCharacterGroupHandler(repo *repository.CharacterGroupRepo, logger *zap.Logger) *CharacterGroupHandler {
	return &CharacterGroupHandler{repo: repo, logger: logger}
}

// List GET /api/v1/projects/:pid/character-groups
func (h *CharacterGroupHandler) List(c *gin.Context) {
	pid, err := parseProjectIDInt64(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	groups, err := h.repo.FindByProjectID(pid)
	if err != nil {
		h.logger.Error("list character groups", zap.Error(err))
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, groups)
}

// Create POST /api/v1/projects/:pid/character-groups
func (h *CharacterGroupHandler) Create(c *gin.Context) {
	pid, err := parseProjectIDInt64(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	var req struct {
		Name           string `json:"name" binding:"required"`
		Description    string `json:"description"`
		VoiceModel     string `json:"voice_model"`
		VoiceSampleURL string `json:"voice_sample_url"`
		SortOrder      int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	g := &model.CharacterGroup{
		ProjectID:      pid,
		Name:           req.Name,
		Description:    req.Description,
		VoiceModel:     req.VoiceModel,
		VoiceSampleURL: req.VoiceSampleURL,
		SortOrder:      req.SortOrder,
	}
	if err := h.repo.Create(g); err != nil {
		h.logger.Error("create character group", zap.Error(err))
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": g})
}

// Get GET /api/v1/projects/:pid/character-groups/:gid
func (h *CharacterGroupHandler) Get(c *gin.Context) {
	gid, err := strconv.ParseInt(c.Param("gid"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid group id")
		return
	}
	g, err := h.repo.FindByID(gid)
	if err != nil {
		response.Error(c, http.StatusNotFound, "group not found")
		return
	}
	response.Success(c, g)
}

// Update PUT /api/v1/projects/:pid/character-groups/:gid
func (h *CharacterGroupHandler) Update(c *gin.Context) {
	gid, err := strconv.ParseInt(c.Param("gid"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid group id")
		return
	}
	g, err := h.repo.FindByID(gid)
	if err != nil {
		response.Error(c, http.StatusNotFound, "group not found")
		return
	}
	var req struct {
		Name           string `json:"name"`
		Description    string `json:"description"`
		VoiceModel     string `json:"voice_model"`
		VoiceSampleURL string `json:"voice_sample_url"`
		SortOrder      int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name != "" {
		g.Name = req.Name
	}
	g.Description = req.Description
	g.VoiceModel = req.VoiceModel
	g.VoiceSampleURL = req.VoiceSampleURL
	g.SortOrder = req.SortOrder
	if err := h.repo.Update(g); err != nil {
		h.logger.Error("update character group", zap.Error(err))
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, g)
}

// Delete DELETE /api/v1/projects/:pid/character-groups/:gid
func (h *CharacterGroupHandler) Delete(c *gin.Context) {
	pid, err := parseProjectIDInt64(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	gid, err := strconv.ParseInt(c.Param("gid"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid group id")
		return
	}
	if err := h.repo.Delete(gid, pid); err != nil {
		h.logger.Error("delete character group", zap.Error(err))
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// AssignVariant POST /api/v1/projects/:pid/character-groups/:gid/variants
// Body: { asset_id, variant_name, sort_order }
func (h *CharacterGroupHandler) AssignVariant(c *gin.Context) {
	gid, err := strconv.ParseInt(c.Param("gid"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid group id")
		return
	}
	var req struct {
		AssetID     uint64 `json:"asset_id" binding:"required"`
		VariantName string `json:"variant_name"`
		SortOrder   int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := h.repo.AssignAssetToGroup(req.AssetID, gid, req.VariantName, req.SortOrder)
	if err != nil {
		h.logger.Error("assign variant", zap.Error(err))
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if rows == 0 {
		response.Error(c, http.StatusNotFound, "asset not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"assigned": true})
}

// RemoveVariant DELETE /api/v1/projects/:pid/character-groups/:gid/variants/:aid
func (h *CharacterGroupHandler) RemoveVariant(c *gin.Context) {
	aidStr := c.Param("aid")
	aid, err := strconv.ParseUint(aidStr, 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid asset id")
		return
	}
	if err := h.repo.RemoveAssetFromGroup(aid); err != nil {
		h.logger.Error("remove variant", zap.Error(err))
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"removed": true})
}

// parseProjectIDInt64 parses the :pid path param as int64.
func parseProjectIDInt64(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("pid"), 10, 64)
}
