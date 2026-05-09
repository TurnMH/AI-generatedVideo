package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/autovideo/script-service/internal/model"
	"github.com/autovideo/script-service/internal/repository"
	"github.com/autovideo/script-service/internal/service"
	"github.com/gin-gonic/gin"
)

type ScriptLibraryHandler struct {
	repo      repository.ScriptLibraryRepository
	llmClient service.LLMClient
}

// NewScriptLibraryHandler —— 创建剧本库处理器实例，返回 *ScriptLibraryHandler
func NewScriptLibraryHandler(repo repository.ScriptLibraryRepository, llmClient service.LLMClient) *ScriptLibraryHandler {
	return &ScriptLibraryHandler{repo: repo, llmClient: llmClient}
}

// List —— 查询剧本库列表，支持按来源筛选，返回公开和用户自有的条目
func (h *ScriptLibraryHandler) List(c *gin.Context) {
	source := c.Query("source") // "uploaded", "showcase", or "" for all
	userID, _ := c.Get("user_id")
	uid, _ := userID.(float64)

	items, err := h.repo.List(c.Request.Context(), source, int64(uid))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": items})
}

// Create —— 创建新的剧本库条目
func (h *ScriptLibraryHandler) Create(c *gin.Context) {
	var item model.ScriptLibrary
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if item.Source == "" {
		item.Source = model.SourceUploaded
	}
	userID, _ := c.Get("user_id")
	if uid, ok := userID.(float64); ok {
		item.UserID = int64(uid)
	}

	if err := h.repo.Create(c.Request.Context(), &item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 201, "data": item})
}

// Update —— 更新指定的剧本库条目
func (h *ScriptLibraryHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id"})
		return
	}
	existing, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
		return
	}
	if err := c.ShouldBindJSON(existing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	existing.ID = id
	if err := h.repo.Update(c.Request.Context(), existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": existing})
}

// Delete —— 删除指定的剧本库条目
func (h *ScriptLibraryHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "deleted"})
}

type aiGenerateReq struct {
	Mode            string `json:"mode" binding:"required"`
	ModelName       string `json:"model_name"`
	Title           string `json:"title"`
	Genre           string `json:"genre"`
	Platform        string `json:"platform"`
	DeliveryFormat  string `json:"delivery_format"`
	EpisodeDuration string `json:"episode_duration"`
	ReferenceStyle  string `json:"reference_style"`
	Premise         string `json:"premise"`
	CharacterSetup  string `json:"character_setup"`
	WorldSetting    string `json:"world_setting"`
	Outline         string `json:"outline"`
	ChapterBrief    string `json:"chapter_brief"`
	SourceText      string `json:"source_text"`
	TargetWords     int    `json:"target_words"`
	ChapterCount    int    `json:"chapter_count"`
	Audience        string `json:"audience"`
	Tone            string `json:"tone"`
	Requirements    string `json:"requirements"`
}

// GenerateAI —— 为剧本库生成 AI 创作内容
func (h *ScriptLibraryHandler) GenerateAI(c *gin.Context) {
	var req aiGenerateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if h.llmClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "llm service unavailable"})
		return
	}

	result, err := h.llmClient.GenerateScript(c.Request.Context(), &service.ScriptGenerateReq{
		Mode:            req.Mode,
		ModelName:       req.ModelName,
		Title:           req.Title,
		Genre:           req.Genre,
		Platform:        req.Platform,
		DeliveryFormat:  req.DeliveryFormat,
		EpisodeDuration: req.EpisodeDuration,
		ReferenceStyle:  req.ReferenceStyle,
		Premise:         req.Premise,
		CharacterSetup:  req.CharacterSetup,
		WorldSetting:    req.WorldSetting,
		Outline:         req.Outline,
		ChapterBrief:    req.ChapterBrief,
		SourceText:      req.SourceText,
		TargetWords:     req.TargetWords,
		ChapterCount:    req.ChapterCount,
		Audience:        req.Audience,
		Tone:            req.Tone,
		Requirements:    req.Requirements,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	if result == nil || strings.TrimSpace(result.Content) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": 422, "message": "LLM returned empty content; check model configuration or prompt"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": result})
}
