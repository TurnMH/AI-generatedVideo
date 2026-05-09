package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/autovideo/script-service/internal/model"
	"github.com/autovideo/script-service/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UploadScriptReq 上传剧本请求
type UploadScriptReq struct {
	ProjectID  int64
	Title      string
	File       multipart.File
	FileHeader *multipart.FileHeader
	// Mode 继承自 Project.Mode: "script" / "storyboard"
	Mode       string
}

// UpdateSceneReq 更新场景请求
type UpdateSceneReq struct {
	Description string `json:"description"`
	PromptDraft string `json:"prompt_draft"`
	Status      string `json:"status"`
}

// UpdateSplitConfigReq 更新拆分配置请求
type UpdateSplitConfigReq struct {
	SplitMethod     string        `json:"split_method"`
	TargetWordCount int           `json:"target_word_count"`
	TargetEpisodes  int           `json:"target_episodes"`
	CustomParams    model.JSONMap `json:"custom_params"`
}

type ScriptService interface {
	Upload(ctx context.Context, req *UploadScriptReq) (*model.Script, error)
	ListByProjectID(ctx context.Context, projectID int64, page, size int) ([]*model.Script, int64, error)
	GetByID(ctx context.Context, id int64) (*model.Script, error)
	Delete(ctx context.Context, scriptID int64) error
	TriggerAnalyze(ctx context.Context, scriptID int64) error
	GetScenes(ctx context.Context, scriptID int64) ([]*model.Scene, error)
	UpdateScene(ctx context.Context, scriptID, sceneID int64, req *UpdateSceneReq) (*model.Scene, error)
	GetCharacters(ctx context.Context, scriptID int64) ([]*model.CharacterExtracted, error)
	GetAssets(ctx context.Context, scriptID int64) ([]*model.ScriptAsset, error)
	GetSplitConfig(ctx context.Context, scriptID int64) (*model.SplitConfig, error)
	UpdateSplitConfig(ctx context.Context, scriptID int64, req *UpdateSplitConfigReq) (*model.SplitConfig, error)
	ReSplit(ctx context.Context, scriptID int64) error
	GenerateSceneImage(ctx context.Context, sceneID int64) (*model.Scene, error)
	BatchGenerateSceneImages(ctx context.Context, sceneIDs []int64) (int, error)
}

type scriptService struct {
	scriptRepo          repository.ScriptRepository
	sceneRepo           repository.SceneRepository
	characterRepo       repository.CharacterRepository
	assetRepo           repository.AssetRepository
	splitConfigRepo     repository.SplitConfigRepository
	llmClient           LLMClient
	kafkaSvc            KafkaService
	logger              *zap.Logger
	imageServiceURL     string
	imageModelName      string
	characterServiceURL string
	jwtSecret           string
}

// NewScriptService —— 创建剧本服务实例，注入所有依赖，返回 ScriptService 接口
func NewScriptService(
	scriptRepo repository.ScriptRepository,
	sceneRepo repository.SceneRepository,
	characterRepo repository.CharacterRepository,
	assetRepo repository.AssetRepository,
	splitConfigRepo repository.SplitConfigRepository,
	llmClient LLMClient,
	kafkaSvc KafkaService,
	logger *zap.Logger,
	imageServiceURL string,
	imageModelName string,
	characterServiceURL string,
	jwtSecret string,
) ScriptService {
	return &scriptService{
		scriptRepo:          scriptRepo,
		sceneRepo:           sceneRepo,
		characterRepo:       characterRepo,
		assetRepo:           assetRepo,
		splitConfigRepo:     splitConfigRepo,
		llmClient:           llmClient,
		kafkaSvc:            kafkaSvc,
		logger:              logger,
		imageServiceURL:     imageServiceURL,
		imageModelName:      imageModelName,
		characterServiceURL: characterServiceURL,
		jwtSecret:           jwtSecret,
	}
}

// Upload —— 读取上传文件内容并创建剧本记录，返回创建的剧本
func (s *scriptService) Upload(ctx context.Context, req *UploadScriptReq) (*model.Script, error) {
	content, err := io.ReadAll(req.File)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	script := &model.Script{
		ProjectID:   req.ProjectID,
		Title:       req.Title,
		RawText:     string(content),
		ParseStatus: "pending",
		Mode:        req.Mode,
	}

	if err := s.scriptRepo.Create(ctx, script); err != nil {
		return nil, fmt.Errorf("create script: %w", err)
	}

	s.logger.Info("script uploaded",
		zap.Int64("script_id", script.ID),
		zap.Int64("project_id", req.ProjectID),
		zap.Int("bytes", len(content)),
	)
	return script, nil
}

// ListByProjectID —— 根据项目 ID 获取剧本列表，按最新优先返回
func (s *scriptService) ListByProjectID(ctx context.Context, projectID int64, page, size int) ([]*model.Script, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	return s.scriptRepo.ListByProjectID(ctx, projectID, page, size)
}

// GetByID —— 根据 ID 查询剧本，不存在时返回 ErrNotFound
func (s *scriptService) GetByID(ctx context.Context, id int64) (*model.Script, error) {
	script, err := s.scriptRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return script, nil
}

// Delete —— 级联删除剧本及其所有子表数据（scenes/characters/assets/split_config）
func (s *scriptService) Delete(ctx context.Context, scriptID int64) error {
	if _, err := s.GetByID(ctx, scriptID); err != nil {
		return err
	}
	// 按依赖顺序删除子表
	if err := s.sceneRepo.DeleteByScriptID(ctx, scriptID); err != nil {
		return fmt.Errorf("delete scenes: %w", err)
	}
	if err := s.characterRepo.DeleteByScriptID(ctx, scriptID); err != nil {
		return fmt.Errorf("delete characters: %w", err)
	}
	if err := s.assetRepo.DeleteByScriptID(ctx, scriptID); err != nil {
		return fmt.Errorf("delete assets: %w", err)
	}
	if err := s.splitConfigRepo.DeleteByScriptID(ctx, scriptID); err != nil {
		return fmt.Errorf("delete split config: %w", err)
	}
	return s.scriptRepo.DeleteByID(ctx, scriptID)
}

// TriggerAnalyze —— 触发剧本的 LLM 分析，更新状态并异步执行分析流程
func (s *scriptService) TriggerAnalyze(ctx context.Context, scriptID int64) error {
	script, err := s.scriptRepo.GetByID(ctx, scriptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get script: %w", err)
	}

	if script.ParseStatus == "parsing" {
		return nil // 已在解析中，幂等
	}

	if err := s.scriptRepo.UpdateStatus(ctx, scriptID, "parsing"); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// 异步执行 LLM 分析
	go s.runAnalyze(scriptID, script.ProjectID, script.RawText)
	return nil
}

// runAnalyze —— 异步执行 LLM 剧本分析，解析场景、角色和资产并写入数据库
func (s *scriptService) runAnalyze(scriptID int64, projectID int64, rawText string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := s.llmClient.Analyze(ctx, rawText)
	if err != nil {
		s.logger.Error("llm analyze failed",
			zap.Int64("script_id", scriptID),
			zap.Error(err),
		)
		_ = s.scriptRepo.UpdateStatus(ctx, scriptID, "failed")
		_ = s.kafkaSvc.PublishAnalyzeResult(ctx, scriptID, "failed", 0)
		return
	}

	// 将结果转换为 JSONMap 存储
	resultBytes, _ := json.Marshal(result)
	var llmResult model.JSONMap
	_ = json.Unmarshal(resultBytes, &llmResult)

	if err := s.scriptRepo.UpdateLLMResult(ctx, scriptID, "done", llmResult); err != nil {
		s.logger.Error("update llm result failed",
			zap.Int64("script_id", scriptID),
			zap.Error(err),
		)
	}

	// 清除旧数据（重新分析时）
	_ = s.sceneRepo.DeleteByScriptID(ctx, scriptID)
	_ = s.characterRepo.DeleteByScriptID(ctx, scriptID)
	_ = s.assetRepo.DeleteByScriptID(ctx, scriptID)

	// 写入 scenes
	sceneOrder := 1
	var scenes []*model.Scene
	for _, ep := range result.Episodes {
		for _, sc := range ep.Scenes {
			chars := make(model.JSONSlice, len(sc.Characters))
			for i, c := range sc.Characters {
				chars[i] = c
			}
			// Convert storyboard shots to JSONSlice for JSONB storage
			storyboard := make(model.JSONSlice, len(sc.Storyboard))
			for i, shot := range sc.Storyboard {
				shotBytes, _ := json.Marshal(shot)
				var shotMap interface{}
				_ = json.Unmarshal(shotBytes, &shotMap)
				storyboard[i] = shotMap
			}
			scenes = append(scenes, &model.Scene{
				ScriptID:    scriptID,
				SceneOrder:  sceneOrder,
				Description: sc.Description,
				Setting:     sc.Setting,
				Emotion:     sc.Emotion,
				Characters:  chars,
				PromptDraft: sc.PromptDraft,
				Storyboard:  storyboard,
				Status:      "draft",
			})
			sceneOrder++
		}
	}
	if err := s.sceneRepo.BatchCreate(ctx, scenes); err != nil {
		s.logger.Error("batch create scenes failed",
			zap.Int64("script_id", scriptID),
			zap.Error(err),
		)
	}

	// 写入 characters
	var chars []*model.CharacterExtracted
	for _, ch := range result.Characters {
		rels := model.JSONMap{}
		for k, v := range ch.Relationships {
			rels[k] = v
		}
		kw := model.JSONMap{}
		for k, v := range ch.Keywords {
			kw[k] = v
		}
		// Build a flat appearance description from structured keywords for AI image generation
		var apParts []string
		for _, key := range []string{"age_body", "appearance", "clothing", "emotion_baseline"} {
			if v, ok := kw[key]; ok {
				if s, ok := v.(string); ok && s != "" {
					apParts = append(apParts, s)
				}
			}
		}
		skillTags := make(model.JSONSlice, len(ch.SkillTags))
		for i, t := range ch.SkillTags {
			skillTags[i] = t
		}
		chars = append(chars, &model.CharacterExtracted{
			ScriptID:       scriptID,
			Name:           ch.Name,
			RoleDesc:       ch.RoleDesc,
			AppearanceDesc: strings.Join(apParts, "; "),
			Keywords:       kw,
			SkillTags:      skillTags,
			Relationships:  rels,
		})
	}
	if err := s.characterRepo.BatchCreate(ctx, chars); err != nil {
		s.logger.Error("batch create characters failed",
			zap.Int64("script_id", scriptID),
			zap.Error(err),
		)
	}

	// 写入 assets
	var assets []*model.ScriptAsset
	for _, a := range result.Assets {
		kw := model.JSONMap{}
		for k, v := range a.Keywords {
			kw[k] = v
		}
		assets = append(assets, &model.ScriptAsset{
			ScriptID:    scriptID,
			AssetType:   a.Type,
			Name:        a.Name,
			Description: a.Description,
			Keywords:    kw,
			SceneIDs:    model.JSONSlice{},
		})
	}
	if err := s.assetRepo.BatchCreate(ctx, assets); err != nil {
		s.logger.Error("batch create assets failed",
			zap.Int64("script_id", scriptID),
			zap.Error(err),
		)
	}

	s.logger.Info("script analysis completed",
		zap.Int64("script_id", scriptID),
		zap.Int("scene_count", len(scenes)),
	)

	_ = s.kafkaSvc.PublishAnalyzeResult(ctx, scriptID, "done", len(scenes))

	// Async: extract characters with appearance info and sync to character-service.
	// Failures here do not affect the script analysis result.
	go s.extractAndSyncCharacters(projectID, rawText)
}

// extractAndSyncCharacters calls the LLM to extract characters with appearance
// descriptions, then pushes any new characters to character-service.
func (s *scriptService) extractAndSyncCharacters(projectID int64, rawText string) {
	if s.characterServiceURL == "" || s.jwtSecret == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extracted, err := s.llmClient.ExtractCharacters(ctx, rawText)
	if err != nil {
		s.logger.Warn("extract characters from script failed",
			zap.Int64("project_id", projectID),
			zap.Error(err),
		)
		return
	}
	if len(extracted) == 0 {
		return
	}

	charClient := newCharacterServiceClient(s.characterServiceURL, s.jwtSecret)

	existing, err := charClient.listCharacterNames(ctx, projectID)
	if err != nil {
		s.logger.Warn("list existing characters from character-service failed",
			zap.Int64("project_id", projectID),
			zap.Error(err),
		)
		existing = make(map[string]bool)
	}

	created := 0
	for _, ch := range extracted {
		if existing[ch.Name] {
			continue
		}
		if err := charClient.createCharacter(ctx, projectID, ch); err != nil {
			s.logger.Warn("create character in character-service failed",
				zap.Int64("project_id", projectID),
				zap.String("name", ch.Name),
				zap.Error(err),
			)
			continue
		}
		created++
	}

	s.logger.Info("characters synced to character-service",
		zap.Int64("project_id", projectID),
		zap.Int("extracted", len(extracted)),
		zap.Int("created", created),
	)
}

// GetScenes —— 获取指定剧本的所有场景列表
func (s *scriptService) GetScenes(ctx context.Context, scriptID int64) ([]*model.Scene, error) {
	if _, err := s.GetByID(ctx, scriptID); err != nil {
		return nil, err
	}
	return s.sceneRepo.ListByScriptID(ctx, scriptID)
}

// UpdateScene —— 更新指定场景的描述、提示词或状态，返回更新后的场景
func (s *scriptService) UpdateScene(ctx context.Context, scriptID, sceneID int64, req *UpdateSceneReq) (*model.Scene, error) {
	scene, err := s.sceneRepo.GetByID(ctx, sceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if scene.ScriptID != scriptID {
		return nil, ErrNotFound
	}

	if req.Description != "" {
		scene.Description = req.Description
	}
	if req.PromptDraft != "" {
		scene.PromptDraft = req.PromptDraft
	}
	if req.Status != "" {
		scene.Status = req.Status
	}

	if err := s.sceneRepo.Update(ctx, scene); err != nil {
		return nil, fmt.Errorf("update scene: %w", err)
	}
	return scene, nil
}

// GetCharacters —— 获取指定剧本中提取的所有角色列表
func (s *scriptService) GetCharacters(ctx context.Context, scriptID int64) ([]*model.CharacterExtracted, error) {
	if _, err := s.GetByID(ctx, scriptID); err != nil {
		return nil, err
	}
	return s.characterRepo.ListByScriptID(ctx, scriptID)
}

// GetAssets —— 获取指定剧本中提取的所有资产列表
func (s *scriptService) GetAssets(ctx context.Context, scriptID int64) ([]*model.ScriptAsset, error) {
	if _, err := s.GetByID(ctx, scriptID); err != nil {
		return nil, err
	}
	return s.assetRepo.ListByScriptID(ctx, scriptID)
}

// ErrNotFound 资源不存在
var ErrNotFound = errors.New("not found")

// GetSplitConfig —— 获取剧本的拆分配置，不存在时自动创建默认配置
func (s *scriptService) GetSplitConfig(ctx context.Context, scriptID int64) (*model.SplitConfig, error) {
	if _, err := s.GetByID(ctx, scriptID); err != nil {
		return nil, err
	}

	config, err := s.splitConfigRepo.GetByScriptID(ctx, scriptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return a default config
			defaultConfig := &model.SplitConfig{
				ScriptID:        scriptID,
				SplitMethod:     "scene_based",
				TargetWordCount: 3000,
			}
			if err := s.splitConfigRepo.Create(ctx, defaultConfig); err != nil {
				return nil, fmt.Errorf("create default split config: %w", err)
			}
			return defaultConfig, nil
		}
		return nil, fmt.Errorf("get split config: %w", err)
	}
	return config, nil
}

// UpdateSplitConfig —— 更新剧本的拆分配置参数，不存在时自动创建
func (s *scriptService) UpdateSplitConfig(ctx context.Context, scriptID int64, req *UpdateSplitConfigReq) (*model.SplitConfig, error) {
	if _, err := s.GetByID(ctx, scriptID); err != nil {
		return nil, err
	}

	config, err := s.splitConfigRepo.GetByScriptID(ctx, scriptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			config = &model.SplitConfig{ScriptID: scriptID}
		} else {
			return nil, fmt.Errorf("get split config: %w", err)
		}
	}

	if req.SplitMethod != "" {
		config.SplitMethod = req.SplitMethod
	}
	if req.TargetWordCount > 0 {
		config.TargetWordCount = req.TargetWordCount
	}
	if req.TargetEpisodes > 0 {
		config.TargetEpisodes = req.TargetEpisodes
	}
	if req.CustomParams != nil {
		config.CustomParams = req.CustomParams
	}

	if config.ID == 0 {
		if err := s.splitConfigRepo.Create(ctx, config); err != nil {
			return nil, fmt.Errorf("create split config: %w", err)
		}
	} else {
		if err := s.splitConfigRepo.Update(ctx, config); err != nil {
			return nil, fmt.Errorf("update split config: %w", err)
		}
	}

	s.logger.Info("split config updated",
		zap.Int64("script_id", scriptID),
		zap.String("method", config.SplitMethod),
	)
	return config, nil
}

// ReSplit —— 使用当前拆分配置重新触发剧本分析
func (s *scriptService) ReSplit(ctx context.Context, scriptID int64) error {
	return s.TriggerAnalyze(ctx, scriptID)
}

// GenerateSceneImage —— 为指定场景调用图片服务生成 AI 图片，返回更新后的场景
func (s *scriptService) GenerateSceneImage(ctx context.Context, sceneID int64) (*model.Scene, error) {
	scene, err := s.sceneRepo.GetByID(ctx, sceneID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get scene: %w", err)
	}

	// Mark as generating
	scene.Status = "generating"
	if err := s.sceneRepo.Update(ctx, scene); err != nil {
		return nil, fmt.Errorf("update scene status: %w", err)
	}

	// Build prompt from scene fields
	prompt := s.buildImagePrompt(scene)

	// Call image-service asynchronously
	go s.callImageService(scene.ID, scene.ScriptID, prompt)

	return scene, nil
}

// buildImagePrompt —— 根据场景信息构建 T4 标准「固定描述词 + 补充描述词」图片提示词
func (s *scriptService) buildImagePrompt(scene *model.Scene) string {
	if scene.PromptDraft != "" {
		// If PromptDraft was set manually by the user, respect it as-is
		return scene.PromptDraft
	}
	zh := IsChineseModel(s.imageModelName)
	storyboard := []interface{}(scene.Storyboard)
	return BuildSceneImagePrompt(scene.Description, scene.Setting, scene.Emotion, storyboard, zh)
}

// callImageService —— 异步调用图片服务 API 生成图片，并更新场景的图片 URL 和状态
func (s *scriptService) callImageService(sceneID, scriptID int64, prompt string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	reqBody := map[string]interface{}{
		"project_id": scriptID,
		"prompt":     prompt,
		"model_name": s.imageModelName,
		"width":      1024,
		"height":     1024,
	}
	body, _ := json.Marshal(reqBody)

	url := strings.TrimRight(s.imageServiceURL, "/") + "/api/v1/images/generate"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		s.logger.Error("build image-service request failed", zap.Int64("scene_id", sceneID), zap.Error(err))
		s.updateSceneImageStatus(ctx, sceneID, "failed", "")
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		s.logger.Error("image-service call failed",
			zap.Int64("scene_id", sceneID),
			zap.Error(err),
		)
		s.updateSceneImageStatus(ctx, sceneID, "failed", "")
		return
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		s.logger.Error("image-service returned error",
			zap.Int64("scene_id", sceneID),
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)),
		)
		s.updateSceneImageStatus(ctx, sceneID, "failed", "")
		return
	}

	var result struct {
		Data struct {
			ImageURL string `json:"image_url"`
			URL      string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		s.logger.Error("parse image-service response failed",
			zap.Int64("scene_id", sceneID),
			zap.Error(err),
		)
		s.updateSceneImageStatus(ctx, sceneID, "failed", "")
		return
	}

	imageURL := result.Data.ImageURL
	if imageURL == "" {
		imageURL = result.Data.URL
	}

	s.updateSceneImageStatus(ctx, sceneID, "image_ready", imageURL)
	s.logger.Info("scene image generated",
		zap.Int64("scene_id", sceneID),
		zap.String("image_url", imageURL),
	)
}

// updateSceneImageStatus —— 更新场景的图片生成状态和图片 URL
func (s *scriptService) updateSceneImageStatus(ctx context.Context, sceneID int64, status, imageURL string) {
	scene, err := s.sceneRepo.GetByID(ctx, sceneID)
	if err != nil {
		s.logger.Error("get scene for status update failed",
			zap.Int64("scene_id", sceneID),
			zap.Error(err),
		)
		return
	}
	scene.Status = status
	if imageURL != "" {
		scene.ImageURL = imageURL
	}
	if err := s.sceneRepo.Update(ctx, scene); err != nil {
		s.logger.Error("update scene image status failed",
			zap.Int64("scene_id", sceneID),
			zap.Error(err),
		)
	}
}

// BatchGenerateSceneImages —— 批量触发多个场景的图片生成，返回成功触发的数量
func (s *scriptService) BatchGenerateSceneImages(ctx context.Context, sceneIDs []int64) (int, error) {
	count := 0
	for _, id := range sceneIDs {
		if _, err := s.GenerateSceneImage(ctx, id); err != nil {
			s.logger.Warn("batch generate: skip scene",
				zap.Int64("scene_id", id),
				zap.Error(err),
			)
			continue
		}
		count++
	}
	return count, nil
}
