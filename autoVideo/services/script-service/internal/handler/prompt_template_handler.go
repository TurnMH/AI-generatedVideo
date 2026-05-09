package handler

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/autovideo/script-service/internal/model"
	"github.com/autovideo/script-service/internal/repository"
	"github.com/autovideo/script-service/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PromptTemplateHandler struct {
	repo repository.PromptTemplateRepository
}

func NewPromptTemplateHandler(repo repository.PromptTemplateRepository) *PromptTemplateHandler {
	return &PromptTemplateHandler{repo: repo}
}

// List GET /api/v1/prompt-templates[?style_key=xxx][&resource_type=xxx][&model_binding=xxx][&active_only=true]
func (h *PromptTemplateHandler) List(c *gin.Context) {
	activeOnly := c.Query("active_only") == "true"
	list, err := h.repo.List(
		c.Query("style_key"),
		c.Query("resource_type"),
		c.Query("model_binding"),
		activeOnly,
	)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, list)
}

// Get GET /api/v1/prompt-templates/:id
func (h *PromptTemplateHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	t, err := h.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "template not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, t)
}

// Create POST /api/v1/prompt-templates
func (h *PromptTemplateHandler) Create(c *gin.Context) {
	var t model.PromptTemplate
	if err := c.ShouldBindJSON(&t); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if t.Name == "" || t.StyleKey == "" || t.Content == "" {
		response.Error(c, http.StatusBadRequest, "name, style_key and content are required")
		return
	}
	if err := h.repo.Create(&t); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": t})
}

// Update PUT /api/v1/prompt-templates/:id
func (h *PromptTemplateHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := h.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "template not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	var req model.PromptTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.StyleKey != "" {
		existing.StyleKey = req.StyleKey
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.Content != "" {
		existing.Content = req.Content
	}
	if req.ResourceType != "" {
		existing.ResourceType = req.ResourceType
	}
	existing.ModelBinding = req.ModelBinding
	if req.Version != "" {
		existing.Version = req.Version
	}
	if req.SortOrder != 0 {
		existing.SortOrder = req.SortOrder
	}
	existing.IsActive = req.IsActive
	if err := h.repo.Update(existing); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, existing)
}

// Delete DELETE /api/v1/prompt-templates/:id
func (h *PromptTemplateHandler) Delete(c *gin.Context) {
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

// Preview POST /api/v1/prompt-templates/:id/preview
// Body: {"variables": {"角色名": "孙悟空", "场景名": "花果山"}}
// Returns the content with placeholders replaced.
func (h *PromptTemplateHandler) Preview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	t, err := h.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "template not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	var body struct {
		Variables map[string]string `json:"variables"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	preview := renderTemplate(t.Content, body.Variables)
	response.Success(c, gin.H{"preview": preview, "variables": extractVariables(t.Content)})
}

// renderTemplate replaces {variable} placeholders in content with provided values.
func renderTemplate(content string, vars map[string]string) string {
	if len(vars) == 0 {
		return content
	}
	result := content
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}

// extractVariables finds all {variable} placeholders in the template content.
var varPattern = regexp.MustCompile(`\{([^{}]+)\}`)

func extractVariables(content string) []string {
	matches := varPattern.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	var vars []string
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			vars = append(vars, m[1])
		}
	}
	return vars
}
