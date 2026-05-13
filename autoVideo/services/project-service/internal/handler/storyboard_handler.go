package handler

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/autovideo/project-service/internal/model"
	"github.com/autovideo/project-service/internal/service"
	"github.com/autovideo/project-service/pkg/response"
)

type StoryboardHandler struct {
	svc              *service.StoryboardService
	logger           *zap.Logger
	characterBaseURL string
	httpClient       *http.Client
}

// NewStoryboardHandler —— 创建分镜处理器实例
func NewStoryboardHandler(svc *service.StoryboardService, logger *zap.Logger, characterBaseURL string) *StoryboardHandler {
	return &StoryboardHandler{
		svc:              svc,
		logger:           logger,
		characterBaseURL: strings.TrimRight(characterBaseURL, "/"),
		httpClient:       &http.Client{Timeout: 30 * time.Second},
	}
}

type storyboardAssetSnapshot struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// storyboardEpisodeID looks up the episode_id for a given storyboard ID.
// Returns nil if not found or if the storyboard has no episode assigned.
func (h *StoryboardHandler) storyboardEpisodeID(storyboardID uint64) *uint64 {
	sb, err := h.svc.GetByID(storyboardID)
	if err != nil || sb == nil {
		return nil
	}
	return sb.EpisodeID
}

func (h *StoryboardHandler) ensureProjectAssetsReady(c *gin.Context, projectID uint64, episodeID *uint64) error {
	if h.characterBaseURL == "" {
		return nil
	}

	url := fmt.Sprintf("%s/api/v1/projects/%d/assets", h.characterBaseURL, projectID)
	if episodeID != nil {
		url = fmt.Sprintf("%s?episode_id=%d", url, *episodeID)
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if userID, exists := c.Get("user_id"); exists {
		req.Header.Set("X-User-ID", fmt.Sprintf("%v", userID))
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("query assets: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("query assets failed: %s", strings.TrimSpace(string(body)))
	}

	var payload struct {
		Data []storyboardAssetSnapshot `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("decode assets: %w", err)
	}

	total := 0
	generating := 0
	pending := 0
	paused := 0
	failed := 0
	for _, asset := range payload.Data {
		if asset.Name == "__extracting__" {
			continue
		}
		total++
		switch asset.Status {
		case "completed":
		case "generating":
			generating++
		case "pending":
			pending++
		case "paused":
			paused++
		default:
			failed++
		}
	}

	if total == 0 {
		// When scoped to a specific episode, 0 assets is allowed — the episode may not use any characters.
		// Only block when checking the whole project (episodeID == nil) and nothing has been extracted.
		if episodeID == nil {
			return fmt.Errorf("请先提取资源并完成资源图生成后再开始分镜")
		}
		return nil
	}
	// Allow storyboard generation when assets are no longer actively in progress.
	// Failed assets are a terminal state — blocking on them would prevent any storyboard work.
	if pending > 0 || generating > 0 || paused > 0 {
		return fmt.Errorf("资源图尚未全部完成：待生成 %d，生成中 %d，已暂停 %d，失败 %d", pending, generating, paused, failed)
	}

	return nil
}

// ListStoryboards —— 处理分页查询分镜列表的请求
// ListStoryboards GET /api/v1/projects/:id/storyboards
func (h *StoryboardHandler) ListStoryboards(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	keyword := strings.TrimSpace(c.Query("keyword"))
	includeVersions := c.Query("include_versions") == "true"

	var episodeID *uint64
	if eidStr := c.Query("episode_id"); eidStr != "" {
		eid, err := strconv.ParseUint(eidStr, 10, 64)
		if err != nil || eid == 0 {
			response.BadRequest(c, "invalid episode_id")
			return
		}
		episodeID = &eid
	}

	storyboards, total, err := h.svc.List(projectID, episodeID, status, keyword, page, pageSize, includeVersions)
	if err != nil {
		response.InternalError(c, "failed to list storyboards: "+err.Error())
		return
	}

	response.OKList(c, storyboards, page, pageSize, total)
}

// GetStoryboard —— 处理获取单个分镜详情的请求
// GetStoryboard GET /api/v1/projects/:id/storyboards/:sid
func (h *StoryboardHandler) GetStoryboard(c *gin.Context) {
	sid, err := parseUint64Param(c, "sid")
	if err != nil {
		response.BadRequest(c, "invalid storyboard id")
		return
	}

	sb, err := h.svc.GetByID(sid)
	if err != nil {
		if isStoryboardNotFound(err) {
			response.NotFound(c, "storyboard not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, sb)
}

// CreateStoryboard —— 处理创建新分镜的请求
// CreateStoryboard POST /api/v1/projects/:id/storyboards
func (h *StoryboardHandler) CreateStoryboard(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req service.CreateStoryboardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	sb, err := h.svc.Create(projectID, req)
	if err != nil {
		response.InternalError(c, "failed to create storyboard: "+err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    201,
		"message": "created",
		"data":    sb,
	})
}

// UpdateStoryboard —— 处理局部更新分镜字段的请求
// UpdateStoryboard PATCH /api/v1/projects/:id/storyboards/:sid
func (h *StoryboardHandler) UpdateStoryboard(c *gin.Context) {
	sid, err := parseUint64Param(c, "sid")
	if err != nil {
		response.BadRequest(c, "invalid storyboard id")
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	sb, err := h.svc.Update(sid, req)
	if err != nil {
		if isStoryboardNotFound(err) {
			response.NotFound(c, "storyboard not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, sb)
}

// GenerateStoryboard —— 处理触发单个分镜图片生成的请求
// GenerateStoryboard POST /api/v1/projects/:id/storyboards/:sid/generate
func (h *StoryboardHandler) GenerateStoryboard(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	sid, err := parseUint64Param(c, "sid")
	if err != nil {
		response.BadRequest(c, "invalid storyboard id")
		return
	}
	if err := h.ensureProjectAssetsReady(c, projectID, h.storyboardEpisodeID(sid)); err != nil {
		response.Fail(c, http.StatusConflict, 409, err.Error())
		return
	}

	var req struct {
		ModelName string `json:"model_name"`
	}
	_ = c.ShouldBindJSON(&req)

	sb, err := h.svc.Generate(sid, req.ModelName)
	if err != nil {
		if isStoryboardNotFound(err) {
			response.NotFound(c, "storyboard not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, sb)
}

// SwitchVersion —— 处理切换分镜当前版本的请求
// SwitchVersion POST /api/v1/projects/:id/storyboards/:sid/switch-version
func (h *StoryboardHandler) SwitchVersion(c *gin.Context) {
	sid, err := parseUint64Param(c, "sid")
	if err != nil {
		response.BadRequest(c, "invalid storyboard id")
		return
	}

	var req struct {
		VersionID uint64 `json:"version_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	sb, err := h.svc.SwitchVersion(sid, req.VersionID)
	if err != nil {
		if isStoryboardNotFound(err) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, sb)
}

// DeleteVersion —— 处理删除分镜指定版本的请求
// DeleteVersion DELETE /api/v1/projects/:id/storyboards/:sid/versions/:vid
func (h *StoryboardHandler) DeleteVersion(c *gin.Context) {
	sid, err := parseUint64Param(c, "sid")
	if err != nil {
		response.BadRequest(c, "invalid storyboard id")
		return
	}

	vid, err := parseUint64Param(c, "vid")
	if err != nil {
		response.BadRequest(c, "invalid version id")
		return
	}

	if err := h.svc.DeleteVersion(sid, vid); err != nil {
		if isStoryboardNotFound(err) {
			response.NotFound(c, "storyboard not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, nil)
}

// VoidStoryboard —— 处理作废分镜的请求
// VoidStoryboard POST /api/v1/projects/:id/storyboards/:sid/void
func (h *StoryboardHandler) VoidStoryboard(c *gin.Context) {
	sid, err := parseUint64Param(c, "sid")
	if err != nil {
		response.BadRequest(c, "invalid storyboard id")
		return
	}

	if err := h.svc.Void(sid); err != nil {
		if isStoryboardNotFound(err) {
			response.NotFound(c, "storyboard not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{"voided": true})
}

// DeleteStoryboard —— 永久删除单条分镜及其所有版本
// DeleteStoryboard DELETE /api/v1/projects/:id/storyboards/:sid
func (h *StoryboardHandler) DeleteStoryboard(c *gin.Context) {
	sid, err := parseUint64Param(c, "sid")
	if err != nil {
		response.BadRequest(c, "invalid storyboard id")
		return
	}

	if err := h.svc.Delete(sid); err != nil {
		if isStoryboardNotFound(err) {
			response.NotFound(c, "storyboard not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{"deleted": true})
}

// RetryStoryboard —— 处理重试单个失败分镜生成的请求
// RetryStoryboard POST /api/v1/projects/:id/storyboards/:sid/retry
func (h *StoryboardHandler) RetryStoryboard(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	sid, err := parseUint64Param(c, "sid")
	if err != nil {
		response.BadRequest(c, "invalid storyboard id")
		return
	}

	var req struct {
		ModelName string `json:"model_name"`
	}
	_ = c.ShouldBindJSON(&req) // optional body
	if err := h.ensureProjectAssetsReady(c, projectID, h.storyboardEpisodeID(sid)); err != nil {
		response.Fail(c, http.StatusConflict, 409, err.Error())
		return
	}

	sb, err := h.svc.Retry(sid, req.ModelName)
	if err != nil {
		if isStoryboardNotFound(err) {
			response.NotFound(c, "storyboard not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, sb)
}

// RetryBatch —— 处理批量重试所有失败分镜的请求
// RetryBatch POST /api/v1/projects/:id/storyboards/retry-failed
func (h *StoryboardHandler) RetryBatch(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req struct {
		ModelName  string   `json:"model_name"`
		ModelNames []string `json:"model_names"`
		EpisodeID  *uint64  `json:"episode_id"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.ensureProjectAssetsReady(c, projectID, req.EpisodeID); err != nil {
		response.Fail(c, http.StatusConflict, 409, err.Error())
		return
	}

	// Count eligible storyboards, then dispatch in background
	count, err := h.svc.CountFailed(projectID, req.EpisodeID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if count == 0 {
		response.OK(c, gin.H{"retried": 0})
		return
	}

	go func() {
		_, _ = h.svc.RetryBatch(projectID, req.EpisodeID, req.ModelName, req.ModelNames)
	}()

	payload := gin.H{"retried": count}
	if req.EpisodeID != nil {
		payload["episode_id"] = *req.EpisodeID
	}
	response.OK(c, payload)
}

// GenerateAll —— 处理一键生成项目下所有待处理分镜的请求
// GenerateAll POST /api/v1/projects/:id/storyboards/generate-all
func (h *StoryboardHandler) GenerateAll(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req struct {
		EpisodeID  *uint64  `json:"episode_id"`
		ModelName  string   `json:"model_name"`
		ModelNames []string `json:"model_names"`
		Force      bool     `json:"force"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.ensureProjectAssetsReady(c, projectID, req.EpisodeID); err != nil {
		response.Fail(c, http.StatusConflict, 409, err.Error())
		return
	}

	// Force-regenerate: auto-resume paused generation and reset completed storyboards to pending
	if req.Force {
		// Clear paused state so the background goroutine is not silently rejected.
		if _, rerr := h.svc.ResumeProjectGeneration(projectID, req.EpisodeID); rerr != nil {
			h.logger.Warn("auto-resume before force-regenerate failed", zap.Error(rerr))
		}
		if req.EpisodeID != nil {
			if n, ferr := h.svc.ForceResetEpisode(projectID, *req.EpisodeID); ferr != nil {
				h.logger.Warn("force reset episode completed storyboards failed", zap.Error(ferr))
			} else if n > 0 {
				h.logger.Info("force reset completed storyboards to pending",
					zap.Uint64("episode_id", *req.EpisodeID), zap.Int64("count", n))
			}
		}
	}

	// Count eligible storyboards first, then dispatch asynchronously
	count, err := h.svc.CountPendingOrFailed(projectID, req.EpisodeID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if count == 0 {
		response.OK(c, gin.H{"triggered": 0, "message": "no pending or failed storyboards"})
		return
	}

	// Return immediately; generation dispatches in background
	go func() {
		if _, err := h.svc.GenerateAll(projectID, req.EpisodeID, req.ModelName, req.ModelNames); err != nil {
			fields := []zap.Field{zap.Uint64("project_id", projectID), zap.Error(err)}
			if req.EpisodeID != nil {
				fields = append(fields, zap.Uint64("episode_id", *req.EpisodeID))
			}
			h.logger.Error("GenerateAll background error", fields...)
		}
	}()

	payload := gin.H{"triggered": count, "status": "dispatching"}
	if req.EpisodeID != nil {
		payload["episode_id"] = *req.EpisodeID
	}
	response.OK(c, payload)
}

// PauseGeneration —— 暂停项目下分镜图像生成
// PauseGeneration POST /api/v1/projects/:id/storyboards/pause-generation
func (h *StoryboardHandler) PauseGeneration(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	var req struct {
		EpisodeID *uint64 `json:"episode_id"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}
	count, err := h.svc.PauseProjectGeneration(projectID, req.EpisodeID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	payload := gin.H{"paused": count, "status": "paused"}
	if req.EpisodeID != nil {
		payload["episode_id"] = *req.EpisodeID
	}
	response.OK(c, payload)
}

// ResumeGeneration —— 继续项目下已暂停的分镜图像生成
// ResumeGeneration POST /api/v1/projects/:id/storyboards/resume-generation
func (h *StoryboardHandler) ResumeGeneration(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	var req struct {
		EpisodeID *uint64 `json:"episode_id"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}
	if err := h.ensureProjectAssetsReady(c, projectID, req.EpisodeID); err != nil {
		response.Fail(c, http.StatusConflict, 409, err.Error())
		return
	}
	count, err := h.svc.ResumeProjectGeneration(projectID, req.EpisodeID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	payload := gin.H{"triggered": count, "status": "resumed"}
	if req.EpisodeID != nil {
		payload["episode_id"] = *req.EpisodeID
	}
	response.OK(c, payload)
}

// ChatStoryboard —— 处理对分镜进行对话式修改的请求
// ChatStoryboard POST /api/v1/projects/:id/storyboards/:sid/chat
func (h *StoryboardHandler) ChatStoryboard(c *gin.Context) {
	sid, err := parseUint64Param(c, "sid")
	if err != nil {
		response.BadRequest(c, "invalid storyboard id")
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	sb, err := h.svc.Chat(sid, req)
	if err != nil {
		if isStoryboardNotFound(err) {
			response.NotFound(c, "storyboard not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, sb)
}

// AuditContinuity —— 触发 AI 场景连贯性审核并自动补全分镜信息
// AuditContinuity POST /api/v1/projects/:id/storyboards/audit-continuity
func (h *StoryboardHandler) AuditContinuity(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req struct {
		EpisodeID *uint64 `json:"episode_id"`
	}
	_ = c.ShouldBindJSON(&req)

	ctx := c.Request.Context()
	result, err := h.svc.AuditSceneContinuity(ctx, projectID, req.EpisodeID)
	if err != nil {
		h.logger.Warn("audit continuity failed",
			zap.Uint64("project_id", projectID),
			zap.Error(err),
		)
		response.Fail(c, http.StatusInternalServerError, 500, err.Error())
		return
	}

	response.OK(c, gin.H{
		"total_groups":  result.TotalGroups,
		"total_patched": result.TotalPatched,
		"patches":       result.Patches,
	})
}

// Stats —— 处理获取项目分镜各状态统计数据的请求
// Stats GET /api/v1/projects/:id/storyboards/stats
func (h *StoryboardHandler) Stats(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	counts, err := h.svc.StatusCounts(projectID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	var total int64
	for _, v := range counts {
		total += v
	}

	response.OK(c, gin.H{
		"total":      total,
		"pending":    counts["pending"],
		"generating": counts["generating"],
		"paused":     counts["paused"],
		"completed":  counts["completed"],
		"failed":     counts["failed"],
		"voided":     counts["voided"],
	})
}

// UpdateConfig —— 处理更新项目分镜全局配置的请求
// UpdateConfig PATCH /api/v1/projects/:id/storyboards/config
func (h *StoryboardHandler) UpdateConfig(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.svc.UpdateConfig(projectID, req); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{"project_id": projectID, "config": req})
}

// isStoryboardNotFound —— 判断错误是否为分镜或版本未找到
func isStoryboardNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "storyboard not found" || msg == "version not found"
}

// EpisodeStats GET /api/v1/projects/:id/storyboards/episode-stats
// Returns completed storyboard counts per episode for the project.
func (h *StoryboardHandler) EpisodeStats(c *gin.Context) {
projectID, err := parseUint64Param(c, "id")
if err != nil {
response.BadRequest(c, "invalid project id")
return
}

counts, err := h.svc.EpisodeCompletedCounts(projectID)
if err != nil {
response.InternalError(c, err.Error())
return
}

// Use string keys for JSON compatibility
result := make(map[string]int64, len(counts))
for k, v := range counts {
result[strconv.FormatUint(k, 10)] = v
}
response.OK(c, result)
}

// Export —— 将项目所有已完成的分镜图片打包为 ZIP 文件并供下载
// Export godoc
// GET /api/v1/projects/:id/storyboards/export
func (h *StoryboardHandler) Export(c *gin.Context) {
	projectID, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	sbs, err := h.svc.GetCompletedWithImages(projectID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if len(sbs) == 0 {
		response.BadRequest(c, "no completed storyboard images found for this project")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"storyboards_%d.zip\"", projectID))
	c.Header("Content-Type", "application/zip")

	client := h.httpClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	zw := zip.NewWriter(c.Writer)
	defer zw.Close()
	writeStoryboardsZip(sbs, client, zw, h.logger)
}

// writeStoryboardsZip downloads each storyboard image and writes it into zw.
// Errors on individual images are logged and skipped so a partial ZIP is still returned.
func writeStoryboardsZip(sbs []model.Storyboard, client *http.Client, zw *zip.Writer, logger *zap.Logger) {
	for i, sb := range sbs {
		imgResp, err := client.Get(sb.ImageURL)
		if err != nil {
			if logger != nil {
				logger.Warn("export: skip image (download failed)", zap.Uint64("storyboard_id", sb.ID), zap.Error(err))
			}
			continue
		}

		ext := ".jpg"
		ct := imgResp.Header.Get("Content-Type")
		switch {
		case strings.Contains(ct, "png"):
			ext = ".png"
		case strings.Contains(ct, "webp"):
			ext = ".webp"
		}

		fw, err := zw.Create(fmt.Sprintf("%04d_scene%d%s", i+1, sb.SequenceNumber, ext))
		if err != nil {
			imgResp.Body.Close()
			if logger != nil {
				logger.Warn("export: skip image (zip entry failed)", zap.Uint64("storyboard_id", sb.ID), zap.Error(err))
			}
			continue
		}
		_, _ = io.Copy(fw, imgResp.Body)
		imgResp.Body.Close()
	}
}
