package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/autovideo/pipeline-service/internal/model"
)

// ─── StageScriptAnalyzing ─────────────────────────────────────────────────────

// runScriptAnalyzing 触发 script-service 执行 LLM 分析，轮询直到完成
func (s *PipelineService) runScriptAnalyzing(ctx context.Context, state *model.PipelineState) error {
	if err := s.httpClient.TriggerScriptAnalyze(ctx, state.ScriptID); err != nil {
		return fmt.Errorf("trigger script analyze: %w", err)
	}

	timeout := time.After(10 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("script analyzing timed out after 10 minutes")
		case <-ticker.C:
			if s.isAborted(state.ID) {
				return fmt.Errorf("aborted")
			}

			script, err := s.httpClient.GetScript(ctx, state.ScriptID)
			if err != nil {
				s.appendLog(state, fmt.Sprintf("[SCRIPT_ANALYZING] 轮询出错: %v，继续等待", err))
				continue
			}

			switch script.ParseStatus {
			case "done":
				state.Progress = 10
				s.saveState(state)
				return nil
			case "failed":
				return fmt.Errorf("script parse failed")
			default:
				s.appendLog(state, fmt.Sprintf("[SCRIPT_ANALYZING] 状态: %s，继续等待...", script.ParseStatus))
				s.saveState(state)
			}
		}
	}
}

// ─── StageAssetsExtracting ────────────────────────────────────────────────────

// runAssetsExtracting 从分析结果中提取角色和场景
func (s *PipelineService) runAssetsExtracting(ctx context.Context, state *model.PipelineState) error {
	// 1. 获取并创建角色
	characters, err := s.httpClient.GetScriptCharacters(ctx, state.ScriptID)
	if err != nil {
		return fmt.Errorf("get script characters: %w", err)
	}

	for _, ch := range characters {
		req := model.CreateCharacterReq{
			ProjectID:      state.ProjectID,
			Name:           ch.Name,
			RoleDesc:       ch.RoleDesc,
			AppearanceDesc: ch.AppearanceDesc,
			StylePreset:    state.Config.StylePreset,
		}
		charID, err := s.httpClient.CreateCharacter(ctx, req)
		if err != nil {
			s.appendLog(state, fmt.Sprintf("[ASSETS_EXTRACTING] 创建角色 %s 失败: %v", ch.Name, err))
			continue
		}
		s.appendLog(state, fmt.Sprintf("[ASSETS_EXTRACTING] 角色 %s 创建成功 id=%d", ch.Name, charID))

		if s.isAborted(state.ID) {
			return fmt.Errorf("aborted")
		}
	}

	// 2. 获取场景列表
	scenes, err := s.httpClient.GetScriptScenes(ctx, state.ScriptID)
	if err != nil {
		return fmt.Errorf("get script scenes: %w", err)
	}
	if len(scenes) == 0 {
		return fmt.Errorf("no scenes found for script %d", state.ScriptID)
	}

	sceneIDs := make([]int64, 0, len(scenes))
	for _, sc := range scenes {
		sceneIDs = append(sceneIDs, sc.ID)
	}
	state.SceneIDs = sceneIDs
	state.Progress = 25
	s.appendLog(state, fmt.Sprintf("[ASSETS_EXTRACTING] 共提取 %d 个场景，%d 个角色", len(scenes), len(characters)))
	return nil
}

// ─── StageStoryboardGen ───────────────────────────────────────────────────────

// runStoryboardGen 分镜生成阶段
// prompt_draft 已由 script-service LLM 生成，quality 模式下可做进一步优化（本实现直接使用）
func (s *PipelineService) runStoryboardGen(ctx context.Context, state *model.PipelineState) error {
	s.appendLog(state, fmt.Sprintf("[STORYBOARD_GEN] 共 %d 个场景，使用 script-service 生成的 prompt_draft", len(state.SceneIDs)))
	state.Progress = 35
	return nil
}

// ─── StageImagesGenerating ────────────────────────────────────────────────────

// runImagesGenerating 并发批量生成图片
func (s *PipelineService) runImagesGenerating(ctx context.Context, state *model.PipelineState) error {
	imageModel := resolveImageModel(state.Config.ImageModel, state.Config.QualityMode)
	s.appendLog(state, fmt.Sprintf("[IMAGES_GEN] 使用模型: %s，并发数: %d", imageModel, state.Config.MaxConcurrent))

	maxConcurrent := state.Config.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	taskIDs := make([]int64, 0, len(state.SceneIDs))
	var firstErr error

	for _, sceneID := range state.SceneIDs {
		if s.isAborted(state.ID) {
			break
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(sid int64) {
			defer wg.Done()
			defer func() { <-sem }()

			// 获取场景详情，取 prompt_draft
			scene, err := s.httpClient.GetScene(ctx, sid)
			if err != nil {
				mu.Lock()
				s.appendLog(state, fmt.Sprintf("[IMAGES_GEN] 获取场景 %d 失败: %v", sid, err))
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			prompt := scene.PromptDraft
			if prompt == "" {
				prompt = scene.Description
			}

			req := model.CreateImageReq{
				SceneID:     sid,
				Prompt:      prompt,
				Model:       imageModel,
				StylePreset: state.Config.StylePreset,
				ProjectID:   state.ProjectID,
			}

			taskID, err := s.httpClient.CreateImageTask(ctx, req)
			if err != nil {
				mu.Lock()
				s.appendLog(state, fmt.Sprintf("[IMAGES_GEN] 场景 %d 提交图片任务失败: %v", sid, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			taskIDs = append(taskIDs, taskID)
			mu.Unlock()
		}(sceneID)
	}
	wg.Wait()

	if s.isAborted(state.ID) {
		return fmt.Errorf("aborted")
	}

	state.ImageTaskIDs = taskIDs
	s.appendLog(state, fmt.Sprintf("[IMAGES_GEN] 共提交 %d 个图片任务，开始轮询状态...", len(taskIDs)))
	s.saveState(state)

	// 轮询等待所有图片任务完成
	pollTimeout := time.After(30 * time.Minute)
	pollTicker := time.NewTicker(10 * time.Second)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pollTimeout:
			return fmt.Errorf("image generation timed out after 30 minutes")
		case <-pollTicker.C:
			if s.isAborted(state.ID) {
				return fmt.Errorf("aborted")
			}

			allDone := true
			succeeded := 0
			for _, tid := range state.ImageTaskIDs {
				task, err := s.httpClient.GetImageTask(ctx, tid)
				if err != nil {
					allDone = false
					continue
				}
				switch task.Status {
				case "succeeded":
					succeeded++
				case "failed":
					// 留给 review 阶段处理
				case "pending", "running":
					allDone = false
				}
			}

			total := len(state.ImageTaskIDs)
			if total > 0 {
				state.Progress = 35 + int(float64(succeeded)/float64(total)*40)
				s.saveState(state)
			}

			s.appendLog(state, fmt.Sprintf("[IMAGES_GEN] 进度: %d/%d 完成", succeeded, total))

			if allDone {
				return nil
			}
		}
	}
}

// ─── StageImagesReviewing ─────────────────────────────────────────────────────

// runImagesReviewing 检查失败任务并自动重试一次
func (s *PipelineService) runImagesReviewing(ctx context.Context, state *model.PipelineState) error {
	retried := 0
	for _, tid := range state.ImageTaskIDs {
		if s.isAborted(state.ID) {
			return fmt.Errorf("aborted")
		}

		task, err := s.httpClient.GetImageTask(ctx, tid)
		if err != nil {
			s.appendLog(state, fmt.Sprintf("[IMAGES_REVIEW] 查询任务 %d 失败: %v", tid, err))
			continue
		}

		if task.Status == "failed" {
			s.appendLog(state, fmt.Sprintf("[IMAGES_REVIEW] 任务 %d 失败，触发重试", tid))
			if err := s.httpClient.RetryImageTask(ctx, tid); err != nil {
				s.appendLog(state, fmt.Sprintf("[IMAGES_REVIEW] 重试任务 %d 失败: %v", tid, err))
				continue
			}
			retried++
			// 等待30秒再检查
			time.Sleep(30 * time.Second)

			task2, err := s.httpClient.GetImageTask(ctx, tid)
			if err == nil {
				s.appendLog(state, fmt.Sprintf("[IMAGES_REVIEW] 任务 %d 重试后状态: %s", tid, task2.Status))
			}
		}
	}

	s.appendLog(state, fmt.Sprintf("[IMAGES_REVIEW] 共重试 %d 个失败任务", retried))
	state.Progress = 75
	return nil
}

// ─── StageVideosGenerating ────────────────────────────────────────────────────

// runVideosGenerating 按 episode 分组，批量生成视频任务
func (s *PipelineService) runVideosGenerating(ctx context.Context, state *model.PipelineState) error {
	videoModel := resolveVideoModel(state.Config.VideoModel, state.Config.QualityMode)
	s.appendLog(state, fmt.Sprintf("[VIDEOS_GEN] 使用模型: %s", videoModel))

	// 获取所有成功的 image task，按 episode 分组
	episodeImages := make(map[int64][]int64) // episodeID -> []imageTaskID

	for _, tid := range state.ImageTaskIDs {
		task, err := s.httpClient.GetImageTask(ctx, tid)
		if err != nil {
			continue
		}
		if task.Status != "succeeded" {
			continue
		}

		// 从 scene 获取 episodeID
		scene, err := s.httpClient.GetScene(ctx, task.SceneID)
		if err != nil {
			// 无法获取 episode，归到默认分组 0
			episodeImages[0] = append(episodeImages[0], tid)
			continue
		}
		episodeImages[scene.EpisodeID] = append(episodeImages[scene.EpisodeID], tid)
	}

	if len(episodeImages) == 0 {
		return fmt.Errorf("no succeeded image tasks to compose into video")
	}

	videoTaskIDs := make([]int64, 0, len(episodeImages))

	for episodeID, imgTaskIDs := range episodeImages {
		if s.isAborted(state.ID) {
			return fmt.Errorf("aborted")
		}

		req := model.CreateVideoReq{
			ProjectID:    state.ProjectID,
			EpisodeID:    episodeID,
			ImageTaskIDs: imgTaskIDs,
			Model:        videoModel,
			StylePreset:  state.Config.StylePreset,
		}

		taskID, err := s.httpClient.CreateVideoTask(ctx, req)
		if err != nil {
			s.appendLog(state, fmt.Sprintf("[VIDEOS_GEN] 集 %d 创建视频任务失败: %v", episodeID, err))
			continue
		}
		videoTaskIDs = append(videoTaskIDs, taskID)
		s.appendLog(state, fmt.Sprintf("[VIDEOS_GEN] 集 %d 视频任务已提交 task_id=%d", episodeID, taskID))
	}

	state.VideoTaskIDs = videoTaskIDs
	s.saveState(state)

	// 轮询等待视频任务完成
	pollTimeout := time.After(60 * time.Minute)
	pollTicker := time.NewTicker(15 * time.Second)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pollTimeout:
			return fmt.Errorf("video generation timed out after 60 minutes")
		case <-pollTicker.C:
			if s.isAborted(state.ID) {
				return fmt.Errorf("aborted")
			}

			allDone := true
			succeeded := 0
			for _, tid := range state.VideoTaskIDs {
				task, err := s.httpClient.GetVideoTask(ctx, tid)
				if err != nil {
					allDone = false
					continue
				}
				switch task.Status {
				case "succeeded":
					succeeded++
				case "failed":
					// 不重试，记录到报告
				case "pending", "running":
					allDone = false
				}
			}

			s.appendLog(state, fmt.Sprintf("[VIDEOS_GEN] 视频进度: %d/%d", succeeded, len(state.VideoTaskIDs)))

			if allDone {
				state.Progress = 85
				return nil
			}
		}
	}
}

// ─── StageComposing ───────────────────────────────────────────────────────────

// runComposing 触发 FFmpeg 合成
func (s *PipelineService) runComposing(ctx context.Context, state *model.PipelineState) error {
	for _, vid := range state.VideoTaskIDs {
		if s.isAborted(state.ID) {
			return fmt.Errorf("aborted")
		}

		if err := s.httpClient.ComposeVideo(ctx, vid); err != nil {
			s.appendLog(state, fmt.Sprintf("[COMPOSING] 视频 %d 合成触发失败: %v", vid, err))
			continue
		}
		s.appendLog(state, fmt.Sprintf("[COMPOSING] 视频 %d 合成已触发", vid))
	}

	state.Progress = 92
	return nil
}

// ─── StageAutoFixing ─────────────────────────────────────────────────────────

// runAutoFixing 生成汇总报告，统计成功/失败数量
func (s *PipelineService) runAutoFixing(ctx context.Context, state *model.PipelineState) error {
	report := &model.PipelineReport{
		TotalScenes:  len(state.SceneIDs),
		FailedScenes: make([]int64, 0),
	}

	var totalDuration float64

	for _, tid := range state.ImageTaskIDs {
		task, err := s.httpClient.GetImageTask(ctx, tid)
		if err != nil {
			report.FailedImages++
			continue
		}
		switch task.Status {
		case "succeeded":
			report.SuccessImages++
		default:
			report.FailedImages++
			report.FailedScenes = append(report.FailedScenes, task.SceneID)
		}
	}

	for _, tid := range state.VideoTaskIDs {
		task, err := s.httpClient.GetVideoTask(ctx, tid)
		if err != nil {
			continue
		}
		totalDuration += task.Duration
	}

	report.TotalDuration = totalDuration
	state.Report = report
	state.Progress = 98

	s.appendLog(state, fmt.Sprintf(
		"[AUTO_FIX] 报告: 场景总数=%d，图片成功=%d，图片失败=%d，视频总时长=%.1fs",
		report.TotalScenes, report.SuccessImages, report.FailedImages, report.TotalDuration,
	))

	return nil
}

// ─── 辅助函数 ─────────────────────────────────────────────────────────────────

// resolveImageModel —— 根据用户配置和质量模式自动选择图片生成模型，返回模型名称
func resolveImageModel(imageModel, qualityMode string) string {
	if imageModel != "auto" && imageModel != "" {
		return imageModel
	}
	switch qualityMode {
	case "quality":
		return "flux"
	case "speed", "cost":
		return "sdxl"
	default:
		return "sdxl"
	}
}

// resolveVideoModel —— 根据用户配置和质量模式自动选择视频生成模型，返回模型名称
func resolveVideoModel(videoModel, qualityMode string) string {
	if videoModel != "auto" && videoModel != "" {
		return videoModel
	}
	switch qualityMode {
	case "quality":
		return "kling"
	case "cost":
		return "cogvideo"
	default:
		return "wan"
	}
}
