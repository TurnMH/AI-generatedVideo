package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/autovideo/character-service/internal/model"
	"github.com/autovideo/character-service/internal/repository"
	"github.com/autovideo/character-service/internal/seed"
	"github.com/autovideo/character-service/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ProductionSkillHandler manages film-department production skills (CRUD).
type ProductionSkillHandler struct {
	repo repository.ProductionSkillRepository
	db   *gorm.DB // for seeding
}

func NewProductionSkillHandler(repo repository.ProductionSkillRepository, db *gorm.DB) *ProductionSkillHandler {
	return &ProductionSkillHandler{repo: repo, db: db}
}

// List GET /api/v1/projects/:pid/production-skills
func (h *ProductionSkillHandler) List(c *gin.Context) {
	pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
	if err != nil || pid <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	list, err := h.repo.ListByProject(pid)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{"items": list, "total": len(list)})
}

// Create POST /api/v1/projects/:pid/production-skills
func (h *ProductionSkillHandler) Create(c *gin.Context) {
	pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
	if err != nil || pid <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	var skill model.ProductionSkill
	if err := c.ShouldBindJSON(&skill); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if skill.Department == "" || skill.Name == "" || skill.LabelTag == "" {
		response.Error(c, http.StatusBadRequest, "department, name and label_tag are required")
		return
	}
	skill.ProjectID = pid
	if err := h.repo.Create(&skill); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessWithStatus(c, http.StatusCreated, skill)
}

// Get GET /api/v1/projects/:pid/production-skills/:id
func (h *ProductionSkillHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	skill, err := h.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "production skill not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, skill)
}

// Update PUT /api/v1/projects/:pid/production-skills/:id
func (h *ProductionSkillHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	skill, err := h.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "production skill not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	var req struct {
		Name         *string `json:"name"`
		LabelTag     *string `json:"label_tag"`
		SystemPrompt *string `json:"system_prompt"`
		IsActive     *bool   `json:"is_active"`
		SortOrder    *int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name != nil {
		skill.Name = *req.Name
	}
	if req.LabelTag != nil {
		skill.LabelTag = *req.LabelTag
	}
	if req.SystemPrompt != nil {
		skill.SystemPrompt = *req.SystemPrompt
	}
	if req.IsActive != nil {
		skill.IsActive = *req.IsActive
	}
	if req.SortOrder != nil {
		skill.SortOrder = *req.SortOrder
	}
	if err := h.repo.Update(skill); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, skill)
}

// Delete DELETE /api/v1/projects/:pid/production-skills/:id
func (h *ProductionSkillHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.repo.Delete(id); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{"deleted": id})
}

// SeedDefaults POST /api/v1/projects/:pid/production-skills/seed-defaults
// 可选 query param: style_preset=live-action-film|live-action-short|anime-2d|anime-3d
func (h *ProductionSkillHandler) SeedDefaults(c *gin.Context) {
	pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
	if err != nil || pid <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	stylePreset := c.Query("style_preset")
	if err := seed.SeedDefaultProductionSkillsForProject(h.db, pid, stylePreset); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	list, _ := h.repo.ListByProject(pid)
	response.Success(c, gin.H{"items": list, "seeded": true, "style_preset": stylePreset})
}

// ReseedDefaults POST /api/v1/projects/:pid/production-skills/reseed-defaults
// 重置 system_prompt 为最新内置默认值，保留 is_active。
// 可选 query param: style_preset=live-action-film|live-action-short|anime-2d|anime-3d
func (h *ProductionSkillHandler) ReseedDefaults(c *gin.Context) {
	pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
	if err != nil || pid <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid project id")
		return
	}
	stylePreset := c.Query("style_preset")
	if err := seed.UpsertDefaultProductionSkillsForProject(h.db, pid, stylePreset); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	list, _ := h.repo.ListByProject(pid)
	response.Success(c, gin.H{"items": list, "reseeded": true, "style_preset": stylePreset})
}
