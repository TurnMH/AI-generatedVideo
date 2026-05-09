package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/autovideo/character-service/internal/model"
	"github.com/autovideo/character-service/internal/repository"
	"github.com/autovideo/character-service/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SkillHandler struct {
	repo repository.SkillRepository
}

func NewSkillHandler(repo repository.SkillRepository) *SkillHandler {
	return &SkillHandler{repo: repo}
}

// List GET /api/v1/skills?project_id=xxx[&skill_type=xxx][&use_case=xxx]
// project_id=0 (or omitted) returns global/default skills.
func (h *SkillHandler) List(c *gin.Context) {
	var projectID int64
	if s := c.Query("project_id"); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id < 0 {
			response.Error(c, http.StatusBadRequest, "invalid project_id")
			return
		}
		projectID = id
	}
	list, err := h.repo.ListByProject(projectID, c.Query("skill_type"), c.Query("use_case"))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, list)
}

// Create POST /api/v1/skills
func (h *SkillHandler) Create(c *gin.Context) {
	var skill model.Skill
	if err := c.ShouldBindJSON(&skill); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if skill.Name == "" {
		response.Error(c, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.repo.Create(&skill); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessWithStatus(c, http.StatusCreated, skill)
}

// Get GET /api/v1/skills/:id
func (h *SkillHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	skill, err := h.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "skill not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, skill)
}

// Update PUT /api/v1/skills/:id
func (h *SkillHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := h.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "skill not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	var req model.Skill
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.SkillType != "" {
		existing.SkillType = req.SkillType
	}
	if req.UseCase != "" {
		existing.UseCase = req.UseCase
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	existing.IsActive = req.IsActive
	if req.CharacterID != nil {
		existing.CharacterID = req.CharacterID
	}
	if err := h.repo.Update(existing); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, existing)
}

// Delete DELETE /api/v1/skills/:id
func (h *SkillHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.repo.Delete(id); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// ListByCharacter GET /api/v1/characters/:id/skills
func (h *SkillHandler) ListByCharacter(c *gin.Context) {
	charID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid character id")
		return
	}
	list, err := h.repo.ListByCharacter(charID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, list)
}
