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

func renderConfigStringSlice(rc model.RenderConfig, key string, n int) []string {
	result := make([]string, n)
	if len(rc) == 0 {
		return result
	}
	raw, ok := rc[key]
	if !ok {
		return result
	}
	switch v := raw.(type) {
	case []string:
		for i := 0; i < n && i < len(v); i++ {
			result[i] = v[i]
		}
	case []interface{}:
		for i := 0; i < n && i < len(v); i++ {
			if s, ok := v[i].(string); ok {
				result[i] = s
			}
		}
	}
	return result
}

func renderConfigStringValues(rc model.RenderConfig, key string) []string {
	if len(rc) == 0 {
		return nil
	}
	raw, ok := rc[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func renderConfigBoolValue(rc model.RenderConfig, key string) bool {
	if len(rc) == 0 {
		return false
	}
	raw, ok := rc[key]
	if !ok {
		return false
	}
	v, _ := raw.(bool)
	return v
}

func normalizeContinuityRenderConfig(rc model.RenderConfig, spatialAnchors, subjectPositions, transitionNotes []string, sceneCharacters [][]string) model.RenderConfig {
	rc = normalizeRenderConfig(rc)
	if len(spatialAnchors) > 0 {
		rc["spatial_anchors"] = spatialAnchors
	}
	if len(subjectPositions) > 0 {
		rc["subject_positions"] = subjectPositions
	}
	if len(transitionNotes) > 0 {
		rc["transition_notes"] = transitionNotes
	}
	if len(sceneCharacters) > 0 {
		rc["scene_characters"] = sceneCharacters
	}
	return rc
}

func applyCharacterIdentityConfig(rc model.RenderConfig, enabled bool, requireSameCharacter bool, anchorAssetID int64, anchorImageURL string, anchorSource string, identityConstraints []string, sameCharacterAsFirstScene bool) model.RenderConfig {
	rc = normalizeRenderConfig(rc)
	rc["character_consistency_enabled"] = enabled
	rc["require_same_character"] = requireSameCharacter
	if anchorAssetID > 0 {
		rc["character_anchor_asset_id"] = anchorAssetID
	}
	if trimmed := strings.TrimSpace(anchorImageURL); trimmed != "" {
		rc["character_anchor_image_url"] = trimmed
	}
	if trimmed := strings.TrimSpace(anchorSource); trimmed != "" {
		rc["character_anchor_source"] = trimmed
	}
	if len(identityConstraints) > 0 {
		rc["identity_constraints"] = identityConstraints
	}
	rc["same_character_as_first_scene"] = sameCharacterAsFirstScene
	return rc
}

func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func buildNativeAudioSubmissionText(prompt, voiceText string, generateAudio bool) string {
	prompt = strings.TrimSpace(prompt)
	voiceText = strings.TrimSpace(voiceText)
	if !generateAudio || voiceText == "" {
		return prompt
	}
	if prompt == "" {
		return voiceText
	}
	if strings.Contains(prompt, voiceText) {
		return prompt
	}
	return prompt + "\n\n[Audio dialogue / narration]\n" + voiceText
}

func buildTaskSubmissionPreview(task *model.VideoTask) gin.H {
	if task == nil {
		return gin.H{}
	}
	visualTexts := renderConfigStringValues(task.RenderConfig, "scene_descriptions")
	if len(visualTexts) == 0 {
		if visual := strings.TrimSpace(firstNonEmptyString(
			fmt.Sprintf("%v", task.RenderConfig["scene_description"]),
			task.SceneDescription,
		)); visual != "" && visual != "%!v(<nil>)" {
			visualTexts = []string{visual}
		}
	}
	dialogueTexts := renderConfigStringValues(task.RenderConfig, "dialogues")
	if len(dialogueTexts) == 0 {
		if subtitle := strings.TrimSpace(task.SubtitleText); subtitle != "" {
			dialogueTexts = []string{subtitle}
		}
	}
	generateAudio := renderConfigBoolValue(task.RenderConfig, "generate_audio")
	clipCount := len(task.Clips)
	if clipCount == 0 {
		clipCount = 1
	}
	if len(visualTexts) > clipCount {
		clipCount = len(visualTexts)
	}
	if len(dialogueTexts) > clipCount {
		clipCount = len(dialogueTexts)
	}
	items := make([]gin.H, 0, clipCount)
	for i := 0; i < clipCount; i++ {
		visualText := ""
		if i < len(visualTexts) {
			visualText = strings.TrimSpace(visualTexts[i])
		}
		voiceText := ""
		if i < len(dialogueTexts) {
			voiceText = strings.TrimSpace(dialogueTexts[i])
		}
		if voiceText == "" {
			voiceText = strings.TrimSpace(task.SubtitleText)
		}
		items = append(items, gin.H{
			"clip_order":              i,
			"visual_prompt":           visualText,
			"voice_text":              voiceText,
			"actual_submission_text":  buildNativeAudioSubmissionText(visualText, voiceText, generateAudio),
			"generate_audio":          generateAudio,
			"native_audio_model_hint": firstNonEmptyString(task.RoutedGenerator, task.RequestedModel, task.ModelName),
		})
	}
	return gin.H{
		"generate_audio": generateAudio,
		"strategy":       "current-native-audio-implementation-appends-dialogue-into-content-text",
		"note":           "当前实现里，native audio 文本并无独立 provider 字段证据；video-service 会把对白/旁白并入 content.text。模型可能据此生成口播，但不保证逐字朗读。",
		"items":          items,
	}
}

func buildClipDebugSummaries(task *model.VideoTask) []gin.H {
	if task == nil || len(task.Clips) == 0 {
		return nil
	}
	clipCount := len(task.Clips)
	spatialAnchors := renderConfigStringSlice(task.RenderConfig, "spatial_anchors", clipCount)
	subjectPositions := renderConfigStringSlice(task.RenderConfig, "subject_positions", clipCount)
	transitionNotes := renderConfigStringSlice(task.RenderConfig, "transition_notes", clipCount)
	items := make([]gin.H, 0, clipCount)
	for i := range task.Clips {
		clip := task.Clips[i]
		item := gin.H{
			"clip_order":        clip.ClipOrder,
			"requested_model":   clip.RequestedModel,
			"routed_generator":  clip.RoutedGenerator,
			"runtime_provider":  clip.RuntimeProvider,
			"effective_model":   clip.EffectiveModel,
			"scene_group_key":   clip.SceneGroupKey,
			"scene_seq":         clip.SceneSeq,
			"spatial_anchor":    "",
			"subject_positions": "",
			"transition_note":   "",
		}
		if clip.ClipOrder >= 0 && clip.ClipOrder < len(spatialAnchors) {
			item["spatial_anchor"] = strings.TrimSpace(spatialAnchors[clip.ClipOrder])
		}
		if clip.ClipOrder >= 0 && clip.ClipOrder < len(subjectPositions) {
			item["subject_positions"] = strings.TrimSpace(subjectPositions[clip.ClipOrder])
		}
		if clip.ClipOrder >= 0 && clip.ClipOrder < len(transitionNotes) {
			item["transition_note"] = strings.TrimSpace(transitionNotes[clip.ClipOrder])
		}
		items = append(items, item)
	}
	return items
}

func buildTaskDebugSummary(task *model.VideoTask) gin.H {
	if task == nil {
		return gin.H{}
	}
	summary := gin.H{
		"requested_model":  task.RequestedModel,
		"routed_generator": task.RoutedGenerator,
		"runtime_provider": task.RuntimeProvider,
		"effective_model":  task.EffectiveModel,
		"route_reason":     task.RouteReason,
	}
	if len(task.Clips) > 0 {
		clipMissingRoute := 0
		clipWithSpatialHints := 0
		clipWithPositionHints := 0
		clipWithTransitionHints := 0
		spatialAnchors := renderConfigStringSlice(task.RenderConfig, "spatial_anchors", len(task.Clips))
		subjectPositions := renderConfigStringSlice(task.RenderConfig, "subject_positions", len(task.Clips))
		transitionNotes := renderConfigStringSlice(task.RenderConfig, "transition_notes", len(task.Clips))
		for i := range task.Clips {
			clip := task.Clips[i]
			if strings.TrimSpace(clip.RequestedModel) == "" || strings.TrimSpace(clip.RoutedGenerator) == "" || strings.TrimSpace(clip.RuntimeProvider) == "" {
				clipMissingRoute++
			}
			if i < len(spatialAnchors) && strings.TrimSpace(spatialAnchors[i]) != "" {
				clipWithSpatialHints++
			}
			if i < len(subjectPositions) && strings.TrimSpace(subjectPositions[i]) != "" {
				clipWithPositionHints++
			}
			if i < len(transitionNotes) && strings.TrimSpace(transitionNotes[i]) != "" {
				clipWithTransitionHints++
			}
		}
		summary["clip_count"] = len(task.Clips)
		summary["clip_missing_route"] = clipMissingRoute
		summary["clip_with_spatial_hints"] = clipWithSpatialHints
		summary["clip_with_position_hints"] = clipWithPositionHints
		summary["clip_with_transition_hints"] = clipWithTransitionHints
	}
	return summary
}

func logTaskRouteState(logger *zap.Logger, event string, task *model.VideoTask) {
	if logger == nil || task == nil {
		return
	}
	logger.Info(event,
		zap.Int64("task_id", task.ID),
		zap.Int64("project_id", task.ProjectID),
		zap.Any("episode_id", task.EpisodeID),
		zap.String("status", task.Status),
		zap.String("requested_model", task.RequestedModel),
		zap.String("routed_generator", task.RoutedGenerator),
		zap.String("runtime_provider", task.RuntimeProvider),
		zap.String("effective_model", task.EffectiveModel),
		zap.String("route_reason", task.RouteReason),
	)
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

type routeCoverageSummary struct {
	TaskMissingRoute bool `json:"task_missing_route"`
	ClipTotal        int  `json:"clip_total"`
	ClipMissingRoute int  `json:"clip_missing_route"`
}

func summarizeRouteCoverage(task *model.VideoTask) routeCoverageSummary {
	summary := routeCoverageSummary{}
	if task == nil {
		return summary
	}
	if strings.TrimSpace(task.RequestedModel) == "" || strings.TrimSpace(task.RoutedGenerator) == "" || strings.TrimSpace(task.RuntimeProvider) == "" {
		summary.TaskMissingRoute = true
	}
	summary.ClipTotal = len(task.Clips)
	for i := range task.Clips {
		clip := task.Clips[i]
		if strings.TrimSpace(clip.RequestedModel) == "" || strings.TrimSpace(clip.RoutedGenerator) == "" || strings.TrimSpace(clip.RuntimeProvider) == "" {
			summary.ClipMissingRoute++
		}
	}
	return summary
}

type generateReq struct {
	ProjectID                  int64              `json:"project_id" binding:"required"`
	EpisodeID                  *int64             `json:"episode_id"`
	ImageURLs                  []string           `json:"image_urls"`
	SceneDescriptions          []string           `json:"scene_descriptions"` // per-clip descriptions, parallel to image_urls
	Dialogues                  []string           `json:"dialogues"`          // per-clip dialogue / subtitle lines
	MotionDescs                []string           `json:"motion_descs"`       // opt-p7: per-clip camera/motion from storyboard
	SpatialAnchors             []string           `json:"spatial_anchors"`
	SubjectPositions           []string           `json:"subject_positions"`
	TransitionNotes            []string           `json:"transition_notes"`
	SceneCharacters            [][]string         `json:"scene_characters"`
	StylePreset                string             `json:"style_preset"`
	MotionMode                 string             `json:"motion_mode"`
	ModelName                  string             `json:"model_name"`
	AudioURL                   string             `json:"audio_url"`
	SubtitleText               string             `json:"subtitle_text"`
	SceneDescription           string             `json:"scene_description"`
	RenderConfig               model.RenderConfig `json:"render_config"`
	ClipDurationSec            float64            `json:"clip_duration_sec"` // desired clip duration from project storyboard_config
	SerialScene                bool               `json:"serial_scene"`
	SceneGroupKeys             []string           `json:"scene_group_keys"`
	CharacterConsistencyEnabled bool              `json:"character_consistency_enabled"`
	RequireSameCharacter       bool               `json:"require_same_character"`
	CharacterAnchorAssetID     int64              `json:"character_anchor_asset_id"`
	CharacterAnchorImageURL    string             `json:"character_anchor_image_url"`
	CharacterAnchorSource      string             `json:"character_anchor_source"`
	IdentityConstraints        []string           `json:"identity_constraints"`
	SameCharacterAsFirstScene  bool               `json:"same_character_as_first_scene"`
}

type extractVideoContentReq struct {
	VideoURL  string `json:"video_url" binding:"required"`
	Language  string `json:"language"`
	OnlyAudio bool   `json:"only_audio"`
}

type composeAdVideoReq struct {
	TaskIDs []int64 `json:"task_ids"`
}

func supportsTextOnlyVideoRequest(modelName string, renderConfig model.RenderConfig) bool {
	trimmed := strings.ToLower(strings.TrimSpace(modelName))
	if strings.Contains(trimmed, "wan-t2v") || strings.Contains(trimmed, "t2v") {
		return true
	}
	if trimmed == "vidu" || trimmed == "vidu-offpeak" {
		raw := renderConfig["generate_mode"]
		mode, _ := raw.(string)
		return strings.EqualFold(strings.TrimSpace(mode), "text2video")
	}
	return false
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
	if len(req.ImageURLs) == 0 && !supportsTextOnlyVideoRequest(req.ModelName, req.RenderConfig) {
		response.BadRequest(c, "image_urls is required unless model supports text-to-video")
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
	req.StylePreset = setDefault(req.StylePreset, "anime-2d")
	req.MotionMode = setDefault(req.MotionMode, "gentle")
	req.ModelName = setDefault(req.ModelName, "kling")

	// Store per-clip scene descriptions in render_config for use during generation.
	req.RenderConfig = normalizeContinuityRenderConfig(req.RenderConfig, req.SpatialAnchors, req.SubjectPositions, req.TransitionNotes, req.SceneCharacters)
	req.RenderConfig = applyCharacterIdentityConfig(req.RenderConfig, req.CharacterConsistencyEnabled, req.RequireSameCharacter, req.CharacterAnchorAssetID, req.CharacterAnchorImageURL, req.CharacterAnchorSource, req.IdentityConstraints, req.SameCharacterAsFirstScene)
	if len(req.Dialogues) == 0 && strings.TrimSpace(req.SubtitleText) != "" {
		for _, line := range strings.Split(strings.ReplaceAll(req.SubtitleText, "\r\n", "\n"), "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				req.Dialogues = append(req.Dialogues, trimmed)
			}
		}
	}
	if len(req.Dialogues) > 0 {
		req.RenderConfig["dialogues"] = req.Dialogues
		if strings.TrimSpace(req.SubtitleText) == "" {
			req.SubtitleText = strings.Join(req.Dialogues, "\n")
		}
	}
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
		RequestedModel:   req.ModelName,
		SceneDescription: req.SceneDescription,
		RenderConfig:     req.RenderConfig,
		DurationSec:      req.ClipDurationSec,
		Status:           model.StatusPending,
		SerialScene:      req.SerialScene,
		SceneGroupKeys:   model.StringArray(req.SceneGroupKeys),
		RoutedGenerator:  "pending-resolve",
		RuntimeProvider:  "pending-resolve",
		EffectiveModel:   "pending-resolve",
		RouteReason:      "request-accepted",
	}

	h.logger.Info("video task accepted",
		zap.Int64("project_id", req.ProjectID),
		zap.Any("episode_id", req.EpisodeID),
		zap.String("requested_model", req.ModelName),
		zap.String("style_preset", req.StylePreset),
		zap.String("motion_mode", req.MotionMode),
		zap.Bool("serial_scene", req.SerialScene),
		zap.Int("image_count", len(req.ImageURLs)),
	)

	ctx := c.Request.Context()
	if err := h.svc.CreateTask(ctx, task); err != nil {
		h.logger.Error("create task", zap.Error(err))
		response.InternalError(c, "failed to create task")
		return
	}

	response.OK(c, gin.H{"task_id": task.ID})
}

// ExtractVideoContent handles POST /api/v1/projects/:pid/videos/content-extract
// MVP: extract one key frame from video and run visual text/content extraction.
func (h *VideoHandler) ExtractVideoContent(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req extractVideoContentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := mustUserID(c)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	result, err := h.svc.ExtractVideoContent(ctx, pid, userID, req.VideoURL, req.Language, req.OnlyAudio)
	if err != nil {
		h.logger.Error("extract video content failed",
			zap.Int64("project_id", pid),
			zap.String("video_url", req.VideoURL),
			zap.Error(err))
		response.Fail(c, http.StatusBadRequest, http.StatusBadRequest, err.Error())
		return
	}

	response.OK(c, result)
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
	logTaskRouteState(h.logger, "video task detail fetched", task)
	response.OK(c, gin.H{
		"task":               task,
		"task_debug_summary": buildTaskDebugSummary(task),
		"clips_debug":        buildClipDebugSummaries(task),
		"submission_preview": buildTaskSubmissionPreview(task),
	})
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
	if h.logger != nil {
		missingRoute := 0
		for i := range tasks {
			if strings.TrimSpace(tasks[i].RequestedModel) == "" || strings.TrimSpace(tasks[i].RoutedGenerator) == "" || strings.TrimSpace(tasks[i].RuntimeProvider) == "" {
				missingRoute++
			}
		}
		h.logger.Info("video tasks listed",
			zap.Int64("project_id", projectID),
			zap.Int64("episode_id", episodeID),
			zap.Int64("total", total),
			zap.Int("page", page),
			zap.Int("page_size", pageSize),
			zap.Int("items", len(tasks)),
			zap.Int("missing_route_items", missingRoute),
		)
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

// ComposeAdVideo merges multiple existing task result videos into one ad video.
// POST /api/v1/videos/ad-compose
func (h *VideoHandler) ComposeAdVideo(c *gin.Context) {
	var req composeAdVideoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(req.TaskIDs) == 0 {
		response.BadRequest(c, "task_ids is required")
		return
	}
	resultTask, meta, err := h.svc.ComposeAdVideo(c.Request.Context(), mustUserID(c), req.TaskIDs)
	if err != nil {
		h.logger.Error("compose ad video", zap.Int64s("task_ids", req.TaskIDs), zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{
		"task_ids":   req.TaskIDs,
		"task_id":    resultTask.ID,
		"task":       resultTask,
		"result_url": resultTask.ResultURL,
		"meta":       meta,
	})
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
	missingRoute := 0
	for i := range tasks {
		if strings.TrimSpace(tasks[i].RequestedModel) == "" || strings.TrimSpace(tasks[i].RoutedGenerator) == "" || strings.TrimSpace(tasks[i].RuntimeProvider) == "" {
			missingRoute++
		}
	}
	items := make([]gin.H, 0, len(tasks))
	for i := range tasks {
		items = append(items, gin.H{
			"task":               tasks[i],
			"task_debug_summary": buildTaskDebugSummary(&tasks[i]),
			"clips_debug":        buildClipDebugSummaries(&tasks[i]),
		})
	}
	response.OK(c, gin.H{
		"total":               total,
		"page":                page,
		"page_size":           pageSize,
		"items":               items,
		"missing_route_items": missingRoute,
	})
}

type projectGenerateReq struct {
	EpisodeID                   *int64             `json:"episode_id"`
	ImageURLs                   []string           `json:"image_urls" binding:"required,min=1"`
	SceneDescriptions           []string           `json:"scene_descriptions"` // per-clip visual descriptions
	Dialogues                   []string           `json:"dialogues"`          // per-clip dialogue/subtitle text
	Durations                   []float64          `json:"durations"`          // per-clip duration in seconds (from storyboard)
	CameraMovements             []string           `json:"camera_movements"`   // per-clip camera movement hint
	Moods                       []string           `json:"moods"`              // per-clip mood/emotion
	SpatialAnchors              []string           `json:"spatial_anchors"`
	SubjectPositions            []string           `json:"subject_positions"`
	TransitionNotes             []string           `json:"transition_notes"`
	SceneCharacters             [][]string         `json:"scene_characters"` // per-clip character names for ref image filtering
	SceneAssetIDs               [][]int64          `json:"scene_asset_ids"`  // per-clip related asset IDs for scene/prop continuity
	StylePreset                 string             `json:"style_preset"`
	MotionMode                  string             `json:"motion_mode"`
	ModelName                   string             `json:"model_name"`
	AudioURL                    string             `json:"audio_url"`
	SubtitleText                string             `json:"subtitle_text"`
	VideoMode                   string             `json:"video_mode"`
	ExportFormat                string             `json:"export_format"`
	SceneDescription            string             `json:"scene_description"`
	RenderConfig                model.RenderConfig `json:"render_config"`
	ClipDurationSec             float64            `json:"clip_duration_sec"`
	CharacterConsistencyEnabled bool               `json:"character_consistency_enabled"`
	RequireSameCharacter        bool               `json:"require_same_character"`
	CharacterAnchorAssetID      int64              `json:"character_anchor_asset_id"`
	CharacterAnchorImageURL     string             `json:"character_anchor_image_url"`
	CharacterAnchorSource       string             `json:"character_anchor_source"`
	IdentityConstraints         []string           `json:"identity_constraints"`
	SameCharacterAsFirstScene   bool               `json:"same_character_as_first_scene"`
	// 视频串行生成
	SerialScene    bool     `json:"serial_scene"`     // true = 同场景分镜串行生成（末帧约束）
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

	req.RenderConfig = normalizeContinuityRenderConfig(req.RenderConfig, req.SpatialAnchors, req.SubjectPositions, req.TransitionNotes, req.SceneCharacters)
	req.RenderConfig = applyCharacterIdentityConfig(req.RenderConfig, req.CharacterConsistencyEnabled, req.RequireSameCharacter, req.CharacterAnchorAssetID, req.CharacterAnchorImageURL, req.CharacterAnchorSource, req.IdentityConstraints, req.SameCharacterAsFirstScene)
	if len(req.SceneDescriptions) > 0 {
		req.RenderConfig["scene_descriptions"] = req.SceneDescriptions
	}
	if len(req.Dialogues) > 0 {
		req.RenderConfig["dialogues"] = req.Dialogues
		if strings.TrimSpace(req.SubtitleText) == "" {
			req.SubtitleText = strings.Join(req.Dialogues, "\n")
		}
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

	h.logger.Info("video task accepted",
		zap.Int64("project_id", pid),
		zap.Any("episode_id", req.EpisodeID),
		zap.String("requested_model", req.ModelName),
		zap.String("style_preset", req.StylePreset),
		zap.String("motion_mode", req.MotionMode),
		zap.Bool("serial_scene", req.SerialScene),
		zap.Int("image_count", len(req.ImageURLs)),
	)

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
		SpatialAnchors    []string   `json:"spatial_anchors"`
		SubjectPositions  []string   `json:"subject_positions"`
		TransitionNotes   []string   `json:"transition_notes"`
		SceneCharacters   [][]string `json:"scene_characters"` // per-clip character names
		SceneAssetIDs     [][]int64  `json:"scene_asset_ids"`  // per-clip related asset IDs
		AudioURL          string     `json:"audio_url"`
		SceneDescription  string     `json:"scene_description"`
		SceneGroupKeys    []string   `json:"scene_group_keys"` // 串行模式：每 clip 的场景 key
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
		SerialScene     bool               `json:"serial_scene"` // 串行模式
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
		rc = normalizeContinuityRenderConfig(rc, ep.SpatialAnchors, ep.SubjectPositions, ep.TransitionNotes, ep.SceneCharacters)
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
			SubtitleText:     strings.Join(ep.Dialogues, "\n"),
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

	response.OK(c, gin.H{"task_id": vid, "status": model.StatusPending})
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
