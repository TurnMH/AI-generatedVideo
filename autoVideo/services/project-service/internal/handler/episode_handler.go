package handler

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/autovideo/project-service/internal/service"
	"github.com/autovideo/project-service/pkg/response"
)

type EpisodeHandler struct {
	svc *service.EpisodeService

	// Per-project generation lock to prevent concurrent episode generation
	genMu      sync.Mutex
	genRunning map[uint64]bool
}

// NewEpisodeHandler —— 创建剧集处理器实例
func NewEpisodeHandler(svc *service.EpisodeService) *EpisodeHandler {
	return &EpisodeHandler{
		svc:        svc,
		genRunning: make(map[uint64]bool),
	}
}

// ListEpisodes —— 处理获取项目下所有剧集列表的请求
// ListEpisodes godoc
// GET /api/v1/projects/:id/episodes
func (h *EpisodeHandler) ListEpisodes(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	episodes, err := h.svc.ListByProject(projectID)
	if err != nil {
		response.InternalError(c, "failed to list episodes: "+err.Error())
		return
	}

	response.OK(c, episodes)
}

// CreateEpisode —— 处理手动创建单集的请求
// CreateEpisode godoc
// POST /api/v1/projects/:id/episodes
func (h *EpisodeHandler) CreateEpisode(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req service.CreateEpisodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	episode, err := h.svc.Create(projectID, req)
	if err != nil {
		response.InternalError(c, "failed to create episode: "+err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    201,
		"message": "created",
		"data":    episode,
	})
}

// UpdateEpisode —— 处理更新指定剧集信息的请求
// UpdateEpisode godoc
// PUT /api/v1/projects/:id/episodes/:eid
func (h *EpisodeHandler) UpdateEpisode(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	episodeID, err := parseUint64Param(c, "eid")
	if err != nil {
		response.BadRequest(c, "invalid episode id")
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	episode, err := h.svc.Update(episodeID, projectID, req)
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "episode not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, episode)
}

// DeleteEpisode —— 处理删除指定剧集的请求
// DeleteEpisode godoc
// DELETE /api/v1/projects/:id/episodes/:eid
func (h *EpisodeHandler) DeleteEpisode(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	episodeID, err := parseUint64Param(c, "eid")
	if err != nil {
		response.BadRequest(c, "invalid episode id")
		return
	}

	if err := h.svc.Delete(episodeID, projectID); err != nil {
		if isNotFound(err) {
			response.NotFound(c, "episode not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, nil)
}

// PolishEpisode —— AI 润色指定分集的标题、简介和内容
// POST /api/v1/projects/:id/episodes/:eid/polish
func (h *EpisodeHandler) PolishEpisode(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	episodeID, err := parseUint64Param(c, "eid")
	if err != nil {
		response.BadRequest(c, "invalid episode id")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Minute)
	defer cancel()

	episode, err := h.svc.PolishEpisode(ctx, episodeID, projectID)
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "episode not found")
			return
		}
		response.InternalError(c, "polish failed: "+err.Error())
		return
	}
	response.OK(c, episode)
}

// ExtractStoryboards —— 为整个项目拆分生成分镜（异步执行，立即返回）
// POST /api/v1/projects/:id/episodes/extract-storyboards
func (h *EpisodeHandler) ExtractStoryboards(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	go func(pid uint64) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		_, _ = h.svc.ExtractStoryboards(ctx, pid, nil)
	}(projectID)

	response.OK(c, gin.H{
		"status": "started",
	})
}

// ExtractEpisodeStoryboards —— 为单集拆分生成分镜（异步执行，立即返回）
// POST /api/v1/projects/:id/episodes/:eid/extract-storyboards
func (h *EpisodeHandler) ExtractEpisodeStoryboards(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	episodeID, err := parseUint64Param(c, "eid")
	if err != nil {
		response.BadRequest(c, "invalid episode id")
		return
	}

	go func(pid, eid uint64) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		if strings.EqualFold(c.GetHeader("X-Autovideo-Skip-Asset-Refresh"), "true") {
			ctx = service.WithSkipEpisodeAssetRefresh(ctx)
		}
		_, _ = h.svc.ExtractStoryboards(ctx, pid, &eid)
	}(projectID, episodeID)

	response.OK(c, gin.H{
		"status":     "started",
		"episode_id": episodeID,
	})
}

// GenerateEpisodes —— 处理根据剧本自动生成剧集的请求，异步执行
// GenerateEpisodes godoc
// POST /api/v1/projects/:id/episodes/generate
func (h *EpisodeHandler) GenerateEpisodes(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	// Parse optional user-provided keywords from request body
	var body struct {
		Keywords       *service.KeywordLibrary `json:"keywords,omitempty"`
		Force          bool                    `json:"force,omitempty"`
		AutoStoryboard bool                    `json:"auto_storyboard,omitempty"`
	}
	_ = c.ShouldBindJSON(&body) // ignore errors — body is optional

	// ── Layer 1: In-memory per-project mutex ──
	// Reject concurrent generation requests for the same project.
	h.genMu.Lock()
	alreadyRunning := h.genRunning[projectID]
	if alreadyRunning {
		active, err := h.svc.HasActiveGeneration(projectID)
		if err != nil {
			h.genMu.Unlock()
			response.InternalError(c, "failed to inspect generation state: "+err.Error())
			return
		}
		if !active {
			delete(h.genRunning, projectID)
			alreadyRunning = false
		}
	}
	if alreadyRunning && !body.Force {
		h.genMu.Unlock()
		response.Fail(c, http.StatusConflict, 409, "该项目正在生成集数，请勿重复提交")
		return
	}
	h.genRunning[projectID] = true
	h.genMu.Unlock()

	// Run generation asynchronously to avoid HTTP timeout.
	// The frontend polls episode list via SWR to detect completion.
	if body.Force {
		stalled, err := h.svc.IsGenerationStalled(projectID, 2*time.Minute)
		if err != nil {
			if !alreadyRunning {
				h.genMu.Lock()
				delete(h.genRunning, projectID)
				h.genMu.Unlock()
			}
			response.InternalError(c, "failed to inspect stalled progress: "+err.Error())
			return
		}
		if !stalled {
			if !alreadyRunning {
				h.genMu.Lock()
				delete(h.genRunning, projectID)
				h.genMu.Unlock()
			}
			response.Fail(c, http.StatusConflict, http.StatusConflict, "当前任务未达到可强制重试条件")
			return
		}
	}

	go func() {
		defer func() {
			h.genMu.Lock()
			delete(h.genRunning, projectID)
			h.genMu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
		defer cancel()
		_, _ = h.svc.GenerateFromScriptWithOptions(ctx, projectID, body.Keywords, body.Force, body.AutoStoryboard)
	}()

	response.OK(c, gin.H{
		"message": "episode generation started",
		"status":  "processing",
	})
}

// OptimizeEpisode — POST /api/v1/projects/:id/episodes/:eid/optimize
// 将分集内容转化为标准剧本格式
func (h *EpisodeHandler) OptimizeEpisode(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	episodeID, err := parseUint64Param(c, "eid")
	if err != nil {
		response.BadRequest(c, "invalid episode id")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	episode, err := h.svc.OptimizeEpisode(ctx, episodeID, projectID)
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "episode not found")
			return
		}
		response.InternalError(c, "optimize failed: "+err.Error())
		return
	}
	response.OK(c, episode)
}

// ApplyOptimizedText — POST /api/v1/projects/:id/episodes/:eid/apply-optimized
// 用户确认后将优化内容应用为正式 script_excerpt
func (h *EpisodeHandler) ApplyOptimizedText(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	episodeID, err := parseUint64Param(c, "eid")
	if err != nil {
		response.BadRequest(c, "invalid episode id")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	episode, err := h.svc.ApplyOptimizedText(ctx, episodeID, projectID)
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "episode not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, episode)
}

// ReviewEpisode — POST /api/v1/projects/:id/episodes/:eid/review
// AI 审查分集剧本（一致性、衔接性、台词质量）
func (h *EpisodeHandler) ReviewEpisode(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	episodeID, err := parseUint64Param(c, "eid")
	if err != nil {
		response.BadRequest(c, "invalid episode id")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Minute)
	defer cancel()
	episode, err := h.svc.ReviewEpisode(ctx, episodeID, projectID)
	if err != nil {
		if isNotFound(err) {
			response.NotFound(c, "episode not found")
			return
		}
		response.InternalError(c, "review failed: "+err.Error())
		return
	}
	response.OK(c, episode)
}

// AutoOptimizeReview — POST /api/v1/projects/:id/episodes/:eid/auto-optimize-review
// 一键完成：转剧本格式 → AI 审查 → 自动弥补不足（如有 critical/低分问题）
// 异步执行（立即返回 202），前端通过轮询 episode 状态（optimize_status/review_status）跟踪进度。
func (h *EpisodeHandler) AutoOptimizeReview(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	episodeID, err := parseUint64Param(c, "eid")
	if err != nil {
		response.BadRequest(c, "invalid episode id")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		_, _ = h.svc.AutoOptimizeReview(ctx, episodeID, projectID)
	}()
	c.JSON(202, gin.H{"message": "auto-optimize-review started", "status": "processing"})
}

// BatchOptimizeEpisodes — POST /api/v1/projects/:id/episodes/batch-optimize
// 批量将所有分集转化为剧本格式（异步）
func (h *EpisodeHandler) BatchOptimizeEpisodes(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
		defer cancel()
		_, _ = h.svc.BatchOptimizeEpisodes(ctx, projectID)
	}()
	response.OK(c, gin.H{"message": "batch optimize started", "status": "processing"})
}

// BatchReviewEpisodes — POST /api/v1/projects/:id/episodes/batch-review
// 批量 AI 审查所有分集（异步）
func (h *EpisodeHandler) BatchReviewEpisodes(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
		defer cancel()
		_, _ = h.svc.BatchReviewEpisodes(ctx, projectID)
	}()
	response.OK(c, gin.H{"message": "batch review started", "status": "processing"})
}
