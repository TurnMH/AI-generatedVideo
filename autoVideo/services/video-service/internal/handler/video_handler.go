package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/autovideo/video-service/internal/model"
	"github.com/autovideo/video-service/internal/service"
	"github.com/autovideo/video-service/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const videoRenderConfigVersion = 1

func validateSerialScenePayload(imageURLs []string, sceneGroupKeys []string, serialScene bool) error {
	if !serialScene {
		return nil
	}
	if len(imageURLs) == 0 {
		return fmt.Errorf("serial_scene requires at least one clip")
	}
	if len(sceneGroupKeys) != len(imageURLs) {
		return fmt.Errorf("serial_scene requires scene_group_keys for every clip")
	}

	seenGroups := make(map[string]struct{}, len(sceneGroupKeys))
	for idx, rawURL := range imageURLs {
		trimmedURL := strings.TrimSpace(rawURL)
		groupKey := strings.TrimSpace(sceneGroupKeys[idx])
		if groupKey == "" {
			if trimmedURL == "" {
				return fmt.Errorf("serial_scene clip %d must provide a first-frame image when scene_group_key is empty", idx+1)
			}
			continue
		}
		if _, seen := seenGroups[groupKey]; seen {
			continue
		}
		if trimmedURL == "" {
			return fmt.Errorf("serial_scene group %q is missing its first-frame image", groupKey)
		}
		seenGroups[groupKey] = struct{}{}
	}

	return nil
}

func normalizeRenderConfig(rc model.RenderConfig) model.RenderConfig {
	if rc == nil {
		rc = model.RenderConfig{}
	}
	if _, ok := rc["config_version"]; !ok {
		rc["config_version"] = videoRenderConfigVersion
	}
	return rc
}

// VideoHandler exposes all HTTP endpoints for the video service.
type VideoHandler struct {
	svc          *service.VideoService
	watermarkSvc *service.WatermarkService
	logger       *zap.Logger
}

// NewVideoHandler —— 创建视频处理器实例，返回 *VideoHandler
func NewVideoHandler(svc *service.VideoService, watermarkSvc *service.WatermarkService, logger *zap.Logger) *VideoHandler {
	return &VideoHandler{svc: svc, watermarkSvc: watermarkSvc, logger: logger}
}

// ---- request / response DTOs ----

type generateReq struct {
	ProjectID         int64              `json:"project_id" binding:"required"`
	EpisodeID         *int64             `json:"episode_id"`
	ImageURLs         []string           `json:"image_urls" binding:"required,min=1"`
	SceneDescriptions []string           `json:"scene_descriptions"` // per-clip descriptions, parallel to image_urls
	MotionDescs       []string           `json:"motion_descs"`       // opt-p7: per-clip camera/motion from storyboard
	StylePreset       string             `json:"style_preset"`
	MotionMode        string             `json:"motion_mode"`
	ModelName         string             `json:"model_name"`
	AudioURL          string             `json:"audio_url"`
	SubtitleText      string             `json:"subtitle_text"`
	SceneDescription  string             `json:"scene_description"`
	RenderConfig      model.RenderConfig `json:"render_config"`
	ClipDurationSec   float64            `json:"clip_duration_sec"` // desired clip duration from project storyboard_config
}

// Generate —— 处理视频生成请求，创建任务并分发到 Kafka，返回 task_id
// Generate godoc
// POST /api/v1/videos/generate
func (h *VideoHandler) Generate(c *gin.Context) {
	var req generateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := mustUserID(c)

	setDefault := func(v, def string) string {
		if v == "" {
			return def
		}
		return v
	}
	req.StylePreset = setDefault(req.StylePreset, "anime-2d")
	req.MotionMode = setDefault(req.MotionMode, "gentle")
	req.ModelName = setDefault(req.ModelName, "kling")

	// Store per-clip scene descriptions in render_config for use during generation.
	req.RenderConfig = normalizeRenderConfig(req.RenderConfig)
	if len(req.SceneDescriptions) > 0 {
		req.RenderConfig["scene_descriptions"] = req.SceneDescriptions
	}
	// opt-p7: store motion descriptions from storyboard in render_config
	if len(req.MotionDescs) > 0 {
		req.RenderConfig["motion_descs"] = req.MotionDescs
	}

	task := &model.VideoTask{
		ProjectID:        req.ProjectID,
		EpisodeID:        req.EpisodeID,
		UserID:           userID,
		ImageURLs:        model.StringArray(req.ImageURLs),
		StylePreset:      req.StylePreset,
		MotionMode:       req.MotionMode,
		AudioURL:         req.AudioURL,
		SubtitleText:     req.SubtitleText,
		ModelName:        req.ModelName,
		SceneDescription: req.SceneDescription,
		RenderConfig:     req.RenderConfig,
		DurationSec:      req.ClipDurationSec,
		Status:           model.StatusPending,
	}

	ctx := c.Request.Context()
	if err := h.svc.CreateTask(ctx, task); err != nil {
		h.logger.Error("create task", zap.Error(err))
		response.InternalError(c, "failed to create task")
		return
	}

	response.OK(c, gin.H{"task_id": task.ID})
}

// GetTask —— 根据 ID 查询单个视频任务详情，返回任务 JSON
// GetTask godoc
// GET /api/v1/videos/tasks/:id
func (h *VideoHandler) GetTask(c *gin.Context) {
	id, err := pathInt64(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	task, err := h.svc.GetTask(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "task not found")
		return
	}
	response.OK(c, task)
}

// ListTasks —— 分页查询视频任务列表，支持按项目和集数过滤
// ListTasks godoc
// GET /api/v1/videos/tasks?project_id=&episode_id=&page=&page_size=
func (h *VideoHandler) ListTasks(c *gin.Context) {
	projectID := queryInt64(c, "project_id")
	episodeID := queryInt64(c, "episode_id")
	page := queryInt(c, "page", 1)
	pageSize := queryInt(c, "page_size", 20)

	tasks, total, err := h.svc.ListTasks(c.Request.Context(), projectID, episodeID, page, pageSize)
	if err != nil {
		h.logger.Error("list tasks", zap.Error(err))
		response.InternalError(c, "failed to list tasks")
		return
	}
	response.OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     tasks,
	})
}

// DeleteTask —— 软删除指定 ID 的视频任务
// DeleteTask godoc
// DELETE /api/v1/videos/tasks/:id
func (h *VideoHandler) DeleteTask(c *gin.Context) {
	id, err := pathInt64(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.svc.DeleteTask(c.Request.Context(), id); err != nil {
		h.logger.Error("delete task", zap.Int64("task_id", id), zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "task deleted"})
}

// DeleteProjectData —— 删除项目下所有视频相关运行数据
// DELETE /api/v1/projects/:pid/videos/runtime-data
func (h *VideoHandler) DeleteProjectData(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	if err := h.svc.DeleteProjectData(c.Request.Context(), pid); err != nil {
		h.logger.Error("delete project video runtime data", zap.Int64("project_id", pid), zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// DeleteEpisodeData —— 删除指定剧集下所有视频任务、片段和配音任务（幂等）
// DELETE /api/v1/projects/:pid/episodes/:eid/videos/runtime-data
func (h *VideoHandler) DeleteEpisodeData(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	eid, err := pathInt64(c, "eid")
	if err != nil {
		response.BadRequest(c, "invalid episode id")
		return
	}
	if err := h.svc.DeleteEpisodeData(c.Request.Context(), pid, eid); err != nil {
		h.logger.Error("delete episode video runtime data", zap.Int64("project_id", pid), zap.Int64("episode_id", eid), zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// Compose —— 触发已有片段的合成，异步执行并立即返回
// Compose godoc
// POST /api/v1/videos/tasks/:id/compose
func (h *VideoHandler) Compose(c *gin.Context) {
	id, err := pathInt64(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	// Run compose asynchronously so the HTTP response is immediate
	go func() {
		if err := h.svc.ComposeTask(context.Background(), id); err != nil {
			h.logger.Error("compose task", zap.Int64("task_id", id), zap.Error(err))
		}
	}()

	response.OK(c, gin.H{"task_id": id, "message": "composition started"})
}

// Download —— 重定向到已完成视频的下载地址
// Download godoc
// GET /api/v1/videos/:id/download
func (h *VideoHandler) Download(c *gin.Context) {
	id, err := pathInt64(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	task, err := h.svc.GetTask(c.Request.Context(), id)
	if err != nil || task.ResultURL == "" {
		response.NotFound(c, "video not found or not ready")
		return
	}
	c.Redirect(http.StatusFound, task.ResultURL)
}

// ---- helpers ----

// mustUserID —— 从 Gin 上下文中获取当前用户 ID，未找到则返回 0
func mustUserID(c *gin.Context) int64 {
	if v, ok := c.Get("user_id"); ok {
		if uid, ok := v.(int64); ok {
			return uid
		}
	}
	return 0
}

// pathInt64 —— 从 URL 路径参数中解析 int64 值
func pathInt64(c *gin.Context, key string) (int64, error) {
	return strconv.ParseInt(c.Param(key), 10, 64)
}

// queryInt64 —— 从 URL 查询参数中解析 int64 值，失败时返回 0
func queryInt64(c *gin.Context, key string) int64 {
	v, _ := strconv.ParseInt(c.Query(key), 10, 64)
	return v
}

// queryInt —— 从 URL 查询参数中解析 int 值，失败时返回默认值 def
func queryInt(c *gin.Context, key string, def int) int {
	s := c.Query(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// ── Project-scoped video endpoints ─────────────────────────

// ListProjectVideos —— 分页查询指定项目下的视频列表
// ListProjectVideos godoc
// GET /api/v1/projects/:pid/videos
func (h *VideoHandler) ListProjectVideos(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	page := queryInt(c, "page", 1)
	pageSize := queryInt(c, "page_size", 20)
	episodeID := queryInt64(c, "episode_id")

	tasks, total, err := h.svc.ListTasks(c.Request.Context(), pid, episodeID, page, pageSize)
	if err != nil {
		h.logger.Error("list project videos", zap.Error(err))
		response.InternalError(c, "failed to list videos")
		return
	}
	response.OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     tasks,
	})
}

type projectGenerateReq struct {
	EpisodeID         *int64             `json:"episode_id"`
	ImageURLs         []string           `json:"image_urls" binding:"required,min=1"`
	SceneDescriptions []string           `json:"scene_descriptions"` // per-clip visual descriptions
	Dialogues         []string           `json:"dialogues"`          // per-clip dialogue/subtitle text
	Durations         []float64          `json:"durations"`          // per-clip duration in seconds (from storyboard)
	CameraMovements   []string           `json:"camera_movements"`   // per-clip camera movement hint
	Moods             []string           `json:"moods"`              // per-clip mood/emotion
	SceneCharacters   [][]string         `json:"scene_characters"`   // per-clip character names for ref image filtering
	SceneAssetIDs     [][]int64          `json:"scene_asset_ids"`    // per-clip related asset IDs for scene/prop continuity
	StylePreset       string             `json:"style_preset"`
	MotionMode        string             `json:"motion_mode"`
	ModelName         string             `json:"model_name"`
	AudioURL          string             `json:"audio_url"`
	SubtitleText      string             `json:"subtitle_text"`
	VideoMode         string             `json:"video_mode"`
	ExportFormat      string             `json:"export_format"`
	SceneDescription  string             `json:"scene_description"`
	RenderConfig      model.RenderConfig `json:"render_config"`
	ClipDurationSec   float64            `json:"clip_duration_sec"`
	// 视频串行生成
	SerialScene    bool     `json:"serial_scene"`    // true = 同场景分镜串行生成（末帧约束）
	SceneGroupKeys []string `json:"scene_group_keys"` // 与 image_urls 一一对应的场景 key
}

// GenerateProjectVideo —— 为指定项目创建视频生成任务
// GenerateProjectVideo godoc
// POST /api/v1/projects/:pid/videos/generate
func (h *VideoHandler) GenerateProjectVideo(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req projectGenerateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := validateSerialScenePayload(req.ImageURLs, req.SceneGroupKeys, req.SerialScene); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := mustUserID(c)

	setDefault := func(v, def string) string {
		if v == "" {
			return def
		}
		return v
	}

	if req.RenderConfig == nil {
		req.RenderConfig = model.RenderConfig{}
	}
	if len(req.SceneDescriptions) > 0 {
		req.RenderConfig["scene_descriptions"] = req.SceneDescriptions
	}
	if len(req.Dialogues) > 0 {
		req.RenderConfig["dialogues"] = req.Dialogues
	}
	if len(req.Durations) > 0 {
		req.RenderConfig["durations"] = req.Durations
	}
	if len(req.CameraMovements) > 0 {
		req.RenderConfig["camera_movements"] = req.CameraMovements
	}
	if len(req.Moods) > 0 {
		req.RenderConfig["moods"] = req.Moods
	}
	if len(req.SceneCharacters) > 0 {
		req.RenderConfig["scene_characters"] = req.SceneCharacters
	}
	if len(req.SceneAssetIDs) > 0 {
		req.RenderConfig["scene_asset_ids"] = req.SceneAssetIDs
	}

	task := &model.VideoTask{
		ProjectID:        pid,
		EpisodeID:        req.EpisodeID,
		UserID:           userID,
		ImageURLs:        model.StringArray(req.ImageURLs),
		StylePreset:      setDefault(req.StylePreset, "anime-2d"),
		MotionMode:       setDefault(req.MotionMode, "gentle"),
		AudioURL:         req.AudioURL,
		SubtitleText:     req.SubtitleText,
		ModelName:        setDefault(req.ModelName, "kling"),
		VideoMode:        setDefault(req.VideoMode, "frame_animation"),
		ExportFormat:     setDefault(req.ExportFormat, "mp4"),
		SceneDescription: req.SceneDescription,
		RenderConfig:     req.RenderConfig,
		DurationSec:      req.ClipDurationSec,
		Status:           model.StatusPending,
		// 视频串行生成
		SerialScene:    req.SerialScene,
		SceneGroupKeys: model.StringArray(req.SceneGroupKeys),
	}

	ctx := c.Request.Context()
	if err := h.svc.CreateTask(ctx, task); err != nil {
		h.logger.Error("create project video task", zap.Error(err))
		response.InternalError(c, "failed to create video task")
		return
	}

	response.OK(c, gin.H{"task_id": task.ID, "message": "video generation started"})
}

// GenerateVariants —— 批量生成多版本视频，用于 A/B 效果对比（feat-6）
// GenerateVariants godoc
// POST /api/v1/projects/:pid/videos/generate-variants
// Creates variant_count parallel tasks from the same source, each using a different random seed.
// All tasks share a variant_group_id for easy grouping in the UI.
func (h *VideoHandler) GenerateVariants(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req struct {
		EpisodeID        *int64             `json:"episode_id"`
		ImageURLs        []string           `json:"image_urls" binding:"required,min=1"`
		VariantCount     int                `json:"variant_count"` // 1-5, default 2
		StylePreset      string             `json:"style_preset"`
		MotionMode       string             `json:"motion_mode"`
		ModelName        string             `json:"model_name"`
		AudioURL         string             `json:"audio_url"`
		SubtitleText     string             `json:"subtitle_text"`
		SceneDescription string             `json:"scene_description"`
		RenderConfig     model.RenderConfig `json:"render_config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	variantCount := req.VariantCount
	if variantCount < 1 {
		variantCount = 2
	}
	if variantCount > 5 {
		variantCount = 5
	}

	userID := mustUserID(c)
	ctx := c.Request.Context()

	setDefault := func(v, def string) string {
		if v == "" {
			return def
		}
		return v
	}

	// Create tasks; assign variant_group_id = first task's ID after creation
	var taskIDs []int64
	var groupID int64

	for i := 0; i < variantCount; i++ {
		rc := req.RenderConfig
		if rc == nil {
			rc = model.RenderConfig{}
		}
		// Stamp a unique seed so each variant generates different clips
		rc["variant_seed"] = i*1000 + int(time.Now().UnixNano()%1000)

		task := &model.VideoTask{
			ProjectID:        pid,
			EpisodeID:        req.EpisodeID,
			UserID:           userID,
			ImageURLs:        model.StringArray(req.ImageURLs),
			StylePreset:      setDefault(req.StylePreset, "anime-2d"),
			MotionMode:       setDefault(req.MotionMode, "gentle"),
			ModelName:        setDefault(req.ModelName, "kling"),
			AudioURL:         req.AudioURL,
			SubtitleText:     req.SubtitleText,
			SceneDescription: req.SceneDescription,
			RenderConfig:     rc,
			Status:           model.StatusPending,
			VariantIndex:     i,
		}
		if err := h.svc.CreateTask(ctx, task); err != nil {
			h.logger.Error("create variant task", zap.Int("variant", i), zap.Error(err))
			continue
		}
		taskIDs = append(taskIDs, task.ID)
		if i == 0 {
			groupID = task.ID
		}
	}

	// Back-fill variant_group_id on all created tasks
	if len(taskIDs) > 0 && groupID > 0 {
		_ = h.svc.SetVariantGroupID(ctx, taskIDs, groupID)
	}

	response.OK(c, gin.H{
		"task_ids":         taskIDs,
		"variant_group_id": groupID,
		"count":            len(taskIDs),
	})
}

// GenerateProjectVideosBatch godoc
// POST /api/v1/projects/:pid/videos/generate-batch
// Creates one video task per episode, each with its corresponding storyboard images.
func (h *VideoHandler) GenerateProjectVideosBatch(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	type episodeImages struct {
		EpisodeID         int64      `json:"episode_id" binding:"required"`
		ImageURLs         []string   `json:"image_urls" binding:"required,min=1"`
		SceneDescriptions []string   `json:"scene_descriptions"` // per-clip visual descriptions
		Dialogues         []string   `json:"dialogues"`          // per-clip dialogue text
		Durations         []float64  `json:"durations"`          // per-clip duration in seconds
		CameraMovements   []string   `json:"camera_movements"`   // per-clip camera movement hint
		Moods             []string   `json:"moods"`              // per-clip mood
		SceneCharacters   [][]string `json:"scene_characters"`   // per-clip character names
		SceneAssetIDs     [][]int64  `json:"scene_asset_ids"`    // per-clip related asset IDs
		AudioURL          string     `json:"audio_url"`
		SceneDescription  string     `json:"scene_description"`
		SceneGroupKeys    []string   `json:"scene_group_keys"`  // 串行模式：每 clip 的场景 key
	}
	var req struct {
		Episodes        []episodeImages    `json:"episodes" binding:"required,min=1"`
		StylePreset     string             `json:"style_preset"`
		MotionMode      string             `json:"motion_mode"`
		ModelName       string             `json:"model_name"`
		VideoMode       string             `json:"video_mode"`
		ExportFormat    string             `json:"export_format"`
		RenderConfig    model.RenderConfig `json:"render_config"`
		ClipDurationSec float64            `json:"clip_duration_sec"`
		SerialScene     bool               `json:"serial_scene"`    // 串行模式
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	for _, ep := range req.Episodes {
		if err := validateSerialScenePayload(ep.ImageURLs, ep.SceneGroupKeys, req.SerialScene); err != nil {
			response.BadRequest(c, fmt.Sprintf("episode %d: %s", ep.EpisodeID, err.Error()))
			return
		}
	}

	userID := mustUserID(c)

	setDefault := func(v, def string) string {
		if v == "" {
			return def
		}
		return v
	}

	ctx := c.Request.Context()
	var taskIDs []int64

	for _, ep := range req.Episodes {
		epID := ep.EpisodeID
		rc := req.RenderConfig
		if rc == nil {
			rc = model.RenderConfig{}
		} else {
			// copy to avoid mutating shared RenderConfig across episodes
			copy := make(model.RenderConfig, len(rc)+5)
			for k, v := range rc {
				copy[k] = v
			}
			rc = copy
		}
		rc = normalizeRenderConfig(rc)
		if len(ep.SceneDescriptions) > 0 {
			rc["scene_descriptions"] = ep.SceneDescriptions
		}
		if len(ep.Dialogues) > 0 {
			rc["dialogues"] = ep.Dialogues
		}
		if len(ep.Durations) > 0 {
			rc["durations"] = ep.Durations
		}
		if len(ep.CameraMovements) > 0 {
			rc["camera_movements"] = ep.CameraMovements
		}
		if len(ep.Moods) > 0 {
			rc["moods"] = ep.Moods
		}
		if len(ep.SceneCharacters) > 0 {
			rc["scene_characters"] = ep.SceneCharacters
		}
		if len(ep.SceneAssetIDs) > 0 {
			rc["scene_asset_ids"] = ep.SceneAssetIDs
		}
		task := &model.VideoTask{
			ProjectID:        pid,
			EpisodeID:        &epID,
			UserID:           userID,
			ImageURLs:        model.StringArray(ep.ImageURLs),
			AudioURL:         ep.AudioURL,
			StylePreset:      setDefault(req.StylePreset, "anime-2d"),
			MotionMode:       setDefault(req.MotionMode, "gentle"),
			ModelName:        setDefault(req.ModelName, "kling"),
			VideoMode:        setDefault(req.VideoMode, "frame_animation"),
			ExportFormat:     setDefault(req.ExportFormat, "mp4"),
			SceneDescription: ep.SceneDescription,
			RenderConfig:     rc,
			DurationSec:      req.ClipDurationSec,
			Status:           model.StatusPending,
			SerialScene:      req.SerialScene,
			SceneGroupKeys:   model.StringArray(ep.SceneGroupKeys),
		}
		if err := h.svc.CreateTask(ctx, task); err != nil {
			h.logger.Error("create batch video task", zap.Int64("episode_id", epID), zap.Error(err))
			continue
		}
		taskIDs = append(taskIDs, task.ID)
	}

	response.OK(c, gin.H{
		"task_ids": taskIDs,
		"count":    len(taskIDs),
		"message":  "batch video generation started",
	})
}

// PauseVideo —— 暂停指定视频任务
// PauseVideo godoc
// POST /api/v1/projects/:pid/videos/:vid/pause
func (h *VideoHandler) PauseVideo(c *gin.Context) {
	vid, err := pathInt64(c, "vid")
	if err != nil {
		response.BadRequest(c, "invalid video id")
		return
	}

	if err := h.svc.PauseTask(c.Request.Context(), vid); err != nil {
		h.logger.Error("pause video", zap.Int64("vid", vid), zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{"task_id": vid, "status": model.StatusPaused})
}

// ResumeVideo —— 恢复暂停的视频任务
// ResumeVideo godoc
// POST /api/v1/projects/:pid/videos/:vid/resume
func (h *VideoHandler) ResumeVideo(c *gin.Context) {
	vid, err := pathInt64(c, "vid")
	if err != nil {
		response.BadRequest(c, "invalid video id")
		return
	}

	if err := h.svc.ResumeTask(c.Request.Context(), vid); err != nil {
		h.logger.Error("resume video", zap.Int64("vid", vid), zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{"task_id": vid, "status": model.StatusProcessing})
}

// ExportVideo —— 返回已完成视频的导出信息（URL、格式、时长等）
// ExportVideo godoc
// GET /api/v1/projects/:pid/videos/:vid/export
func (h *VideoHandler) ExportVideo(c *gin.Context) {
	vid, err := pathInt64(c, "vid")
	if err != nil {
		response.BadRequest(c, "invalid video id")
		return
	}

	task, err := h.svc.GetTask(c.Request.Context(), vid)
	if err != nil || task.ResultURL == "" {
		response.NotFound(c, "video not found or not ready for export")
		return
	}

	response.OK(c, gin.H{
		"task_id":       task.ID,
		"status":        task.Status,
		"export_format": task.ExportFormat,
		"result_url":    task.ResultURL,
		"hls_url":       task.HlsURL,
		"duration_sec":  task.DurationSec,
	})
}

// ApplyWatermark —— 为指定视频添加水印
// ApplyWatermark godoc
// POST /api/v1/projects/:pid/videos/:vid/watermark
func (h *VideoHandler) ApplyWatermark(c *gin.Context) {
	vid, err := pathInt64(c, "vid")
	if err != nil {
		response.BadRequest(c, "invalid video id")
		return
	}

	task, err := h.svc.GetTask(c.Request.Context(), vid)
	if err != nil || task.ResultURL == "" {
		response.NotFound(c, "video not found or not ready")
		return
	}

	var cfg service.WatermarkConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	outputPath := task.ResultURL + "_watermarked"
	if err := h.watermarkSvc.ApplyWatermark(task.ResultURL, outputPath, cfg); err != nil {
		h.logger.Error("apply watermark", zap.Error(err))
		response.InternalError(c, "failed to apply watermark")
		return
	}

	response.OK(c, gin.H{
		"task_id": vid,
		"message": "watermark applied (dry-run)",
	})
}

// RetryVideo —— 重试指定的失败视频任务，可选切换模型
// RetryVideo godoc
// POST /api/v1/projects/:pid/videos/:vid/retry
func (h *VideoHandler) RetryVideo(c *gin.Context) {
	vid, err := pathInt64(c, "vid")
	if err != nil {
		response.BadRequest(c, "invalid video id")
		return
	}

	var req struct {
		ModelName string `json:"model_name"`
	}
	_ = c.ShouldBindJSON(&req)

	if err := h.svc.RetryTask(c.Request.Context(), vid, req.ModelName); err != nil {
		h.logger.Error("retry video", zap.Int64("vid", vid), zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{"task_id": vid, "message": "retry started"})
}

// RetryVideoClip retries a single failed clip within a video task.
// POST /api/v1/projects/:pid/videos/:vid/clips/:cid/retry
func (h *VideoHandler) RetryVideoClip(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	vid, err := pathInt64(c, "vid")
	if err != nil {
		response.BadRequest(c, "invalid video id")
		return
	}
	cid, err := pathInt64(c, "cid")
	if err != nil {
		response.BadRequest(c, "invalid clip id")
		return
	}

	var req struct {
		ModelName string `json:"model_name"`
	}
	_ = c.ShouldBindJSON(&req)

	go func() {
		if err := h.svc.RetryClip(context.Background(), pid, vid, cid, req.ModelName); err != nil {
			h.logger.Error("retry video clip",
				zap.Int64("project_id", pid),
				zap.Int64("video_id", vid),
				zap.Int64("clip_id", cid),
				zap.Error(err),
			)
		}
	}()

	response.OK(c, gin.H{"task_id": vid, "clip_id": cid, "message": "clip retry started"})
}

// RetryAllFailed —— 批量重试项目下所有失败的视频任务
// RetryAllFailed godoc
// POST /api/v1/projects/:pid/videos/retry-failed
func (h *VideoHandler) RetryAllFailed(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req struct {
		ModelName string `json:"model_name"`
	}
	_ = c.ShouldBindJSON(&req)

	count, err := h.svc.RetryBatchFailed(c.Request.Context(), pid, req.ModelName)
	if err != nil {
		h.logger.Error("retry all failed", zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, gin.H{"retried": count})
}

// VideoStats —— 查询项目下视频任务的状态统计
// VideoStats godoc
// GET /api/v1/projects/:pid/videos/stats
func (h *VideoHandler) VideoStats(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	counts, err := h.svc.StatusCounts(c.Request.Context(), pid)
	if err != nil {
		h.logger.Error("video stats", zap.Error(err))
		response.InternalError(c, "failed to get stats")
		return
	}

	response.OK(c, counts)
}

// ModelStatus —— 返回所有已注册视频生成器的可用状态
// GET /api/v1/videos/model-status
func (h *VideoHandler) ModelStatus(c *gin.Context) {
	items := h.svc.ModelStatus(c.Request.Context())
	response.OK(c, gin.H{"models": items})
}

// ── 自动审片 ────────────────────────────────────────────────────────────────

// shotsMeta 是 clip-service / MVP pipeline 期望的 shots_metadata.json 格式
type shotsMeta struct {
	EpisodeID    string      `json:"episode_id"`
	EpisodeTitle string      `json:"episode_title"`
	TotalShots   int         `json:"total_shots"`
	Shots        []shotEntry `json:"shots"`
}

type shotEntry struct {
	ShotID string `json:"shot_id"`
	File   string `json:"file"`
	URL    string `json:"url"`
}

// GetEpisodeShotsMetadata —— 将 episode 下所有已生成的 VideoClip 组装成 shots_metadata 格式
// GET /api/v1/projects/:pid/episodes/:eid/videos/shots-metadata
func (h *VideoHandler) GetEpisodeShotsMetadata(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	eid, err := pathInt64(c, "eid")
	if err != nil {
		response.BadRequest(c, "invalid episode id")
		return
	}

	clips, err := h.svc.GetClipsByEpisode(c.Request.Context(), pid, eid)
	if err != nil {
		h.logger.Error("get clips by episode", zap.Error(err))
		response.InternalError(c, "failed to query clips")
		return
	}

	shots := make([]shotEntry, 0, len(clips))
	for _, clip := range clips {
		shots = append(shots, shotEntry{
			ShotID: fmt.Sprintf("shot_%d", clip.ID),
			File:   fmt.Sprintf("shot_%d.mp4", clip.ID),
			URL:    clip.ClipURL,
		})
	}

	meta := shotsMeta{
		EpisodeID:  fmt.Sprintf("ep_%d", eid),
		TotalShots: len(shots),
		Shots:      shots,
	}
	response.OK(c, meta)
}

// clipTriggerReq 是触发自动审片流水线的请求体
type clipTriggerReq struct {
	ScriptText string `json:"script_text"`
}

// TriggerClipPipeline —— 查询 episode 的 VideoClip，拼装 shots_metadata 后调用 clip-service
// POST /api/v1/projects/:pid/episodes/:eid/videos/clip-trigger
func (h *VideoHandler) TriggerClipPipeline(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	eid, err := pathInt64(c, "eid")
	if err != nil {
		response.BadRequest(c, "invalid episode id")
		return
	}

	var req clipTriggerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "script_text is required")
		return
	}

	clips, err := h.svc.GetClipsByEpisode(c.Request.Context(), pid, eid)
	if err != nil {
		h.logger.Error("get clips by episode", zap.Error(err))
		response.InternalError(c, "failed to query clips")
		return
	}
	if len(clips) == 0 {
		response.BadRequest(c, "no generated clips found for this episode")
		return
	}

	shots := make([]shotEntry, 0, len(clips))
	for _, clip := range clips {
		shots = append(shots, shotEntry{
			ShotID: fmt.Sprintf("shot_%d", clip.ID),
			File:   fmt.Sprintf("shot_%d.mp4", clip.ID),
			URL:    clip.ClipURL,
		})
	}

	meta := shotsMeta{
		EpisodeID:  fmt.Sprintf("ep_%d", eid),
		TotalShots: len(shots),
		Shots:      shots,
	}

	clipServiceURL := os.Getenv("CLIP_SERVICE_URL")
	if clipServiceURL == "" {
		clipServiceURL = "http://localhost:8092"
	}

	body, _ := json.Marshal(map[string]interface{}{
		"episode_id":     meta.EpisodeID,
		"shots_metadata": meta,
		"script_text":    req.ScriptText,
	})

	httpReq, err := http.NewRequestWithContext(
		c.Request.Context(),
		http.MethodPost,
		clipServiceURL+"/api/v1/clips/process",
		bytes.NewReader(body),
	)
	if err != nil {
		response.InternalError(c, "failed to build clip-service request")
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		h.logger.Error("call clip-service", zap.Error(err))
		response.InternalError(c, "clip-service unavailable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	var clipResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&clipResp); err != nil {
		response.InternalError(c, "failed to decode clip-service response")
		return
	}

	if resp.StatusCode >= 400 {
		response.InternalError(c, fmt.Sprintf("clip-service error %d", resp.StatusCode))
		return
	}

	response.OK(c, clipResp)
}
