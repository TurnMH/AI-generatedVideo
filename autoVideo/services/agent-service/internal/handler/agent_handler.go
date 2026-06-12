package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/autovideo/agent-service/internal/model"
	"github.com/autovideo/agent-service/internal/service"
	"github.com/gin-gonic/gin"
)

type AgentHandler struct {
	svc *service.AgentService
}

func NewAgentHandler(svc *service.AgentService) *AgentHandler {
	return &AgentHandler{svc: svc}
}

func (h *AgentHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AgentHandler) ListTools(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"tools": h.svc.ListTools()})
}

func (h *AgentHandler) BuildPlan(c *gin.Context) {
	var req model.PlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	plan, err := h.svc.BuildPlan(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, plan)
}

func (h *AgentHandler) ExecutePlan(c *gin.Context) {
	var req model.ExecutePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.svc.ExecutePlan(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AgentHandler) GetExecution(c *gin.Context) {
	executionID := strings.TrimSpace(c.Param("id"))
	if executionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "execution id is required"})
		return
	}
	record, ok := h.svc.GetExecution(executionID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "execution not found"})
		return
	}
	c.JSON(http.StatusOK, record)
}

func (h *AgentHandler) ListExecutions(c *gin.Context) {
	planID := strings.TrimSpace(c.Param("id"))
	if planID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan id is required"})
		return
	}
	limit := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a non-negative integer"})
			return
		}
		limit = parsed
	}
	filter := service.ExecutionListFilter{
		Status: strings.TrimSpace(c.Query("status")),
		Limit:  limit,
	}
	c.JSON(http.StatusOK, gin.H{
		"plan_id":    planID,
		"filters":    filter,
		"executions": h.svc.ListExecutions(planID, filter),
	})
}

func (h *AgentHandler) RetryExecution(c *gin.Context) {
	executionID := strings.TrimSpace(c.Param("id"))
	if executionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "execution id is required"})
		return
	}
	resp, err := h.svc.RetryExecution(c.Request.Context(), executionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AgentHandler) ResumeExecution(c *gin.Context) {
	executionID := strings.TrimSpace(c.Param("id"))
	if executionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "execution id is required"})
		return
	}
	resp, err := h.svc.ResumeExecution(c.Request.Context(), executionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AgentHandler) ReplayFromStep(c *gin.Context) {
	executionID := strings.TrimSpace(c.Param("id"))
	stepID := strings.TrimSpace(c.Param("stepId"))
	if executionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "execution id is required"})
		return
	}
	if stepID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "step id is required"})
		return
	}
	resp, err := h.svc.ReplayFromStep(c.Request.Context(), executionID, stepID)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "not found"):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case strings.Contains(err.Error(), "cannot replay from step") || strings.Contains(err.Error(), "step id is required"):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, resp)
}
