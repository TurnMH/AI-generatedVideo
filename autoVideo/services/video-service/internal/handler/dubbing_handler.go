package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/autovideo/video-service/internal/model"
	"github.com/autovideo/video-service/internal/service"
	"github.com/autovideo/video-service/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// maxDubbingTextRunes is the per-request text size cap to prevent DoS via oversized payloads.
// At maxChunkRunes=800 this allows ~125 TTS chunks per task.
const maxDubbingTextRunes = 100_000

// DubbingHandler exposes endpoints for dubbing and subtitle generation.
type DubbingHandler struct {
	svc    *service.DubbingService
	logger *zap.Logger
}

// NewDubbingHandler —— 创建配音处理器实例，返回 *DubbingHandler
// NewDubbingHandler creates a new DubbingHandler.
func NewDubbingHandler(svc *service.DubbingService, logger *zap.Logger) *DubbingHandler {
	return &DubbingHandler{svc: svc, logger: logger}
}

type generateDubbingReq struct {
	EpisodeID       uint64 `json:"episode_id"`
	Text            string `json:"text"`
	VoiceModel      string `json:"voice_model"`
	VoiceRate       string `json:"voice_rate"`
	VoicePitch      string `json:"voice_pitch"`
	VoiceVolume     string `json:"voice_volume"`
	CustomAudioURL  string `json:"custom_audio_url"` // feat-7: bypass TTS entirely
}

type batchGenerateDubbingReq struct {
	Items []generateDubbingReq `json:"items" binding:"required,min=1"`
}

type batchTaskResult struct {
	EpisodeID uint64 `json:"episode_id"`
	TaskID    int64  `json:"task_id,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

func normalizeVoiceOptions(voiceModel, voiceRate, voicePitch, voiceVolume string) (string, string, string, string) {
	if voiceModel == "" {
		voiceModel = "default"
	}
	if voiceRate == "" {
		voiceRate = "+0%"
	}
	if voicePitch == "" {
		voicePitch = "+0Hz"
	}
	if voiceVolume == "" {
		voiceVolume = "+0%"
	}
	return voiceModel, voiceRate, voicePitch, voiceVolume
}

// GenerateDubbing —— 处理配音生成请求，异步创建任务并返回 task_id
// GenerateDubbing handles POST /api/v1/projects/:pid/dubbing/generate
// Now async: creates a task and returns task_id immediately.
func (h *DubbingHandler) GenerateDubbing(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req generateDubbingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// feat-7: custom audio bypasses TTS; text not required in that case
	if strings.TrimSpace(req.CustomAudioURL) == "" && strings.TrimSpace(req.Text) == "" {
		response.BadRequest(c, "text is required when custom_audio_url is not provided")
		return
	}
	if len([]rune(req.Text)) > maxDubbingTextRunes {
		response.BadRequest(c, fmt.Sprintf("text exceeds maximum length of %d characters", maxDubbingTextRunes))
		return
	}

	req.VoiceModel, req.VoiceRate, req.VoicePitch, req.VoiceVolume = normalizeVoiceOptions(req.VoiceModel, req.VoiceRate, req.VoicePitch, req.VoiceVolume)

	userID, _ := strconv.ParseInt(c.GetHeader("X-User-ID"), 10, 64)

	task := &model.DubbingTask{
		ProjectID:      pid,
		EpisodeID:      int64(req.EpisodeID),
		UserID:         userID,
		TaskType:       "dubbing",
		VoiceModel:     req.VoiceModel,
		VoiceRate:      req.VoiceRate,
		VoicePitch:     req.VoicePitch,
		VoiceVolume:    req.VoiceVolume,
		CustomAudioURL: strings.TrimSpace(req.CustomAudioURL),
	}

	if err := h.svc.CreateTask(c.Request.Context(), task, req.Text); err != nil {
		if errors.Is(err, service.ErrActiveTaskExists) {
			response.Fail(c, http.StatusConflict, http.StatusConflict, "an active dubbing task already exists for this episode")
			return
		}
		h.logger.Error("create dubbing task", zap.Error(err))
		response.InternalError(c, "failed to create task: "+err.Error())
		return
	}

	response.OK(c, gin.H{
		"message": "dubbing task created",
		"task_id": task.ID,
	})
}

// GenerateDubbingBatch —— 批量创建配音任务，减少前端一键生成时的突发请求
// GenerateDubbingBatch handles POST /api/v1/projects/:pid/dubbing/generate-batch
func (h *DubbingHandler) GenerateDubbingBatch(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req batchGenerateDubbingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID, _ := strconv.ParseInt(c.GetHeader("X-User-ID"), 10, 64)
	results := make([]batchTaskResult, 0, len(req.Items))
	created := 0
	conflicts := 0
	failed := 0

	for _, item := range req.Items {
		voiceModel, voiceRate, voicePitch, voiceVolume := normalizeVoiceOptions(item.VoiceModel, item.VoiceRate, item.VoicePitch, item.VoiceVolume)
		task := &model.DubbingTask{
			ProjectID:   pid,
			EpisodeID:   int64(item.EpisodeID),
			UserID:      userID,
			TaskType:    "dubbing",
			VoiceModel:  voiceModel,
			VoiceRate:   voiceRate,
			VoicePitch:  voicePitch,
			VoiceVolume: voiceVolume,
		}

		if err := h.svc.CreateTask(c.Request.Context(), task, item.Text); err != nil {
			if errors.Is(err, service.ErrActiveTaskExists) {
				conflicts++
				results = append(results, batchTaskResult{
					EpisodeID: item.EpisodeID,
					Status:    "conflict",
					Message:   "an active dubbing task already exists for this episode",
				})
				continue
			}
			failed++
			h.logger.Error("create batch dubbing task", zap.Int64("project_id", pid), zap.Uint64("episode_id", item.EpisodeID), zap.Error(err))
			results = append(results, batchTaskResult{
				EpisodeID: item.EpisodeID,
				Status:    "failed",
				Message:   err.Error(),
			})
			continue
		}

		created++
		results = append(results, batchTaskResult{
			EpisodeID: item.EpisodeID,
			TaskID:    task.ID,
			Status:    "created",
		})
	}

	response.OK(c, gin.H{
		"message":   "batch dubbing tasks processed",
		"created":   created,
		"conflicts": conflicts,
		"failed":    failed,
		"results":   results,
	})
}

type generateSubtitleReq struct {
	EpisodeID   uint64 `json:"episode_id"`
	Text        string `json:"text"`
	AudioURL    string `json:"audio_url"`
	VoiceModel  string `json:"voice_model"`
	VoiceRate   string `json:"voice_rate"`
	VoicePitch  string `json:"voice_pitch"`
	VoiceVolume string `json:"voice_volume"`
}

type batchGenerateSubtitleReq struct {
	Items []generateSubtitleReq `json:"items" binding:"required,min=1"`
}

type batchRetryTaskReq struct {
	Items []struct {
		TaskID int64  `json:"task_id" binding:"required"`
		Text   string `json:"text"`
	} `json:"items" binding:"required,min=1"`
}

// GenerateSubtitle —— 处理字幕生成请求，异步创建任务并返回 task_id
// GenerateSubtitle handles POST /api/v1/projects/:pid/subtitle/generate
// Now async: creates a task and returns task_id immediately.
func (h *DubbingHandler) GenerateSubtitle(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req generateSubtitleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.Text == "" && req.AudioURL == "" {
		response.BadRequest(c, "text or audio_url required")
		return
	}
	if len([]rune(req.Text)) > maxDubbingTextRunes {
		response.BadRequest(c, fmt.Sprintf("text exceeds maximum length of %d characters", maxDubbingTextRunes))
		return
	}

	text := req.Text
	if text == "" {
		text = "(从音频生成字幕暂未实现)"
	}
	req.VoiceModel, req.VoiceRate, req.VoicePitch, req.VoiceVolume = normalizeVoiceOptions(req.VoiceModel, req.VoiceRate, req.VoicePitch, req.VoiceVolume)

	userID, _ := strconv.ParseInt(c.GetHeader("X-User-ID"), 10, 64)

	task := &model.DubbingTask{
		ProjectID:   pid,
		EpisodeID:   int64(req.EpisodeID),
		UserID:      userID,
		TaskType:    "subtitle",
		VoiceModel:  req.VoiceModel,
		VoiceRate:   req.VoiceRate,
		VoicePitch:  req.VoicePitch,
		VoiceVolume: req.VoiceVolume,
	}

	if err := h.svc.CreateTask(c.Request.Context(), task, text); err != nil {
		if errors.Is(err, service.ErrActiveTaskExists) {
			response.Fail(c, http.StatusConflict, http.StatusConflict, "an active subtitle task already exists for this episode")
			return
		}
		h.logger.Error("create subtitle task", zap.Error(err))
		response.InternalError(c, "failed to create task: "+err.Error())
		return
	}

	response.OK(c, gin.H{
		"message": "subtitle task created",
		"task_id": task.ID,
	})
}

// GenerateSubtitleBatch —— 批量创建字幕任务，减少前端一键生成时的突发请求
// GenerateSubtitleBatch handles POST /api/v1/projects/:pid/subtitle/generate-batch
func (h *DubbingHandler) GenerateSubtitleBatch(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req batchGenerateSubtitleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID, _ := strconv.ParseInt(c.GetHeader("X-User-ID"), 10, 64)
	results := make([]batchTaskResult, 0, len(req.Items))
	created := 0
	conflicts := 0
	failed := 0

	for _, item := range req.Items {
		if item.Text == "" && item.AudioURL == "" {
			failed++
			results = append(results, batchTaskResult{
				EpisodeID: item.EpisodeID,
				Status:    "failed",
				Message:   "text or audio_url required",
			})
			continue
		}

		text := item.Text
		if text == "" {
			text = "(从音频生成字幕暂未实现)"
		}
		voiceModel, voiceRate, voicePitch, voiceVolume := normalizeVoiceOptions(item.VoiceModel, item.VoiceRate, item.VoicePitch, item.VoiceVolume)
		task := &model.DubbingTask{
			ProjectID:   pid,
			EpisodeID:   int64(item.EpisodeID),
			UserID:      userID,
			TaskType:    "subtitle",
			VoiceModel:  voiceModel,
			VoiceRate:   voiceRate,
			VoicePitch:  voicePitch,
			VoiceVolume: voiceVolume,
		}

		if err := h.svc.CreateTask(c.Request.Context(), task, text); err != nil {
			if errors.Is(err, service.ErrActiveTaskExists) {
				conflicts++
				results = append(results, batchTaskResult{
					EpisodeID: item.EpisodeID,
					Status:    "conflict",
					Message:   "an active subtitle task already exists for this episode",
				})
				continue
			}
			failed++
			h.logger.Error("create batch subtitle task", zap.Int64("project_id", pid), zap.Uint64("episode_id", item.EpisodeID), zap.Error(err))
			results = append(results, batchTaskResult{
				EpisodeID: item.EpisodeID,
				Status:    "failed",
				Message:   err.Error(),
			})
			continue
		}

		created++
		results = append(results, batchTaskResult{
			EpisodeID: item.EpisodeID,
			TaskID:    task.ID,
			Status:    "created",
		})
	}

	response.OK(c, gin.H{
		"message":   "batch subtitle tasks processed",
		"created":   created,
		"conflicts": conflicts,
		"failed":    failed,
		"results":   results,
	})
}

// ListTasks —— 查询指定项目下的配音/字幕任务列表，返回 JSON
// ListTasks handles GET /api/v1/projects/:pid/dubbing/tasks
func (h *DubbingHandler) ListTasks(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	tasks, err := h.svc.ListTasks(c.Request.Context(), pid)
	if err != nil {
		h.logger.Error("list dubbing tasks", zap.Error(err))
		response.InternalError(c, "failed to list tasks")
		return
	}

	response.OK(c, tasks)
}

// GetTask —— 根据任务 ID 查询单个配音/字幕任务详情，返回 JSON
// GetTask handles GET /api/v1/projects/:pid/dubbing/tasks/:tid
func (h *DubbingHandler) GetTask(c *gin.Context) {
	tid, err := pathInt64(c, "tid")
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}

	task, err := h.svc.GetTask(c.Request.Context(), tid)
	if err != nil {
		h.logger.Error("get dubbing task", zap.Error(err))
		response.InternalError(c, "task not found")
		return
	}

	response.OK(c, task)
}

// RetryTask —— 重试长时间卡住或失败的配音/字幕任务
func (h *DubbingHandler) RetryTask(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	tid, err := pathInt64(c, "tid")
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}

	var body struct {
		Text string `json:"text"`
	}
	_ = c.ShouldBindJSON(&body)

	task, err := h.svc.GetTask(c.Request.Context(), tid)
	if err != nil || task.ProjectID != pid {
		response.NotFound(c, "task not found")
		return
	}

	newTask, err := h.svc.RetryTask(c.Request.Context(), tid, body.Text)
	if err != nil {
		if errors.Is(err, service.ErrTaskRetryNotAllowed) {
			response.Fail(c, http.StatusConflict, http.StatusConflict, "task is not eligible for retry")
			return
		}
		if errors.Is(err, service.ErrActiveTaskExists) {
			response.Fail(c, http.StatusConflict, http.StatusConflict, "an active task already exists for this episode")
			return
		}
		h.logger.Error("retry dubbing task", zap.Error(err))
		response.InternalError(c, "failed to retry task: "+err.Error())
		return
	}

	response.OK(c, gin.H{
		"message": "task retried",
		"task_id": newTask.ID,
	})
}

// RetryTasksBatch —— 批量重试长时间卡住或失败的配音/字幕任务
// POST /api/v1/projects/:pid/dubbing/tasks/retry-batch
func (h *DubbingHandler) RetryTasksBatch(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	var req batchRetryTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	type retryTaskBatchResult struct {
		TaskID    int64  `json:"task_id"`
		NewTaskID int64  `json:"new_task_id,omitempty"`
		Status    string `json:"status"`
		Message   string `json:"message,omitempty"`
		EpisodeID int64  `json:"episode_id,omitempty"`
		TaskType  string `json:"task_type,omitempty"`
	}

	results := make([]retryTaskBatchResult, 0, len(req.Items))
	retried := 0
	conflicts := 0
	failed := 0

	for _, item := range req.Items {
		task, err := h.svc.GetTask(c.Request.Context(), item.TaskID)
		if err != nil || task.ProjectID != pid {
			failed++
			results = append(results, retryTaskBatchResult{
				TaskID:  item.TaskID,
				Status:  "failed",
				Message: "task not found",
			})
			continue
		}

		newTask, err := h.svc.RetryTask(c.Request.Context(), item.TaskID, item.Text)
		if err != nil {
			if errors.Is(err, service.ErrTaskRetryNotAllowed) {
				failed++
				results = append(results, retryTaskBatchResult{
					TaskID:    item.TaskID,
					Status:    "failed",
					Message:   "task is not eligible for retry",
					EpisodeID: task.EpisodeID,
					TaskType:  task.TaskType,
				})
				continue
			}
			if errors.Is(err, service.ErrActiveTaskExists) {
				conflicts++
				results = append(results, retryTaskBatchResult{
					TaskID:    item.TaskID,
					Status:    "conflict",
					Message:   "an active task already exists for this episode",
					EpisodeID: task.EpisodeID,
					TaskType:  task.TaskType,
				})
				continue
			}
			failed++
			h.logger.Error("retry batch dubbing task", zap.Int64("task_id", item.TaskID), zap.Error(err))
			results = append(results, retryTaskBatchResult{
				TaskID:    item.TaskID,
				Status:    "failed",
				Message:   err.Error(),
				EpisodeID: task.EpisodeID,
				TaskType:  task.TaskType,
			})
			continue
		}

		retried++
		results = append(results, retryTaskBatchResult{
			TaskID:    item.TaskID,
			NewTaskID: newTask.ID,
			Status:    "retried",
			EpisodeID: task.EpisodeID,
			TaskType:  task.TaskType,
		})
	}

	response.OK(c, gin.H{
		"message":   "batch retry processed",
		"retried":   retried,
		"conflicts": conflicts,
		"failed":    failed,
		"results":   results,
	})
}

// DeleteProjectData —— 删除项目下所有配音/字幕任务数据
// DELETE /api/v1/projects/:pid/dubbing/runtime-data
func (h *DubbingHandler) DeleteProjectData(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	if err := h.svc.DeleteProjectData(c.Request.Context(), pid); err != nil {
		h.logger.Error("delete project dubbing runtime data", zap.Int64("project_id", pid), zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// GenerateStoryboardDubbing —— 为单个分镜生成语音，异步创建任务
// POST /api/v1/projects/:pid/storyboards/:sid/dubbing
func (h *DubbingHandler) GenerateStoryboardDubbing(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}
	sid, err := pathInt64(c, "sid")
	if err != nil {
		response.BadRequest(c, "invalid storyboard id")
		return
	}

	var req struct {
		EpisodeID  uint64 `json:"episode_id" binding:"required"`
		Text       string `json:"text"`
		VoiceModel string `json:"voice_model"`
		VoiceRate  string `json:"voice_rate"`
		VoicePitch string `json:"voice_pitch"`
		VoiceVolume string `json:"voice_volume"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		response.BadRequest(c, "text is required")
		return
	}
	if len([]rune(req.Text)) > maxDubbingTextRunes {
		response.BadRequest(c, fmt.Sprintf("text exceeds maximum length of %d characters", maxDubbingTextRunes))
		return
	}

	req.VoiceModel, req.VoiceRate, req.VoicePitch, req.VoiceVolume = normalizeVoiceOptions(req.VoiceModel, req.VoiceRate, req.VoicePitch, req.VoiceVolume)
	userID, _ := strconv.ParseInt(c.GetHeader("X-User-ID"), 10, 64)

	task := &model.DubbingTask{
		ProjectID:    pid,
		EpisodeID:    int64(req.EpisodeID),
		StoryboardID: &sid,
		UserID:       userID,
		TaskType:     "dubbing",
		VoiceModel:   req.VoiceModel,
		VoiceRate:    req.VoiceRate,
		VoicePitch:   req.VoicePitch,
		VoiceVolume:  req.VoiceVolume,
	}

	if err := h.svc.CreateStoryboardTask(c.Request.Context(), task, req.Text); err != nil {
		if errors.Is(err, service.ErrActiveTaskExists) {
			response.Fail(c, http.StatusConflict, http.StatusConflict, "an active dubbing task already exists for this storyboard")
			return
		}
		h.logger.Error("create storyboard dubbing task", zap.Error(err))
		response.InternalError(c, "failed to create task: "+err.Error())
		return
	}

	response.OK(c, gin.H{
		"message": "storyboard dubbing task created",
		"task_id": task.ID,
	})
}

// ListStoryboardTasks —— 返回项目下所有分镜配音任务（每个分镜最新一条）
// GET /api/v1/projects/:pid/dubbing/storyboard-tasks
func (h *DubbingHandler) ListStoryboardTasks(c *gin.Context) {
	pid, err := pathInt64(c, "pid")
	if err != nil {
		response.BadRequest(c, "invalid project id")
		return
	}

	tasks, err := h.svc.ListStoryboardTasks(c.Request.Context(), pid)
	if err != nil {
		h.logger.Error("list storyboard dubbing tasks", zap.Error(err))
		response.InternalError(c, "failed to list tasks")
		return
	}

	response.OK(c, tasks)
}

// ListVoices —— 返回所有支持的 TTS 音色列表（供前端动态渲染选项）
// GET /api/v1/voices
func (h *DubbingHandler) ListVoices(c *gin.Context) {
	type voiceEntry struct {
		Key   string `json:"key"`
		Name  string `json:"name"`
		Label string `json:"label"`
	}
	// Convert EdgeVoices map to ordered list; well-known keys first
	orderedKeys := []string{
		"default", "male1", "male2", "male3",
		"female1", "female2",
		"dialect", "dialect-northeast", "dialect-shaanxi",
		"cantonese-female1", "cantonese-female2", "cantonese-male1",
		"taiwan-female1", "taiwan-female2", "taiwan-male1",
	}
	voiceLabelMap := map[string]string{
		"default":           "云剑 (男·激昂)",
		"male1":             "云剑 (男·激昂)",
		"male2":             "云希 (男·活泼)",
		"male3":             "云扬 (男·专业)",
		"female1":           "晓晓 (女·温暖)",
		"female2":           "晓伊 (女·活泼)",
		"dialect":           "晓北 (东北方言)",
		"dialect-northeast": "晓北 (东北方言)",
		"dialect-shaanxi":   "晓妮 (陕西方言)",
		"cantonese-female1": "晓佳 (粤语·女)",
		"cantonese-female2": "晓曼 (粤语·女)",
		"cantonese-male1":   "云龙 (粤语·男)",
		"taiwan-female1":    "晓臻 (台湾普通话·女)",
		"taiwan-female2":    "晓雨 (台湾普通话·女)",
		"taiwan-male1":      "云哲 (台湾普通话·男)",
	}

	seen := make(map[string]bool)
	result := make([]voiceEntry, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		if azureName, ok := service.EdgeVoices[key]; ok && !seen[key] {
			seen[key] = true
			label := voiceLabelMap[key]
			if label == "" {
				label = azureName
			}
			result = append(result, voiceEntry{Key: key, Name: azureName, Label: label})
		}
	}
	// Append any keys in EdgeVoices not covered above
	for key, azureName := range service.EdgeVoices {
		if !seen[key] {
			label := voiceLabelMap[key]
			if label == "" {
				label = azureName
			}
			result = append(result, voiceEntry{Key: key, Name: azureName, Label: label})
		}
	}
	response.OK(c, gin.H{"voices": result})
}
