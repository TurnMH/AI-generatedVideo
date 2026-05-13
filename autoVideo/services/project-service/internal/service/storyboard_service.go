package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/autovideo/project-service/internal/model"
	"github.com/autovideo/project-service/internal/repository"
)

const (
	maxVersionsPerStoryboard     = 4
	defaultStoryboardMaxInFlight = 24
)

type storyboardGenerationScope struct {
	EpisodeID  *uint64
	ModelName  string
	ModelNames []string
}

type StoryboardService struct {
	repo               *repository.StoryboardRepo
	kafkaProducer      *KafkaProducer
	logger             *zap.Logger
	maxInFlight        int
	generationScopes   sync.Map
	pausedProjects     sync.Map
	auditor            *PromptAuditorService
	continuityAuditor  *SceneContinuityAuditor
}

// NewStoryboardService —— 创建分镜服务实例
func NewStoryboardService(repo *repository.StoryboardRepo) *StoryboardService {
	return &StoryboardService{
		repo:        repo,
		logger:      zap.NewNop(),
		maxInFlight: defaultStoryboardMaxInFlight,
		auditor:     NewPromptAuditorService("", "", "", nil, nil),
	}
}

// DeleteByProjectID —— 删除项目下所有分镜及其版本
// DeleteByProjectID removes all storyboards for a project.
func (s *StoryboardService) DeleteByProjectID(projectID uint64) error {
	return s.repo.DeleteByProjectID(projectID)
}

// SetKafkaProducer —— 注入 Kafka 生产者依赖，用于异步图片生成
// SetKafkaProducer wires up the Kafka producer for async generation.
func (s *StoryboardService) SetKafkaProducer(p *KafkaProducer) {
	s.kafkaProducer = p
}

// SetLogger —— 设置分镜服务的日志记录器
// SetLogger sets the logger for the service.
func (s *StoryboardService) SetLogger(l *zap.Logger) {
	s.logger = l
}

func (s *StoryboardService) SetMaxInFlight(max int) {
	if max <= 0 {
		max = defaultStoryboardMaxInFlight
	}
	s.maxInFlight = max
}

// SetContinuityAuditor wires up the scene continuity auditor.
func (s *StoryboardService) SetContinuityAuditor(a *SceneContinuityAuditor) {
	s.continuityAuditor = a
}

// AuditSceneContinuity fetches all non-voided storyboards for a project/episode,
// groups them by scene_group_key (or location), sends each group to the LLM
// continuity auditor, and writes the patches back to the database.
// Returns the number of patched storyboards and a flat list of all patches.
func (s *StoryboardService) AuditSceneContinuity(ctx context.Context, projectID uint64, episodeID *uint64) (*ContinuityAuditResult, error) {
	if s.continuityAuditor == nil {
		return nil, fmt.Errorf("continuity auditor not configured")
	}

	// Fetch all active storyboards.
	sbs, err := s.repo.FindAllActive(projectID, episodeID)
	if err != nil {
		return nil, fmt.Errorf("fetch storyboards for audit: %w", err)
	}
	if len(sbs) == 0 {
		return &ContinuityAuditResult{}, nil
	}

	// Convert to audit view.
	// Use scene_group_key as "location" hint so the grouper respects serial grouping.
	inputs := make([]StoryboardForAudit, 0, len(sbs))
	for _, sb := range sbs {
		loc := sb.Location
		if sb.SceneGroupKey != "" {
			loc = sb.SceneGroupKey
		}
		inputs = append(inputs, StoryboardForAudit{
			ID:               sb.ID,
			Seq:              sb.SequenceNumber,
			SceneDescription: sb.SceneDescription,
			Characters:       []string(sb.Characters),
			Location:         loc,
			Mood:             sb.Mood,
			CameraMovement:   sb.CameraMovement,
		})
	}

	// Group by scene_group_key (serial projects) or location (non-serial projects).
	groups := GroupStoryboardsBySceneKey(inputs)

	// Build an id→storyboard pointer map for fast patch application.
	sbByID := make(map[uint64]*model.Storyboard, len(sbs))
	for i := range sbs {
		sbByID[sbs[i].ID] = &sbs[i]
	}

	var allPatches []ContinuityPatch
	for groupKey, groupSBs := range groups {
		patches, err := s.continuityAuditor.AuditGroup(ctx, groupKey, groupSBs)
		if err != nil {
			// Log and continue; a single group failure must not abort the whole audit.
			s.logger.Warn("continuity audit group failed",
				zap.String("group", groupKey),
				zap.Error(err),
			)
			continue
		}
		allPatches = append(allPatches, patches...)
	}

	// Apply patches to the database.
	patched := 0
	for _, p := range allPatches {
		sb, ok := sbByID[p.ID]
		if !ok {
			continue
		}
		applied := false
		if strings.TrimSpace(p.SceneDescription) != "" {
			sb.SceneDescription = p.SceneDescription
			sb.PromptUsed = "" // clear so the next generation re-translates
			applied = true
		}
		if len(p.Characters) > 0 {
			sb.Characters = p.Characters
			applied = true
		}
		// Only overwrite location when the original storyboard had none.
		if strings.TrimSpace(p.Location) != "" && strings.TrimSpace(sb.Location) == "" {
			sb.Location = p.Location
			applied = true
		}
		if strings.TrimSpace(p.Mood) != "" && strings.TrimSpace(sb.Mood) == "" {
			sb.Mood = p.Mood
			applied = true
		}
		if applied {
			sb.UpdatedAt = time.Now()
			if err := s.repo.Update(sb); err != nil {
				s.logger.Warn("continuity audit patch write failed",
					zap.Uint64("storyboard_id", p.ID),
					zap.Error(err),
				)
				continue
			}
			patched++
		}
	}

	return &ContinuityAuditResult{
		TotalGroups:  len(groups),
		TotalPatched: patched,
		Patches:      allPatches,
	}, nil
}

func (s *StoryboardService) isProjectGenerationPaused(projectID uint64) bool {
	paused, ok := s.pausedProjects.Load(projectID)
	return ok && paused == true
}

func cloneUint64Ptr(v *uint64) *uint64 {
	if v == nil {
		return nil
	}
	copy := *v
	return &copy
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}


func normalizeStoryboardModelNames(modelName string, modelNames []string) []string {
	normalized := make([]string, 0, len(modelNames)+1)
	seen := make(map[string]struct{}, len(modelNames)+1)
	appendModel := func(name string) {
		trimmed := strings.TrimSpace(name)
		key := trimmed
		if trimmed != "" {
			key = strings.ToLower(trimmed)
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	for _, name := range modelNames {
		appendModel(name)
	}
	if len(normalized) == 0 {
		appendModel(modelName)
	}
	if len(normalized) == 0 {
		normalized = append(normalized, "")
	}
	return normalized

}

func cloneStoryboardGenerationScope(scope storyboardGenerationScope) storyboardGenerationScope {
	return storyboardGenerationScope{
		EpisodeID:  cloneUint64Ptr(scope.EpisodeID),
		ModelName:  strings.TrimSpace(scope.ModelName),
		ModelNames: cloneStringSlice(scope.ModelNames),
	}

}

func (s *StoryboardService) setProjectGenerationScope(projectID uint64, scope storyboardGenerationScope) {
	normalized := normalizeStoryboardModelNames(scope.ModelName, scope.ModelNames)
	storedScope := storyboardGenerationScope{
		EpisodeID:  cloneUint64Ptr(scope.EpisodeID),
		ModelNames: cloneStringSlice(normalized),
	}
	if len(normalized) > 0 {
		storedScope.ModelName = normalized[0]
	}
	s.generationScopes.Store(projectID, storedScope)

}

func (s *StoryboardService) resolveProjectGenerationScope(projectID uint64, episodeID *uint64, modelName string, modelNames []string) storyboardGenerationScope {
	if scope, ok := s.getProjectGenerationScope(projectID); ok {
		resolved := cloneStoryboardGenerationScope(scope)
		if episodeID != nil {
			resolved.EpisodeID = cloneUint64Ptr(episodeID)
		}
		if strings.TrimSpace(modelName) != "" || len(modelNames) > 0 {
			resolved.ModelName = modelName
			resolved.ModelNames = cloneStringSlice(modelNames)
		}
		return resolved
	}
	return storyboardGenerationScope{
		EpisodeID:  cloneUint64Ptr(episodeID),
		ModelName:  modelName,
		ModelNames: cloneStringSlice(modelNames),
	}
}

func (s *StoryboardService) getProjectGenerationScope(projectID uint64) (storyboardGenerationScope, bool) {
	value, ok := s.generationScopes.Load(projectID)
	if !ok {
		return storyboardGenerationScope{}, false
	}
	scope, ok := value.(storyboardGenerationScope)
	if !ok {
		return storyboardGenerationScope{}, false
	}
	return cloneStoryboardGenerationScope(scope), true
}

func (s *StoryboardService) clearProjectGenerationScope(projectID uint64) {
	s.generationScopes.Delete(projectID)
}

func (s *StoryboardService) setProjectGenerationPaused(projectID uint64, paused bool) {
	if paused {
		s.pausedProjects.Store(projectID, true)
		return
	}
	s.pausedProjects.Delete(projectID)
}

func (s *StoryboardService) markStoryboardPaused(sb *model.Storyboard) error {
	sb.Status = "paused"
	sb.UpdatedAt = time.Now()
	return s.repo.Update(sb)
}

func (s *StoryboardService) publishStoryboardGeneration(sb *model.Storyboard, versionID uint64, modelName string) error {
	if s.isProjectGenerationPaused(sb.ProjectID) {
		return s.markStoryboardPaused(sb)
	}
	// 串行模式：非首帧分镜无需 AI 生成图片，直接标记为 completed。
	// 视频生成时会以前一 clip 的末帧作为其首帧，image_url 留空即可。
	if sb.SceneGroupKey != "" && !sb.IsSceneFirstClip {
		sb.Status = "completed"
		sb.UpdatedAt = time.Now()
		if s.logger != nil {
			s.logger.Info("serial non-first clip: skipping image generation",
				zap.Uint64("storyboard_id", sb.ID),
				zap.String("scene_group_key", sb.SceneGroupKey),
			)
		}
		return s.repo.Update(sb)
	}
	if versionID == 0 && sb.CurrentVersion > 0 {
		versions, err := s.repo.GetVersions(sb.ID)
		if err == nil {
			for _, version := range versions {
				if version.VersionNumber == sb.CurrentVersion && version.IsCurrent {
					versionID = version.ID
					break
				}
			}
		}
	}

	sb.Status = "generating"
	sb.ErrorMsg = ""
	sb.UpdatedAt = time.Now()
	if err := s.repo.Update(sb); err != nil {
		return err
	}

	if s.kafkaProducer != nil {
		// If PromptUsed is empty (e.g. created before refinement or description was edited),
		// fall back to SceneDescription so image generator always has meaningful input.
		promptUsed := sb.PromptUsed
		if strings.TrimSpace(promptUsed) == "" {
			promptUsed = sb.SceneDescription
		}
		// Look up the preceding storyboard for visual continuity context.
		// Only the English prompt is useful; Chinese or full generated prompts are handled
		// at generation time so we pass the raw stored value and let the consumer filter.
		// For serial projects, the immediately preceding storyboard may be a non-first-clip
		// with no image_url (image generation was skipped). In that case we fall back to the
		// nearest preceding storyboard that actually has a generated image.
		var prevPromptUsed string
		var prevImageURL string
		if strings.TrimSpace(sb.SceneGroupKey) != "" {
			if !sb.IsSceneFirstClip {
				if prevSB, _ := s.repo.FindAdjacentInSceneBySequence(sb.ProjectID, sb.SequenceNumber, sb.EpisodeID, sb.SceneGroupKey); prevSB != nil {
					prevPromptUsed = prevSB.PromptUsed
					prevImageURL = prevSB.ImageURL
					if prevImageURL == "" {
						if anchor := s.repo.FindPrecedingWithImageInScene(sb.ProjectID, sb.SequenceNumber, sb.EpisodeID, sb.SceneGroupKey); anchor != nil {
							prevImageURL = anchor.ImageURL
							if prevPromptUsed == "" {
								prevPromptUsed = anchor.PromptUsed
							}
						}
					}
				}
			}
		} else if prevSB, _ := s.repo.FindAdjacentBySequence(sb.ProjectID, sb.SequenceNumber, sb.EpisodeID); prevSB != nil {
			prevPromptUsed = prevSB.PromptUsed
			prevImageURL = prevSB.ImageURL
			if prevImageURL == "" {
				if anchor := s.repo.FindPrecedingWithImage(sb.ProjectID, sb.SequenceNumber, sb.EpisodeID); anchor != nil {
					prevImageURL = anchor.ImageURL
					if prevPromptUsed == "" {
						prevPromptUsed = anchor.PromptUsed
					}
				}
			}
		}
		genReq := StoryboardGenerateRequest{
			StoryboardID:     sb.ID,
			VersionID:        versionID,
			ProjectID:        sb.ProjectID,
			SceneDescription: sb.SceneDescription,
			Characters:       []string(sb.Characters),
			Location:         sb.Location,
			CameraMovement:   sb.CameraMovement,
			Mood:             sb.Mood,
			AspectRatio:      sb.AspectRatio,
			PromptUsed:       promptUsed,
			ModelName:        modelName,
			PrevPromptUsed:   prevPromptUsed,
			PrevImageURL:     prevImageURL,
			AssetIDs:         []int64(sb.AssetIDs),
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.kafkaProducer.Publish(ctx, genReq); err != nil {
			sb.Status = "failed"
			_ = s.repo.Update(sb)
			return fmt.Errorf("publish generation request: %w", err)
		}
	}

	return nil
}

func (s *StoryboardService) PauseProjectGeneration(projectID uint64, episodeID *uint64) (int, error) {
	scope := s.resolveProjectGenerationScope(projectID, episodeID, "", nil)
	effectiveEpisodeID := scope.EpisodeID
	s.setProjectGenerationPaused(projectID, true)
	s.setProjectGenerationScope(projectID, scope)

	storyboards, _, err := s.repo.FindByProjectID(projectID, effectiveEpisodeID, "", "", 1, 10000, false)
	if err != nil {
		return 0, err
	}

	paused := 0
	for i := range storyboards {
		if storyboards[i].Status != "pending" && storyboards[i].Status != "generating" {
			continue
		}
		if err := s.markStoryboardPaused(&storyboards[i]); err != nil {
			s.logger.Warn("pause storyboard generation skipped", zap.Uint64("storyboard_id", storyboards[i].ID), zap.Error(err))
			continue
		}
		paused++
	}
	return paused, nil
}

func (s *StoryboardService) ResumeProjectGeneration(projectID uint64, episodeID *uint64) (int, error) {
	scope := s.resolveProjectGenerationScope(projectID, episodeID, "", nil)
	effectiveEpisodeID := scope.EpisodeID
	s.setProjectGenerationScope(projectID, scope)
	s.setProjectGenerationPaused(projectID, false)
	dispatched, err := s.dispatchReadyStoryboards(projectID, scope, []string{"paused"})
	if err != nil {
		return 0, err
	}
	if dispatched == 0 {
		paused, pausedErr := s.repo.FindByProjectAndStatuses(projectID, effectiveEpisodeID, []string{"paused"}, 1)
		if pausedErr == nil && len(paused) == 0 {
			inFlight, countErr := s.repo.CountByProjectAndStatus(projectID, "generating")
			if countErr == nil && inFlight == 0 {
				s.clearProjectGenerationScope(projectID)
			}
		}
	}
	return dispatched, nil
}

// UpdateGenerationResult —— Kafka 消费者回调：更新分镜生成结果
// UpdateGenerationResult is called by the Kafka consumer to update storyboard
// status and image URL after generation completes or fails.
// builtPrompt is the final English image-generation prompt that was sent to the model;
// when non-empty it is saved to PromptUsed so the UI EN toggle shows the real English prompt.
func (s *StoryboardService) UpdateGenerationResult(storyboardID, versionID uint64, imageURL, status, errorMsg, builtPrompt string) error {
	sb, err := s.GetByID(storyboardID)
	if err != nil {
		return err
	}
	sb.Status = status
	if imageURL != "" {
		sb.ImageURL = imageURL
	}
	if status == "failed" {
		sb.ErrorMsg = errorMsg
	} else {
		sb.ErrorMsg = ""
		// Save the final English generation prompt so the frontend EN toggle shows real data.
		if builtPrompt != "" {
			sb.PromptUsed = builtPrompt
		}
	}
	sb.UpdatedAt = time.Now()
	if err := s.repo.Update(sb); err != nil {
		return err
	}

	// Also update the version's image URL (and prompt) if generation succeeded.
	if imageURL != "" && versionID > 0 {
		versions, err := s.repo.GetVersions(storyboardID)
		if err != nil {
			return err
		}
		for i := range versions {
			if versions[i].ID == versionID {
				versions[i].ImageURL = imageURL
				if builtPrompt != "" {
					versions[i].PromptUsed = builtPrompt
				}
				return s.repo.UpdateVersion(&versions[i])
			}
		}
	}
	if _, ok := s.getProjectGenerationScope(sb.ProjectID); ok && !s.isProjectGenerationPaused(sb.ProjectID) {
		if queued, refillErr := s.refillProjectGeneration(sb.ProjectID); refillErr != nil {
			s.logger.Warn("storyboard refill failed",
				zap.Uint64("project_id", sb.ProjectID),
				zap.Error(refillErr),
			)
		} else if queued > 0 {
			s.logger.Info("refilled storyboard generation window",
				zap.Uint64("project_id", sb.ProjectID),
				zap.Int("queued", queued),
			)
		}
	}
	return nil
}

type CreateStoryboardReq struct {
	EpisodeID        *uint64  `json:"episode_id"`
	SequenceNumber   int      `json:"sequence_number" binding:"required,min=1"`
	SceneDescription string   `json:"scene_description"`
	Characters       []string `json:"characters"`
	Location         string   `json:"location"`
	CameraMovement   string   `json:"camera_movement"`
	Duration         int      `json:"duration"`
	AspectRatio      string   `json:"aspect_ratio"`
	Resolution       string   `json:"resolution"`
	VideoMode        *string  `json:"video_mode"`
	Dialogue         string   `json:"dialogue"`
	AssetIDs         []int64  `json:"asset_ids"`
	Mood             string   `json:"mood"`        // scene mood (tense/romantic/dramatic etc.)
	PromptUsed        string   `json:"prompt_used"` // pre-computed image-gen prompt from PromptTemplate
	SceneGroupKey     string   `json:"scene_group_key"`  // 标准化场景键（视频串行流程使用）
	IsSceneFirstClip  bool     `json:"is_scene_first_clip"` // 是否为该场景组的首个分镜（串行模式）
	// Mode 工作流模式: "script"（完整流程）/ "storyboard"（直接上传分镜模式）
	Mode              string   `json:"mode"`
}

// Create —— 创建新分镜记录，设置默认值
func (s *StoryboardService) Create(projectID uint64, req CreateStoryboardReq) (*model.Storyboard, error) {
	duration := req.Duration
	if duration <= 0 {
		duration = 4
	}
	aspectRatio := req.AspectRatio
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}
	resolution := req.Resolution
	if resolution == "" {
		resolution = "1080p"
	}

	sb := &model.Storyboard{
		ProjectID:        projectID,
		EpisodeID:        req.EpisodeID,
		SequenceNumber:   req.SequenceNumber,
		SceneDescription: req.SceneDescription,
		Characters:       pq.StringArray(req.Characters),
		Location:         req.Location,
		CameraMovement:   req.CameraMovement,
		Duration:         duration,
		AspectRatio:      aspectRatio,
		Resolution:       resolution,
		VideoMode:        req.VideoMode,
		Dialogue:         req.Dialogue,
		Mood:             req.Mood,
		PromptUsed:       req.PromptUsed,
		SceneGroupKey:    req.SceneGroupKey,
		IsSceneFirstClip: req.IsSceneFirstClip,
		Mode:             req.Mode,
		Status:           "pending",
		CurrentVersion:   1,
		AgentHistory:     datatypes.JSON([]byte("[]")),
		AssetIDs:         pq.Int64Array(req.AssetIDs),
	}
	if err := s.repo.Create(sb); err != nil {
		return nil, err
	}
	return sb, nil
}

// GetByID —— 按 ID 获取分镜详情，预加载版本列表
func (s *StoryboardService) GetByID(id uint64) (*model.Storyboard, error) {
	sb, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("storyboard not found")
		}
		return nil, err
	}
	return sb, nil
}

// List —— 分页查询项目分镜列表，支持按集和状态过滤
func (s *StoryboardService) List(projectID uint64, episodeID *uint64, status, keyword string, page, pageSize int, includeVersions bool) ([]model.Storyboard, int64, error) {
	if page < 1 {
		page = 1
	}
	maxPageSize := 500
	defaultPageSize := 100
	if includeVersions {
		maxPageSize = 100
		defaultPageSize = 20
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return s.repo.FindByProjectID(projectID, episodeID, status, keyword, page, pageSize, includeVersions)
}

// Update —— 按字段映射局部更新分镜信息
func (s *StoryboardService) Update(id uint64, updates map[string]interface{}) (*model.Storyboard, error) {
	sb, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	if v, ok := updates["scene_description"]; ok {
		if str, ok := v.(string); ok {
			sb.SceneDescription = str
		}
	}
	if v, ok := updates["characters"]; ok {
		if arr, ok := v.([]interface{}); ok {
			chars := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					chars = append(chars, s)
				}
			}
			sb.Characters = pq.StringArray(chars)
		}
	}
	if v, ok := updates["location"]; ok {
		if str, ok := v.(string); ok {
			sb.Location = str
		}
	}
	if v, ok := updates["camera_movement"]; ok {
		if str, ok := v.(string); ok {
			sb.CameraMovement = str
		}
	}
	if v, ok := updates["duration"]; ok {
		if num, ok := toInt(v); ok {
			sb.Duration = num
		}
	}
	if v, ok := updates["aspect_ratio"]; ok {
		if str, ok := v.(string); ok {
			sb.AspectRatio = str
		}
	}
	if v, ok := updates["resolution"]; ok {
		if str, ok := v.(string); ok {
			sb.Resolution = str
		}
	}
	if v, ok := updates["video_mode"]; ok {
		if str, ok := v.(string); ok {
			sb.VideoMode = &str
		}
	}
	if v, ok := updates["dialogue"]; ok {
		if str, ok := v.(string); ok {
			sb.Dialogue = str
		}
	}
	if v, ok := updates["prompt_used"]; ok {
		if str, ok := v.(string); ok {
			if s.auditor != nil {
				cleaned, _ := s.auditor.scanAndReplace(str)
				sb.PromptUsed = cleaned
			} else {
				sb.PromptUsed = str
			}
		}
	}
	if v, ok := updates["asset_ids"]; ok {
		if arr, ok := v.([]interface{}); ok {
			ids := make([]int64, 0, len(arr))
			for _, item := range arr {
				if f, ok := item.(float64); ok {
					ids = append(ids, int64(f))
				}
			}
			sb.AssetIDs = pq.Int64Array(ids)
		}
	}
	sb.UpdatedAt = time.Now()

	if err := s.repo.Update(sb); err != nil {
		return nil, err
	}
	return sb, nil
}

// Generate —— 触发单个分镜的图片生成，创建新版本并发布 Kafka 消息
func (s *StoryboardService) Generate(id uint64, modelName string) (*model.Storyboard, error) {
	sb, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if s.isProjectGenerationPaused(sb.ProjectID) {
		return nil, fmt.Errorf("project storyboard generation is paused")
	}

	// Enforce max versions limit
	count, err := s.repo.CountVersions(id)
	if err != nil {
		return nil, err
	}
	if count >= maxVersionsPerStoryboard {
		// Delete oldest version (lowest version_number)
		versions, err := s.repo.GetVersions(id)
		if err != nil {
			return nil, err
		}
		if len(versions) > 0 {
			oldest := versions[len(versions)-1] // sorted DESC, so last is oldest
			if err := s.repo.DeleteVersion(oldest.ID); err != nil {
				return nil, err
			}
		}
	}

	newVersionNum := sb.CurrentVersion + 1
	ver := &model.StoryboardVersion{
		StoryboardID:  id,
		VersionNumber: newVersionNum,
		PromptUsed:    sb.PromptUsed,
		IsCurrent:     true,
	}

	// Unset previous current
	if err := s.repo.SwitchVersion(id, 0); err != nil {
		// This just unsets all, which is fine since we'll set the new one
	}

	if err := s.repo.CreateVersion(ver); err != nil {
		return nil, err
	}

	sb.CurrentVersion = newVersionNum
	if err := s.publishStoryboardGeneration(sb, ver.ID, modelName); err != nil {
		return nil, err
	}

	return sb, nil
}

// Retry —— 重试单个失败或待处理分镜的图片生成
// Retry re-triggers generation for a single failed/pending storyboard with optional model selection.
func (s *StoryboardService) Retry(id uint64, modelName string) (*model.Storyboard, error) {
	sb, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if sb.Status != "failed" && sb.Status != "pending" && sb.Status != "paused" {
		return nil, fmt.Errorf("storyboard %d cannot be retried (status: %s)", id, sb.Status)
	}
	if s.isProjectGenerationPaused(sb.ProjectID) {
		return nil, fmt.Errorf("project storyboard generation is paused")
	}
	if err := s.publishStoryboardGeneration(sb, 0, modelName); err != nil {
		return nil, err
	}
	return sb, nil
}

// RetryBatch —— 批量重试项目或指定剧集下所有失败分镜的图片生成
// RetryBatch retries failed storyboards for a project with the specified model.
func (s *StoryboardService) RetryBatch(projectID uint64, episodeID *uint64, modelName string, modelNames []string) (int, error) {
	if s.isProjectGenerationPaused(projectID) {
		return 0, fmt.Errorf("project storyboard generation is paused")
	}
	sbs, err := s.repo.FindByProjectAndStatuses(projectID, episodeID, []string{"failed"}, 10000)
	if err != nil {
		return 0, err
	}
	resolvedModels := normalizeStoryboardModelNames(modelName, modelNames)
	count := 0
	for _, sb := range sbs {
		if sb.Status != "failed" {
			continue
		}
		usedModel := resolvedModels[count%len(resolvedModels)]
		if err := s.publishStoryboardGeneration(&sb, 0, usedModel); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// SwitchVersion —— 切换分镜的当前版本并更新主记录的图片 URL
func (s *StoryboardService) SwitchVersion(storyboardID, versionID uint64) (*model.Storyboard, error) {
	sb, err := s.GetByID(storyboardID)
	if err != nil {
		return nil, err
	}

	// Find the target version to get its image_url
	versions, err := s.repo.GetVersions(storyboardID)
	if err != nil {
		return nil, err
	}

	var targetVersion *model.StoryboardVersion
	for i := range versions {
		if versions[i].ID == versionID {
			targetVersion = &versions[i]
			break
		}
	}
	if targetVersion == nil {
		return nil, errors.New("version not found")
	}

	if err := s.repo.SwitchVersion(storyboardID, versionID); err != nil {
		return nil, err
	}

	sb.CurrentVersion = targetVersion.VersionNumber
	sb.ImageURL = targetVersion.ImageURL
	sb.UpdatedAt = time.Now()
	if err := s.repo.Update(sb); err != nil {
		return nil, err
	}

	return s.GetByID(storyboardID)
}

// DeleteVersion —— 删除分镜的指定版本
func (s *StoryboardService) DeleteVersion(storyboardID, versionID uint64) error {
	_, err := s.GetByID(storyboardID)
	if err != nil {
		return err
	}
	return s.repo.DeleteVersion(versionID)
}

// Void —— 作废指定分镜
func (s *StoryboardService) Void(id uint64) error {
	_, err := s.GetByID(id)
	if err != nil {
		return err
	}
	return s.repo.VoidByID(id)
}

// Delete —— 永久删除指定分镜及其所有版本
func (s *StoryboardService) Delete(id uint64) error {
	_, err := s.GetByID(id)
	if err != nil {
		return err
	}
	return s.repo.DeleteByID(id)
}

// ForceResetEpisode resets all completed storyboards for an episode back to pending,
// clearing their image URLs so they can be regenerated.
func (s *StoryboardService) ForceResetEpisode(projectID, episodeID uint64) (int64, error) {
	return s.repo.ResetEpisodeCompletedToPending(projectID, episodeID)
}

// CountPendingOrFailed —— 统计项目或指定剧集下待生成或失败的分镜数量
// CountPendingOrFailed returns the number of storyboards eligible for generation.
func (s *StoryboardService) CountPendingOrFailed(projectID uint64, episodeID *uint64) (int, error) {
	storyboards, _, err := s.repo.FindByProjectID(projectID, episodeID, "", "", 1, 10000, false)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, sb := range storyboards {
		if sb.Status == "pending" || sb.Status == "failed" {
			count++
		}
	}
	return count, nil
}

func (s *StoryboardService) dispatchReadyStoryboards(projectID uint64, scope storyboardGenerationScope, statuses []string) (int, error) {
	inFlight, err := s.repo.CountByProjectAndStatus(projectID, "generating")
	if err != nil {
		return 0, err
	}

	available := s.maxInFlight - int(inFlight)
	if available <= 0 {
		return 0, nil
	}

	storyboards, err := s.repo.FindByProjectAndStatuses(projectID, scope.EpisodeID, statuses, available)
	if err != nil {
		return 0, err
	}
	if len(storyboards) == 0 {
		return 0, nil
	}

	resolvedModels := normalizeStoryboardModelNames(scope.ModelName, scope.ModelNames)
	dispatched := 0
	for i := range storyboards {
		usedModel := resolvedModels[dispatched%len(resolvedModels)]
		if err := s.publishStoryboardGeneration(&storyboards[i], 0, usedModel); err != nil {
			s.logger.Warn("dispatch storyboard generation skipped",
				zap.Uint64("storyboard_id", storyboards[i].ID),
				zap.Uint64("project_id", projectID),
				zap.String("model_name", usedModel),
				zap.Error(err),
			)
			continue
		}
		dispatched++
	}

	return dispatched, nil
}

func (s *StoryboardService) refillProjectGeneration(projectID uint64) (int, error) {
	scope, ok := s.getProjectGenerationScope(projectID)
	if !ok {
		return 0, nil
	}
	dispatched, err := s.dispatchReadyStoryboards(projectID, scope, []string{"pending"})
	if err != nil {
		return 0, err
	}
	if dispatched == 0 {
		inFlight, countErr := s.repo.CountByProjectAndStatus(projectID, "generating")
		if countErr == nil && int(inFlight) < s.maxInFlight {
			s.clearProjectGenerationScope(projectID)
		}
	}
	return dispatched, nil
}

// CountFailed —— 统计项目或指定剧集下生成失败的分镜数量
// CountFailed returns the number of failed storyboards for a project.
func (s *StoryboardService) CountFailed(projectID uint64, episodeID *uint64) (int, error) {
	sbs, err := s.repo.FindByProjectAndStatuses(projectID, episodeID, []string{"failed"}, 100000)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, sb := range sbs {
		if sb.Status == "failed" {
			count++
		}
	}
	return count, nil
}

// StatusCounts —— 按状态分组统计项目下分镜数量
// StatusCounts returns storyboard counts grouped by status.
func (s *StoryboardService) StatusCounts(projectID uint64) (map[string]int64, error) {
	return s.repo.StatusCounts(projectID)
}

// ResumeStaleStoryboards —— 启动时恢复卡在 generating 状态的分镜
// ResumeStaleStoryboards resets any "generating" storyboards back to "pending".
// It intentionally does not auto-redispatch pending storyboards on startup,
// because that would re-enqueue old work with an empty generation scope and
// silently fall back to the default model.
func (s *StoryboardService) ResumeStaleStoryboards() {
	reset, err := s.repo.ResetGeneratingToPending()
	if err != nil {
		s.logger.Error("failed to reset stale generating storyboards", zap.Error(err))
		return
	}
	if reset > 0 {
		s.logger.Info("reset stale generating storyboards to pending", zap.Int64("count", reset))
	}
}

// StartStaleCleanup —— 定期检查并重置超时的 generating 状态分镜
// StartStaleCleanup runs a periodic check that marks stalled storyboard generations as failed.
func (s *StoryboardService) StartStaleCleanup(ctx context.Context, interval, staleThreshold time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.repo.ResetStaleGenerating(staleThreshold)
			if err != nil {
				s.logger.Error("stale storyboard cleanup failed", zap.Error(err))
			} else if n > 0 {
				s.logger.Info("reset stale generating storyboards to failed", zap.Int64("count", n))
			}
		}
	}
}

// GenerateAll —— 批量触发项目或指定剧集下所有待处理和失败分镜的图片生成
func (s *StoryboardService) GenerateAll(projectID uint64, episodeID *uint64, modelName string, modelNames []string) (int, error) {
	if s.isProjectGenerationPaused(projectID) {
		return 0, fmt.Errorf("project storyboard generation is paused")
	}
	scope := s.resolveProjectGenerationScope(projectID, episodeID, modelName, modelNames)
	s.setProjectGenerationScope(projectID, scope)
	count, err := s.dispatchReadyStoryboards(projectID, scope, []string{"failed", "pending"})
	if err != nil {
		s.clearProjectGenerationScope(projectID)
		return 0, err
	}
	if count == 0 {
		s.clearProjectGenerationScope(projectID)
	}
	return count, nil
}

// Chat —— 对分镜进行对话式修改，追加消息到历史记录
func (s *StoryboardService) Chat(id uint64, message map[string]interface{}) (*model.Storyboard, error) {
	sb, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	var history []interface{}
	if len(sb.AgentHistory) > 0 {
		_ = json.Unmarshal(sb.AgentHistory, &history)
	}
	if history == nil {
		history = []interface{}{}
	}

	userContent := extractStoryboardChatContent(message)
	if userContent == "" {
		return nil, errors.New("chat content is required")
	}

	now := time.Now().Format(time.RFC3339)
	history = append(history, map[string]interface{}{
		"role":      "user",
		"content":   userContent,
		"timestamp": now,
	})

	updatedDescription := extractStoryboardSceneDescription(message)
	if updatedDescription == "" {
		updatedDescription = mergeStoryboardSceneDescription(sb.SceneDescription, userContent)
	}
	sb.SceneDescription = updatedDescription
	// Clear PromptUsed so the generation pipeline re-translates the new Chinese description
	// fresh via translateIfNeeded. This avoids showing a stale English prompt in the EN toggle
	// after a chat edit. The new English prompt will be saved back after the next generation.
	sb.PromptUsed = ""
	// Pass 1 rule-based audit on the updated description (no LLM call).
	if s.auditor != nil {
		cleaned, flags := s.auditor.scanAndReplace(sb.SceneDescription)
		if len(flags) > 0 {
			sb.SceneDescription = cleaned
			if s.logger != nil {
				s.logger.Info("storyboard chat description sanitized", zap.Strings("flags", flags))
			}
		}
	}

	assistantReply := "已记录这次分镜修改，并同步到当前场景描述。你可以继续补充角色、景别、光线或台词细节，然后重新生成查看新图。"
	if sb.Status == "generating" {
		assistantReply = "已记录这次分镜修改，并同步到当前场景描述。当前分镜仍在生成中，等这一轮完成后可继续重新生成新版本。"
	}
	history = append(history, map[string]interface{}{
		"role":      "assistant",
		"content":   assistantReply,
		"timestamp": now,
	})

	b, err := json.Marshal(history)
	if err != nil {
		return nil, err
	}
	sb.AgentHistory = datatypes.JSON(b)
	sb.UpdatedAt = time.Now()
	if err := s.repo.Update(sb); err != nil {
		return nil, err
	}
	return sb, nil
}

func extractStoryboardChatContent(message map[string]interface{}) string {
	for _, key := range []string{"content", "message", "scene_description"} {
		raw, ok := message[key]
		if !ok {
			continue
		}
		if value, ok := raw.(string); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func extractStoryboardSceneDescription(message map[string]interface{}) string {
	raw, ok := message["scene_description"]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func mergeStoryboardSceneDescription(current, instruction string) string {
	current = strings.TrimSpace(current)
	instruction = strings.TrimSpace(instruction)

	if instruction == "" {
		return current
	}
	if current == "" || strings.Contains(current, instruction) {
		if current != "" {
			return current
		}
		return instruction
	}

	const marker = "修改要求："
	if strings.Contains(current, marker) {
		return current + "\n" + instruction
	}
	return current + "\n\n" + marker + "\n" + instruction
}

// UpdateConfig —— 更新项目级分镜配置
func (s *StoryboardService) UpdateConfig(projectID uint64, config map[string]interface{}) error {
	// Config is stored on the project itself, this is a placeholder
	return nil
}

// EpisodeCompletedCounts returns count of completed storyboards per episode for a project.
func (s *StoryboardService) EpisodeCompletedCounts(projectID uint64) (map[uint64]int64, error) {
	return s.repo.EpisodeCompletedCounts(projectID)
}

// GetCompletedWithImages returns all completed storyboards that have a generated image.
func (s *StoryboardService) GetCompletedWithImages(projectID uint64) ([]model.Storyboard, error) {
	return s.repo.GetCompletedWithImages(projectID)
}
