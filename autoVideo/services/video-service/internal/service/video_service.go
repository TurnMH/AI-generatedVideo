package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/autovideo/video-service/internal/model"
	"github.com/autovideo/video-service/internal/repository"
	"github.com/autovideo/video-service/internal/service/generators"
	"github.com/autovideo/video-service/internal/stylepreset"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// VideoService orchestrates video task creation and processing.
type VideoService struct {
	repo             repository.VideoTaskRepo
	ffmpeg           *FFmpegService
	generators       map[string]generators.VideoGenerator
	storageURL       string
	characterBaseURL string
	logger           *zap.Logger
	kafkaWriter      *kafka.Writer
	kafkaTopic       string
	maxClips         int
	localMaxClips    int                  // clip concurrency for local GPU models (comfyui-video)
	motionPromptSvc       *MotionPromptService    // opt-motion-llm: LLM-based motion prompt refinement
	dubbing               *DubbingService         // per-clip TTS for clip-aligned audio composition
	frameExtractorURL     string                  // http://localhost:8010 — 末帧提取服务（视频串行流程）
	serialFailureAnalyzer *SerialFailureAnalyzer  // AI 串行失败诊断（分析失败原因并给出优化建议）
}

// NewVideoService —— 创建视频服务实例，注册生成器并设置并发限制
func NewVideoService(
	repo repository.VideoTaskRepo,
	ffmpeg *FFmpegService,
	gens []generators.VideoGenerator,
	storageURL string,
	characterBaseURL string,
	logger *zap.Logger,
	maxClips int,
	localMaxClips int,
) *VideoService {
	genMap := make(map[string]generators.VideoGenerator, len(gens))
	for _, g := range gens {
		genMap[g.Name()] = g
	}
	if maxClips <= 0 {
		maxClips = 10
	}
	if localMaxClips <= 0 {
		localMaxClips = 1
	}
	return &VideoService{
		repo:             repo,
		ffmpeg:           ffmpeg,
		generators:       genMap,
		storageURL:       storageURL,
		characterBaseURL: characterBaseURL,
		logger:           logger,
		maxClips:         maxClips,
		localMaxClips:    localMaxClips,
	}
}

// SetKafkaWriter —— 配置 Kafka 生产者，用于分发任务
// SetKafkaWriter configures the Kafka producer for dispatching tasks.
func (s *VideoService) SetKafkaWriter(brokers []string, topic string) {
	s.kafkaTopic = topic
	s.kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
}

// SetMotionPromptService wires in the optional LLM-based motion prompt refiner.
// When set, ProcessTask will call RefineBatch before building per-clip prompts.
func (s *VideoService) SetMotionPromptService(svc *MotionPromptService) {
	s.motionPromptSvc = svc
}

// SetDubbingService wires in the dubbing service so the video composition path
// can synthesize per-clip audio aligned to each storyboard clip, rather than
// attaching a single merged audio track to the fully concatenated video.
func (s *VideoService) SetDubbingService(d *DubbingService) {
	s.dubbing = d
}

// CloseKafka —— 关闭 Kafka writer 释放资源
// CloseKafka releases the Kafka writer.
func (s *VideoService) CloseKafka() error {
	if s.kafkaWriter != nil {
		return s.kafkaWriter.Close()
	}
	return nil
}

// DispatchTask —— 将视频任务发布到 Kafka 进行异步处理
// DispatchTask publishes a task to Kafka for async processing.
func (s *VideoService) DispatchTask(ctx context.Context, task *model.VideoTask) error {
	if s.kafkaWriter == nil {
		return fmt.Errorf("kafka writer not configured")
	}
	// opt-p7: extract stored motion_descs from render_config
	var motionDescs []string
	if raw, ok := task.RenderConfig["motion_descs"]; ok {
		switch v := raw.(type) {
		case []string:
			motionDescs = v
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					motionDescs = append(motionDescs, s)
				}
			}
		}
	}
	msg := KafkaMessage{
		VideoTaskID: task.ID,
		ImageURLs:   []string(task.ImageURLs),
		ModelName:   task.ModelName,
		MotionMode:  task.MotionMode,
		StylePreset: task.StylePreset,
		MotionDescs: motionDescs,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal kafka message: %w", err)
	}
	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.kafkaWriter.WriteMessages(pubCtx, kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", task.ID)),
		Value: b,
	})
}

// CreateTask —— 持久化视频任务到数据库，然后分发到 Kafka
// CreateTask persists a new VideoTask, then dispatches it to Kafka for processing.
func (s *VideoService) CreateTask(ctx context.Context, task *model.VideoTask) error {
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return err
	}
	if err := s.DispatchTask(ctx, task); err != nil {
		s.logger.Error("dispatch task to kafka failed, task stays pending",
			zap.Int64("task_id", task.ID), zap.Error(err))
	}
	return nil
}

// SetVariantGroupID —— 为多版本任务设置 variant_group_id（feat-6）
// SetVariantGroupID assigns a shared group ID to a set of variant tasks.
func (s *VideoService) SetVariantGroupID(ctx context.Context, taskIDs []int64, groupID int64) error {
	return s.repo.SetVariantGroupID(ctx, taskIDs, groupID)
}

// GetTask —— 根据 ID 查询视频任务及其片段，返回 *VideoTask
// GetTask returns the task with its clips pre-loaded.
func (s *VideoService) GetTask(ctx context.Context, id int64) (*model.VideoTask, error) {
	return s.repo.GetTask(ctx, id)
}

// ListTasks —— 分页查询视频任务列表，返回任务切片和总数
// ListTasks returns a paginated list of tasks filtered by project/episode.
func (s *VideoService) ListTasks(ctx context.Context, projectID, episodeID int64, page, pageSize int) ([]model.VideoTask, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListTasks(ctx, projectID, episodeID, page, pageSize)
}

// DeleteTask —— 软删除视频任务；处理中或排队中的任务先置为 cancelled 再删除
// DeleteTask soft-deletes a video task.
func (s *VideoService) DeleteTask(ctx context.Context, taskID int64) error {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}
	// Cancel active tasks before deleting so polling loops stop retrying.
	if task.Status == model.StatusProcessing || task.Status == model.StatusPending {
		if cancelErr := s.repo.UpdateTaskStatus(ctx, taskID, model.StatusCancelled, "", "user cancelled", 0); cancelErr != nil {
			s.logger.Warn("failed to cancel task before delete", zap.Int64("task_id", taskID), zap.Error(cancelErr))
		}
	}
	return s.repo.SoftDeleteTask(ctx, taskID)
}

// DeleteProjectData —— 删除项目下所有视频与配音相关任务数据
func (s *VideoService) DeleteProjectData(ctx context.Context, projectID int64) error {
	return s.repo.DeleteProjectData(ctx, projectID)
}

// DeleteEpisodeData —— 删除指定剧集下所有视频任务、片段和配音任务，用于剧集删除时级联清理
func (s *VideoService) DeleteEpisodeData(ctx context.Context, projectID, episodeID int64) error {
	return s.repo.DeleteEpisodeData(ctx, projectID, episodeID)
}

// GetClipsByEpisode —— 查询指定剧集下所有已生成的 VideoClip（按 clip_order 排序）
func (s *VideoService) GetClipsByEpisode(ctx context.Context, projectID, episodeID int64) ([]model.VideoClip, error) {
	return s.repo.GetClipsByEpisode(ctx, projectID, episodeID)
}

// PauseTask —— 将任务从处理中或待处理状态切换为暂停
// PauseTask transitions a task from processing to paused.
func (s *VideoService) PauseTask(ctx context.Context, taskID int64) error {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}
	if task.Status != model.StatusProcessing && task.Status != model.StatusPending {
		return fmt.Errorf("cannot pause task in status %q", task.Status)
	}
	return s.repo.UpdateTaskStatus(ctx, taskID, model.StatusPaused, "", "", 0)
}

// ResumeTask —— 将暂停的任务恢复为处理中状态
// ResumeTask transitions a task from paused back to pending.
func (s *VideoService) ResumeTask(ctx context.Context, taskID int64) error {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}
	if task.Status != model.StatusPaused {
		return fmt.Errorf("cannot resume task in status %q", task.Status)
	}
	return s.repo.UpdateTaskStatus(ctx, taskID, model.StatusProcessing, "", "", 0)
}

// ProcessTask —— 驱动完整的视频生成流水线：生成片段、拼接、添加音频字幕并上传
// ProcessTask drives the full generation pipeline for a task.
// It is called by the Kafka consumer (or the manual compose endpoint).
// motionDescs (opt-p7) carries per-clip camera/motion descriptions from the storyboard,
// parallel to imageURLs. Pass nil when not available.
func (s *VideoService) ProcessTask(ctx context.Context, taskID int64, imageURLs []string, motionDescs []string, modelName, motionMode, stylePreset string) error {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	resolvedModelName := task.ModelName
	if modelName != "" {
		resolvedModelName = modelName
	}
	resolvedMotionMode := task.MotionMode
	if motionMode != "" {
		resolvedMotionMode = motionMode
	}
	resolvedStylePreset := task.StylePreset
	if stylePreset != "" {
		resolvedStylePreset = stylePreset
	}

	// imageURLs 来自 Kafka 消息；若 Kafka 消息丢失或反序列化出空列表，回退到 DB 存储的 task.ImageURLs，
	// 避免因消息异常导致建 0 个 clip 而直接失败。
	if len(imageURLs) == 0 && len(task.ImageURLs) > 0 {
		imageURLs = []string(task.ImageURLs)
		s.logger.Warn("kafka imageURLs empty, falling back to DB task.ImageURLs",
			zap.Int64("task_id", taskID),
			zap.Int("db_image_count", len(imageURLs)))
	}

	// Mark task as processing
	if err := s.repo.UpdateTaskStatus(ctx, taskID, model.StatusProcessing, "", "", 0); err != nil {
		return err
	}

	gen, err := s.resolveGenerator(ctx, resolvedModelName)
	if err != nil {
		s.markFailed(ctx, taskID, "no available video generator")
		return fmt.Errorf("no generator available for model %q", resolvedModelName)
	}

	// Fetch character reference images for subject consistency, with name→URL map for
	// per-scene filtering so each clip only receives refs for characters actually in it.
	characterImageURLs, characterImagesByName := s.fetchCharacterImageMap(ctx, task.ProjectID)

	// Fetch character appearance descriptions for prompt consistency (char-c3).
	charDescriptions := s.fetchCharacterDescriptions(ctx, task.ProjectID)

	// Extract per-clip scene descriptions from render_config.
	perClipDescs := extractSceneDescriptions(task.RenderConfig, len(imageURLs))
	// Extract per-clip dialogue text (for VoiceText and subtitle fallback).
	perClipDialogues := extractDialogues(task.RenderConfig, len(imageURLs))
	// Extract per-clip duration, camera movement, mood, and character names from render_config.
	perClipDurations := extractDurations(task.RenderConfig, len(imageURLs))
	perClipCameras := extractStringSlice(task.RenderConfig, "camera_movements", len(imageURLs))
	perClipMoods := extractStringSlice(task.RenderConfig, "moods", len(imageURLs))
	perClipSceneChars := extractSceneCharacters(task.RenderConfig, len(imageURLs))
	perClipAssetIDs := extractInt64Matrix(task.RenderConfig, "scene_asset_ids", len(imageURLs))
	assetAnchors := s.fetchAssetPromptAnchors(ctx, task.ProjectID, collectUniqueInt64(perClipAssetIDs))
	// opt-p7: merge Kafka-provided motion descriptions (storyboard camera text) into
	// per-clip prompts, overriding RenderConfig entries when present.
	for i, md := range motionDescs {
		if i < len(perClipDescs) && strings.TrimSpace(md) != "" {
			if perClipDescs[i] == "" {
				perClipDescs[i] = md
			} else {
				perClipDescs[i] = perClipDescs[i] + ". " + md
			}
		}
	}

	// opt-motion-llm: LLM pass — replace rule-based motion prompts with holistic,
	// scene-aware camera directions that consider the full clip sequence.
	if s.motionPromptSvc != nil {
		family := videoModelFamily(resolvedModelName)
		if refined := s.motionPromptSvc.RefineBatch(
			ctx,
			perClipDescs,
			family,
			resolvedMotionMode,
			resolvedStylePreset,
			charDescriptions,
		); len(refined) > 0 {
			perClipDescs = refined
		}
	}

	// Delete stale clips from any previous attempt
	if err := s.repo.DeleteClipsByTaskID(ctx, taskID); err != nil {
		s.logger.Warn("delete stale clips", zap.Error(err))
	}
	s.logger.Info("video prompt context prepared",
		zap.Int64("task_id", taskID),
		zap.Int("clip_count", len(imageURLs)),
		zap.Int("scene_description_count", countNonEmptyStrings(perClipDescs)),
		zap.Int("scene_character_count", countNonEmptyStringSets(perClipSceneChars)),
		zap.Int("scene_asset_anchor_count", len(assetAnchors)),
		zap.Int("character_reference_count", len(characterImageURLs)),
		zap.Int("render_config_version", renderConfigInt(task.RenderConfig, "config_version")),
	)

	// Create clip records
	// 如果是串行模式，从 task.SceneGroupKeys 填充每个 clip 的场景分组和组内序号。
	sceneGroupKeys := []string(task.SceneGroupKeys)
	sceneSeqCounter := map[string]int{}
	var clips []*model.VideoClip
	for i, imgURL := range imageURLs {
		sgKey := ""
		if i < len(sceneGroupKeys) {
			sgKey = sceneGroupKeys[i]
		}
		sceneSeq := 0
		if sgKey != "" && task.SerialScene {
			sceneSeq = sceneSeqCounter[sgKey]
			sceneSeqCounter[sgKey]++
		}
		clip := &model.VideoClip{
			VideoTaskID:    taskID,
			ClipOrder:      i,
			SourceImageURL: imgURL,
			SceneGroupKey:  sgKey,
			SceneSeq:       sceneSeq,
			Status:         model.StatusPending,
		}
		if err := s.repo.CreateClip(ctx, clip); err != nil {
			return fmt.Errorf("create clip record: %w", err)
		}
		clips = append(clips, clip)
	}

	// ─── 辅助函数：为单个 clip 构建 VideoGenerateReq 并执行生成 ─────────────────
	// maxAttempts: 串行模式传 3，并行模式传 6
	buildAndGenClip := func(c *model.VideoClip, overrideTailURL string, maxAttempts int) error {
		clipCharURLs := perSceneCharacterImages(characterImageURLs, characterImagesByName, clipScenePersons(perClipSceneChars, c.ClipOrder))
		clipAssetRefs := perClipAssetReferenceImages(perClipAssetIDs, assetAnchors, c.ClipOrder)
		assetAnchorHint := perClipAssetAnchorHint(perClipAssetIDs, assetAnchors, c.ClipOrder)
		prompt := appendAssetAnchorHint(appendCharacterSheetHint(clipMotionPromptWithHints(
			c.ClipOrder, len(clips), perClipDescs,
			resolvedMotionMode, resolvedStylePreset, resolvedModelName,
			task.RenderConfig, charDescriptions,
			perClipCameras, perClipMoods,
		), clipCharURLs, resolvedModelName), assetAnchorHint)
		// 串行模式非首帧：注入连续性提示，告知视频模型这是同一场景的接续片段。
		if task.SerialScene && c.SceneSeq > 0 && c.SourceImageURL != "" {
			prompt = "Seamless continuation from previous scene clip. Maintain visual continuity with identical character appearance, lighting, and environment. " + prompt
		}
		tailURL := tailImageURL(imageURLs, c.ClipOrder)
		if overrideTailURL != "" {
			tailURL = overrideTailURL
		}
		genReq := generators.VideoGenerateReq{
			SourceImageURL:     c.SourceImageURL,
			TailImageURL:       tailURL,
			CharacterImageURLs: clipCharURLs,
			ClipIndex:          c.ClipOrder,
			TotalClips:         len(clips),
			Prompt:             prompt,
			NegativePrompt:     describeVideoNegativePrompt(resolvedStylePreset),
			StylePreset:        resolvedStylePreset,
			MotionMode:         resolvedMotionMode,
			DurationSec: snapDurationToModel(
				perClipDurationSec(perClipDurations, c.ClipOrder, task.DurationSec),
				videoModelFamily(resolvedModelName),
			),
			VoiceText: func() string {
				if !gen.SupportsNativeAudio() {
					return ""
				}
				if c.ClipOrder < len(perClipDialogues) && perClipDialogues[c.ClipOrder] != "" {
					return perClipDialogues[c.ClipOrder]
				}
				return task.SubtitleText
			}(),
			Resolution:      renderConfigString(task.RenderConfig, "resolution"),
			AspectRatio:     renderConfigString(task.RenderConfig, "aspect_ratio"),
			MotionAmplitude: renderConfigString(task.RenderConfig, "motion_amplitude"),
			VideoMode:       renderConfigString(task.RenderConfig, "video_mode"),
			Count:           renderConfigInt(task.RenderConfig, "count"),
			GenerateMode:    renderConfigString(task.RenderConfig, "generate_mode"),
			GenerateAudio:   renderConfigBool(task.RenderConfig, "generate_audio"),
		}
		if endURL := renderConfigString(task.RenderConfig, "end_image_url"); endURL != "" {
			genReq.TailImageURL = endURL
		}
		// 串行模式非首帧：SourceImageURL 来自前一 clip 末帧（动态），不应被全局 start_image_url 覆盖。
		// start_image_url 是用户为整条任务指定的固定首帧，仅适用于第一个 clip 或并行模式。
		if startURL := renderConfigString(task.RenderConfig, "start_image_url"); startURL != "" {
			if !task.SerialScene || c.SceneSeq == 0 {
				genReq.SourceImageURL = startURL
			}
		}
		genReq = normalizeVideoGenerateReq(gen, resolvedModelName, genReq, clipAssetRefs)
		result, err := generateClipWithRetry(ctx, gen, genReq, maxAttempts, s.logger, c.ID)
		if err != nil {
			c.Status = model.StatusFailed
			c.ErrorMsg = err.Error()
			return err
		}
		c.ClipURL = result.ClipURL
		c.DurationSec = result.DurationSec
		c.ModelUsed = result.ModelUsed
		c.Status = model.StatusSucceeded
		return nil
	}

	var mu sync.Mutex
	var failCount int

	if task.SerialScene && len(sceneGroupKeys) == len(clips) {
		// ── 串行模式：同场景 clips 按序生成，每次成功后提取末帧作为下一个 clip 的首帧 ──
		// 1. 按 SceneGroupKey 分组（保留顺序）
		//    注意：scene_group_key 为空的 clip 不属于任何串行组，每个单独成一组（并行生成）。
		type sceneGroup struct {
			key   string
			clips []*model.VideoClip
		}
		var groups []sceneGroup
		keyToIdx := map[string]int{}
		for _, c := range clips {
			k := c.SceneGroupKey
			if k == "" {
				// 无场景分组的 clip：独立生成，不与其他空 key clip 串联
				groups = append(groups, sceneGroup{key: fmt.Sprintf("__solo_%d__", c.ClipOrder), clips: []*model.VideoClip{c}})
			} else if idx, ok := keyToIdx[k]; ok {
				groups[idx].clips = append(groups[idx].clips, c)
			} else {
				keyToIdx[k] = len(groups)
				groups = append(groups, sceneGroup{key: k, clips: []*model.VideoClip{c}})
			}
		}

		// 2. 各场景组之间并行，组内串行
		// 用信号量限制同时进行 API 调用的场景组数，与并行模式的 maxClips/localMaxClips 保持一致，
		// 避免场景组数量很多时同时打出过多请求导致限速或 OOM。
		groupSlots := s.maxClips
		if gen.Name() == "comfyui-video" {
			groupSlots = s.localMaxClips
		}
		groupSem := make(chan struct{}, groupSlots)
		var wg sync.WaitGroup
		for _, grp := range groups {
			wg.Add(1)
			go func(g sceneGroup) {
				defer wg.Done()
				groupSem <- struct{}{}
				defer func() { <-groupSem }()
				var prevEndFrameURL string
				for _, c := range g.clips {
					// 除了第一个 clip，用前一个 clip 的末帧作为首帧
					if prevEndFrameURL != "" {
						c.SourceImageURL = prevEndFrameURL
					}
					// 串行模式：最多重试 3 次（含首次）
					genErr := buildAndGenClip(c, "", 3)
					saveCtx, saveCancel := context.WithTimeout(context.Background(), 15*time.Second)
					_ = s.repo.UpdateClip(saveCtx, c)
					saveCancel()
					if genErr != nil {
						s.logger.Error("serial clip generation failed after 3 attempts, stopping chain",
							zap.Int64("clip_id", c.ID),
							zap.String("scene_group", g.key),
							zap.Error(genErr))

						// AI 诊断：分析失败原因并给出优化建议
						if s.serialFailureAnalyzer != nil {
							groupDescs := make([]string, len(g.clips))
							for i, gc := range g.clips {
								if gc.ClipOrder < len(perClipDescs) {
									groupDescs[i] = perClipDescs[gc.ClipOrder]
								}
							}
							analyzeCtx, analyzeCancel := context.WithTimeout(context.Background(), 60*time.Second)
							analysis, analysisErr := s.serialFailureAnalyzer.Analyze(
								analyzeCtx,
								g.key,
								c.SceneSeq,
								groupDescs,
								genErr.Error(),
								resolvedModelName,
							)
							analyzeCancel()
							if analysisErr != nil {
								s.logger.Warn("serial failure analyzer error",
									zap.Int64("clip_id", c.ID),
									zap.Error(analysisErr))
							} else if analysis != nil {
								analysisBytes, _ := json.Marshal(analysis)
								c.ChainFailureAnalysis = string(analysisBytes)
								s.logger.Info("serial failure analysis stored",
									zap.Int64("clip_id", c.ID),
									zap.String("reason", analysis.Reason))
								saveCtx2, saveCancel2 := context.WithTimeout(context.Background(), 10*time.Second)
								_ = s.repo.UpdateClip(saveCtx2, c)
								saveCancel2()
							}
						}

						mu.Lock()
						failCount++
						mu.Unlock()

						// 将本组剩余 clips 标记为失败（串行链已断）
						foundFailed := false
						for _, remaining := range g.clips {
							if remaining.ID == c.ID {
								foundFailed = true
								continue
							}
							if !foundFailed {
								continue
							}
							remaining.Status = model.StatusFailed
							remaining.ErrorMsg = fmt.Sprintf("serial chain broken at clip %d (scene_seq=%d)", c.ID, c.SceneSeq)
							rmCtx, rmCancel := context.WithTimeout(context.Background(), 10*time.Second)
							_ = s.repo.UpdateClip(rmCtx, remaining)
							rmCancel()
							mu.Lock()
							failCount++
							mu.Unlock()
						}
						return
					}
					s.logger.Info("serial clip generation succeeded",
						zap.Int64("clip_id", c.ID),
						zap.String("scene_group", g.key),
						zap.Int("scene_seq", c.SceneSeq),
						zap.Int("group_size", len(g.clips)),
						zap.String("clip_url", c.ClipURL))
					// 提取末帧 URL（如果配置了 frame-extractor-service）
					if s.frameExtractorURL != "" && c.ClipURL != "" {
						frameURL, err := callFrameExtractor(ctx, c.ClipURL, task.ProjectID, task.UserID, s.frameExtractorURL)
						if err != nil {
							s.logger.Warn("frame extractor failed, falling back to clip source image as chain anchor",
								zap.Int64("clip_id", c.ID), zap.Error(err))
							// 提取失败时退而用本 clip 的首帧图作为下一个 clip 的锚点，
							// 避免后续非首帧 clip 因 SourceImageURL 为空而报错中断串行链。
							if c.SourceImageURL != "" {
								prevEndFrameURL = c.SourceImageURL
							}
							// 若 prevEndFrameURL 已有历史成功提取的帧（来自更早的 clip），
							// 则保留旧值比用空值更好，故仅在当前 prevEndFrameURL 为空时才更新。
							// 上方的 if 已处理此逻辑：只有 c.SourceImageURL != "" 时才覆盖。
						} else {
							prevEndFrameURL = frameURL
							// 保存末帧 URL 到 clip 记录
							c.EndFrameImageURL = frameURL
							saveCtx2, saveCancel2 := context.WithTimeout(context.Background(), 10*time.Second)
							_ = s.repo.UpdateClip(saveCtx2, c)
							saveCancel2()
						}
					} else if c.SourceImageURL != "" && prevEndFrameURL == "" {
						// 未配置 frame-extractor 时，用本 clip 首帧作为下一 clip 的锚点，
						// 至少保证非首帧 clip 有一个视觉参考起点。
						prevEndFrameURL = c.SourceImageURL
					}
				}
			}(grp)
		}
		wg.Wait()
	} else {
		// ── 原有并发模式 ─────────────────────────────────────────────────────────
		// 如果 serial_scene=true 但 scene_group_keys 长度与 clips 不一致，则静默降级为并行。
		// 此时非首帧 clip 的 SourceImageURL 为空，可能导致生成失败，记录警告便于排查。
		if task.SerialScene {
			s.logger.Warn("serial_scene=true but scene_group_keys length mismatch, falling back to parallel mode",
				zap.Int64("task_id", taskID),
				zap.Int("scene_group_keys_count", len(sceneGroupKeys)),
				zap.Int("clips_count", len(clips)))
		}
		// Generate clips concurrently.
		// Local GPU models (comfyui-video) use a tighter limit to avoid VRAM exhaustion.
		clipSlots := s.maxClips
		if gen.Name() == "comfyui-video" {
			clipSlots = s.localMaxClips
		}
		sem := make(chan struct{}, clipSlots)
		var wg sync.WaitGroup

		for _, clip := range clips {
			wg.Add(1)
		go func(c *model.VideoClip) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				genErr := buildAndGenClip(c, "", 6)
				if genErr != nil {
					s.logger.Error("clip generation failed",
						zap.Int64("clip_id", c.ID),
						zap.Error(genErr))
					mu.Lock()
					failCount++
					mu.Unlock()
				}
				saveCtx, saveCancel := context.WithTimeout(context.Background(), 15*time.Second)
				if err := s.repo.UpdateClip(saveCtx, c); err != nil {
					s.logger.Error("update clip failed", zap.Error(err))
				}
				saveCancel()
			}(clip)
		}
		wg.Wait()
	} // end else (concurrent mode)

	if failCount == len(clips) {
		s.markFailed(ctx, taskID, "all clips failed")
		return fmt.Errorf("task %d: all clips failed", taskID)
	}

	// Collect succeeded clip URLs in order (skip failed)
	var clipURLs []string
	var totalDur float64
	for _, c := range clips {
		if c.Status == model.StatusSucceeded && c.ClipURL != "" {
			clipURLs = append(clipURLs, c.ClipURL)
			totalDur += c.DurationSec
		}
	}

	if len(clipURLs) == 0 {
		s.markFailed(ctx, taskID, "no succeeded clips")
		return fmt.Errorf("task %d: no succeeded clips", taskID)
	}

	s.logger.Info("clips generation done",
		zap.Int64("task_id", taskID),
		zap.Int("succeeded", len(clipURLs)),
		zap.Int("failed", failCount),
		zap.Int("total", len(clips)))

	// Concatenate clips (aspect-ratio-aware)
	_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageConcating)
	transition, _ := task.RenderConfig["transition"].(string)
	transitionDur, _ := task.RenderConfig["transition_duration"].(float64)
	// Default to dissolve crossfade when user hasn't explicitly set a transition.
	// "none" opts out; anything else uses xfade.
	if transition == "" && len(clipURLs) > 1 {
		transition = "dissolve"
	}
	if transition == "none" {
		transition = ""
	}
	if transitionDur <= 0 && transition != "" {
		transitionDur = 0.5
	}

	// Manual override: when the generator embeds native audio, skip external
	// dubbing attachment by default. Users can opt-in by setting
	// render_config.attach_dubbing=true to mix TTS on top of native audio.
	attachDubbing := renderConfigBool(task.RenderConfig, "attach_dubbing")
	nativeAudio := gen.SupportsNativeAudio()
	allowDubbingAttach := !nativeAudio || attachDubbing

	// Per-clip audio synthesis path — produce TTS per clip, mux each with its
	// storyboard video, then concat. This eliminates the dialogue↔frame drift
	// that the legacy "merge-all-audio-then-attach" flow could introduce.
	mergedPath := ""
	perClipAudioUsed := false
	if s.dubbing != nil && allowDubbingAttach && hasAnyNonEmpty(perClipDialogues) && len(perClipDialogues) >= len(clipURLs) {
		mergedPath, perClipAudioUsed = s.tryPerClipAudioCompose(
			ctx, task, clipURLs, perClipDialogues, transition, transitionDur,
		)
	}

	if !perClipAudioUsed {
		var err error
		mergedPath, err = s.ffmpeg.ConcatClipsWithTransitions(ctx, clipURLs, task.VideoMode, transition, transitionDur)
		if err != nil {
			_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageNone)
			s.markFailed(ctx, taskID, err.Error())
			return err
		}
	}

	finalPath := mergedPath

	// Attach dubbing audio — skip when the video model already embeds native audio.
	audioURL := task.AudioURL
	subtitleText := task.SubtitleText
	// If no explicit subtitle text, build from per-clip dialogues.
	if subtitleText == "" {
		subtitleText = joinDialogues(perClipDialogues)
	}
	subtitleURL := "" // VTT subtitle URL from dubbing task
	if nativeAudio && !attachDubbing {
		s.logger.Info("skipping dubbing attachment: model supports native audio (set render_config.attach_dubbing=true to override)",
			zap.Int64("task_id", taskID),
			zap.String("model", gen.Name()))
	} else if perClipAudioUsed {
		s.logger.Info("skipping dubbing attachment: per-clip audio already muxed",
			zap.Int64("task_id", taskID))
		// Still pick up the stored VTT (if any) so subtitles can be burnt below.
		if subtitleText == "" {
			if _, dubSub := s.repo.FindDubbingAudio(ctx, task.ProjectID, task.EpisodeID); dubSub != "" {
				subtitleURL = dubSub
			}
		}
	} else {
		if audioURL == "" {
			if dubAudio, dubSub := s.repo.FindDubbingAudio(ctx, task.ProjectID, task.EpisodeID); dubAudio != "" {
				audioURL = dubAudio
				// dubSub is a VTT URL, not plain text
				if subtitleText == "" && dubSub != "" {
					subtitleURL = dubSub
				}
				s.logger.Info("auto-attached dubbing audio",
					zap.Int64("task_id", taskID),
					zap.String("audio_url", dubAudio))
			}
		}

		// Add audio if available
		if audioURL != "" {
			_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageAudio)
			// Probe durations for drift diagnostics — helps debugging 配音与视频输出对应不上.
			if vDur, vErr := s.ffmpeg.ProbeDuration(ctx, finalPath); vErr == nil && vDur > 0 {
				if aPath, aErr := downloadToTemp(ctx, s.ffmpeg.TempDir, audioURL); aErr == nil {
					if aDur, aErr2 := s.ffmpeg.ProbeDuration(ctx, aPath); aErr2 == nil && aDur > 0 {
						drift := vDur - aDur
						if drift < 0 {
							drift = -drift
						}
						if drift > 2.0 {
							s.logger.Warn("video/audio duration drift >2s — final output may look out of sync, audio will be silence-padded if shorter",
								zap.Int64("task_id", taskID),
								zap.Float64("video_seconds", vDur),
								zap.Float64("audio_seconds", aDur),
								zap.Float64("drift_seconds", drift))
						} else {
							s.logger.Info("video/audio duration check",
								zap.Int64("task_id", taskID),
								zap.Float64("video_seconds", vDur),
								zap.Float64("audio_seconds", aDur))
						}
					}
					_ = os.Remove(aPath)
				}
			}
			withAudio, err := s.ffmpeg.AddAudio(ctx, finalPath, audioURL)
			if err != nil {
				_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageNone)
				s.markFailed(ctx, taskID, err.Error())
				return fmt.Errorf("attach audio: %w", err)
			}
			finalPath = withAudio
		}
	}

	// Mix BGM if configured
	if bgmURL, _ := task.RenderConfig["bgm_url"].(string); bgmURL != "" {
		bgmVolume, _ := task.RenderConfig["bgm_volume"].(float64)
		if p, err := s.ffmpeg.AddBGM(ctx, finalPath, bgmURL, bgmVolume); err == nil {
			finalPath = p
		} else {
			s.logger.Warn("add bgm failed, continuing without bgm", zap.Error(err))
		}
	}

	// Burn subtitles: prefer timed VTT URL over plain text
	_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageSubtitle)
	subtitleStyle := parseSubtitleStyle(task.RenderConfig)
	if subtitleURL != "" {
		if p, err := s.ffmpeg.AddSubtitleFromVTTWithStyle(ctx, finalPath, subtitleURL, subtitleStyle); err == nil {
			finalPath = p
		} else {
			s.logger.Warn("add vtt subtitle failed", zap.Error(err))
		}
	} else if subtitleText != "" {
		if p, err := s.ffmpeg.AddSubtitleWithStyle(ctx, finalPath, subtitleText, subtitleStyle); err == nil {
			finalPath = p
		} else {
			s.logger.Warn("add subtitle failed, continuing without subtitles", zap.Error(err))
		}
	}

	// Upload final video to storage-service
	_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageUploading)
	resultURL, err := s.uploadVideo(ctx, taskID, task.ProjectID, finalPath)
	if err != nil {
		_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageNone)
		s.markFailed(ctx, taskID, err.Error())
		return err
	}

	// Clean up temp files
	go os.RemoveAll(filepath.Dir(mergedPath))

	_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageDone)
	finalDuration := totalDur
	if probedDuration, probeErr := s.ffmpeg.ProbeDuration(ctx, finalPath); probeErr == nil && probedDuration > 0 {
		finalDuration = probedDuration
	}
	return s.repo.UpdateTaskStatus(ctx, taskID, model.StatusSucceeded, resultURL, "", finalDuration)
}

// ComposeTask —— 对已有片段重新执行 FFmpeg 合成流程
// ComposeTask re-runs FFmpeg composition for an existing task whose clips are already generated.
func (s *VideoService) ComposeTask(ctx context.Context, taskID int64) error {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	clips, err := s.repo.GetClipsByTaskID(ctx, taskID)
	if err != nil {
		return err
	}

	var clipURLs []string
	var totalDur float64
	for _, c := range clips {
		if c.Status == model.StatusSucceeded && c.ClipURL != "" {
			clipURLs = append(clipURLs, c.ClipURL)
			totalDur += c.DurationSec
		}
	}
	if len(clipURLs) == 0 {
		return fmt.Errorf("no succeeded clips to compose for task %d", taskID)
	}

	// Mark as processing + track compose stages
	_ = s.repo.UpdateTaskStatus(ctx, taskID, model.StatusProcessing, "", "", 0)

	_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageConcating)
	transition2, _ := task.RenderConfig["transition"].(string)
	transitionDur2, _ := task.RenderConfig["transition_duration"].(float64)
	if transition2 == "" && len(clipURLs) > 1 {
		transition2 = "dissolve"
	}
	if transition2 == "none" {
		transition2 = ""
	}
	if transitionDur2 <= 0 && transition2 != "" {
		transitionDur2 = 0.5
	}
	mergedPath, err := s.ffmpeg.ConcatClipsWithTransitions(ctx, clipURLs, task.VideoMode, transition2, transitionDur2)
	if err != nil {
		_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageNone)
		s.markFailed(ctx, taskID, err.Error())
		return err
	}

	finalPath := mergedPath

	// Resolve generator to check for native audio support.
	composeGen, _ := s.resolveGenerator(ctx, task.ModelName)

	// Attach dubbing audio — skip when the video model already embeds native audio.
	audioURL := task.AudioURL
	subtitleText := task.SubtitleText
	// If no explicit subtitle text, build from per-clip dialogues stored in render_config.
	if subtitleText == "" {
		subtitleText = joinDialogues(extractDialogues(task.RenderConfig, len(clipURLs)))
	}
	// Manual override: attach_dubbing=true lets users mix external dubbing
	// even when the video model already embeds native audio.
	composeAttachDubbing := renderConfigBool(task.RenderConfig, "attach_dubbing")
	composeNativeAudio := composeGen != nil && composeGen.SupportsNativeAudio()

	subtitleURL2 := ""
	if composeNativeAudio && !composeAttachDubbing {
		s.logger.Info("compose: skipping dubbing attachment: model supports native audio (set render_config.attach_dubbing=true to override)",
			zap.Int64("task_id", taskID),
			zap.String("model", composeGen.Name()))
	} else {
		if audioURL == "" {
			if dubAudio, dubSub := s.repo.FindDubbingAudio(ctx, task.ProjectID, task.EpisodeID); dubAudio != "" {
				audioURL = dubAudio
				if subtitleText == "" && dubSub != "" {
					subtitleURL2 = dubSub
				}
				s.logger.Info("compose: auto-attached dubbing audio",
					zap.Int64("task_id", taskID),
					zap.String("audio_url", dubAudio))
			}
		}

		if audioURL != "" {
			_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageAudio)
			finalPath, err = s.ffmpeg.AddAudio(ctx, finalPath, audioURL)
			if err != nil {
				_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageNone)
				s.markFailed(ctx, taskID, err.Error())
				return fmt.Errorf("attach audio: %w", err)
			}
		}
	}

	// Mix BGM if configured
	if bgmURL2, _ := task.RenderConfig["bgm_url"].(string); bgmURL2 != "" {
		bgmVolume2, _ := task.RenderConfig["bgm_volume"].(float64)
		if p, err := s.ffmpeg.AddBGM(ctx, finalPath, bgmURL2, bgmVolume2); err == nil {
			finalPath = p
		} else {
			s.logger.Warn("compose: add bgm failed", zap.Error(err))
		}
	}

	// Burn subtitles: prefer timed VTT URL over plain text
	_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageSubtitle)
	subtitleStyle2 := parseSubtitleStyle(task.RenderConfig)
	if subtitleURL2 != "" {
		if p, err := s.ffmpeg.AddSubtitleFromVTTWithStyle(ctx, finalPath, subtitleURL2, subtitleStyle2); err == nil {
			finalPath = p
		} else {
			s.logger.Warn("compose: add vtt subtitle failed", zap.Error(err))
		}
	} else if subtitleText != "" {
		if p, err := s.ffmpeg.AddSubtitleWithStyle(ctx, finalPath, subtitleText, subtitleStyle2); err == nil {
			finalPath = p
		}
	}

	// opt-p2: apply frame template overlays (title/caption/watermark/logo) if configured
	if tpl := parseFrameTemplate(task.RenderConfig); tpl != nil {
		tplPath := strings.TrimSuffix(finalPath, filepath.Ext(finalPath)) + "_tpl.mp4"
		if err := s.ffmpeg.ApplyFrameTemplate(ctx, finalPath, *tpl, tplPath); err != nil {
			s.logger.Warn("compose: apply frame template failed", zap.Error(err))
		} else {
			finalPath = tplPath
		}
	}

	_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageUploading)
	resultURL, err := s.uploadVideo(ctx, taskID, task.ProjectID, finalPath)
	if err != nil {
		_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageNone)
		s.markFailed(ctx, taskID, err.Error())
		return err
	}
	go os.RemoveAll(filepath.Dir(mergedPath))
	_ = s.repo.UpdateComposeStage(ctx, taskID, model.ComposeStageDone)
	finalDuration := totalDur
	if probedDuration, probeErr := s.ffmpeg.ProbeDuration(ctx, finalPath); probeErr == nil && probedDuration > 0 {
		finalDuration = probedDuration
	}
	return s.repo.UpdateTaskStatus(ctx, taskID, model.StatusSucceeded, resultURL, "", finalDuration)
}

// RetryClip regenerates a single failed clip and recomposes the task when possible.
func (s *VideoService) RetryClip(ctx context.Context, projectID, taskID, clipID int64, modelName string) error {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.ProjectID != projectID {
		return fmt.Errorf("task %d does not belong to project %d", taskID, projectID)
	}
	if task.Status == model.StatusProcessing || task.Status == model.StatusPending {
		return fmt.Errorf("task %d is already running", taskID)
	}

	var clip *model.VideoClip
	for i := range task.Clips {
		if task.Clips[i].ID == clipID {
			clip = &task.Clips[i]
			break
		}
	}
	if clip == nil {
		return fmt.Errorf("clip %d not found in task %d", clipID, taskID)
	}
	if clip.Status != model.StatusFailed && clip.Status != model.StatusPending {
		return fmt.Errorf("clip %d cannot be retried (status: %s)", clipID, clip.Status)
	}

	resolvedModelName := strings.TrimSpace(modelName)
	if resolvedModelName == "" {
		resolvedModelName = task.ModelName
	}
	gen, err := s.resolveGenerator(ctx, resolvedModelName)
	if err != nil {
		return err
	}

	prevTaskStatus := task.Status
	prevTaskError := task.ErrorMsg
	task.Status = model.StatusProcessing
	task.ErrorMsg = ""
	task.ComposeStage = model.ComposeStageNone
	if modelName != "" {
		task.ModelName = resolvedModelName
	}
	if err := s.repo.UpdateTask(ctx, task); err != nil {
		return err
	}

	clip.Status = model.StatusProcessing
	clip.ErrorMsg = ""
	clip.ClipURL = ""
	clip.DurationSec = 0
	clip.ModelUsed = ""
	if err := s.repo.UpdateClip(ctx, clip); err != nil {
		return err
	}

	// Reconstruct per-clip context from render_config so the retried clip uses the same
	// prompt/duration/dialogue/camera/mood as the original batch, preserving cross-clip continuity.
	imageURLs := []string(task.ImageURLs)
	totalClips := len(imageURLs)
	if totalClips == 0 {
		totalClips = len(task.Clips)
	}
	perClipDescs := extractSceneDescriptions(task.RenderConfig, totalClips)
	perClipDialogues := extractDialogues(task.RenderConfig, totalClips)
	perClipDurations := extractDurations(task.RenderConfig, totalClips)
	perClipCameras := extractStringSlice(task.RenderConfig, "camera_movements", totalClips)
	perClipMoods := extractStringSlice(task.RenderConfig, "moods", totalClips)
	perClipSceneChars := extractSceneCharacters(task.RenderConfig, totalClips)
	perClipAssetIDs := extractInt64Matrix(task.RenderConfig, "scene_asset_ids", totalClips)
	charDescriptions := s.fetchCharacterDescriptions(ctx, task.ProjectID)
	retryAllCharURLs, retryCharByName := s.fetchCharacterImageMap(ctx, task.ProjectID)
	retryAssetAnchors := s.fetchAssetPromptAnchors(ctx, task.ProjectID, collectUniqueInt64(perClipAssetIDs))
	retryClipAssetRefs := perClipAssetReferenceImages(perClipAssetIDs, retryAssetAnchors, clip.ClipOrder)

	retryClipCharURLs := perSceneCharacterImages(retryAllCharURLs, retryCharByName, clipScenePersons(perClipSceneChars, clip.ClipOrder))
	retryPrompt := clipMotionPromptWithHints(
		clip.ClipOrder, totalClips, perClipDescs,
		task.MotionMode, task.StylePreset, resolvedModelName,
		task.RenderConfig, charDescriptions,
		perClipCameras, perClipMoods,
	)
	if strings.TrimSpace(retryPrompt) == "" {
		retryPrompt = motionPrompt(task.MotionMode, task.StylePreset, task.SceneDescription, task.RenderConfig)
	}
	retryPrompt = appendAssetAnchorHint(appendCharacterSheetHint(retryPrompt, retryClipCharURLs, resolvedModelName), perClipAssetAnchorHint(perClipAssetIDs, retryAssetAnchors, clip.ClipOrder))

	retryVoiceText := ""
	if gen.SupportsNativeAudio() {
		if clip.ClipOrder < len(perClipDialogues) && perClipDialogues[clip.ClipOrder] != "" {
			retryVoiceText = perClipDialogues[clip.ClipOrder]
		} else {
			retryVoiceText = task.SubtitleText
		}
	}

	// 串行链感知：非首帧 clip 重试时，重新获取上一 clip 的末帧作为首帧。
	// 避免因上次生成时帧提取失败而导致本次重试仍用空 SourceImageURL。
	if task.SerialScene && clip.SceneSeq > 0 && clip.SceneGroupKey != "" {
		for i := range task.Clips {
			prev := &task.Clips[i]
			if prev.SceneGroupKey == clip.SceneGroupKey && prev.SceneSeq == clip.SceneSeq-1 {
				if prev.EndFrameImageURL != "" {
					clip.SourceImageURL = prev.EndFrameImageURL
					s.logger.Info("retry: using previous clip end_frame_image_url as source",
						zap.Int64("clip_id", clip.ID),
						zap.Int64("prev_clip_id", prev.ID),
						zap.String("source_url", clip.SourceImageURL))
				} else if prev.ClipURL != "" && s.frameExtractorURL != "" {
					// 前一 clip 没有末帧记录，尝试重新提取
					if frameURL, extractErr := callFrameExtractor(ctx, prev.ClipURL, task.ProjectID, task.UserID, s.frameExtractorURL); extractErr == nil {
						clip.SourceImageURL = frameURL
						prev.EndFrameImageURL = frameURL
						saveCtx, saveCancel := context.WithTimeout(context.Background(), 10*time.Second)
						_ = s.repo.UpdateClip(saveCtx, prev)
						saveCancel()
						s.logger.Info("retry: re-extracted end frame from previous clip",
							zap.Int64("clip_id", clip.ID),
							zap.Int64("prev_clip_id", prev.ID))
					} else if prev.SourceImageURL != "" {
						// 帧提取再次失败，降级使用前一 clip 的首帧作为锚点
						clip.SourceImageURL = prev.SourceImageURL
						s.logger.Warn("retry: frame re-extraction failed, falling back to prev clip source image",
							zap.Int64("clip_id", clip.ID), zap.Error(extractErr))
					}
				}
				break
			}
		}
		// 非首帧连续性提示（与正常生成路径保持一致）
		if clip.SourceImageURL != "" {
			retryPrompt = "Seamless continuation from previous scene clip. Maintain visual continuity with identical character appearance, lighting, and environment. " + retryPrompt
		}
	}

	// Per-clip duration takes priority over task-level global duration, then snapped.
	retryDuration := snapDurationToModel(
		perClipDurationSec(perClipDurations, clip.ClipOrder, task.DurationSec),
		videoModelFamily(resolvedModelName),
	)

	genReq := generators.VideoGenerateReq{
		SourceImageURL:     clip.SourceImageURL,
		TailImageURL:       tailImageURL(imageURLs, clip.ClipOrder),
		CharacterImageURLs: retryClipCharURLs,
		ClipIndex:          clip.ClipOrder,
		TotalClips:         totalClips,
		Prompt:             retryPrompt,
		NegativePrompt:     describeVideoNegativePrompt(task.StylePreset),
		StylePreset:        task.StylePreset,
		MotionMode:         task.MotionMode,
		DurationSec:        retryDuration,
		VoiceText:          retryVoiceText,
		Resolution:         renderConfigString(task.RenderConfig, "resolution"),
		AspectRatio:        renderConfigString(task.RenderConfig, "aspect_ratio"),
		MotionAmplitude:    renderConfigString(task.RenderConfig, "motion_amplitude"),
		VideoMode:          renderConfigString(task.RenderConfig, "video_mode"),
		Count:              renderConfigInt(task.RenderConfig, "count"),
		// Video generation mode and audio
		GenerateMode:  renderConfigString(task.RenderConfig, "generate_mode"),
		GenerateAudio: renderConfigBool(task.RenderConfig, "generate_audio"),
	}
	// Override TailImageURL from RenderConfig if provided
	if endURL := renderConfigString(task.RenderConfig, "end_image_url"); endURL != "" {
		genReq.TailImageURL = endURL
	}
	// 串行模式非首帧：SourceImageURL 来自串行链前帧末帧（动态），不应被全局 start_image_url 覆盖。
	if startURL := renderConfigString(task.RenderConfig, "start_image_url"); startURL != "" {
		if !task.SerialScene || clip.SceneSeq == 0 {
			genReq.SourceImageURL = startURL
		}
	}
	genReq = normalizeVideoGenerateReq(gen, resolvedModelName, genReq, retryClipAssetRefs)
	s.logger.Info("retry clip generation request prepared",
		zap.Int64("task_id", taskID),
		zap.Int64("clip_id", clip.ID),
		zap.Int("clip_index", clip.ClipOrder),
		zap.String("model", resolvedModelName),
		zap.String("generate_mode", normalizedGenerateMode(genReq)),
		zap.Int("character_reference_count", len(retryClipCharURLs)),
		zap.Int("asset_reference_count", len(retryClipAssetRefs)),
		zap.Int("total_reference_count", len(genReq.CharacterImageURLs)),
		zap.Bool("has_tail_frame", strings.TrimSpace(genReq.TailImageURL) != ""),
		zap.Int("render_config_version", renderConfigInt(task.RenderConfig, "config_version")),
	)
	result, err := gen.Generate(ctx, genReq)
	if err != nil {
		clip.Status = model.StatusFailed
		clip.ErrorMsg = err.Error()
		if updateErr := s.repo.UpdateClip(ctx, clip); updateErr != nil {
			s.logger.Error("update clip retry failure", zap.Int64("clip_id", clipID), zap.Error(updateErr))
		}

		task.Status = prevTaskStatus
		task.ComposeStage = model.ComposeStageNone
		if prevTaskStatus != model.StatusSucceeded {
			task.Status = model.StatusFailed
			task.ErrorMsg = err.Error()
		} else {
			task.ErrorMsg = prevTaskError
		}
		if updateErr := s.repo.UpdateTask(ctx, task); updateErr != nil {
			s.logger.Error("restore task after clip retry failure", zap.Int64("task_id", taskID), zap.Error(updateErr))
		}
		return err
	}

	clip.ClipURL = result.ClipURL
	clip.DurationSec = result.DurationSec
	clip.ModelUsed = result.ModelUsed
	clip.Status = model.StatusSucceeded
	if err := s.repo.UpdateClip(ctx, clip); err != nil {
		return err
	}

	return s.ComposeTask(ctx, taskID)
}

// uploadVideo —— 将本地视频文件上传到存储服务，返回公开访问 URL
// uploadVideo pushes the local file to the storage-service and returns the public URL.
func (s *VideoService) uploadVideo(ctx context.Context, taskID, projectID int64, filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open video: %w", err)
	}
	defer f.Close()

	// Build multipart form: file + bucket + user_id
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("bucket", "videos")
	w.WriteField("user_id", "1") // system upload
	w.WriteField("project_id", fmt.Sprintf("%d", projectID))
	w.WriteField("category", "video")
	fw, err := w.CreateFormFile("file", fmt.Sprintf("%d_%d.mp4", taskID, time.Now().Unix()))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return "", err
	}
	w.Close()

	uploadURL := fmt.Sprintf("%s/api/v1/storage/upload", s.storageURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("upload: status %d — %s", resp.StatusCode, string(body))
	}

	// storage-service returns { "data": { "cdn_url": "...", "object_key": "..." } }
	type uploadResp struct {
		Data struct {
			CdnURL    string `json:"cdn_url"`
			ObjectKey string `json:"object_key"`
		} `json:"data"`
	}
	var ur uploadResp
	if err := parseJSON(body, &ur); err == nil && ur.Data.CdnURL != "" {
		return ur.Data.CdnURL, nil
	}
	return "", fmt.Errorf("upload: no url in response: %s", string(body))
}

// markFailed —— 将任务标记为失败状态并记录错误信息
func (s *VideoService) markFailed(ctx context.Context, taskID int64, msg string) {
	// Use a fresh background context: the caller's ctx may already be cancelled
	// (e.g. 30-minute task timeout expired), and we must always persist the failure.
	cleanCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := s.repo.UpdateTaskStatus(cleanCtx, taskID, model.StatusFailed, "", msg, 0); err != nil {
		s.logger.Error("markFailed db error", zap.Error(err))
	}
}

// WatchStaleTasks —— 定期检测长时间卡在 processing 状态的任务并将其标记为 failed
// WatchStaleTasks periodically scans for tasks stuck in "processing" longer than
// threshold and marks them failed so they can be retried by the user.
// Runs every interval until ctx is cancelled.
func (s *VideoService) WatchStaleTasks(ctx context.Context, threshold, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.recoverStaleTasks(threshold)
		}
	}
}

func (s *VideoService) recoverStaleTasks(threshold time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tasks, err := s.repo.FindStaleProcessing(ctx, threshold, 100)
	if err != nil {
		s.logger.Error("find stale video tasks", zap.Error(err))
		return
	}
	if len(tasks) == 0 {
		return
	}
	s.logger.Warn("resetting stale video tasks to failed",
		zap.Int("count", len(tasks)),
		zap.Duration("threshold", threshold))
	for _, t := range tasks {
		if err := s.repo.UpdateTaskStatus(ctx, t.ID, model.StatusFailed, "", "task timed out (watchdog)", 0); err != nil {
			s.logger.Error("reset stale task", zap.Int64("id", t.ID), zap.Error(err))
		}
	}
}

// ResumeOrphanedTasks —— 将重启前处于 processing 状态的任务重置为 pending
// ResumeOrphanedTasks resets "processing" tasks to "pending" so they get
// picked up by ResumePendingTasks (called next at startup).
func (s *VideoService) ResumeOrphanedTasks(ctx context.Context) {
	tasks, err := s.repo.FindByStatus(ctx, model.StatusProcessing, 1000)
	if err != nil {
		s.logger.Error("find orphaned tasks", zap.Error(err))
		return
	}
	if len(tasks) == 0 {
		return
	}
	s.logger.Info("resuming orphaned video tasks", zap.Int("count", len(tasks)))
	for _, t := range tasks {
		if err := s.repo.UpdateTaskStatus(ctx, t.ID, model.StatusPending, "", "", 0); err != nil {
			s.logger.Error("reset orphan", zap.Int64("id", t.ID), zap.Error(err))
		}
	}
}

// ResumePendingTasks —— 将未发送到 Kafka 的 pending 任务重新分发
// ResumePendingTasks dispatches pending tasks that were never sent to Kafka.
func (s *VideoService) ResumePendingTasks(ctx context.Context) {
	tasks, err := s.repo.FindByStatus(ctx, model.StatusPending, 1000)
	if err != nil {
		s.logger.Error("find pending tasks", zap.Error(err))
		return
	}
	if len(tasks) == 0 {
		return
	}
	s.logger.Info("dispatching pending video tasks", zap.Int("count", len(tasks)))
	for _, t := range tasks {
		if err := s.DispatchTask(ctx, &t); err != nil {
			s.logger.Warn("dispatch pending failed", zap.Int64("id", t.ID), zap.Error(err))
		}
	}
}

// RetryTask —— 重试单个失败任务，可选更换模型
// RetryTask retries a single failed task, optionally with a different model.
func (s *VideoService) RetryTask(ctx context.Context, taskID int64, modelName string) error {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status != model.StatusFailed && task.Status != model.StatusPending {
		return fmt.Errorf("task %d cannot be retried (status: %s)", taskID, task.Status)
	}
	if modelName != "" {
		task.ModelName = modelName
	}
	task.Status = model.StatusPending
	task.ErrorMsg = ""
	if err := s.repo.UpdateTask(ctx, task); err != nil {
		return err
	}
	// Delete old clips for a clean retry
	if err := s.repo.DeleteClipsByTaskID(ctx, taskID); err != nil {
		s.logger.Warn("delete old clips", zap.Error(err))
	}
	return s.DispatchTask(ctx, task)
}

// RetryBatchFailed —— 批量重试项目下所有失败任务，返回成功重试数
// RetryBatchFailed retries all failed tasks for a project.
func (s *VideoService) RetryBatchFailed(ctx context.Context, projectID int64, modelName string) (int, error) {
	tasks, err := s.repo.FindFailedByProject(ctx, projectID, 1000)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, t := range tasks {
		if err := s.RetryTask(ctx, t.ID, modelName); err != nil {
			s.logger.Warn("retry batch item", zap.Int64("id", t.ID), zap.Error(err))
			continue
		}
		count++
	}
	return count, nil
}

// StatusCounts —— 返回项目下按状态分组的任务计数
// StatusCounts returns task counts grouped by status for a project.
func (s *VideoService) StatusCounts(ctx context.Context, projectID int64) (map[string]int, error) {
	return s.repo.StatusCounts(ctx, projectID)
}

// ModelStatusItem represents a single video model's availability.
type ModelStatusItem struct {
	Key         string                        `json:"key"`
	Available   bool                          `json:"available"`
	NativeAudio bool                          `json:"native_audio"`
	Params      []generators.ModelParamOption `json:"params"`
}

// ModelStatus —— 检查所有已注册视频生成器的可用状态，按质量排序返回
func (s *VideoService) ModelStatus(ctx context.Context) []ModelStatusItem {
	primary := []string{"sora2", "hubagi-voe3.1", "hubagi-TC-GV", "kling", "wan", "comfyui-video"}
	seen := map[string]bool{}
	items := make([]ModelStatusItem, 0, len(s.generators))
	for _, key := range primary {
		if gen, ok := s.generators[key]; ok {
			seen[key] = true
			items = append(items, ModelStatusItem{Key: key, Available: gen.IsAvailable(ctx), NativeAudio: gen.SupportsNativeAudio(), Params: gen.ParamOptions()})
		}
	}
	for key, gen := range s.generators {
		if !seen[key] {
			items = append(items, ModelStatusItem{Key: key, Available: gen.IsAvailable(ctx), NativeAudio: gen.SupportsNativeAudio(), Params: gen.ParamOptions()})
		}
	}
	return items
}

func (s *VideoService) resolveGenerator(ctx context.Context, modelName string) (generators.VideoGenerator, error) {
	resolvedKey, providerModel := resolveVideoGeneratorRoute(modelName)
	if resolvedKey != "" {
		if gen, ok := s.generators[resolvedKey]; ok && gen.IsAvailable(ctx) {
			if providerModel != "" {
				return bindRequestedVideoModel(gen, resolvedKey, providerModel), nil
			}
			return gen, nil
		}
		return nil, fmt.Errorf("video generator %q is unavailable", modelName)
	}
	for _, gen := range s.generators {
		if gen.IsAvailable(ctx) {
			return gen, nil
		}
	}
	return nil, fmt.Errorf("no available generator")
}

func (s *VideoService) SetFrameExtractorURL(url string) {
	s.frameExtractorURL = url
}

func (s *VideoService) SetSerialFailureAnalyzer(a *SerialFailureAnalyzer) {
	s.serialFailureAnalyzer = a
}

func callFrameExtractor(ctx context.Context, videoURL string, projectID, userID int64, extractorBaseURL string) (string, error) {
	body, err := json.Marshal(map[string]interface{}{
		"video_url":  videoURL,
		"project_id": projectID,
		"user_id":    userID,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, extractorBaseURL+"/extract", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("frame-extractor HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var result struct {
		FrameURL string `json:"frame_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse frame-extractor response: %w", err)
	}
	if result.FrameURL == "" {
		return "", fmt.Errorf("empty frame_url in response")
	}
	return result.FrameURL, nil
}

func bindRequestedVideoModel(gen generators.VideoGenerator, generatorKey, providerModel string) generators.VideoGenerator {
	switch typed := gen.(type) {
	case *generators.KlingGenerator:
		clone := typed.Clone()
		clone.WithModel(providerModel)
		return clone.WithName(generatorKey)
	case *generators.WanGenerator:
		return typed.CloneWithModel(providerModel)
	case *generators.ViduGenerator:
		return typed.CloneWithModel(providerModel, generatorKey)
	case *generators.DoubaoGenerator:
		return typed.CloneWithModel(providerModel, generatorKey)
	case *generators.HubagiGenerator:
		return typed.CloneWithModel(providerModel)
	case *generators.SuannengGenerator:
		return typed.CloneWithModel(providerModel)
	default:
		return gen
	}
}

func resolveVideoGeneratorRoute(modelName string) (string, string) {
	raw := strings.TrimSpace(modelName)
	trimmed := strings.ToLower(raw)
	switch trimmed {
	case "", "auto":
		return "", ""
	case "comfyui", "comfyui-video", "comfyui-local-video", "local", "local-video":
		return "comfyui-video", ""
	case "sora", "sora2", "sora-2":
		return "sora2", ""
	case "wan", "wanx", "wan2.1", "wanx2.1", "wanx2.1-i2v-turbo", "tongyi":
		return "wan", "wanx2.1-i2v-turbo"
	case "wan2.6":
		return "wan", "wan2.6"
	case "veo", "veo3.1", "hubagi-veo3.1", "voe3.1", "hubagi-voe3.1", "xingwei-voe3.1":
		return "hubagi-voe3.1", "voe3.1"
	case "tc-gv", "hubagi-tc-gv", "xingwei-3.1", "hubagi-xingwei-3.1":
		return "hubagi-TC-GV", "TC-GV"
	// kling 系列路由到 tencent-vclm（腾讯云 VCLM API, TC3-HMAC-SHA256, vclm.tencentcloudapi.com）
	// 当直连 kling_key 不可用时，使用已配置的腾讯云 SecretId/SecretKey 通过 VCLM 接口生成。
	// 若将来配置了直连 kling_key，可将这些 case 改回返回 "kling"。
	case "kling", "kling-v3", "kling-3.0", "xinghe-3.0",
		"kling-v3-omni", "kling-3.0-omni", "xinghe-3.0-omni",
		"kling-v1.5", "kling-v1.6", "kling-v2", "kling-v2.1":
		return "tencent-vclm", ""
	case "doubao", "v4.0", "xingguang-3.0", "doubao-v4", "doubao-v4.0":
		return "doubao", "V4.0"
	case "doubao-seedance", "doubao-seedream", "seedream", "xingtu":
		return "doubao-seedance", ""
	case "doubao-seedream-4-0-250828":
		return "doubao-seedance", "doubao-seedream-4-0-250828"
	case "doubao-seedream-4-5-251128", "seedream-4-5", "doubao-seedream-4.5":
		return "doubao-seedance", "doubao-seedream-4-5-251128"
	case "vidu", "viduq3-pro", "vidu-q3-pro", "xingcheng-2.6", "vidu-3pro", "vidu官方-3pro":
		return "vidu", "viduq3-pro"
	case "vidu-mix", "viduq3-mix", "vidu-q3-mix", "xingchen-3.1", "vidu官方-mix":
		return "vidu-mix", "viduq3-mix"
	case "suanneng", "seedance-1.5-pro", "seedance", "xingguang-2.5", "sophnet":
		return "suanneng", "doubao-seedance-1-5-pro-251215"
	case "gaga", "gaga-1", "xingdian2.0", "xingdian-2.0":
		return "gaga", ""
	default:
		return raw, ""
	}
}

func normalizeVideoGeneratorKey(modelName string) string {
	key, _ := resolveVideoGeneratorRoute(modelName)
	return key
}

// motionPrompt —— 根据运动模式、风格预设和分镜场景描述生成视频生成提示词
func motionPrompt(motionMode, stylePreset, sceneDescription string, renderConfig model.RenderConfig) string {
	styleDescription := describeVideoStyle(stylePreset)
	basePrompt := ""
	switch motionMode {
	case "dynamic":
		basePrompt = fmt.Sprintf("dynamic camera movement, energetic motion beats, clear action direction, %s", styleDescription)
	case "cinematic":
		basePrompt = fmt.Sprintf("cinematic slow pan, controlled camera rhythm, elegant lens language, %s, film grain", styleDescription)
	default:
		basePrompt = fmt.Sprintf("gentle motion, smooth transition, restrained camera drift, %s", styleDescription)
	}

	qualityPrompt := "stable subject identity, coherent anatomy, readable silhouette, believable lighting, natural cloth and hair motion, continuous action across the whole clip"
	renderPrompt := describeRenderConfig(renderConfig)
	avoidPrompt := describeVideoNegativePrompt(stylePreset)
	segments := []string{basePrompt, qualityPrompt}
	if scenePrompt := buildVideoScenePrompt(sceneDescription); scenePrompt != "" {
		segments = append(segments, scenePrompt)
	}
	if renderPrompt != "" {
		segments = append(segments, renderPrompt)
	}
	if avoidPrompt != "" {
		segments = append(segments, "avoid "+avoidPrompt)
	}
	return strings.Join(segments, ", ")
}

// clipMotionPrompt builds a per-clip prompt that includes:
//   - position-aware role (opening / mid-sequence / closing)
//   - this clip's own scene description
//   - transition hints so adjacent clips cut together smoothly
//   - optional character appearance descriptions for visual consistency
//   - model-specific language hints (Chinese for Kling/WAN, English otherwise)
func clipMotionPrompt(clipIdx, totalClips int, perClipDescs []string, motionMode, stylePreset, modelName string, renderConfig model.RenderConfig, charDescriptions string) string {
	// Pick the scene description for this specific clip.
	sceneDesc := ""
	if clipIdx < len(perClipDescs) {
		sceneDesc = perClipDescs[clipIdx]
	}

	// Fall back to the global prompt if no per-clip desc available.
	if sceneDesc == "" {
		return motionPromptForModel(motionMode, stylePreset, modelName, sceneDesc, renderConfig)
	}

	family := videoModelFamily(modelName)
	if family == "kling" || family == "wan" || family == "doubao" || family == "vidu" || family == "suanneng" {
		return clipMotionPromptChineseFamily(clipIdx, totalClips, sceneDesc, motionMode, stylePreset, family, renderConfig, charDescriptions)
	}

	styleDescription := describeVideoStyle(stylePreset)
	var basePrompt string
	switch motionMode {
	case "dynamic":
		basePrompt = fmt.Sprintf("dynamic camera movement, energetic motion, %s", styleDescription)
	case "cinematic":
		basePrompt = fmt.Sprintf("cinematic slow pan, controlled rhythm, elegant lens language, %s, film grain", styleDescription)
	default:
		basePrompt = fmt.Sprintf("gentle motion, restrained camera drift, %s", styleDescription)
	}

	// Position-aware continuity cues.
	var positionCue string
	switch {
	case totalClips == 1:
		positionCue = "self-contained scene, complete narrative moment"
	case clipIdx == 0:
		positionCue = "opening sequence, establish scene and mood, motion exits frame smoothly toward next scene"
	case clipIdx == totalClips-1:
		positionCue = "closing sequence, resolve action naturally, motion settles to rest"
	default:
		positionCue = "mid-sequence, continue action flow from previous cut, motion exits frame smoothly toward next scene"
	}

	qualityPrompt := "stable subject identity, coherent anatomy, consistent environment, seamless visual continuity, readable silhouette, believable lighting, natural cloth and hair motion, consistent color palette across clips"
	renderPrompt := describeRenderConfig(renderConfig)
	avoidPrompt := describeVideoNegativePrompt(stylePreset)

	// Global style anchor to lock visual consistency across all clips.
	anchor := buildGlobalStyleAnchorEN(stylePreset, charDescriptions)

	// Model-family-specific quality hints for English-prompt models.
	var familyHint string
	switch family {
	case "sora":
		familyHint = "photoreal world model coherence, consistent physics, stable subject tracking across frames, natural parallax, believable occlusion"
	case "veo":
		familyHint = "cinematic composition, precise lighting continuity, sharp focus on subject, natural depth of field, filmic color grade"
	case "gaga":
		familyHint = "crisp motion rendering, stable facial identity, expressive lip sync, clean background separation"
	default:
		familyHint = "temporally coherent frames, identity lock, stable geometry, no morphing or flicker"
	}

	segments := []string{anchor, basePrompt, positionCue, qualityPrompt, familyHint}
	if scenePrompt := buildVideoScenePrompt(sceneDesc); scenePrompt != "" {
		segments = append(segments, scenePrompt)
	}
	if renderPrompt != "" {
		segments = append(segments, renderPrompt)
	}
	if avoidPrompt != "" {
		segments = append(segments, "avoid: "+avoidPrompt)
	}
	return strings.Join(segments, ", ")
}

// clipMotionPromptChinese builds a Chinese-language per-clip prompt for Chinese video models (Kling/WAN/Vidu/Doubao/Suanneng).
// modelFamily is used to add model-specific hints.
func clipMotionPromptChinese(clipIdx, totalClips int, sceneDesc, motionMode, stylePreset string, renderConfig model.RenderConfig, charDescriptions string) string {
	return clipMotionPromptChineseFamily(clipIdx, totalClips, sceneDesc, motionMode, stylePreset, "generic-cn", renderConfig, charDescriptions)
}

// extractInlineAnnotation extracts the first value of a named inline annotation [key:value] from text.
// Returns "" if not found.
func extractInlineAnnotation(text, key string) string {
	prefix := "[" + key + ":"
	idx := strings.Index(text, prefix)
	if idx < 0 {
		return ""
	}
	start := idx + len(prefix)
	end := strings.Index(text[start:], "]")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(text[start : start+end])
}

// clipMotionPromptChineseFamily builds a per-clip Chinese prompt with model-family-specific optimizations.
// It extracts structured cinematography fields from inline annotations in sceneDesc:
// [景别:xxx] [视角:xxx] [运镜:xxx] [构图:xxx] [氛围:xxx] [摄影:景别/角度/运镜]
func clipMotionPromptChineseFamily(clipIdx, totalClips int, sceneDesc, motionMode, stylePreset, family string, renderConfig model.RenderConfig, charDescriptions string) string {
	stylePart := describeVideoStyleChinese(stylePreset)
	var basePart string
	switch motionMode {
	case "dynamic":
		basePart = "动感镜头运动，充满活力的动作节奏，清晰方向感"
	case "cinematic":
		basePart = "电影感缓慢推镜，节奏克制，优雅镜头语言，胶片质感"
	default:
		basePart = "轻柔运动，流畅过渡，镜头稳定"
	}

	var positionCue string
	switch {
	case totalClips == 1:
		positionCue = "独立完整场景，叙事完整"
	case clipIdx == 0:
		positionCue = "开篇镜头，建立场景氛围，运动自然衔接下一幕"
	case clipIdx == totalClips-1:
		positionCue = "结尾镜头，动作自然收尾，镜头稳定落定"
	default:
		positionCue = "中段镜头，承接上一镜头动作，运动平滑衔接下一幕"
	}

	qualityPart := "主体身份稳定，骨骼结构正确，环境连贯，光影自然，服装与头发运动真实，全片色调统一"
	renderPart := describeRenderConfig(renderConfig)

	// Global style anchor for visual consistency across all clips.
	anchor := buildGlobalStyleAnchor(stylePreset, charDescriptions)

	// Model-family-specific quality hints.
	var familyHint string
	switch family {
	case "kling":
		familyHint = "角色运动弧线流畅，动作帧连贯，肢体关节比例准确，镜头内运动轨迹自然"
	case "wan":
		familyHint = "环境层次丰富，大气透视真实，远近景纵深感强，背景细节清晰稳定"
	case "vidu":
		familyHint = "物理运动真实，速度感与惯性自然，镜头切换节奏明快，运动模糊适度"
	case "doubao":
		familyHint = "口型与台词同步精准，面部细节丰富，表情自然过渡，情绪层次清晰"
	case "suanneng":
		familyHint = "画面锐利清晰，色彩饱和度适中，人物面部表现力强，动作幅度与情节匹配"
	}

	// --- Extract structured cinematography fields from inline annotations ---
	// Priority: dedicated annotations ([景别:xxx]) over sub-fields in [摄影:景别/角度/运镜]
	shotSize := extractInlineAnnotation(sceneDesc, "景别")
	viewAngle := extractInlineAnnotation(sceneDesc, "视角")
	cameraMove := extractInlineAnnotation(sceneDesc, "运镜")
	composition := extractInlineAnnotation(sceneDesc, "构图")
	atmosphere := extractInlineAnnotation(sceneDesc, "氛围")
	// Fall back to [摄影:景别/角度/运镜] if dedicated tags are missing.
	if cineAnno := extractInlineAnnotation(sceneDesc, "摄影"); cineAnno != "" {
		sub := strings.SplitN(cineAnno, "/", 3)
		if shotSize == "" && len(sub) >= 1 && strings.TrimSpace(sub[0]) != "" {
			shotSize = strings.TrimSpace(sub[0])
		}
		if viewAngle == "" && len(sub) >= 2 && strings.TrimSpace(sub[1]) != "" {
			viewAngle = strings.TrimSpace(sub[1])
		}
		if cameraMove == "" && len(sub) >= 3 && strings.TrimSpace(sub[2]) != "" {
			cameraMove = strings.TrimSpace(sub[2])
		}
	}

	parts := []string{anchor, stylePart, basePart, positionCue, qualityPart}
	if familyHint != "" {
		parts = append(parts, familyHint)
	}

	// Append structured cinematography fields as a concise block.
	var cinemaParts []string
	if shotSize != "" {
		cinemaParts = append(cinemaParts, "景别："+shotSize)
	}
	if viewAngle != "" {
		cinemaParts = append(cinemaParts, "视角："+viewAngle)
	}
	if cameraMove != "" {
		cinemaParts = append(cinemaParts, "运镜："+cameraMove)
	}
	if composition != "" {
		cinemaParts = append(cinemaParts, "构图："+composition)
	}
	if atmosphere != "" {
		cinemaParts = append(cinemaParts, "氛围："+atmosphere)
	}
	if len(cinemaParts) > 0 {
		parts = append(parts, strings.Join(cinemaParts, "，"))
	}

	if sceneDesc != "" {
		if hasMeaningfulChinese(sceneDesc) {
			parts = append(parts, "场景："+sceneDesc)
		} else {
			parts = append(parts, "scene: "+sceneDesc)
		}
	}
	if renderPart != "" {
		parts = append(parts, renderPart)
	}
	parts = append(parts, "避免：画面闪烁，帧抖动，身份漂移，扭曲肢体，文字叠加，水印，跨镜头风格突变，色调跳变，人物外貌改变，服装颜色变化")
	return strings.Join(parts, "，")
}

// extractSceneDescriptions pulls the per-clip scene_descriptions array stored
// in RenderConfig. Returns a slice of length n padded with empty strings if needed.
func extractSceneDescriptions(renderConfig model.RenderConfig, n int) []string {
	result := make([]string, n)
	if len(renderConfig) == 0 {
		return result
	}
	raw, ok := renderConfig["scene_descriptions"]
	if !ok {
		return result
	}
	// RenderConfig values survive a JSON round-trip as []interface{}.
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

// extractDialogues pulls the per-clip dialogues array stored in RenderConfig.
// Returns a slice of length n padded with empty strings if needed.
func extractDialogues(renderConfig model.RenderConfig, n int) []string {
	result := make([]string, n)
	if len(renderConfig) == 0 {
		return result
	}
	raw, ok := renderConfig["dialogues"]
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

// joinDialogues concatenates non-empty dialogue lines with newlines.
func joinDialogues(dialogues []string) string {
	var parts []string
	for _, d := range dialogues {
		if strings.TrimSpace(d) != "" {
			parts = append(parts, d)
		}
	}
	return strings.Join(parts, "\n")
}

// extractSceneCharacters pulls per-clip character name lists from RenderConfig
// (stored as [][]string under key "scene_characters"). Returns a slice of
// length n; missing entries are empty slices so the caller can fall back to
// the project-wide character list.
func extractSceneCharacters(renderConfig model.RenderConfig, n int) [][]string {
	result := make([][]string, n)
	if len(renderConfig) == 0 {
		return result
	}
	raw, ok := renderConfig["scene_characters"]
	if !ok {
		return result
	}
	assign := func(i int, names []string) {
		if i < 0 || i >= n {
			return
		}
		out := make([]string, 0, len(names))
		for _, nm := range names {
			trimmed := strings.TrimSpace(nm)
			if trimmed != "" {
				out = append(out, trimmed)
			}
		}
		result[i] = out
	}
	switch v := raw.(type) {
	case [][]string:
		for i := 0; i < n && i < len(v); i++ {
			assign(i, v[i])
		}
	case []interface{}:
		for i := 0; i < n && i < len(v); i++ {
			switch inner := v[i].(type) {
			case []string:
				assign(i, inner)
			case []interface{}:
				names := make([]string, 0, len(inner))
				for _, x := range inner {
					if s, ok := x.(string); ok {
						names = append(names, s)
					}
				}
				assign(i, names)
			}
		}
	}
	return result
}

// clipScenePersons safely returns the character names assigned to the i-th clip, or nil.
func clipScenePersons(scenes [][]string, i int) []string {
	if i < 0 || i >= len(scenes) {
		return nil
	}
	return scenes[i]
}

// perSceneCharacterImages returns reference images for the given scene's characters.
// Returns nil when no scene character matches — falling back to the full project
// character list would inject UNRELATED subjects into the clip, which actively
// degrades character consistency (the generator would try to blend every
// character together). An empty result means the model uses only the source
// image as reference, which is the safer baseline.
// Lookup normalizes names (trim + lowercase) so storyboard-extracted names
// (which may vary in casing/whitespace or differ in language) still match.
func perSceneCharacterImages(allURLs []string, byName map[string]string, sceneChars []string) []string {
	if len(sceneChars) == 0 {
		// No per-scene character list → fall back to all project chars so a
		// general-subject project (e.g. single protagonist across every clip)
		// still benefits from the reference set.
		return allURLs
	}
	if len(byName) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(sceneChars))
	var picked []string
	for _, nm := range sceneChars {
		raw := strings.TrimSpace(nm)
		if raw == "" {
			continue
		}
		url, ok := byName[raw]
		if !ok {
			url, ok = byName[strings.ToLower(raw)]
		}
		if !ok || url == "" {
			continue
		}
		if _, dup := seen[url]; dup {
			continue
		}
		seen[url] = struct{}{}
		picked = append(picked, url)
	}
	// Explicitly return picked (possibly empty) — do NOT fall back to allURLs
	// when the scene lists characters but none match, because injecting the
	// full project cast produces identity drift in the generated clip.
	return picked
}

// extractDurations pulls per-clip duration (seconds) from RenderConfig.
// Returns zeros (use global) for missing/invalid entries.
func extractDurations(renderConfig model.RenderConfig, n int) []float64 {
	result := make([]float64, n)
	if len(renderConfig) == 0 {
		return result
	}
	raw, ok := renderConfig["durations"]
	if !ok {
		return result
	}
	switch v := raw.(type) {
	case []float64:
		for i := 0; i < n && i < len(v); i++ {
			result[i] = v[i]
		}
	case []interface{}:
		for i := 0; i < n && i < len(v); i++ {
			switch fv := v[i].(type) {
			case float64:
				result[i] = fv
			case int:
				result[i] = float64(fv)
			case int64:
				result[i] = float64(fv)
			}
		}
	}
	return result
}

// extractStringSlice pulls a per-clip string array from RenderConfig by key.
func extractStringSlice(renderConfig model.RenderConfig, key string, n int) []string {
	result := make([]string, n)
	if len(renderConfig) == 0 {
		return result
	}
	raw, ok := renderConfig[key]
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

func extractInt64Matrix(renderConfig model.RenderConfig, key string, n int) [][]int64 {
	result := make([][]int64, n)
	if len(renderConfig) == 0 {
		return result
	}
	raw, ok := renderConfig[key]
	if !ok {
		return result
	}
	convert := func(items []interface{}) []int64 {
		var out []int64
		for _, item := range items {
			switch v := item.(type) {
			case float64:
				out = append(out, int64(v))
			case int:
				out = append(out, int64(v))
			case int64:
				out = append(out, v)
			}
		}
		return out
	}
	switch v := raw.(type) {
	case [][]int64:
		for i := 0; i < n && i < len(v); i++ {
			result[i] = append([]int64(nil), v[i]...)
		}
	case []interface{}:
		for i := 0; i < n && i < len(v); i++ {
			switch inner := v[i].(type) {
			case []interface{}:
				result[i] = convert(inner)
			case []int64:
				result[i] = append([]int64(nil), inner...)
			}
		}
	}
	return result
}

func collectUniqueInt64(matrix [][]int64) []int64 {
	seen := make(map[int64]struct{})
	var out []int64
	for _, row := range matrix {
		for _, id := range row {
			if id <= 0 {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

// perClipDurationSec returns the per-clip duration for index i.
// Falls back to the task-level globalDurationSec (then to default 5s) when per-clip is zero.
func perClipDurationSec(perClip []float64, i int, globalDurationSec float64) float64 {
	if i < len(perClip) && perClip[i] > 0 {
		return perClip[i]
	}
	return clipDuration(globalDurationSec)
}

// clipMotionPromptWithHints is clipMotionPrompt extended with per-clip camera and mood hints.
func clipMotionPromptWithHints(
	clipIndex, totalClips int,
	perClipDescs []string,
	motionMode, stylePreset, modelName string,
	renderConfig model.RenderConfig,
	charDescriptions string,
	cameraHints, moodHints []string,
) string {
	// Inject camera and mood into the scene description for this clip before building the full prompt.
	descs := make([]string, len(perClipDescs))
	copy(descs, perClipDescs)
	if clipIndex < len(descs) {
		var extra []string
		if clipIndex < len(cameraHints) && strings.TrimSpace(cameraHints[clipIndex]) != "" {
			extra = append(extra, cameraHints[clipIndex])
		}
		if clipIndex < len(moodHints) && strings.TrimSpace(moodHints[clipIndex]) != "" {
			extra = append(extra, moodHints[clipIndex])
		}
		if len(extra) > 0 {
			if descs[clipIndex] == "" {
				descs[clipIndex] = strings.Join(extra, ", ")
			} else {
				descs[clipIndex] = descs[clipIndex] + ". " + strings.Join(extra, ", ")
			}
		}
	}
	return clipMotionPrompt(clipIndex, totalClips, descs, motionMode, stylePreset, modelName, renderConfig, charDescriptions)
}

// appendCharacterSheetHint adds a short explanation to a video prompt when any
// of the character reference images is a 4-panel composite sheet produced by
// character-service (filename pattern "asset_*_composite.jpg"). Without this,
// video models tend to read the strip as four different characters.
// The hint uses the same language family as the rest of the generated prompt:
// Chinese models get Chinese text; others get English.
func appendCharacterSheetHint(prompt string, characterRefs []string, modelName string) string {
	if !containsCharacterPanelSheetURL(characterRefs) {
		return prompt
	}
	family := videoModelFamily(modelName)
	useChinese := family == "kling" || family == "wan" || family == "doubao" || family == "vidu" || family == "suanneng"
	var hint string
	if useChinese {
		hint = "【人物参考图说明】所提供的人物参考图是同一个角色的四视图合成表，从左到右依次为：①头部与上半身特写（清晰五官）；②全身正面；③全身正侧面；④全身背面。请把四栏理解为同一个人物，还原同一位角色的脸型、发型与服装，不得将其视为四个不同的人。"
	} else {
		hint = "[Character reference note] The provided character reference image is a 4-panel reference sheet of ONE single character, arranged left-to-right as: (1) head-and-upper-body close-up with clear face, (2) front full-body, (3) side full-body, (4) back full-body. Treat all four panels as the SAME person and reconstruct that same character's face, hairstyle and costume. Do not treat them as four different people."
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return hint
	}
	return prompt + "\n" + hint
}

func appendAssetAnchorHint(prompt, assetHint string) string {
	assetHint = strings.TrimSpace(assetHint)
	if assetHint == "" {
		return prompt
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return assetHint
	}
	return prompt + "\n" + assetHint
}

type videoAssetPromptAnchor struct {
	ID          int64
	Name        string
	Type        string
	ImageURL    string
	PromptUsed  string
	Description string
}

func (s *VideoService) fetchAssetPromptAnchors(ctx context.Context, projectID int64, assetIDs []int64) map[int64]videoAssetPromptAnchor {
	if s.characterBaseURL == "" || len(assetIDs) == 0 {
		return nil
	}
	// Build lookup set so we ignore assets we don't need.
	needed := make(map[int64]struct{}, len(assetIDs))
	for _, id := range assetIDs {
		if id > 0 {
			needed[id] = struct{}{}
		}
	}
	if len(needed) == 0 {
		return nil
	}
	// Fetch all project assets in a single request instead of N serial per-ID calls.
	// character-service ListAssets returns { data: [...] } without pagination when no
	// page/page_size/status query params are present.
	listURL := fmt.Sprintf("%s/api/v1/projects/%d/assets", s.characterBaseURL, projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("fetch asset anchors failed", zap.Int64("project_id", projectID), zap.Error(err))
		}
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var wrapped struct {
		Data []struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			Type        string `json:"type"`
			ImageURL    string `json:"image_url"`
			PromptUsed  string `json:"prompt_used"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&wrapped); decodeErr != nil {
		return nil
	}
	out := make(map[int64]videoAssetPromptAnchor, len(needed))
	for _, a := range wrapped.Data {
		if _, want := needed[a.ID]; !want {
			continue
		}
		out[a.ID] = videoAssetPromptAnchor{
			ID:          a.ID,
			Name:        a.Name,
			Type:        a.Type,
			ImageURL:    a.ImageURL,
			PromptUsed:  a.PromptUsed,
			Description: a.Description,
		}
	}
	return out
}

func perClipAssetReferenceImages(sceneAssetIDs [][]int64, anchorMap map[int64]videoAssetPromptAnchor, clipIndex int) []string {
	if clipIndex < 0 || clipIndex >= len(sceneAssetIDs) || len(anchorMap) == 0 {
		return nil
	}
	const maxAssetRefs = 2
	seen := make(map[string]struct{}, maxAssetRefs)
	var sceneRefs []string
	var propRefs []string
	for _, assetID := range sceneAssetIDs[clipIndex] {
		anchor, ok := anchorMap[assetID]
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(anchor.Type), "character") {
			continue
		}
		url := strings.TrimSpace(anchor.ImageURL)
		if url == "" {
			continue
		}
		if _, dup := seen[url]; dup {
			continue
		}
		seen[url] = struct{}{}
		switch strings.ToLower(strings.TrimSpace(anchor.Type)) {
		case "scene", "location":
			sceneRefs = append(sceneRefs, url)
		default:
			propRefs = append(propRefs, url)
		}
	}
	refs := append(sceneRefs, propRefs...)
	if len(refs) > maxAssetRefs {
		refs = refs[:maxAssetRefs]
	}
	return refs
}

func mergeReferenceURLs(groups ...[]string) []string {
	seen := make(map[string]struct{})
	var merged []string
	for _, group := range groups {
		for _, url := range group {
			trimmed := strings.TrimSpace(url)
			if trimmed == "" {
				continue
			}
			if _, dup := seen[trimmed]; dup {
				continue
			}
			seen[trimmed] = struct{}{}
			merged = append(merged, trimmed)
		}
	}
	return merged
}

func generatorSupportsReferenceImages(modelName string) bool {
	switch videoModelFamily(modelName) {
	case "doubao", "suanneng", "vidu", "wan":
		return true
	default:
		return false
	}
}

func generatorSupportsStartEnd(modelName string) bool {
	switch videoModelFamily(modelName) {
	case "doubao", "suanneng", "kling":
		return true
	default:
		return false
	}
}

func normalizeVideoGenerateReq(gen generators.VideoGenerator, modelName string, req generators.VideoGenerateReq, assetRefs []string) generators.VideoGenerateReq {
	supportsRefs := generatorSupportsReferenceImages(modelName)
	supportsStartEnd := generatorSupportsStartEnd(modelName)
	req.CharacterImageURLs = mergeReferenceURLs(req.CharacterImageURLs, assetRefs)
	if !supportsRefs {
		req.CharacterImageURLs = nil
	}
	if req.GenerateMode == "" && strings.TrimSpace(req.TailImageURL) != "" && supportsStartEnd {
		req.GenerateMode = "startEnd2video"
	}
	switch req.GenerateMode {
	case "reference2video":
		if !supportsRefs {
			if strings.TrimSpace(req.TailImageURL) != "" && supportsStartEnd {
				req.GenerateMode = "startEnd2video"
			} else {
				req.GenerateMode = "img2video"
			}
		}
	case "startEnd2video":
		if !supportsStartEnd {
			if len(req.CharacterImageURLs) > 0 && supportsRefs {
				req.GenerateMode = "reference2video"
			} else {
				req.GenerateMode = "img2video"
			}
		}
	case "":
		if len(req.CharacterImageURLs) > 0 && supportsRefs && strings.TrimSpace(req.SourceImageURL) == "" {
			req.GenerateMode = "reference2video"
		}
	}
	if !gen.SupportsNativeAudio() {
		req.GenerateAudio = false
		req.VoiceText = ""
	}
	return req
}

func normalizedGenerateMode(req generators.VideoGenerateReq) string {
	mode := strings.TrimSpace(req.GenerateMode)
	if mode == "" {
		return "img2video"
	}
	return mode
}

func perClipAssetAnchorHint(sceneAssetIDs [][]int64, anchorMap map[int64]videoAssetPromptAnchor, clipIndex int) string {
	if clipIndex < 0 || clipIndex >= len(sceneAssetIDs) || len(anchorMap) == 0 {
		return ""
	}
	var sceneParts []string
	var propParts []string
	for _, assetID := range sceneAssetIDs[clipIndex] {
		anchor, ok := anchorMap[assetID]
		if !ok {
			continue
		}
		assetType := strings.ToLower(strings.TrimSpace(anchor.Type))
		if assetType == "character" {
			continue
		}
		text := strings.TrimSpace(anchor.PromptUsed)
		if text == "" {
			text = strings.TrimSpace(anchor.Description)
		}
		if text == "" {
			continue
		}
		entry := strings.TrimSpace(anchor.Name)
		if entry == "" {
			entry = fmt.Sprintf("asset_%d", assetID)
		}
		entry += ": " + truncateForPrompt(text, 220)
		switch assetType {
		case "scene", "location":
			sceneParts = append(sceneParts, entry)
		case "prop", "item":
			propParts = append(propParts, entry)
		default:
			// Unknown asset types (clothing, vehicle, object…) fall back to prop continuity.
			propParts = append(propParts, entry)
		}
	}
	var parts []string
	if len(sceneParts) > 0 {
		parts = append(parts, "SCENE CONTINUITY: "+strings.Join(sceneParts, " | "))
	}
	if len(propParts) > 0 {
		parts = append(parts, "PROP CONTINUITY: "+strings.Join(propParts, " | "))
	}
	return strings.Join(parts, "\n")
}

func truncateForPrompt(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 || utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit]) + "..."
}

func countNonEmptyStrings(items []string) int {
	count := 0
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			count++
		}
	}
	return count
}

func countNonEmptyStringSets(items [][]string) int {
	count := 0
	for _, item := range items {
		if len(item) > 0 {
			count++
		}
	}
	return count
}

func containsCharacterPanelSheetURL(urls []string) bool {
	for _, u := range urls {
		if strings.Contains(strings.ToLower(u), "_composite.") {
			return true
		}
	}
	return false
}

// opening of the following clip. Returns "" for the last clip.
func tailImageURL(imageURLs []string, clipOrder int) string {
	next := clipOrder + 1
	if next < len(imageURLs) {
		return imageURLs[next]
	}
	return ""
}

// ─── Scene-Description Prompt Builder ────────────────────────────────────────

var (
	// reEnglishParen matches English phrases inside parentheses, e.g. "(Full Shot)".
	reEnglishParen = regexp.MustCompile(`\(([A-Za-z][A-Za-z0-9 \-/]+)\)`)
	// reLightingSection matches the value after "光线" (Lighting) label.
	// Excludes both colon forms from the intermediate portion to prevent greedy overrun.
	reLightingSection = regexp.MustCompile(`光线[^:：\n。【]*[:：]([^。\n【]+)`)
	// reCharAction matches character action state values in the tracking block.
	reCharAction = regexp.MustCompile(`[\x{4e00}-\x{9fff}A-Za-z0-9]+?-动作状态[：:]([^；\n【]+)`)
	// reCharEmotion matches character emotion state values in the tracking block.
	reCharEmotion = regexp.MustCompile(`[\x{4e00}-\x{9fff}A-Za-z0-9]+?-情绪状态[：:]([^；\n【]+)`)
	// reSceneDesc extracts the content of the "画面描述" (Screen description) section.
	reSceneDesc = regexp.MustCompile(`画面描述[^:：\n]*[:：]\s*([^【\n]{1,200})`)
)

// cinematicLabelSet contains English section-header annotations that appear
// inside parentheses as labels (not cinematic values to keep).
var cinematicLabelSet = map[string]bool{
	"screen description": true,
	"composition":        true,
	"shot scale":         true,
	"camera position":    true,
	"angle":              true,
	"lens type":          true,
	"lighting":           true,
	"visual focus":       true,
}

// emotionTranslations maps common Chinese emotion descriptors to English equivalents.
// Used to convert 情绪状态 values in the character tracking block into mood cues.
var emotionTranslations = map[string]string{
	"凝重":  "solemn",
	"紧张":  "tense",
	"不耐烦": "impatient",
	"好奇":  "curious",
	"愤怒":  "furious",
	"悲伤":  "sorrowful",
	"恐惧":  "fearful",
	"喜悦":  "joyful",
	"平静":  "calm",
	"严肃":  "stern",
	"警惕":  "vigilant",
	"痛苦":  "anguished",
	"绝望":  "desperate",
	"兴奋":  "excited",
	"惊讶":  "startled",
	"忧郁":  "melancholic",
	"冷漠":  "indifferent",
	"坚定":  "resolute",
	"迷茫":  "confused",
}

// translateEmotion returns the English equivalent of a Chinese emotion term,
// or the original Chinese if no mapping exists. Strips trailing punctuation.
func translateEmotion(zh string) string {
	zh = strings.TrimRight(strings.TrimSpace(zh), "。！？,.!?、；;")
	if en, ok := emotionTranslations[zh]; ok {
		return en
	}
	return zh
}

// hasMeaningfulChinese reports whether s contains enough CJK characters to be
// treated as a Chinese-structured storyboard description worth parsing.
func hasMeaningfulChinese(s string) bool {
	var cnt int
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			cnt++
			if cnt >= 6 {
				return true
			}
		}
	}
	return false
}

// buildVideoScenePrompt converts a storyboard scene description into a concise
// video generation prompt fragment.
//
// When the input is an English description (e.g. a pre-refined prompt_used),
// it is returned directly (truncated to 300 chars) — no parsing is needed.
//
// When the input is a Chinese structured storyboard description, it extracts
// (in order):
//  1. English cinematic value terms from parentheses (e.g. "Full Shot", "Eye-Level",
//     "Wide-Angle Lens"), skipping section-label annotations.
//  2. Lighting description from the "光线" section.
//  3. Unique character action states from the "角色状态追踪" block.
//  4. Unique character emotions (情绪状态), translated to English mood cues.
//  5. The first sentence of the "画面描述" section as scene atmosphere context.
func buildVideoScenePrompt(sceneDescription string) string {
	desc := strings.TrimSpace(sceneDescription)
	if desc == "" {
		return ""
	}

	// English input (e.g. pre-refined prompt_used): return directly without parsing
	// Chinese structure markers — the text is already a ready-to-use prompt fragment.
	if !hasMeaningfulChinese(desc) {
		if len([]rune(desc)) > 300 {
			runes := []rune(desc)
			desc = string(runes[:300]) + "..."
		}
		return desc
	}

	var parts []string

	// 1. Extract English value terms from parentheses.
	//    Skip labels whose paren is immediately followed by '：' or ':'.
	seen := map[string]bool{}
	for _, loc := range reEnglishParen.FindAllStringSubmatchIndex(desc, -1) {
		term := strings.TrimSpace(desc[loc[2]:loc[3]])
		lower := strings.ToLower(term)
		if cinematicLabelSet[lower] || seen[lower] {
			continue
		}
		// If the character immediately after the closing ')' is a colon, this is
		// a section label translation — skip it.
		after := loc[1]
		if after < len(desc) {
			rest := desc[after:]
			if strings.HasPrefix(rest, ":") || strings.HasPrefix(rest, "：") {
				continue
			}
		}
		seen[lower] = true
		parts = append(parts, term)
	}

	// 2. Extract lighting description.
	if m := reLightingSection.FindStringSubmatch(desc); len(m) > 1 {
		if lighting := strings.TrimSpace(m[1]); lighting != "" {
			parts = append(parts, "lighting: "+lighting)
		}
	}

	// 3. Extract character action states as motion cues.
	actionSet := map[string]bool{}
	for _, m := range reCharAction.FindAllStringSubmatch(desc, -1) {
		act := strings.TrimRight(strings.TrimSpace(m[1]), "。！？,.!?、；;")
		if act != "" {
			actionSet[act] = true
		}
	}
	if len(actionSet) > 0 {
		acts := make([]string, 0, len(actionSet))
		for a := range actionSet {
			acts = append(acts, a)
		}
		parts = append(parts, "character actions: "+strings.Join(acts, " / "))
	}

	// 4. Extract character emotions and translate to English mood cues.
	emotionSet := map[string]bool{}
	for _, m := range reCharEmotion.FindAllStringSubmatch(desc, -1) {
		if emo := translateEmotion(m[1]); emo != "" {
			emotionSet[emo] = true
		}
	}
	if len(emotionSet) > 0 {
		emos := make([]string, 0, len(emotionSet))
		for e := range emotionSet {
			emos = append(emos, e)
		}
		parts = append(parts, "emotional tone: "+strings.Join(emos, ", "))
	}

	// 5. Extract first sentence of 画面描述 as scene atmosphere context.
	if m := reSceneDesc.FindStringSubmatch(desc); len(m) > 1 {
		scene := strings.TrimSpace(m[1])
		// Trim to the first sentence boundary.
		for _, ch := range []string{"。", "！", "？", "!", "?"} {
			if idx := strings.Index(scene, ch); idx >= 0 {
				scene = scene[:idx+len(ch)]
				break
			}
		}
		if scene != "" {
			parts = append(parts, "scene: "+scene)
		}
	}

	return strings.Join(parts, ", ")
}

func describeVideoStyle(stylePreset string) string {
	switch strings.TrimSpace(stylePreset) {
	case "realistic-drama":
		return "grounded realistic drama style, natural skin tone, restrained color palette, believable live-action framing"
	case "fashion-commercial":
		return "high-end fashion commercial style, refined live-action portrait lighting, premium fabric texture, polished beauty details"
	case "documentary-natural":
		return "documentary natural style, observational camera language, realistic environment detail, soft natural light"
	case "cinematic-epic":
		return "epic cinematic style, dramatic contrast lighting, grand scale framing, rich atmospheric depth"
	}
	switch stylepreset.Canonical(stylePreset) {
	case "anime-2d":
		return "2D anime style, clean hand-drawn linework, vibrant cel-shaded color, expressive character acting, bold keyframe poses, readable silhouette, flat stylized backgrounds"
	case "anime-3d":
		return "3D anime style, soft toon-shaded materials, dimensional character volume, stylised CG depth-of-field, smooth motion arcs, clear material contrast between characters and environment"
	case "live-action-film":
		return "live-action cinematic film style, ARRI Alexa anamorphic look, motivated three-point lighting, realistic subsurface skin scattering, premium costume fabric texture, true cinematic depth of field, no CGI artifacts"
	case "live-action-short":
		return "live-action short drama style, natural handheld intimacy, believable close-up performance, realistic skin tone with natural imperfections, grounded emotional framing, everyday environment detail"
	default:
		return fmt.Sprintf("%s style, smooth motion, coherent design", stylePreset)
	}
}

func describeVideoNegativePrompt(stylePreset string) string {
	parts := []string{
		// Visual glitch
		"flicker", "frame jitter", "strobing",
		// Identity / anatomy
		"identity drift", "warped anatomy", "bad hands",
		"extra fingers", "extra limbs", "duplicate subject",
		"morphing face", "character transformation",
		// Cross-clip coherence killers
		"style change between clips", "inconsistent color palette",
		"abrupt lighting change", "scene teleportation",
		"inconsistent costume", "costume color change",
		// Generic quality
		"text overlay", "subtitle burn-in", "watermark", "logo",
		"low quality", "blurry", "pixelated",
	}

	switch stylepreset.Canonical(stylePreset) {
	case "anime-2d", "anime-3d":
		parts = append(parts,
			"photorealistic skin",
			"live-action photography",
			"lens bokeh",
			"mixed art style",
		)
	case "live-action-film", "live-action-short":
		parts = append(parts,
			"anime", "cartoon", "illustration", "cel shading",
			"plastic CGI skin", "airbrushed skin",
			"uncanny valley face", "unnatural smooth skin",
			"latex skin", "over-saturated color grading",
			"neon glow", "color grade shift",
		)
	}

	return strings.Join(parts, ", ")
}

// buildGlobalStyleAnchor returns a short consistency prefix injected into every
// per-clip prompt to lock visual style, palette and character look across all clips.
// Falls back gracefully when info is sparse.
func buildGlobalStyleAnchor(stylePreset, charDescriptions string) string {
	styleCN := describeVideoStyleChinese(stylePreset)
	switch stylepreset.Canonical(stylePreset) {
	case "anime-2d":
		return fmt.Sprintf("【全局风格锁定】%s，全片保持统一配色方案与线条风格。%s", styleCN, charDescriptions)
	case "anime-3d":
		return fmt.Sprintf("【全局风格锁定】%s，全片材质与渲染风格保持一致。%s", styleCN, charDescriptions)
	case "live-action-film", "live-action-short":
		return fmt.Sprintf("【全局风格锁定】%s，全片色调、布光方案与演员外貌保持高度一致。%s", styleCN, charDescriptions)
	default:
		if charDescriptions != "" {
			return fmt.Sprintf("全片风格统一，保持一致的视觉基调。%s", charDescriptions)
		}
		return "全片风格统一，保持一致的视觉基调"
	}
}

// buildGlobalStyleAnchorEN is the English variant for Sora/Veo/generic models.
func buildGlobalStyleAnchorEN(stylePreset, charDescriptions string) string {
	style := describeVideoStyle(stylePreset)
	base := fmt.Sprintf("[Global Style Lock] %s; maintain identical color palette, lighting scheme, and art direction across all clips", style)
	if charDescriptions != "" {
		base += "; consistent character appearance: " + charDescriptions
	}
	return base
}

// videoModelFamily returns a canonical family name from a resolved model name.
// This is used to choose between Chinese and English prompt strategies.
//
// Chinese-optimised models: Kling, WAN/Wan2.1, Doubao, Vidu, Suanneng —
// trained predominantly on Chinese video content, respond better to Chinese-language scene descriptions.
//
// English-optimised models: Sora 2, Veo 3 — trained by OpenAI/Google with
// English as the primary prompt language.
func videoModelFamily(modelName string) string {
	lower := strings.ToLower(modelName)
	switch {
	case strings.Contains(lower, "kling"):
		return "kling"
	case strings.Contains(lower, "wan"):
		return "wan"
	case strings.Contains(lower, "sora"):
		return "sora"
	case strings.Contains(lower, "veo"):
		return "veo"
	case strings.Contains(lower, "doubao"), strings.Contains(lower, "seedream"), strings.Contains(lower, "seedance"):
		return "doubao"
	case strings.Contains(lower, "vidu"):
		return "vidu"
	case strings.Contains(lower, "suanneng"), strings.Contains(lower, "sophnet"):
		return "suanneng"
	case strings.Contains(lower, "gaga"):
		return "gaga"
	default:
		return "generic"
	}
}

// describeVideoStyleChinese returns a Chinese-language style description for
// Chinese video models such as Kling and WAN that respond better to Chinese prompts.
func describeVideoStyleChinese(stylePreset string) string {
	switch stylepreset.Canonical(stylePreset) {
	case "anime-2d":
		return "二维日式动漫风格，手绘线条，鲜艳赛璐璐配色，表情丰富，角色造型清晰"
	case "anime-3d":
		return "三维动漫风格，卡通质感材质，立体角色，CG渲染景深，流畅运动弧线"
	case "live-action-film":
		return "电影级真人影像，ARRI摄影机胶片质感，专业三点布光，真实肤感与细节，电影景深，无CG痕迹"
	case "live-action-short":
		return "真人短剧写实风格，自然手持镜头，近景表演，真实肤色与自然光影，日常生活氛围"
	default:
		return "高品质影像，流畅运动，画面清晰稳定"
	}
}

// motionPromptForModel is like motionPrompt but adds model-specific language selection:
// Chinese-language prompts for Kling/WAN/Doubao/Vidu/Suanneng models; English for Sora/Veo/generic.
func motionPromptForModel(motionMode, stylePreset, modelName, sceneDescription string, renderConfig model.RenderConfig) string {
	family := videoModelFamily(modelName)

	if family == "kling" || family == "wan" || family == "doubao" || family == "vidu" || family == "suanneng" {
		return motionPromptChinese(motionMode, stylePreset, sceneDescription, renderConfig)
	}
	return motionPrompt(motionMode, stylePreset, sceneDescription, renderConfig)
}

// motionPromptChinese builds a Chinese-language prompt for Kling/WAN models.
func motionPromptChinese(motionMode, stylePreset, sceneDescription string, renderConfig model.RenderConfig) string {
	stylePart := describeVideoStyleChinese(stylePreset)
	var basePart string
	switch motionMode {
	case "dynamic":
		basePart = "动感镜头运动，充满活力的动作节奏，清晰的动作方向"
	case "cinematic":
		basePart = "电影感缓慢推镜，控制节奏，优雅的镜头语言，胶片颗粒感"
	default:
		basePart = "轻柔运动，流畅过渡，克制的镜头漂移"
	}
	qualityPart := "主体身份稳定，骨骼结构清晰，光影自然，服装和头发运动真实，画面连贯流畅"
	renderPart := describeRenderConfig(renderConfig)
	negativePart := "避免：画面闪烁，帧抖动，身份漂移，扭曲肢体，多余手指，文字叠加，水印"

	parts := []string{stylePart, basePart, qualityPart}
	if sceneDescription != "" {
		parts = append(parts, "场景："+sceneDescription)
	}
	if renderPart != "" {
		parts = append(parts, renderPart)
	}
	parts = append(parts, negativePart)
	return strings.Join(parts, "，")
}

func describeRenderConfig(renderConfig model.RenderConfig) string {
	if len(renderConfig) == 0 {
		return ""
	}
	segments := make([]string, 0, 4)
	switch renderConfigString(renderConfig, "frame_size") {
	case "portrait-9-16":
		segments = append(segments, "portrait 9:16 composition, optimized for vertical mobile framing")
	case "landscape-16-9":
		segments = append(segments, "landscape 16:9 composition, cinematic horizontal framing")
	case "square-1-1":
		segments = append(segments, "square 1:1 composition, centered balanced framing")
	case "ultrawide-21-9":
		segments = append(segments, "ultra wide 21:9 composition, panoramic cinematic framing")
	}
	switch renderConfigString(renderConfig, "subject_size") {
	case "close-up":
		segments = append(segments, "close-up framing, large subject presence, emphasize facial and texture details")
	case "medium-shot":
		segments = append(segments, "medium shot framing, balanced subject and environment")
	case "wide-shot":
		segments = append(segments, "wide shot framing, smaller subject scale, emphasize world-building and scene depth")
	}
	switch renderConfigString(renderConfig, "clarity") {
	case "standard":
		segments = append(segments, "natural clarity, balanced detail")
	case "high":
		segments = append(segments, "high clarity, crisp edges, refined texture detail")
	case "ultra":
		segments = append(segments, "ultra clear, sharp focus, premium fine detail")
	}
	if customPrompt := renderConfigString(renderConfig, "custom_prompt"); customPrompt != "" {
		segments = append(segments, customPrompt)
	}
	return strings.Join(segments, ", ")
}

func renderConfigString(renderConfig model.RenderConfig, key string) string {
	if len(renderConfig) == 0 {
		return ""
	}
	value, ok := renderConfig[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

// renderConfigInt reads an integer value (or numeric float) from RenderConfig.
func renderConfigInt(renderConfig model.RenderConfig, key string) int {
	if len(renderConfig) == 0 {
		return 0
	}
	switch v := renderConfig[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n := 0
		fmt.Sscanf(v, "%d", &n)
		return n
	}
	return 0
}

// renderConfigBool reads a boolean value from RenderConfig (supports bool, "true"/"1").
func renderConfigBool(renderConfig model.RenderConfig, key string) bool {
	if len(renderConfig) == 0 {
		return false
	}
	switch v := renderConfig[key].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	case float64:
		return v != 0
	}
	return false
}

// fetchCharacterImages calls character-service to retrieve generated character
// asset image URLs for the given project. These are passed to video generators
// as subject/character reference images to improve cross-clip consistency.
// Returns an empty slice on any error — caller continues without references.
func (s *VideoService) fetchCharacterImages(ctx context.Context, projectID int64) []string {
	urls, _ := s.fetchCharacterImageMap(ctx, projectID)
	return urls
}

// fetchCharacterImageMap returns both the ordered URL list and a name→URL map
// for per-scene character reference filtering. The map is keyed by both the
// raw character/asset name and a normalized lowercase-trimmed form so scene
// character names from the storyboard (which may differ in casing/whitespace
// or be in Chinese) still match the asset records.
func (s *VideoService) fetchCharacterImageMap(ctx context.Context, projectID int64) ([]string, map[string]string) {
	if s.characterBaseURL == "" {
		return nil, nil
	}

	var urls []string
	byName := make(map[string]string)
	seen := make(map[string]struct{})

	addName := func(name, url string) {
		raw := strings.TrimSpace(name)
		if raw == "" || url == "" {
			return
		}
		if _, ok := byName[raw]; !ok {
			byName[raw] = url
		}
		lower := strings.ToLower(raw)
		if _, ok := byName[lower]; !ok {
			byName[lower] = url
		}
	}

	// Paginate through all completed character assets. Cap at 500 to bound
	// worst-case memory; projects are expected to have far fewer.
	const pageSize = 100
	const maxPages = 5
	for page := 1; page <= maxPages; page++ {
		url := fmt.Sprintf("%s/api/v1/projects/%d/assets?type=character&status=completed&page=%d&page_size=%d",
			s.characterBaseURL, projectID, page, pageSize)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			break
		}
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			s.logger.Warn("fetch character images failed", zap.Int64("project_id", projectID), zap.Error(err))
			break
		}
		var result struct {
			Data struct {
				Items []struct {
					Name        string `json:"name"`
					ImageURL    string `json:"image_url"`
					CharacterID *int64 `json:"character_id,omitempty"`
				} `json:"items"`
			} `json:"data"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || decodeErr != nil {
			break
		}
		if len(result.Data.Items) == 0 {
			break
		}
		for _, item := range result.Data.Items {
			if item.ImageURL == "" {
				continue
			}
			if _, dup := seen[item.ImageURL]; !dup {
				seen[item.ImageURL] = struct{}{}
				urls = append(urls, item.ImageURL)
			}
			addName(item.Name, item.ImageURL)
		}
		if len(result.Data.Items) < pageSize {
			break
		}
	}

	// Also look up Character.Name → asset image by matching CharacterID — the
	// Asset.Name and Character.Name can diverge (asset may be renamed per-style),
	// but scene_characters in storyboards references Character.Name.
	if s.characterBaseURL != "" {
		charURL := fmt.Sprintf("%s/api/v1/characters?project_id=%d&page=1&page_size=200", s.characterBaseURL, projectID)
		if req, err := http.NewRequestWithContext(ctx, http.MethodGet, charURL, nil); err == nil {
			client := &http.Client{Timeout: 5 * time.Second}
			if resp, err := client.Do(req); err == nil {
				var cr struct {
					Data struct {
						Items []struct {
							ID                int64  `json:"id"`
							Name              string `json:"name"`
							ReferenceImageURL string `json:"reference_image_url"`
						} `json:"items"`
					} `json:"data"`
				}
				if resp.StatusCode == http.StatusOK {
					_ = json.NewDecoder(resp.Body).Decode(&cr)
				}
				resp.Body.Close()
				// Seed byName from Character table entries as well. ReferenceImageURL
				// is the character's "canonical" image (from Character record); use it
				// if we don't already have a URL for this name.
				for _, it := range cr.Data.Items {
					if it.ReferenceImageURL == "" || it.Name == "" {
						continue
					}
					if _, dup := seen[it.ReferenceImageURL]; !dup {
						seen[it.ReferenceImageURL] = struct{}{}
						urls = append(urls, it.ReferenceImageURL)
					}
					addName(it.Name, it.ReferenceImageURL)
				}
			}
		}
	}

	if len(urls) > 0 {
		s.logger.Info("fetched character reference images",
			zap.Int64("project_id", projectID), zap.Int("count", len(urls)), zap.Int("names", len(byName)))
	}
	return urls, byName
}

// fetchCharacterDescriptions calls character-service to retrieve appearance
// descriptions for all characters in the project. Returns a semicolon-joined
// string suitable for appending to clip motion prompts, or "" on error.
// Prefers Asset.PromptUsed (which already bakes in era/region/ethnicity/gender
// wardrobe constraints) over raw Character.AppearanceDesc so that video motion
// prompts stay aligned with the storyboard prompts produced by Fix E.
func (s *VideoService) fetchCharacterDescriptions(ctx context.Context, projectID int64) string {
	if s.characterBaseURL == "" {
		return ""
	}

	type namedDesc struct {
		name string
		desc string
	}

	byKey := make(map[string]namedDesc)
	addEntry := func(name, desc string) {
		name = strings.TrimSpace(name)
		desc = strings.TrimSpace(desc)
		if name == "" || desc == "" {
			return
		}
		key := strings.ToLower(name)
		// Prefer longer / more detailed descriptions (Asset.PromptUsed typically
		// much richer than raw Character.AppearanceDesc).
		if existing, ok := byKey[key]; !ok || len(desc) > len(existing.desc) {
			byKey[key] = namedDesc{name: name, desc: desc}
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// First, walk character assets and use PromptUsed where available.
	const pageSize = 100
	const maxPages = 5
	for page := 1; page <= maxPages; page++ {
		url := fmt.Sprintf("%s/api/v1/projects/%d/assets?type=character&status=completed&page=%d&page_size=%d",
			s.characterBaseURL, projectID, page, pageSize)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			break
		}
		resp, err := client.Do(req)
		if err != nil {
			break
		}
		var ar struct {
			Data struct {
				Items []struct {
					Name       string `json:"name"`
					PromptUsed string `json:"prompt_used"`
				} `json:"items"`
			} `json:"data"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&ar)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || decodeErr != nil {
			break
		}
		if len(ar.Data.Items) == 0 {
			break
		}
		for _, it := range ar.Data.Items {
			addEntry(it.Name, it.PromptUsed)
		}
		if len(ar.Data.Items) < pageSize {
			break
		}
	}

	// Second, fall back to Character.AppearanceDesc for any character not yet
	// covered by an asset's PromptUsed.
	charURL := fmt.Sprintf("%s/api/v1/characters?project_id=%d&page=1&page_size=200", s.characterBaseURL, projectID)
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet, charURL, nil); err == nil {
		if resp, err := client.Do(req); err == nil {
			var cr struct {
				Data struct {
					Items []struct {
						Name           string `json:"name"`
						AppearanceDesc string `json:"appearance_desc"`
					} `json:"items"`
				} `json:"data"`
			}
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				_ = json.Unmarshal(body, &cr)
			}
			resp.Body.Close()
			for _, it := range cr.Data.Items {
				// Only add if we don't already have a richer Asset-derived entry.
				addEntry(it.Name, it.AppearanceDesc)
			}
		} else {
			s.logger.Warn("fetch character descriptions failed", zap.Int64("project_id", projectID), zap.Error(err))
		}
	}

	if len(byKey) == 0 {
		return ""
	}
	parts := make([]string, 0, len(byKey))
	for _, v := range byKey {
		parts = append(parts, v.name+": "+v.desc)
	}
	return strings.Join(parts, "; ")
}

// parseSubtitleStyle reads the optional "subtitle_style" key from RenderConfig and
// converts it into a SubtitleStyle. All fields are optional; zero values use defaults.
func parseSubtitleStyle(rc model.RenderConfig) SubtitleStyle {
	raw, ok := rc["subtitle_style"]
	if !ok {
		return SubtitleStyle{}
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return SubtitleStyle{}
	}
	style := SubtitleStyle{}
	if v, ok := m["font_name"].(string); ok {
		style.FontName = v
	}
	if v, ok := m["font_color"].(string); ok {
		style.FontColor = v
	}
	if v, ok := m["outline_color"].(string); ok {
		style.OutlineColor = v
	}
	if v, ok := m["font_size"].(float64); ok {
		style.FontSize = int(v)
	}
	if v, ok := m["outline_width"].(float64); ok {
		style.OutlineWidth = int(v)
	}
	if v, ok := m["alignment"].(float64); ok {
		style.Alignment = int(v)
	}
	if v, ok := m["margin_v"].(float64); ok {
		style.MarginV = int(v)
	}
	if v, ok := m["bold"].(bool); ok {
		style.Bold = v
	}
	return style
}

// parseFrameTemplate reads the "frame_template" map from RenderConfig and returns
// a *VideoFrameTemplate for opt-p2 overlay support. Returns nil when not configured.
func parseFrameTemplate(rc model.RenderConfig) *VideoFrameTemplate {
	raw, ok := rc["frame_template"]
	if !ok {
		return nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	tpl := &VideoFrameTemplate{}
	if v, ok := m["title_text"].(string); ok {
		tpl.TitleText = v
	}
	if v, ok := m["title_font"].(string); ok {
		tpl.TitleFont = v
	}
	if v, ok := m["title_size"].(float64); ok {
		tpl.TitleSize = int(v)
	}
	if v, ok := m["title_color"].(string); ok {
		tpl.TitleColor = v
	}
	if v, ok := m["caption_text"].(string); ok {
		tpl.CaptionText = v
	}
	if v, ok := m["caption_font"].(string); ok {
		tpl.CaptionFont = v
	}
	if v, ok := m["caption_size"].(float64); ok {
		tpl.CaptionSize = int(v)
	}
	if v, ok := m["caption_color"].(string); ok {
		tpl.CaptionColor = v
	}
	if v, ok := m["watermark_text"].(string); ok {
		tpl.WatermarkText = v
	}
	if v, ok := m["watermark_color"].(string); ok {
		tpl.WatermarkColor = v
	}
	if v, ok := m["watermark_size"].(float64); ok {
		tpl.WatermarkSize = int(v)
	}
	if v, ok := m["logo_url"].(string); ok {
		tpl.LogoURL = v
	}
	if v, ok := m["logo_width"].(float64); ok {
		tpl.LogoWidth = int(v)
	}
	if tpl.TitleText == "" && tpl.CaptionText == "" && tpl.WatermarkText == "" && tpl.LogoURL == "" {
		return nil
	}
	return tpl
}

// clipDuration returns d if it is a positive value, otherwise falls back to 5 seconds.
func clipDuration(d float64) float64 {
	if d > 0 {
		return d
	}
	return 5
}

// snapDurationToModel snaps dur to the nearest valid duration accepted by the model family.
// Each model's API only accepts specific integer values; passing an unsupported value causes rejection.
//   - kling:         5 or 10
//   - wan:           5 (fixed)
//   - vidu/vidu-mix: 4 or 8
//   - doubao:        5, 8, or 10
//   - suanneng:      5, 8, or 10
//   - others:        passthrough (clamped to [3, 20])
func snapDurationToModel(dur float64, family string) float64 {
	snapToNearest := func(v float64, allowed []float64) float64 {
		best := allowed[0]
		for _, a := range allowed[1:] {
			if abs(v-a) < abs(v-best) {
				best = a
			}
		}
		return best
	}
	switch family {
	case "kling":
		return snapToNearest(dur, []float64{5, 10})
	case "wan":
		return 5
	case "vidu":
		return snapToNearest(dur, []float64{4, 8})
	case "doubao":
		return snapToNearest(dur, []float64{5, 8, 10})
	case "suanneng":
		return snapToNearest(dur, []float64{5, 8, 10})
	default:
		if dur < 3 {
			return 3
		}
		if dur > 20 {
			return 20
		}
		return dur
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// isTransientVideoError reports whether the error is a temporary condition
// that is safe to retry (network timeouts, rate limits, transient server errors).
func isTransientVideoError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"timeout",
		"timed out",
		"context deadline exceeded",
		"connection reset",
		"connection refused",
		"broken pipe",
		"unexpected eof",
		"eof",
		"no such host",
		"i/o timeout",
		"tls handshake timeout",
		"temporary failure",
		"503",
		"502",
		"429",
		"rate limit",
		"requestlimitexceeded",
		"jobnumexceed",
		"请稍后重试",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// generateClipWithRetry calls gen.Generate, retrying up to maxAttempts times
// with exponential backoff when a transient error is encountered.
func generateClipWithRetry(ctx context.Context, gen generators.VideoGenerator, req generators.VideoGenerateReq, maxAttempts int, logger *zap.Logger, clipID int64) (*generators.VideoClip, error) {
	var lastErr error
	backoff := 5 * time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Bail immediately if the task context has been cancelled or timed out.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		result, err := gen.Generate(ctx, req)
		if err == nil {
			return result, nil
		}
		lastErr = err
		// Do not retry if the error is due to the parent context expiring — retrying
		// would just fail again immediately.
		if ctx.Err() != nil || !isTransientVideoError(err) || attempt == maxAttempts {
			break
		}
		// API concurrent-job limits (e.g. Tencent VCLM free tier) require waiting
		// for the previous job to complete before submitting a new one. Use a long
		// initial backoff so the phantom job has time to finish.
		if isJobLimitError(err) && backoff < 2*time.Minute {
			backoff = 2 * time.Minute
		}
		logger.Warn("clip generation transient error, retrying",
			zap.Int64("clip_id", clipID),
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 5*time.Minute {
			backoff = 5 * time.Minute
		}
	}
	return nil, lastErr
}

// isJobLimitError reports whether the error is a concurrent-job limit from a remote API.
func isJobLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{"requestlimitexceeded", "jobnumexceed", "请稍后重试"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
