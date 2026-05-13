package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/autovideo/character-service/internal/model"
	"github.com/autovideo/character-service/internal/repository"
	"github.com/autovideo/character-service/internal/stylepreset"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// llmProvider holds base_url + api_key for a specific LLM provider.
type llmProvider struct {
	baseURL string
	apiKey  string
}

type chatRuntimeModel struct {
	ModelKey    string  `json:"model_key"`
	Name        string  `json:"name"`
	Provider    string  `json:"provider"`
	APIEndpoint string  `json:"api_endpoint"`
	APIKeyRef   *string `json:"api_key_ref,omitempty"`
	IsActive    bool    `json:"is_active"`
}

type AssetService struct {
	repo           *repository.AssetRepo
	storage        *StorageClient
	log            *zap.Logger
	kafkaProducer  *KafkaProducer
	skillRepo      repository.SkillRepository // optional; injected after construction
	llmBaseURL      string
	llmAPIKey       string
	llmModel        string
	llmVisionModel  string
	llmTimeout      time.Duration
	// extra providers for multi-vendor routing in ChatFree
	llmClaude       llmProvider // Anthropic Claude proxy (claude* models)
	llmQwen         llmProvider // Alibaba DashScope (qwen* models)
	llmZhipu        llmProvider // ZhipuAI BigModel (glm* models)
	llmGemini       llmProvider // Gemini proxy (gemini* models)
	modelServiceBaseURL string
	chatCatalogMu      sync.RWMutex
	chatCatalog        []chatRuntimeModel
	chatCatalogExpiry  time.Time
	pausedProjects  sync.Map
	panelRegen      PanelRegenFunc
}

type assetGenerationSpec struct {
	ModelName    string
	PromptSuffix string
}

// NewAssetService —— 创建资产服务实例，返回 *AssetService
func NewAssetService(
	repo *repository.AssetRepo,
	storage *StorageClient,
	log *zap.Logger,
	llmBaseURL, llmAPIKey, llmModel, llmVisionModel string,
	llmTimeout time.Duration,
	claudeBaseURL, claudeAPIKey string,
	qwenBaseURL, qwenAPIKey string,
	zhipuBaseURL, zhipuAPIKey string,
	geminiBaseURL, geminiAPIKey string,
	modelServiceBaseURL string,
) *AssetService {
	if llmTimeout <= 0 {
		llmTimeout = 120 * time.Second
	}
	visionModel := strings.TrimSpace(llmVisionModel)
	if visionModel == "" {
		visionModel = llmModel
	}

	return &AssetService{
		repo:           repo,
		storage:        storage,
		log:            log,
		llmBaseURL:     strings.TrimRight(llmBaseURL, "/"),
		llmAPIKey:      llmAPIKey,
		llmModel:       llmModel,
		llmVisionModel: visionModel,
		llmTimeout:     llmTimeout,
		llmClaude:      llmProvider{baseURL: strings.TrimRight(claudeBaseURL, "/"), apiKey: claudeAPIKey},
		llmQwen:        llmProvider{baseURL: strings.TrimRight(qwenBaseURL, "/"), apiKey: qwenAPIKey},
		llmZhipu:       llmProvider{baseURL: strings.TrimRight(zhipuBaseURL, "/"), apiKey: zhipuAPIKey},
		llmGemini:      llmProvider{baseURL: strings.TrimRight(geminiBaseURL, "/"), apiKey: geminiAPIKey},
		modelServiceBaseURL: strings.TrimRight(modelServiceBaseURL, "/"),
	}
}

// Storage returns the underlying storage client. Used by kafka_consumer to upload
// composite images for character four-panel sheets.
func (s *AssetService) Storage() *StorageClient {
	return s.storage
}

// PanelRegenFunc —— 单栏重绘的注入钩子；由 KafkaConsumer 提供具体实现（编排/拼接复用后端逻辑）。
// 置空时 RegenPanel 返回错误（实际上正常部署都会由 main 注入）。
type PanelRegenFunc func(ctx context.Context, assetID uint64, panel, promptOverride, modelName string) error

// SetPanelRegenerator —— 由 KafkaConsumer 启动后注入，用于前端"重绘某一栏"按钮对应的 API。
func (s *AssetService) SetPanelRegenerator(fn PanelRegenFunc) {
	s.panelRegen = fn
}

// RegenPanel —— 供 handler 调用的单栏重绘入口；转发到 KafkaConsumer 注入的实际执行函数。
func (s *AssetService) RegenPanel(ctx context.Context, assetID uint64, panel, promptOverride, modelName string) error {
	if s.panelRegen == nil {
		return fmt.Errorf("panel regenerator not configured (kafka consumer disabled?)")
	}
	return s.panelRegen(ctx, assetID, panel, promptOverride, modelName)
}

// RecompositePanels —— 不重绘任何分栏，只把已有 panel_images 重新横向拼接上传，修复 composite_image_url。
func (s *AssetService) RecompositePanels(ctx context.Context, assetID uint64) (string, error) {
	asset, err := s.repo.FindByID(assetID)
	if err != nil {
		return "", fmt.Errorf("get asset: %w", err)
	}
	if asset.Type != "character" {
		return "", fmt.Errorf("only character assets support recomposite")
	}
	nonEmpty := make([]string, 0, 4)
	for _, u := range asset.PanelImages {
		if strings.TrimSpace(u) != "" {
			nonEmpty = append(nonEmpty, u)
		}
	}
	if len(nonEmpty) == 0 {
		return "", fmt.Errorf("no panel images to composite")
	}
	compositeBytes, err := composeHorizontalPanels(ctx, http.DefaultClient, nonEmpty)
	if err != nil {
		return "", fmt.Errorf("compose panels: %w", err)
	}
	compositeURL, err := s.storage.Upload(compositeFileName(assetID), bytes.NewReader(compositeBytes))
	if err != nil {
		return "", fmt.Errorf("upload composite: %w", err)
	}
	asset.CompositeImageURL = compositeURL
	asset.ImageURL = compositeURL
	if err := s.repo.Update(asset); err != nil {
		return "", fmt.Errorf("persist composite url: %w", err)
	}
	return compositeURL, nil
}

// SetKafkaProducer —— 绑定 Kafka 生产者，用于异步资产生成
// SetKafkaProducer attaches a Kafka producer for async asset generation.
func (s *AssetService) SetKafkaProducer(p *KafkaProducer) {
	s.kafkaProducer = p
}

// SetSkillRepo injects a skill repository so the service can fetch project skill hints
// for LLM-assisted prompt optimization during asset generation.
func (s *AssetService) SetSkillRepo(repo repository.SkillRepository) {
	s.skillRepo = repo
}

func (s *AssetService) isProjectGenerationPaused(projectID uint64) bool {
	paused, ok := s.pausedProjects.Load(projectID)
	return ok && paused == true
}

func (s *AssetService) setProjectGenerationPaused(projectID uint64, paused bool) {
	if paused {
		s.pausedProjects.Store(projectID, true)
		return
	}
	s.pausedProjects.Delete(projectID)
}

func (s *AssetService) markAssetPaused(asset *model.Asset) error {
	metadata, err := parseAssetMetadata(asset.Metadata)
	if err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}

	progress := buildAssetGenerationProgress("paused", 0, "已暂停生成", "继续生成后会从当前项目队列重新启动", 0, "")
	if raw, ok := metadata["generation_progress"].(map[string]interface{}); ok {
		progress.Percent = max(0, min(99, int(numberValue(raw["percent"]))))
		progress.TaskID = int64(numberValue(raw["task_id"]))
		progress.ModelName = strings.TrimSpace(stringValue(raw["model_name"]))
		if startedAt := strings.TrimSpace(stringValue(raw["started_at"])); startedAt != "" {
			progress.StartedAt = startedAt
		}
	}

	metadata["generation_progress"] = progress
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	asset.Status = "paused"
	asset.Metadata = datatypes.JSON(metadataJSON)
	asset.UpdatedAt = time.Now()
	return s.repo.Update(asset)
}

func restoreAssetGenerationSpec(metadata map[string]interface{}) (string, string) {
	if raw, ok := metadata["generation_request"].(map[string]interface{}); ok {
		modelName := strings.TrimSpace(stringValue(raw["model_name"]))
		promptSuffix := strings.TrimSpace(stringValue(raw["prompt_suffix"]))
		if modelName != "" || promptSuffix != "" {
			return modelName, promptSuffix
		}
	}
	if raw, ok := metadata["generation_progress"].(map[string]interface{}); ok {
		return strings.TrimSpace(stringValue(raw["model_name"])), ""
	}
	return "", ""
}

func (s *AssetService) dispatchAssetGeneration(asset *model.Asset, modelName string, promptSuffix string, stylePreset string) error {
	if s.kafkaProducer == nil {
		return fmt.Errorf("kafka producer is not configured")
	}
	if s.isProjectGenerationPaused(asset.ProjectID) {
		return s.markAssetPaused(asset)
	}

	asset.Status = "generating"
	asset.ErrorMsg = ""
	metadata, err := parseAssetMetadata(asset.Metadata)
	if err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}
	progress := buildAssetGenerationProgress("queued", 6, "已进入生成队列", "", 0, modelName)
	progress.StartedAt = time.Now().Format(time.RFC3339)
	metadata["generation_progress"] = progress
	metadata["generation_request"] = map[string]interface{}{
		"model_name":    strings.TrimSpace(modelName),
		"prompt_suffix": strings.TrimSpace(promptSuffix),
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	asset.Metadata = datatypes.JSON(metadataJSON)
	asset.UpdatedAt = time.Now()
	if err := s.repo.Update(asset); err != nil {
		return err
	}

	go func(a model.Asset) {
		if s.isProjectGenerationPaused(a.ProjectID) {
			if latest, err := s.GetByID(a.ID); err == nil {
				_ = s.markAssetPaused(latest)
			}
			return
		}
		req := AssetGenerateRequest{
			AssetID:      a.ID,
			ProjectID:    a.ProjectID,
			Type:         a.Type,
			Name:         a.Name,
			Description:  a.Description,
			Prompt:       "", // consumer composes style-aware prompt after fetching project style
			PromptSuffix: promptSuffix,
			StylePreset:  stylePreset,
			ModelName:    modelName,
		}
		if err := s.kafkaProducer.PublishGenerate(context.Background(), req); err != nil {
			s.log.Error("publish asset generate, reverting to failed", zap.Uint64("id", a.ID), zap.Error(err))
			_ = s.repo.UpdateStatusOnly(a.ID, "failed", "kafka publish: "+err.Error())
		}
	}(*asset)

	return nil
}

func (s *AssetService) PauseProjectGeneration(projectID uint64) (int, error) {
	s.setProjectGenerationPaused(projectID, true)

	assets, err := s.repo.FindByProjectID(projectID, "", "", nil)
	if err != nil {
		return 0, err
	}

	paused := 0
	for i := range assets {
		if assets[i].Name == "__extracting__" {
			continue
		}
		if assets[i].Status != "pending" && assets[i].Status != "generating" {
			continue
		}
		if err := s.markAssetPaused(&assets[i]); err != nil {
			s.log.Warn("pause asset generation skipped", zap.Uint64("asset_id", assets[i].ID), zap.Error(err))
			continue
		}
		paused++
	}
	return paused, nil
}

func (s *AssetService) ResumeProjectGeneration(projectID uint64) (int, error) {
	s.setProjectGenerationPaused(projectID, false)

	assets, err := s.repo.FindByProjectID(projectID, "", "", nil)
	if err != nil {
		return 0, err
	}

	triggered := 0
	for i := range assets {
		if assets[i].Name == "__extracting__" || assets[i].Status != "paused" {
			continue
		}
		metadata, parseErr := parseAssetMetadata(assets[i].Metadata)
		if parseErr != nil {
			s.log.Warn("resume asset generation skipped: parse metadata failed", zap.Uint64("asset_id", assets[i].ID), zap.Error(parseErr))
			continue
		}
		modelName, promptSuffix := restoreAssetGenerationSpec(metadata)
		if err := s.dispatchAssetGeneration(&assets[i], modelName, promptSuffix, ""); err != nil {
			s.log.Warn("resume asset generation skipped", zap.Uint64("asset_id", assets[i].ID), zap.Error(err))
			continue
		}
		triggered++
	}
	return triggered, nil
}

// List —— 按项目 ID 和可选类型查询资产列表
func (s *AssetService) List(projectID uint64, assetType, keyword string, episodeID *uint64) ([]model.Asset, error) {
	return s.repo.FindByProjectID(projectID, assetType, keyword, episodeID)
}

func (s *AssetService) ListRecoverablePendingAssets(limit int) ([]model.Asset, error) {
	return s.repo.FindRecoverablePending(limit)
}

// ListPaginated —— 按项目 ID 分页查询资产列表，支持类型和状态筛选
func (s *AssetService) ListPaginated(projectID uint64, assetType, status, keyword string, episodeID *uint64, page, pageSize int) (*repository.PaginatedResult, error) {
	return s.repo.FindByProjectIDPaginated(projectID, assetType, status, keyword, episodeID, page, pageSize)
}

// GetByID —— 根据 ID 获取资产，不存在则返回 ErrNotFound
func (s *AssetService) GetByID(id uint64) (*model.Asset, error) {
	asset, err := s.repo.FindByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return asset, err
}

// Create —— 创建新资产，自动设置默认状态和 JSON 字段
func (s *AssetService) Create(asset *model.Asset) error {
	if asset.Status == "" {
		asset.Status = "pending"
	}
	if asset.ConsistencyRef == nil {
		asset.ConsistencyRef = datatypes.JSON([]byte("{}"))
	}
	if asset.Metadata == nil {
		asset.Metadata = datatypes.JSON([]byte("{}"))
	}
	if asset.AgentHistory == nil {
		asset.AgentHistory = datatypes.JSON([]byte("[]"))
	}
	return s.repo.Create(asset)
}

// Update —— 按字段名部分更新资产，返回更新后的资产
func (s *AssetService) Update(id uint64, updates map[string]interface{}) (*model.Asset, error) {
	asset, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	if v, ok := updates["name"]; ok {
		if name, ok := v.(string); ok {
			asset.Name = name
		}
	}
	if v, ok := updates["description"]; ok {
		if desc, ok := v.(string); ok {
			asset.Description = desc
		}
	}
	if v, ok := updates["metadata"]; ok {
		b, _ := json.Marshal(v)
		asset.Metadata = datatypes.JSON(b)
	}
	if v, ok := updates["episode_ids"]; ok {
		if arr, ok := v.([]interface{}); ok {
			ids := make([]int64, 0, len(arr))
			for _, item := range arr {
				if f, ok := item.(float64); ok {
					ids = append(ids, int64(f))
				}
			}
			asset.EpisodeIDs = ids
		}
	}
	if v, ok := updates["is_locked"]; ok {
		if locked, ok := v.(bool); ok {
			asset.IsLocked = locked
		}
	}
	if v, ok := updates["voice_model"]; ok {
		if vm, ok := v.(string); ok {
			asset.VoiceModel = vm
		}
	}
	if v, ok := updates["prompt_used"]; ok {
		if p, ok := v.(string); ok {
			asset.PromptUsed = p
		}
	}
	if v, ok := updates["image_url"]; ok {
		if imageURL, ok := v.(string); ok {
			asset.ImageURL = imageURL
		}
	}
	if v, ok := updates["status"]; ok {
		if s, ok := v.(string); ok {
			asset.Status = s
		}
	}
	if v, ok := updates["error_msg"]; ok {
		if msg, ok := v.(string); ok {
			asset.ErrorMsg = msg
		}
	}
	// char-c9: link or unlink an asset to a character
	if v, ok := updates["character_id"]; ok {
		switch cv := v.(type) {
		case float64:
			cid := int64(cv)
			asset.CharacterID = &cid
		case nil:
			asset.CharacterID = nil
		}
	}
	asset.UpdatedAt = time.Now()

	if err := s.repo.Update(asset); err != nil {
		return nil, err
	}
	return asset, nil
}

// autoVoiceHints mirrors the dubbing-service logic for assigning voices by character name.
var autoVoiceFemaleHints = []string{
	"女", "妈妈", "母亲", "妻子", "姑娘", "女孩", "女生", "小姐", "女士", "阿姨", "奶奶", "姐姐", "妹妹", "太太", "皇后", "公主",
}
var autoVoiceMaleHints = []string{
	"男", "爸爸", "父亲", "丈夫", "男孩", "男生", "先生", "叔叔", "爷爷", "哥哥", "弟弟", "皇帝", "王子",
}
var autoVoiceNarratorHintsLocal = []string{
	"旁白", "内心", "独白", "画外音", "解说", "系统", "广播",
}
var autoVoiceFemaleCycle = []string{"female1", "female2"}
var autoVoiceMaleCycle = []string{"default", "male2", "male3"}
var autoVoiceNeutralCycle = []string{"female1", "male2", "female2", "male3", "default"}

func containsAnyHint(value string, hints []string) bool {
	for _, h := range hints {
		if h != "" && strings.Contains(value, h) {
			return true
		}
	}
	return false
}

// AutoMatchVoices assigns voice models to all character-type assets that have no voice binding.
// Uses name-based hints (female/male keywords) mirroring the dubbing service logic.
// Returns the number of assets updated.
func (s *AssetService) AutoMatchVoices(projectID uint64) (int, error) {
	assets, err := s.repo.FindByProjectID(projectID, "character", "", nil)
	if err != nil {
		return 0, err
	}

	femaleIdx, maleIdx, neutralIdx := 0, 0, 0
	count := 0
	for i := range assets {
		a := &assets[i]
		if a.VoiceModel != "" {
			continue // already bound, skip
		}
		name := a.Name
		var voice string
		switch {
		case containsAnyHint(name, autoVoiceNarratorHintsLocal):
			voice = "male3"
		case containsAnyHint(name, autoVoiceFemaleHints):
			voice = autoVoiceFemaleCycle[femaleIdx%len(autoVoiceFemaleCycle)]
			femaleIdx++
		case containsAnyHint(name, autoVoiceMaleHints):
			voice = autoVoiceMaleCycle[maleIdx%len(autoVoiceMaleCycle)]
			maleIdx++
		default:
			voice = autoVoiceNeutralCycle[neutralIdx%len(autoVoiceNeutralCycle)]
			neutralIdx++
		}
		a.VoiceModel = voice
		a.UpdatedAt = time.Now()
		if err := s.repo.Update(a); err != nil {
			s.log.Warn("auto-match voice: update failed", zap.Uint64("asset_id", a.ID), zap.Error(err))
			continue
		}
		count++
	}
	return count, nil
}
func (s *AssetService) Delete(id uint64) error {
	asset, err := s.GetByID(id)
	if err != nil {
		return err
	}
	if asset.IsLocked {
		return ErrAssetLocked
	}
	return s.repo.Delete(id)
}

// DeleteSentinel —— 删除项目的 __extracting__ 哨兵记录，重置提取状态
// Called before starting a new extraction to clear stale in-progress indicators.
func (s *AssetService) DeleteSentinel(projectID uint64) {
	s.repo.DeleteSentinel(projectID)
}

// DeleteByProjectID —— 删除项目下所有未锁定的资产
// DeleteByProjectID removes all unlocked assets for a project.
func (s *AssetService) DeleteByProjectID(projectID uint64) error {
	return s.repo.DeleteByProjectID(projectID)
}

// Generate —— 触发单个资产的图像生成，通过 Kafka 发布任务
func (s *AssetService) Generate(id uint64, modelName string, promptSuffix string, stylePreset string) (*model.Asset, error) {
	asset, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if s.isProjectGenerationPaused(asset.ProjectID) {
		return nil, fmt.Errorf("project asset generation is paused")
	}
	switch asset.Status {
	case "pending", "failed", "qa_failed", "paused", "completed":
		// allowed
	default:
		return nil, fmt.Errorf("asset %d cannot be generated from status: %s", id, asset.Status)
	}
	if err := s.dispatchAssetGeneration(asset, modelName, promptSuffix, stylePreset); err != nil {
		return nil, err
	}
	s.log.Info("asset generation triggered", zap.Uint64("id", id))
	return asset, nil
}

// GenerateAll —— 批量触发项目下所有待处理/失败资产的图像生成，返回触发数量
// When force is true, completed assets are first voided (image_url cleared) then re-queued.
func normalizeAssetGenerationSpecs(modelName string, modelNames []string, promptSuffix string, promptSuffixes map[string]string) []assetGenerationSpec {
	specs := make([]assetGenerationSpec, 0, len(modelNames)+1)
	seen := make(map[string]struct{}, len(modelNames)+1)
	appendSpec := func(name, fallbackPrompt string) {
		trimmedName := strings.TrimSpace(name)
		key := trimmedName
		if trimmedName != "" {
			key = strings.ToLower(trimmedName)
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		resolvedPrompt := strings.TrimSpace(fallbackPrompt)
		if trimmedName != "" {
			if prompt, ok := promptSuffixes[trimmedName]; ok && strings.TrimSpace(prompt) != "" {
				resolvedPrompt = strings.TrimSpace(prompt)
			}
		}
		specs = append(specs, assetGenerationSpec{
			ModelName:    trimmedName,
			PromptSuffix: resolvedPrompt,
		})
	}

	for _, name := range modelNames {
		appendSpec(name, promptSuffix)
	}
	if len(specs) == 0 {
		appendSpec(modelName, promptSuffix)
	}
	if len(specs) == 0 {
		specs = append(specs, assetGenerationSpec{})
	}
	return specs
}

func (s *AssetService) GenerateAll(projectID uint64, episodeID *uint64, modelName string, modelNames []string, promptSuffix string, promptSuffixes map[string]string, force bool) (int, error) {
	if s.isProjectGenerationPaused(projectID) {
		return 0, fmt.Errorf("project asset generation is paused")
	}
	specs := normalizeAssetGenerationSpecs(modelName, modelNames, promptSuffix, promptSuffixes)
	assets, err := s.repo.FindByProjectID(projectID, "", "", episodeID)
	if err != nil {
		return 0, err
	}
	count := 0
	for i := range assets {
		if assets[i].Name == "__extracting__" {
			continue
		}
		if !force && assets[i].Status != "pending" && assets[i].Status != "failed" && assets[i].Status != "paused" {
			continue
		}
		// force=true: process pending/failed/completed/generating; skip paused and unknown statuses
		if force {
			st := assets[i].Status
			if st != "pending" && st != "failed" && st != "completed" && st != "generating" {
				continue
			}
		}
		// When force-regenerating a completed or in-flight (generating) asset, void first
		if force && (assets[i].Status == "completed" || assets[i].Status == "generating") {
			updated, err := s.voidAssetImage(assets[i].ID)
			if err != nil {
				s.log.Warn("void asset before force-regen failed", zap.Uint64("asset_id", assets[i].ID), zap.Error(err))
				continue
			}
			assets[i] = *updated
		}
		for _, spec := range specs {
			if err := s.dispatchAssetGeneration(&assets[i], spec.ModelName, spec.PromptSuffix, ""); err != nil {
				s.log.Warn("dispatch batch asset generation skipped",
					zap.Uint64("asset_id", assets[i].ID),
					zap.String("model_name", spec.ModelName),
					zap.Error(err),
				)
				continue
			}
			count++
		}
	}
	if count == 0 {
		return 0, nil
	}
	fields := []zap.Field{
		zap.Uint64("project_id", projectID),
		zap.Int("count", count),
		zap.Int("model_count", len(specs)),
		zap.Bool("force", force),
	}
	if episodeID != nil {
		fields = append(fields, zap.Uint64("episode_id", *episodeID))
	}
	s.log.Info("batch asset generation triggered", fields...)
	return count, nil
}

// GenerateBatch dispatches generation for a specific list of asset IDs.
// When force=false only pending/failed/paused assets are triggered.
// When force=true completed/generating assets are voided first then re-queued.
func (s *AssetService) GenerateBatch(projectID uint64, assetIDs []uint64, modelName string, modelNames []string, promptSuffix string, promptSuffixes map[string]string, force bool) (int, error) {
	if s.isProjectGenerationPaused(projectID) {
		return 0, fmt.Errorf("project asset generation is paused")
	}
	specs := normalizeAssetGenerationSpecs(modelName, modelNames, promptSuffix, promptSuffixes)
	count := 0
	for _, id := range assetIDs {
		asset, err := s.GetByID(id)
		if err != nil {
			s.log.Warn("generate-batch: asset not found", zap.Uint64("asset_id", id), zap.Error(err))
			continue
		}
		if asset.ProjectID != projectID {
			s.log.Warn("generate-batch: asset not in project", zap.Uint64("asset_id", id), zap.Uint64("project_id", projectID))
			continue
		}
		if asset.Name == "__extracting__" {
			continue
		}
		switch asset.Status {
		case "pending", "failed", "paused":
			// always proceed
		case "completed", "generating":
			if !force {
				continue
			}
			updated, err := s.voidAssetImage(id)
			if err != nil {
				s.log.Warn("generate-batch: void before regen failed", zap.Uint64("asset_id", id), zap.Error(err))
				continue
			}
			asset = updated
		default:
			continue
		}
		for _, spec := range specs {
			if err := s.dispatchAssetGeneration(asset, spec.ModelName, spec.PromptSuffix, ""); err != nil {
				s.log.Warn("generate-batch: dispatch failed",
					zap.Uint64("asset_id", id),
					zap.String("model_name", spec.ModelName),
					zap.Error(err),
				)
				continue
			}
			count++
		}
	}
	s.log.Info("generate-batch triggered",
		zap.Uint64("project_id", projectID),
		zap.Int("triggered", count),
		zap.Int("requested", len(assetIDs)),
		zap.Int("model_count", len(specs)),
	)
	return count, nil
}

func (s *AssetService) RetryFailed(projectID uint64, episodeID *uint64, modelName string) (int, error) {
	if s.isProjectGenerationPaused(projectID) {
		return 0, fmt.Errorf("project asset generation is paused")
	}
	assets, err := s.repo.FindByProjectID(projectID, "", "", episodeID)
	if err != nil {
		return 0, err
	}
	count := 0
	for i := range assets {
		if assets[i].Name == "__extracting__" || assets[i].Status != "failed" {
			continue
		}
		if err := s.dispatchAssetGeneration(&assets[i], modelName, "", ""); err != nil {
			s.log.Warn("dispatch retry-failed asset generation skipped", zap.Uint64("asset_id", assets[i].ID), zap.Error(err))
			continue
		}
		count++
	}
	return count, nil
}

// RetryOne —— 重试单个失败/待处理资产的图像生成，可指定模型
// RetryOne re-triggers generation for a single failed asset.
func (s *AssetService) RetryOne(id uint64, modelName string) (*model.Asset, error) {
	asset, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	// Allow retrying completed assets (regenerate the image with a new model)
	if asset.Status != "failed" && asset.Status != "pending" && asset.Status != "paused" && asset.Status != "completed" {
		return nil, fmt.Errorf("asset %d cannot be retried (status: %s)", id, asset.Status)
	}
	if s.isProjectGenerationPaused(asset.ProjectID) {
		return nil, fmt.Errorf("project asset generation is paused")
	}
	// Void existing image before regenerating a completed asset
	if asset.Status == "completed" {
		updated, err := s.voidAssetImage(id)
		if err != nil {
			return nil, fmt.Errorf("void asset before retry: %w", err)
		}
		asset = updated
	}
	if err := s.dispatchAssetGeneration(asset, modelName, "", ""); err != nil {
		return nil, err
	}
	return asset, nil
}

// voidAssetImage clears generated outputs and resets the asset back to pending.
// It removes both single-image and four-panel artifacts so the next generation starts cleanly.
func (s *AssetService) voidAssetImage(id uint64) (*model.Asset, error) {
	asset, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	metadata, err := parseAssetMetadata(asset.Metadata)
	if err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}
	delete(metadata, "generation_progress")
	delete(metadata, "panel_images")
	delete(metadata, "composite_image_url")
	delete(metadata, "seed")
	delete(metadata, "selected_generated_image_url")
	delete(metadata, "generation_request")
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	asset.Status = "pending"
	asset.ImageURL = ""
	asset.ErrorMsg = ""
	asset.Metadata = datatypes.JSON(metadataJSON)
	asset.PanelImages = nil
	asset.CompositeImageURL = ""
	asset.Seed = -1
	asset.UpdatedAt = time.Now()
	if err := s.repo.Update(asset); err != nil {
		return nil, err
	}
	return asset, nil
}

// ResetAsset clears the generated image and sets the asset back to pending (manual reset).
func (s *AssetService) ResetAsset(id uint64) (*model.Asset, error) {
	return s.voidAssetImage(id)
}

// Upload —— 手动上传资产图片并标记为已完成
func (s *AssetService) Upload(id uint64, filename string, content interface{ Read([]byte) (int, error) }) (*model.Asset, error) {
	asset, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	cdnURL, err := s.storage.Upload(filename, content)
	if err != nil {
		s.log.Error("upload asset image failed", zap.Uint64("id", id), zap.Error(err))
		return nil, err
	}
	metadata, err := parseAssetMetadata(asset.Metadata)
	if err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}
	delete(metadata, "generation_progress")
	delete(metadata, "panel_images")
	delete(metadata, "composite_image_url")
	delete(metadata, "seed")
	delete(metadata, "generation_request")
	metadata["selected_generated_image_url"] = cdnURL
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	asset.ImageURL = cdnURL
	asset.IsManual = true
	asset.Status = "completed"
	asset.ErrorMsg = ""
	asset.Metadata = datatypes.JSON(metadataJSON)
	asset.PanelImages = nil
	asset.CompositeImageURL = ""
	asset.Seed = -1
	asset.UpdatedAt = time.Now()
	if err := s.repo.Update(asset); err != nil {
		return nil, err
	}
	return asset, nil
}

type assetChatResult struct {
	AssistantReply     string `json:"assistant_reply"`
	UpdatedDescription string `json:"updated_description"`
}

type assetImageVersion struct {
	URL       string `json:"url"`
	Prompt    string `json:"prompt,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	Source    string `json:"source,omitempty"`
	ModelName string `json:"model_name,omitempty"`
}

type assetGenerationProgress struct {
	Stage     string `json:"stage"`
	Label     string `json:"label"`
	Detail    string `json:"detail,omitempty"`
	Percent   int    `json:"percent"`
	TaskID    int64  `json:"task_id,omitempty"`
	ModelName string `json:"model_name,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// Chat —— 记录用户修改要求并调用 LLM 生成资产修改建议与更新描述
func (s *AssetService) Chat(ctx context.Context, id uint64, message map[string]interface{}) (*model.Asset, error) {
	asset, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	history, err := normalizeAssetChatHistory(asset.AgentHistory)
	if err != nil {
		return nil, fmt.Errorf("normalize asset chat history: %w", err)
	}

	userMessage, err := buildAssetUserMessage(message)
	if err != nil {
		return nil, err
	}

	modelName := resolveAssetChatModelName(message, s.llmModel)
	chatResult := buildAssetFallbackChatResult(asset, userMessage)
	if strings.TrimSpace(s.llmBaseURL) != "" && strings.TrimSpace(modelName) != "" {
		llmResult, llmErr := s.callLLMAssetChat(ctx, asset, history, userMessage, modelName)
		if llmErr != nil {
			s.log.Warn("asset chat llm failed, using deterministic fallback",
				zap.Uint64("asset_id", asset.ID),
				zap.String("model", modelName),
				zap.Error(llmErr),
			)
		} else {
			if reply := strings.TrimSpace(llmResult.AssistantReply); reply != "" {
				chatResult.AssistantReply = reply
			}
			if desc := strings.TrimSpace(llmResult.UpdatedDescription); desc != "" {
				chatResult.UpdatedDescription = desc
			}
		}
	}

	history = append(history, userMessage, map[string]interface{}{
		"role":      "assistant",
		"content":   strings.TrimSpace(chatResult.AssistantReply),
		"timestamp": time.Now().Format(time.RFC3339),
	})

	historyJSON, err := json.Marshal(history)
	if err != nil {
		return nil, fmt.Errorf("marshal asset chat history: %w", err)
	}
	asset.AgentHistory = datatypes.JSON(historyJSON)

	if desc := strings.TrimSpace(chatResult.UpdatedDescription); desc != "" {
		asset.Description = desc
		asset.PromptUsed = ""
		// description changed, reset status so the asset can be re-generated
		if asset.Status != "generating" && asset.Status != "extracting" {
			asset.Status = "pending"
		}
	}

	asset.UpdatedAt = time.Now()
	if err := s.repo.Update(asset); err != nil {
		return nil, err
	}
	return asset, nil
}

func normalizeAssetChatHistory(raw datatypes.JSON) ([]map[string]interface{}, error) {
	if len(raw) == 0 {
		return []map[string]interface{}{}, nil
	}

	var history []interface{}
	if err := json.Unmarshal(raw, &history); err != nil {
		return nil, err
	}

	normalized := make([]map[string]interface{}, 0, len(history))
	for _, item := range history {
		normalizedItem := normalizeAssetChatMessage(item)
		if normalizedItem != nil {
			normalized = append(normalized, normalizedItem)
		}
	}
	return normalized, nil
}

func normalizeAssetChatMessage(raw interface{}) map[string]interface{} {
	switch msg := raw.(type) {
	case map[string]interface{}:
		content := strings.TrimSpace(stringValue(msg["content"]))
		legacyMessage := strings.TrimSpace(stringValue(msg["message"]))
		if content == "" {
			content = legacyMessage
		}
		if content == "" {
			return nil
		}

		role := strings.TrimSpace(stringValue(msg["role"]))
		if role != "user" && role != "assistant" {
			if legacyMessage != "" {
				role = "user"
			} else {
				role = "assistant"
			}
		}

		normalized := map[string]interface{}{
			"role":    role,
			"content": content,
		}
		if timestamp := strings.TrimSpace(stringValue(msg["timestamp"])); timestamp != "" {
			normalized["timestamp"] = timestamp
		}
		if imageURL := strings.TrimSpace(stringValue(msg["image_url"])); imageURL != "" {
			normalized["image_url"] = imageURL
		}
		return normalized
	case string:
		content := strings.TrimSpace(msg)
		if content == "" {
			return nil
		}
		return map[string]interface{}{"role": "assistant", "content": content}
	default:
		return nil
	}
}

func buildAssetUserMessage(message map[string]interface{}) (map[string]interface{}, error) {
	content := strings.TrimSpace(stringValue(message["content"]))
	if content == "" {
		content = strings.TrimSpace(stringValue(message["message"]))
	}
	if content == "" {
		return nil, fmt.Errorf("chat message content is required")
	}

	userMessage := map[string]interface{}{
		"role":      "user",
		"content":   content,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	if imageURL := strings.TrimSpace(stringValue(message["image_url"])); imageURL != "" {
		userMessage["image_url"] = imageURL
	}
	return userMessage, nil
}

func resolveAssetChatModelName(message map[string]interface{}, fallback string) string {
	for _, key := range []string{"model_name", "model"} {
		if value := strings.TrimSpace(stringValue(message[key])); value != "" {
			return value
		}
	}
	return strings.TrimSpace(fallback)
}

func buildAssetFallbackChatResult(asset *model.Asset, userMessage map[string]interface{}) *assetChatResult {
	instruction := strings.TrimSpace(stringValue(userMessage["content"]))
	currentDescription := strings.TrimSpace(asset.Description)

	return &assetChatResult{
		AssistantReply:     buildAssetFallbackAssistantReply(asset.Status),
		UpdatedDescription: mergeAssetDescription(currentDescription, instruction),
	}
}

func buildAssetFallbackAssistantReply(assetStatus string) string {
	if assetStatus == "generating" {
		return "已记录这次资源修改，并同步到当前描述。当前图片仍在生成中，完成后可继续重新生成新版本。"
	}
	return "已记录这次资源修改，并同步到当前描述。接下来会按这些要求重新生成图片。"
}

func mergeAssetDescription(currentDescription, instruction string) string {
	currentDescription = strings.TrimSpace(currentDescription)
	instruction = strings.TrimSpace(instruction)

	if instruction == "" {
		return currentDescription
	}
	if currentDescription == "" || strings.Contains(currentDescription, instruction) {
		if currentDescription != "" {
			return currentDescription
		}
		return instruction
	}

	const marker = "补充要求："
	if strings.Contains(currentDescription, marker) {
		return currentDescription + "\n" + instruction
	}
	return currentDescription + "\n\n" + marker + "\n" + instruction
}

func (s *AssetService) callLLMAssetChat(ctx context.Context, asset *model.Asset, history []map[string]interface{}, userMessage map[string]interface{}, modelName string) (*assetChatResult, error) {
	systemPrompt := `你是影视资源设计助手，负责根据用户的修改要求优化当前资源描述。

请结合资产类型、资产名称、当前描述和历史对话，返回严格 JSON：
{
  "assistant_reply": "直接回复用户，说明你理解了哪些修改，并给出本次更新后的重点。使用简洁中文。",
  "updated_description": "给图像生成使用的完整中文描述，保留可视化细节。如果用户要求不明确，也要在现有描述基础上尽可能补全。"
}

要求：
- assistant_reply 必须直接回答用户，不要解释 JSON。
- updated_description 必须适合直接作为图像生成提示。
- 如果用户只是在补充细节，也要输出完整描述，而不是只输出增量。`

	messages := make([]map[string]string, 0, len(history)+3)
	messages = append(messages, map[string]string{"role": "system", "content": systemPrompt})

	assetContext := fmt.Sprintf(`资产信息：
- 类型：%s
- 名称：%s
- 当前描述：%s
- 当前状态：%s
- 当前生成提示词：%s`, asset.Type, asset.Name, emptyFallback(asset.Description, "暂无"), asset.Status, emptyFallback(asset.PromptUsed, "暂无"))
	messages = append(messages, map[string]string{"role": "system", "content": assetContext})

	// Append project visual style hint if provided by the caller (e.g. model defaultPrompt / image_skill)
	if skillCtx := strings.TrimSpace(stringValue(userMessage["skill_context"])); skillCtx != "" {
		styleContext := fmt.Sprintf("当前项目视觉风格提示（请在 updated_description 中保留并融合此风格）：%s", skillCtx)
		messages = append(messages, map[string]string{"role": "system", "content": styleContext})
	}

	start := 0
	if len(history) > 4 {
		start = len(history) - 4
	}
	for _, item := range history[start:] {
		role := strings.TrimSpace(stringValue(item["role"]))
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(stringValue(item["content"]))
		if content == "" {
			continue
		}
		messages = append(messages, map[string]string{"role": role, "content": content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": strings.TrimSpace(stringValue(userMessage["content"]))})

	reqBody := map[string]interface{}{
		"model":           modelName,
		"messages":        messages,
		"temperature":     0.3,
		"max_tokens":      1200,
		"response_format": map[string]string{"type": "json_object"},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal asset chat request: %w", err)
	}

	llmCtx, cancel := context.WithTimeout(ctx, s.llmTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(llmCtx, http.MethodPost, s.llmBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(s.llmAPIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)
	}

	resp, err := (&http.Client{Timeout: s.llmTimeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read llm response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm error %d: %s", resp.StatusCode, string(respBody))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		preview := string(respBody)
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return nil, fmt.Errorf("parse llm response: %w — body: %s", err, preview)
	}
	if llmResp.Error != nil {
		return nil, fmt.Errorf("llm api error: %s", llmResp.Error.Message)
	}
	if len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("llm returned no choices")
	}

	content := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) > 2 {
			lines = lines[1 : len(lines)-1]
		}
		content = strings.Join(lines, "\n")
	}

	var result assetChatResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse asset chat json: %w", err)
	}
	return &result, nil
}

func (s *AssetService) chatProviderByHints(hints ...string) llmProvider {
	joined := strings.ToLower(strings.Join(hints, " "))
	switch {
	// easyart 必须在其他 provider 之前匹配，因为 api_key_ref='easyart' 的模型
	// 无论其 provider 字段是什么（openai/zhipu 等），都应使用 easyart API key。
	case strings.Contains(joined, "easyart"):
		if s.llmClaude.apiKey != "" {
			return llmProvider{apiKey: s.llmClaude.apiKey}
		}
	case strings.Contains(joined, "claude") || strings.Contains(joined, "anthropic"):
		if s.llmClaude.baseURL != "" || s.llmClaude.apiKey != "" {
			return s.llmClaude
		}
	case strings.Contains(joined, "qwen") || strings.Contains(joined, "dashscope") || strings.Contains(joined, "aliyun"):
		if s.llmQwen.baseURL != "" || s.llmQwen.apiKey != "" {
			return s.llmQwen
		}
	case strings.Contains(joined, "zhipu") || strings.Contains(joined, "glm") || strings.Contains(joined, "bigmodel"):
		if s.llmZhipu.baseURL != "" || s.llmZhipu.apiKey != "" {
			return s.llmZhipu
		}
	case strings.Contains(joined, "gemini"):
		if s.llmGemini.baseURL != "" || s.llmGemini.apiKey != "" {
			return s.llmGemini
		}
	case strings.Contains(joined, "openai") || strings.Contains(joined, "llm") || strings.Contains(joined, "default"):
		return llmProvider{baseURL: s.llmBaseURL, apiKey: s.llmAPIKey}
	}
	return llmProvider{}
}

func (s *AssetService) chatProviderByEndpoint(endpoint string) llmProvider {
	joined := strings.ToLower(strings.TrimSpace(endpoint))
	switch {
	case strings.Contains(joined, "api.easyart.cc"):
		if s.llmClaude.apiKey != "" {
			return llmProvider{apiKey: s.llmClaude.apiKey}
		}
	case strings.Contains(joined, "poloai.top"), strings.Contains(joined, "ppapi.vip"), strings.Contains(joined, "openxs.top"):
		if s.llmAPIKey != "" {
			return llmProvider{apiKey: s.llmAPIKey}
		}
	case strings.Contains(joined, "dashscope.aliyuncs.com"):
		if s.llmQwen.baseURL != "" || s.llmQwen.apiKey != "" {
			return s.llmQwen
		}
	case strings.Contains(joined, "bigmodel.cn"):
		if s.llmZhipu.baseURL != "" || s.llmZhipu.apiKey != "" {
			return s.llmZhipu
		}
	}
	return llmProvider{}
}

func isGenericChatProxyEndpoint(endpoint string) bool {
	joined := strings.ToLower(strings.TrimSpace(endpoint))
	switch {
	case strings.Contains(joined, "poloai.top"), strings.Contains(joined, "ppapi.vip"), strings.Contains(joined, "openxs.top"):
		return true
	default:
		return false
	}
}

func isGeminiRuntimeModel(runtimeModel *chatRuntimeModel) bool {
	if runtimeModel == nil {
		return false
	}
	joined := strings.ToLower(strings.Join([]string{
		runtimeModel.Provider,
		runtimeModel.ModelKey,
		runtimeModel.Name,
		stringPtrValue(runtimeModel.APIKeyRef),
	}, " "))
	return strings.Contains(joined, "gemini") || strings.Contains(joined, "google")
}

// chatFreeProvider resolves which base_url + api_key to use based on model name prefix.
func (s *AssetService) chatFreeProvider(modelName string) (baseURL, apiKey string) {
	if provider := s.chatProviderByHints(modelName); provider.baseURL != "" || provider.apiKey != "" {
		if provider.baseURL != "" {
			return provider.baseURL, provider.apiKey
		}
	}
	m := strings.ToLower(modelName)
	switch {
	case strings.HasPrefix(m, "claude"):
		if s.llmClaude.baseURL != "" {
			return s.llmClaude.baseURL, s.llmClaude.apiKey
		}
	case strings.HasPrefix(m, "qwen"):
		if s.llmQwen.baseURL != "" {
			return s.llmQwen.baseURL, s.llmQwen.apiKey
		}
	case strings.HasPrefix(m, "glm") || strings.HasPrefix(m, "chatglm"):
		if s.llmZhipu.baseURL != "" {
			return s.llmZhipu.baseURL, s.llmZhipu.apiKey
		}
	case strings.HasPrefix(m, "gemini"):
		if s.llmGemini.baseURL != "" {
			return s.llmGemini.baseURL, s.llmGemini.apiKey
		}
	}
	return s.llmBaseURL, s.llmAPIKey
}

func (s *AssetService) cachedChatRuntimeModels() ([]chatRuntimeModel, bool) {
	s.chatCatalogMu.RLock()
	defer s.chatCatalogMu.RUnlock()
	if time.Now().After(s.chatCatalogExpiry) || len(s.chatCatalog) == 0 {
		return nil, false
	}
	out := make([]chatRuntimeModel, len(s.chatCatalog))
	copy(out, s.chatCatalog)
	return out, true
}

func (s *AssetService) listChatRuntimeModels(ctx context.Context) ([]chatRuntimeModel, error) {
	if models, ok := s.cachedChatRuntimeModels(); ok {
		return models, nil
	}
	if strings.TrimSpace(s.modelServiceBaseURL) == "" {
		return nil, fmt.Errorf("model service base url not configured")
	}
	metaTimeout := 5 * time.Second
	if s.llmTimeout > 0 && s.llmTimeout < metaTimeout {
		metaTimeout = s.llmTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, metaTimeout)
	defer cancel()
	url := s.modelServiceBaseURL + "/api/v1/models?type=llm&enabled=true"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: metaTimeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		preview := string(body)
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return nil, fmt.Errorf("model service error %d: %s", resp.StatusCode, preview)
	}
	var payload struct {
		Code int                `json:"code"`
		Data []chatRuntimeModel `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse model catalog: %w", err)
	}
	s.chatCatalogMu.Lock()
	s.chatCatalog = make([]chatRuntimeModel, len(payload.Data))
	copy(s.chatCatalog, payload.Data)
	s.chatCatalogExpiry = time.Now().Add(30 * time.Second)
	out := make([]chatRuntimeModel, len(payload.Data))
	copy(out, payload.Data)
	s.chatCatalogMu.Unlock()
	return out, nil
}

func (s *AssetService) resolveChatRuntimeModel(ctx context.Context, modelName string) (*chatRuntimeModel, error) {
	models, err := s.listChatRuntimeModels(ctx)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimSpace(modelName))
	for i := range models {
		if !models[i].IsActive {
			continue
		}
		if strings.ToLower(strings.TrimSpace(models[i].ModelKey)) == needle || strings.ToLower(strings.TrimSpace(models[i].Name)) == needle {
			return &models[i], nil
		}
	}
	return nil, fmt.Errorf("model %q not found in runtime catalog", modelName)
}

func (s *AssetService) resolveChatFreeRoute(ctx context.Context, modelName string) (baseURL, apiKey string) {
	runtimeModel, err := s.resolveChatRuntimeModel(ctx, modelName)
	if err != nil {
		if s.log != nil && strings.TrimSpace(s.modelServiceBaseURL) != "" {
			s.log.Warn("resolve chat runtime model failed", zap.String("model_name", modelName), zap.Error(err))
		}
		return s.chatFreeProvider(modelName)
	}
	fallbackBase, fallbackKey := s.chatFreeProvider(modelName)
	if isGeminiRuntimeModel(runtimeModel) && isGenericChatProxyEndpoint(runtimeModel.APIEndpoint) {
		if fallbackBase != "" || fallbackKey != "" {
			return fallbackBase, fallbackKey
		}
	}
	provider := s.chatProviderByEndpoint(runtimeModel.APIEndpoint)
	if provider.baseURL == "" && provider.apiKey == "" {
		provider = s.chatProviderByHints(stringPtrValue(runtimeModel.APIKeyRef), runtimeModel.Provider, runtimeModel.ModelKey, runtimeModel.Name)
	}
	baseURL = strings.TrimRight(strings.TrimSpace(runtimeModel.APIEndpoint), "/")
	if provider.apiKey == "" {
		provider.apiKey = fallbackKey
	}
	if provider.baseURL == "" {
		provider.baseURL = fallbackBase
	}
	if baseURL == "" {
		baseURL = provider.baseURL
	}
	if baseURL == "" {
		baseURL = fallbackBase
	}
	return baseURL, provider.apiKey
}

func chatCompletionsURL(base string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	if trimmed == "" {
		return ""
	}
	if strings.HasSuffix(trimmed, "/chat/completions") {
		return trimmed
	}
	return trimmed + "/chat/completions"
}

// ChatFree performs a free-form LLM conversation without requiring an asset.
// It is used by the left-side reference chat panel on the frontend.
// Returns the assistant's plain-text reply.
func (s *AssetService) ChatFree(ctx context.Context, messages []map[string]string, modelName string) (string, error) {
	if strings.TrimSpace(s.llmBaseURL) == "" {
		return "LLM 服务未配置，无法响应", nil
	}
	if modelName == "" {
		modelName = s.llmModel
	}

	providerBase, providerKey := s.resolveChatFreeRoute(ctx, modelName)

	reqBody := map[string]interface{}{
		"model":       modelName,
		"messages":    messages,
		"temperature": 0.7,
		"max_tokens":  2000,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal chat request: %w", err)
	}

	llmCtx, cancel := context.WithTimeout(ctx, s.llmTimeout)
	defer cancel()

	providerURL := chatCompletionsURL(providerBase)
	req, err := http.NewRequestWithContext(llmCtx, http.MethodPost, providerURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(providerKey) != "" {
		req.Header.Set("Authorization", "Bearer "+providerKey)
	}

	resp, err := (&http.Client{Timeout: s.llmTimeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("llm call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read llm response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		preview := string(respBody)
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return "", fmt.Errorf("llm error %d (provider=%s): %s", resp.StatusCode, providerURL, preview)
	}

	// Guard against HTML error pages returned with 200 (e.g. WAF/proxy pages).
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "application/json") {
		preview := string(respBody)
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return "", fmt.Errorf("llm returned non-JSON content-type %q (provider=%s): %s", ct, providerURL, preview)
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		preview := string(respBody)
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return "", fmt.Errorf("parse llm response (provider=%s): %w — body: %s", providerURL, err, preview)
	}
	if llmResp.Error != nil {
		return "", fmt.Errorf("llm api error: %s", llmResp.Error.Message)
	}
	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}
	return strings.TrimSpace(llmResp.Choices[0].Message.Content), nil
}

func stringValue(v interface{}) string {
	switch value := v.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	default:
		return ""
	}
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// refineAssetDescriptionForGeneration calls LLM to translate and optimize an asset
// description (often Chinese) into an English image generation description with skill hints.
// It fetches active storyboard skills as visual style guidelines.
// Returns the original description if LLM is unavailable or fails.
func (s *AssetService) refineAssetDescriptionForGeneration(ctx context.Context, projectID uint64, assetType, name, description, stylePreset, visualHint string) string {
	if strings.TrimSpace(s.llmBaseURL) == "" || strings.TrimSpace(description) == "" {
		// Fallback: strip character-related sentences from scene/prop descriptions.
		return stripCharacterMentions(assetType, description)
	}

	// Fetch storyboard skills as project-level visual style guidelines.
	skillHints := ""
	if s.skillRepo != nil {
		if skills, err := s.skillRepo.ListActiveByUseCase(int64(projectID), "storyboard"); err == nil && len(skills) > 0 {
			var parts []string
			for _, sk := range skills {
				if strings.TrimSpace(sk.Description) != "" {
					parts = append(parts, fmt.Sprintf("- [%s] %s", sk.Name, sk.Description))
				}
			}
			skillHints = strings.Join(parts, "\n")
		}
	}

	canonical := stylepreset.Canonical(stylePreset)
	isLiveAction := canonical == stylepreset.LiveActionFilm || canonical == stylepreset.LiveActionShort

	// Base system prompt differs by rendering style.
	var systemPrompt string
	if isLiveAction {
		systemPrompt = `You are a professional image generation prompt engineer specialising in photorealistic live-action film and short drama production.
Your task: Translate and optimize the given asset description into a vivid, precise English image generation prompt for Stable Diffusion / Flux / DALL-E.

Core rules:
1. Output ONLY the optimized English prompt. No JSON, no labels, no preamble, no "Here is the prompt:" prefix.
2. Focus EXCLUSIVELY on visible, tangible attributes: physical form, materials, surface quality, colors, textures, proportions, lighting conditions.
3. Translate ALL Chinese content to fluent, natural English.
4. Expand sparse or abstract descriptions into rich, specific visual detail using photographic and art-direction language.
5. Preserve EVERY specific physical characteristic from the source description exactly — do not summarise or omit.
6. Drop personality traits, story events, narrative context, and anything not directly visible in a still image.
7. Do NOT invent visual elements not implied by the source.
8. Word count: 60–200 words (use the full range for complex subjects).`
	} else {
		systemPrompt = `You are a professional anime / illustration image generation prompt engineer.
Your task: Translate and optimize the given asset description into a structured prompt for anime image generators (NovelAI, Stable Diffusion anime models).

Core rules:
1. Output ONLY the final prompt. No JSON, no labels, no preamble.
2. Format: brief Chinese visual description followed by English quality/style tags in parentheses.
3. Translate any English in the source to Chinese for the description section; keep technical tags in English.
4. Preserve ALL specific visual characteristics from the source.
5. Drop personality traits, story events, and non-visual narrative content.
6. Do NOT invent visual elements not implied by the source.
7. Word count: 40–120 words.`
	}

	// Inject type-specific focus and constraints.
	normalizedType := strings.ToLower(strings.TrimSpace(assetType))
	switch normalizedType {
	case "image":
		// Free-form creative image — preserve ALL subjects including people.
		if isLiveAction {
			systemPrompt += `

**FREE-FORM IMAGE asset:** Translate and enhance the description into a vivid English image generation prompt. Preserve ALL subjects and visual elements exactly as described — including any people, characters, actions, poses, and settings. Do NOT suppress, strip, or add any no-people constraints. Output a natural, coherent description.`
		} else {
			systemPrompt += `

**FREE-FORM IMAGE asset:** 自由创意图片。翻译并优化描述为图片生成提示词，完整保留所有主体（包括人物、动作、场景），不添加任何禁人约束。直接输出自然连贯的描述。`
		}
	case "character", "角色", "人物":
		if isLiveAction {
			systemPrompt += `

**CHARACTER asset — describe from head to toe in this order:**
Face & features: face shape, skin tone, eye shape + color, eyebrow style, nose and lip shape, any distinctive marks.
Hair: length, texture, color, style/arrangement.
Build: height estimate, physique (slender / athletic / stocky), posture tendency.
Costume: each garment layer — fabric type, cut, color, fastening details, surface decoration.
Accessories: belt, headwear, footwear, jewellery, carried items.
**STRICT:** Clothing must match the character's gender exactly. Male → male-only garments. Female → female-only garments. Correct any cross-gender attire silently.
**PERIOD + CULTURAL CONSTRAINTS (authoritative — integrate, do not drop):** Any content the input labels with "时代：" (era), "人物：" (ethnicity), "地域造型：" (region styling), or "项目视觉基调："/"视觉基调：" (project visual tone) carries period wardrobe, hairstyle, and cultural rules that MUST shape the character's garments, accessories, and grooming. Weave those constraints naturally into the head-to-toe description; drop only the label syntax ("时代："/"视觉基调：" etc.), never the content itself. If an individual personal description conflicts with the period constraints (e.g. a Tang-dynasty character wearing a modern suit), silently reconcile to the period. Ignore ONLY the "场景：" (setting) section for characters — scene architecture does not belong in a character portrait.`
		} else {
			systemPrompt += `

**CHARACTER asset:** 描述顺序：面部特征、发型发色、肤色、身形体型、服装（从内到外每层）、配饰、鞋履。
服装必须严格符合角色性别，不得混用异性服装。
**时代与文化约束（权威信息，必须整合而非丢弃）**：输入中凡以"时代："、"人物："、"地域造型："、"项目视觉基调："、"视觉基调："等标签出现的内容，均为该角色服饰、发型、妆容、配饰必须遵循的历史年代与文化规则。请将这些信息自然融入从头到脚的描述中，仅去除标签文字本身（"时代："等），不丢弃内容。若角色个人外貌与时代规则冲突（如唐代角色穿西装），静默调整至符合时代。仅忽略"场景："标签所含建筑环境信息。
**输出格式：三视图角色设定参考图**：画面左侧三分之一为头肩肖像（头部颈部完整可见，蝴蝶光/柔光面部照明）；右侧三分之二为全身正面、侧面、背面三视图并排。九头身比例，修长身材。纯白背景。三视图中角色设计保持完全一致。`
		}
	case "scene", "场景", "地点":
		if isLiveAction {
			systemPrompt += `

**SCENE asset — describe the empty environment in this order:**
Architectural style and structural materials (stone, timber, brick, etc.).
Spatial layout: foreground elements, mid-ground focal point, background depth.
Lighting: quality (soft/hard), direction, colour temperature, time of day or artificial sources.
Atmosphere: weather, air quality (haze/mist/clear), mood conveyed by the space.
Key furnishings or props that define the location.
Era-specific details: surfaces, textures, decay level, period objects.
**STRICT:** Absolutely NO people, faces, silhouettes, or body parts in the description.
**PERIOD + REGIONAL CONSTRAINTS (authoritative — integrate, do not drop):** Any content labelled "时代：" (era), "场景：" (scene/region), or "项目视觉基调："/"视觉基调：" (project visual tone) specifies the period architecture and regional environment and MUST shape the building materials, lighting, and furnishings. Weave those constraints naturally into the environment description; drop only the label syntax, never the content. Ignore ONLY the "人物：" (ethnicity) and "地域造型：" (character styling) sections — those describe people, not scenes.`
		} else {
			systemPrompt += `

**SCENE asset:** 描述空景环境：建筑风格与材料、空间前中后景层次、光线方向与氛围、色调与天气、代表性陈设。
严格禁止：任何人物、人脸、人手或人体部位。
**时代与地域约束（权威信息，必须整合而非丢弃）**：输入中以"时代："、"场景："、"项目视觉基调："、"视觉基调："等标签出现的内容为该场景的历史年代与地域环境规则，必须体现在建筑材料、光线氛围与陈设中。自然融入描述，仅去除标签文字本身，不丢弃内容。仅忽略"人物："、"地域造型："标签（那是人物信息，不属于场景）。`
		}
	case "prop", "道具", "item", "物品":
		if isLiveAction {
			systemPrompt += `

**PROP/ITEM asset — describe the object in this order:**
Overall silhouette and form factor (dimensions, rough shape, weight impression).
Primary material: type, finish (matte/gloss/patina), colour.
Secondary materials and hardware details (hinges, clasps, inlays, etc.).
Surface texture: grain, wear, engraving, paint, lacquer condition.
Construction: visible joints, stitching, metalwork, craftsmanship quality.
Scale reference if helpful (e.g. "fits in one hand", "40 cm wide").
Era or cultural style cues embedded in the design.
**STRICT:** Absolutely NO people, hands, faces, or body parts.
**PERIOD CONSTRAINTS (authoritative — integrate, do not drop):** Any content labelled "时代：" (era) or "项目视觉基调："/"视觉基调：" (project visual tone) specifies the period style the object MUST match — materials, craftsmanship level, motifs, and finish all flow from those constraints. Weave them naturally into the object description; drop only the label syntax, never the content. Ignore ONLY the "人物："/"地域造型：" (character/regional styling) and "场景：" (scene) sections.`
		} else {
			systemPrompt += `

**PROP/ITEM asset:** 描述物品本身：整体形态与尺寸、主要材质与表面处理、次要材质与五金件、表面纹理与工艺细节、时代风格元素。
严格禁止：任何人物、人手、人脸或人体部位。
**时代约束（权威信息，必须整合而非丢弃）**：输入中以"时代："、"项目视觉基调："、"视觉基调："等标签出现的内容为该物品必须遵循的历史年代风格规则，材料、工艺水平、纹饰与表面处理都应由此约束。自然融入物品描述，仅去除标签文字本身，不丢弃内容。仅忽略"人物："、"地域造型："、"场景："标签。`
		}
	}

	if skillHints != "" {
		systemPrompt += "\n\n**Project visual style guidelines — weave these naturally into the description:**\n" + skillHints
	}
	if stylePreset != "" {
		systemPrompt += "\n\nRendering style preset: " + stylePreset
	}
	if visualHint != "" {
		systemPrompt += "\n\nProject visual tone (for context; do not reproduce as labels):\n" + visualHint
	}

	userContent := fmt.Sprintf("Asset type: %s\nName: %s\nDescription:\n%s\n\nOptimized image generation prompt:", assetType, name, description)

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userContent},
		},
		"temperature": 0.4,
		"max_tokens":  512,
	}

	data, _ := json.Marshal(reqBody)
	timeout := s.llmTimeout
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 30 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return stripCharacterMentions(assetType, description)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if s.log != nil {
			s.log.Warn("refine asset description: LLM call failed",
				zap.Uint64("project_id", projectID),
				zap.String("asset_type", assetType),
				zap.String("name", name),
				zap.Error(err))
		}
		return stripCharacterMentions(assetType, description)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return stripCharacterMentions(assetType, description)
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return stripCharacterMentions(assetType, description)
	}

	refined := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	// Sanity check: reject empty or suspiciously short responses
	if len([]rune(refined)) < 20 {
		return stripCharacterMentions(assetType, description)
	}

	if s.log != nil {
		s.log.Info("asset description refined for generation",
			zap.Uint64("project_id", projectID),
			zap.String("type", assetType),
			zap.String("name", name),
		)
	}
	return refined
}

// polishAssetImagePrompt runs a final LLM pass on the fully composed draft prompt.
// It merges multi-language fragments (Chinese structural labels + English tags +
// appended visual hints) into a single coherent, semantic image generation prompt.
// Returns draftPrompt unchanged on LLM failure or when LLM is not configured.
func (s *AssetService) polishAssetImagePrompt(ctx context.Context, draftPrompt, assetType, stylePreset string) string {
	if strings.TrimSpace(s.llmBaseURL) == "" || strings.TrimSpace(draftPrompt) == "" {
		return draftPrompt
	}

	canonical := stylepreset.Canonical(stylePreset)
	isLiveAction := canonical == stylepreset.LiveActionFilm || canonical == stylepreset.LiveActionShort

	systemPrompt := buildPolishSystemPrompt(assetType, isLiveAction)

	userContent := "Draft prompt to polish:\n\n" + draftPrompt + "\n\nPolished image generation prompt:"

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userContent},
		},
		"temperature": 0.3,
		"max_tokens":  700,
	}

	data, _ := json.Marshal(reqBody)
	timeout := 25 * time.Second
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return draftPrompt
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if s.log != nil {
			s.log.Warn("polish asset image prompt: LLM call failed",
				zap.String("asset_type", assetType),
				zap.Error(err))
		}
		return draftPrompt
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return draftPrompt
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return draftPrompt
	}

	polished := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	if len([]rune(polished)) < 30 {
		return draftPrompt
	}

	if s.log != nil {
		s.log.Info("asset image prompt polished",
			zap.String("asset_type", assetType),
			zap.String("style_preset", stylePreset),
		)
	}
	return polished
}

// buildPolishSystemPrompt returns a system prompt for the final prompt polish LLM call,
// tailored to asset type and rendering style (live-action vs anime).
func buildPolishSystemPrompt(assetType string, isLiveAction bool) string {
	normalizedType := strings.TrimSpace(strings.ToLower(assetType))

	// --- Asset-type-specific role and constraints ---
	var role, constraint string
	switch normalizedType {
	case "image":
		role = "image generation prompt engineer"
		constraint = `This is a FREE-FORM IMAGE request. Preserve ALL visual information from the draft, including any people, characters, actions, poses, and environments. Do NOT add no-people constraints or strip any subjects.`
	case "character", "角色", "人物":
		if isLiveAction {
			role = "photorealistic live-action character portrait specialist"
			constraint = `The subject is a CHARACTER.
- Output a single full-body portrait on a pure white studio background.
- Do NOT add a multi-view layout or any reference sheet structure.
- Describe physical attributes in order: face, hair, skin tone, build, costume (head to foot), accessories.
- Clothing must strictly match the character's gender. Silently correct any cross-gender attire.
- Quality tokens first: RAW photo, 8K UHD, professional studio lighting.
- Drop personality traits, story events, and all non-visual content.`
		} else {
			role = "anime/illustration character reference sheet specialist"
			constraint = `The subject is a CHARACTER.
- Output a CHARACTER REFERENCE SHEET on a pure white background.
- Layout: head-and-shoulder portrait on the left third (head and neck fully visible, butterfly/soft lighting); full-body front view, side view, and back view arranged side-by-side on the right two-thirds.
- Body proportions: 9-head body ratio, slender elegant figure, elongated legs.
- Describe appearance tags in order: face features, hair color/style, skin tone, body type, clothing (head to foot), accessories.
- Clothing must strictly match the character's gender.
- Lead quality tokens: (masterpiece:1.5), best quality, ultra-detailed, 8K.
- Ensure character design is visually consistent across all three body views.
- Drop personality, story events, and non-visual content.`
		}
	case "scene", "场景", "location", "地点", "环境":
		if isLiveAction {
			role = "photorealistic live-action location and environment specialist"
			constraint = `The subject is a SCENE / ENVIRONMENT.
- Absolutely NO people, faces, hands, silhouettes, or human figures in the output.
- Describe the empty environment in spatial layers: foreground elements → mid-ground focal point → background depth.
- Preserve: architectural style, structural materials, lighting direction/quality/colour, atmosphere, era-specific props.
- Keep cinematic quality tags (RAW photo, 8K UHD, cinematic depth of field) at the beginning.
- End with: no people, no text, no watermark.`
		} else {
			role = "anime/illustration environment concept artist"
			constraint = `The subject is a SCENE / ENVIRONMENT.
- Absolutely NO people or human figures.
- Lead with: environment concept art, no people, background art.
- Describe: atmosphere, colour palette, architectural details, lighting, era-specific objects.
- Lead quality tokens: (masterpiece:1.5), best quality.`
		}
	case "prop", "道具", "item", "物品":
		if isLiveAction {
			role = "photorealistic product photography and prop design specialist"
			constraint = `The subject is a PROP / OBJECT.
- Absolutely NO people, hands, faces, or human figures in the output.
- Describe the isolated object: overall silhouette and scale → primary material + finish → secondary materials and hardware → surface texture and craftsmanship → era/cultural style cues.
- Keep product photography tags (RAW photo, 8K UHD, hero shot, three-point studio lighting, neutral background) at the beginning.
- End with: no people, no hands, no text, no watermark.`
		} else {
			role = "anime/illustration prop concept artist"
			constraint = `The subject is a PROP / OBJECT.
- Absolutely NO people or human figures.
- Lead with: prop design sheet, product illustration, no people.
- Describe: object form, material, surface texture, era style, colour.
- Lead quality tokens: (masterpiece:1.5), best quality, ultra-detailed.`
		}
	default:
		role = "image generation prompt engineer"
		constraint = "Preserve all visual information from the draft."
	}

	// --- Output format by style ---
	var outputFormat string
	if isLiveAction {
		outputFormat = `**Output format (live-action / photorealistic):**
Line 1: Essential quality tokens — RAW photo, photorealistic, 8K UHD, [asset-type-specific header], separated by commas.
Line 2-3: Vivid, coherent English prose description (2-4 sentences) integrating all visual details (appearance, materials, era, lighting, atmosphere). Write naturally — not as a tag list.
Final tokens: comma-separated technical/negative constraints (no anime, no watermark, no text, etc.).

Total word count: 80–220 words.
NEVER use Chinese structural labels: "视觉基调：", "项目视觉基调：", "核心视觉描述：", "时代：", "人物：", "场景：", "道具名称：".`
	} else {
		outputFormat = `**Output format (anime / illustration):**
Line 1: Quality tokens — (masterpiece:1.5), best quality, ultra-detailed, [anime style tags], separated by commas.
Line 2-4: Chinese description of the visual content, with key English style/type tags inline in parentheses.
Final line: Negative tags — 无文字, 无水印, and any asset-type negatives (无人物 for scene/prop).

Total word count: 60–150 words.
NEVER include raw template labels: "核心视觉描述：", "项目视觉基调：", "道具名称：", "场景名称：", "角色名称："。`
	}

	return fmt.Sprintf(`You are a %s and expert image generation prompt engineer.

You will receive a DRAFT image generation prompt that was assembled mechanically from multiple sources. It may contain:
- A mix of Chinese and English text
- Structural template labels (e.g. "视觉基调：", "项目视觉基调：", "核心视觉描述：", "道具名称：", "场景名称：")
- Comma-separated quality/style tag lists
- Full descriptive sentences in either language
- Visual style guidelines appended as a trailing block

Your task: Rewrite the draft into a single, polished, coherent image generation prompt.

Rules:
1. Output ONLY the final prompt — no preamble, no labels, no "Here is the polished prompt:" prefix.
2. Preserve ALL visual information: appearance, materials, textures, colors, lighting, era, costume, architecture, atmosphere.
3. Remove all structural template labels and boilerplate. Merge fragments into natural, fluent content.
4. Do NOT invent new visual elements that are not implied by the draft.
5. Do NOT include personality traits, story events, or narrative context unless they directly describe visible appearance.
6. If the draft contradicts itself (e.g. male character described in female clothing), silently resolve to the correct option.

Asset-type constraint:
%s

%s`, role, constraint, outputFormat)
}

// empty string falls back to anime-2d behaviour.
func composeAssetImagePrompt(assetType, name, description, promptUsed, stylePreset string) string {
	trimmedType := strings.TrimSpace(assetType)
	trimmedName := strings.TrimSpace(name)
	trimmedDescription := strings.TrimSpace(description)
	trimmedPrompt := strings.TrimSpace(promptUsed)

	canonical := stylepreset.Canonical(stylePreset)
	isLiveAction := canonical == stylepreset.LiveActionFilm || canonical == stylepreset.LiveActionShort

	switch strings.TrimSpace(strings.ToLower(trimmedType)) {
	case "character", "角色", "人物":
		return composeCharacterPrompt(trimmedName, trimmedDescription, trimmedPrompt, isLiveAction)
	case "scene", "场景", "location", "地点", "环境":
		// Strip any character-related content from the description before composing.
		return composeScenePrompt(trimmedName, stripCharacterMentions("scene", trimmedDescription), trimmedPrompt, isLiveAction)
	case "prop", "道具", "item", "物品":
		// Strip any character-related content from the description before composing.
		return composePropPrompt(trimmedName, stripCharacterMentions("prop", trimmedDescription), trimmedPrompt, isLiveAction)
	case "image":
		// Free-form image generation — preserve ALL subjects (including people) exactly as described.
		var parts []string
		if trimmedDescription != "" {
			parts = append(parts, trimmedDescription)
		}
		if trimmedPrompt != "" {
			parts = append(parts, trimmedPrompt)
		}
		if isLiveAction {
			parts = append(parts, "highly detailed", "masterpiece quality", "RAW photo", "8K UHD", "no text", "no watermarks")
		} else {
			parts = append(parts, "highly detailed", "masterpiece quality", "best quality", "no text", "no watermarks")
		}
		return strings.Join(parts, ", ")
	default:
		parts := []string{
			"请为影视前期设定生成一张资源概念图，画面必须聚焦单一核心主体，构图清晰，可直接作为后续分镜和视频生成参考。",
		}
		if trimmedType != "" {
			parts = append(parts, fmt.Sprintf("资源类型：%s。", trimmedType))
		}
		if trimmedName != "" {
			parts = append(parts, fmt.Sprintf("资源名称：%s。", trimmedName))
		}
		if trimmedDescription != "" {
			parts = append(parts, fmt.Sprintf("核心视觉描述：%s。", trimmedDescription))
		}
		parts = append(parts, "主体要求：突出最重要的视觉主体，确保主题集中、细节明确、画面服务于影视开发。")
		if trimmedPrompt != "" {
			parts = append(parts, fmt.Sprintf("补充生成要求：%s。", trimmedPrompt))
		}
		parts = append(parts,
			"画面要求：主体明确、光影统一、材质可信、背景服务主体、细节可读、层次分明。",
			"输出倾向：高质量影视概念设定图，单帧静态画面，无文字、无水印、无拼贴、无多余边框。",
		)
		return strings.Join(parts, " ")
	}
}

// composeCharacterPrompt 生成角色参考图的提示词。
// 真人写实风格：纯白背景单人全身正面图。
// 动漫/插画风格：三视图角色设定参考图（左侧头肩肖像+右侧全身正面/侧面/背面并排）。
func composeCharacterPrompt(name, description, promptUsed string, isLiveAction bool) string {
	var tags []string

	if isLiveAction {
		// Live-action: single full-body portrait on pure white studio background.
		identityAnchor := "the same single character, consistent facial structure, identical hairstyle, identical wardrobe layers, identical accessories, casting-reference precision"
		tags = append(tags,
			identityAnchor,
			"(pure white background:1.6)", "plain white seamless studio backdrop",
			"full body front view", "head to toe fully visible",
			"(single character:1.5)", "(one person only:1.5)",
			"standing in relaxed neutral pose", "neutral A-pose reference stance", "both feet visible",
			"production costume reference", "wardrobe turnaround photo", "garment layering clearly readable",
			"accessories, closures, trims, fabric weight and footwear clearly visible",
			"objective character sheet framing", "clean silhouette separation", "centered subject",
			"exactly one subject, no environmental storytelling", "full outfit silhouette locked for downstream consistency",
		)
		if name != "" {
			tags = append(tags, name)
		}
		if description != "" {
			tags = append(tags, description)
		}
		if promptUsed != "" {
			tags = append(tags, promptUsed)
		}
		tags = append(tags,
			"RAW photo", "(photorealistic:1.4)", "natural skin texture",
			"professional studio lighting", "8K UHD", "ultra-detailed",
			"face, hairline, costume silhouette, and footwear remain fully consistent",
			"no anime", "no illustration",
			"no props", "no extra limbs", "no duplicate person",
			"no cropped head", "no cropped feet", "no side character", "no mannequin",
		)
	} else {
		// Anime/illustration: multi-view character reference sheet.
		// Layout: head-and-shoulder portrait on the left third (butterfly lighting) +
		// full-body front / side / back three-views arranged side-by-side on the right two-thirds.
		identityAnchor := "the same single character in every view, identical face, identical hairstyle, identical costume layers, identical accessories, strict model-sheet consistency"
		tags = append(tags,
			identityAnchor,
			"character reference sheet", "character design sheet", "multi-view character design",
			"(pure white background:1.6)", "plain white seamless backdrop",
			"(single character:1.5)",
			// Left 1/3: head-shoulder portrait
			"head and shoulder portrait on the left third of the image",
			"head and neck fully visible in portrait section",
			"butterfly lighting on face", "soft facial lighting", "ultra-detailed face",
			// Right 2/3: full-body three views
			"full body front view, full body side view, full body back view arranged side by side",
			"three views on the right two thirds of the image",
			// Body proportion
			"9-head body proportion", "slender elegant figure", "elongated legs",
			"consistent character design across all views",
			"costume layers, accessories, hairstyle silhouette, and footwear clearly readable",
			"production-ready turnaround sheet",
			"neutral presentation pose, orthographic design-sheet readability",
		)
		if name != "" {
			tags = append(tags, name)
		}
		if description != "" {
			tags = append(tags, description)
		}
		if promptUsed != "" {
			tags = append(tags, promptUsed)
		}
		tags = append(tags,
			"(masterpiece:1.5)", "best quality", "ultra-detailed illustration",
			"8K", "2D anime style", "clean line art", "detailed fabric texture",
			"same face and same outfit across all views",
			"no extra character", "no duplicate limbs",
			"no cropped panels", "no chibi distortion", "no floating props",
		)
	}

	tags = append(tags,
		"(no text:2.0)", "no watermarks", "no background elements",
	)

	return strings.Join(tags, ", ")
}

// composeScenePrompt builds the prompt for a scene/environment asset.
func composeScenePrompt(name, description, promptUsed string, isLiveAction bool) string {
	// 为了兼容目前强大的自然语言图片模型，我们将去人的约束放在最前面且极其口语化和绝对化。
	var tags []string
	if isLiveAction {
		tags = []string{
			"【纯净无人空镜】(Completely empty environment, absolute ZERO people, no humans, no creatures)",
			"RAW photo", "8K UHD", "ultra-detailed",
			"cinematic film look", "cinematic location photography",
			"production design reference still", "establishing-shot location bible frame",
			"single coherent location, no split scene montage",
		}
	} else {
		tags = []string{
			"【纯净无人空镜】(Completely empty environment, absolute ZERO people, no humans, no creatures)",
			"environment concept art", "background art", "architectural scene illustration",
			"location design sheet", "single coherent environment reference",
		}
	}

	if name != "" {
		tags = append(tags, name)
	}
	if description != "" {
		tags = append(tags, description)
	}
	if promptUsed != "" {
		tags = append(tags, promptUsed)
	}

	if isLiveAction {
		tags = append(tags,
			// 避免模型脑补的物理属性
			"three-point depth composition", "clear foreground, midground, background readability",
			"architectural materials, weathering, practical lighting sources and atmosphere clearly readable",
			"spatial continuity reference for production design",
			"empty scene without any human presence",
			// 强化无人
			"absolutely nobody, vacant place",
			"no silhouettes", "no portrait", "no hands", "no crowd", "no mannequin",
			"cinematic motivated lighting", "detailed shadows",
			"no anime", "no illustration", "no text", "no watermarks",
		)
	} else {
		tags = append(tags,
			"场景概念图，纯空景，画面中绝对禁止出现任何人物或角色",
			"突出空间结构与光线氛围",
			"前景、中景、远景层次清晰，环境逻辑单一明确",
			"建筑结构、陈设关系、材质与光源方向必须可读",
			"(masterpiece:1.5)", "best quality", "clean illustration",
			"no people", "no humans", "absolutely nobody", "vacant place",
			"no silhouette", "no portrait", "no hands", "no crowd",
			"no text", "no watermarks",
		)
	}

	return strings.Join(tags, ", ")
}

// composePropPrompt builds the prompt for a prop/item asset.
func composePropPrompt(name, description, promptUsed string, isLiveAction bool) string {
	if isLiveAction {
		tags := []string{
			"RAW photo", "(photorealistic:1.4)", "product photography", "commercial hero shot",
			"8K UHD", "tack-sharp focus throughout (f/8)", "no depth-of-field blur on subject",
			"professional three-point studio lighting: key + fill + rim/edge light",
			"clean white seamless studio backdrop",
			"production prop reference", "single isolated hero prop", "full prop silhouette completely visible",
		}
		if name != "" {
			tags = append(tags, name)
		}
		if description != "" {
			tags = append(tags, description)
		}
		if promptUsed != "" {
			tags = append(tags, promptUsed)
		}
		tags = append(tags,
			// Object isolation — strict no-person
			"single hero prop isolated",
			"three-quarter front product-reference angle unless the description specifies otherwise",
			"(no human hands:1.8)", "(no people:1.8)", "(no persons:1.8)",
			"(no faces:1.8)", "(no human figures:1.8)", "(no characters:1.8)",
			"no cluttered background", "no secondary objects", "no table clutter", "no holder stand unless explicitly described",
			// Detail emphasis
			"material surface texture macro-sharp", "authentic wear and patina if applicable",
			"visible craftsmanship and construction details",
			"mechanical seams, fasteners, engravings, edge wear, and manufacturing logic clearly readable when present",
			"accurate scale and proportions",
			// Anti-stylization
			"no anime", "no cartoon", "no illustration", "no cel shading",
			// Cleanup
			"(no text:2.0)", "no watermarks", "no labels",
		)
		return strings.Join(tags, ", ")
	}
	// Anime/stylised prop.
	tags := []string{
		// English structural tags
		"prop design sheet", "product illustration", "no people", "no humans",
		"isolated object on clean background", "hero shot",
		"prop turnaround reference", "single isolated object design sheet", "clean silhouette",
	}
	if name != "" {
		tags = append(tags, name)
	}
	if description != "" {
		tags = append(tags, description)
	}
	if promptUsed != "" {
		tags = append(tags, promptUsed)
	}
	tags = append(tags,
		// Chinese context
		"道具概念图，单一主体，绝对不出现任何人物",
		"突出材质、结构、体积和细节",
		"强调正三维轮廓、连接结构、接缝、刻纹、边缘磨损与功能构造可读性",
		// Quality tokens
		"(masterpiece:1.5)", "best quality", "ultra-detailed", "highly detailed illustration",
		"clean illustration", "2D anime art style",
		// Strict no-person + isolation
		"(no people:2.0)", "(no human figures:2.0)", "(no hands:2.0)",
		"(no faces:2.0)", "(no characters:2.0)",
		"clean white background", "single object",
		"front or three-quarter readable presentation, no exploded-view collage unless explicitly described",
		"(no text:2.0)", "no watermarks", "no labels",
	)
	return strings.Join(tags, ", ")
}

// LockByProject —— 锁定项目下所有资产
func (s *AssetService) LockByProject(projectID uint64) error {
	return s.repo.LockByProjectID(projectID)
}

// stripCharacterMentions removes sentences/clauses that mention characters or people
// from scene and prop descriptions, used as a fallback when LLM refinement is unavailable.
// For character assets this is a no-op.
func stripCharacterMentions(assetType, description string) string {
	t := strings.ToLower(strings.TrimSpace(assetType))
	if t != "scene" && t != "场景" && t != "地点" && t != "prop" && t != "道具" && t != "物品" && t != "item" {
		return description
	}

	// Patterns that suggest the fragment is about a person rather than environment/object.
	personKeywords := []string{
		"人物", "角色", "主角", "配角", "老人", "少年", "女子", "男子", "姑娘", "老太",
		"他", "她", "他们", "她们", "身穿", "身着", "穿着", "穿戴", "发髻", "发型",
		"脸庞", "眼神", "表情", "神情", "站在", "坐在", "手持", "拿着", "怀抱",
		"person", "character", "man ", "woman ", "girl ", "boy ", "he ", "she ",
		"wearing", "dressed", "holding", "standing", "sitting", "figure",
	}

	// Split by Chinese sentence delimiters, semicolons, and commas for clause-level filtering.
	// Using comma-level splitting allows partial removal (e.g. "老妇人递来药包，药包系着红绳" →
	// discard first clause, keep "药包系着红绳").
	sentences := splitBySeparators(description, []string{"。", "；", "，", ". ", "; ", ", "})
	if len(sentences) <= 1 {
		// Single indivisible chunk — if it contains person keywords, suppress it entirely
		// so character content never reaches the scene/prop image model.
		lower := strings.ToLower(strings.TrimSpace(description))
		for _, kw := range personKeywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				return "" // suppress; compose function has name + template for fallback
			}
		}
		return description
	}

	var kept []string
	for _, sent := range sentences {
		lower := strings.ToLower(sent)
		hasChar := false
		for _, kw := range personKeywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				hasChar = true
				break
			}
		}
		if !hasChar {
			kept = append(kept, sent)
		}
	}
	if len(kept) == 0 {
		return description // don't discard everything when nothing can be saved
	}
	return strings.Join(kept, "，")
}

// splitBySeparators splits text by any of the given separators, preserving non-empty parts.
func splitBySeparators(text string, seps []string) []string {
	// Replace all separators with a canonical marker.
	const marker = "\x00"
	result := text
	for _, sep := range seps {
		result = strings.ReplaceAll(result, sep, marker)
	}
	var parts []string
	for _, p := range strings.Split(result, marker) {
		if t := strings.TrimSpace(p); t != "" {
			parts = append(parts, t)
		}
	}
	return parts
}

// UnlockByIDs —— 解锁指定 ID 列表的资产
func (s *AssetService) UnlockByIDs(ids []uint64) error {
	return s.repo.UnlockByIDs(ids)
}

// CountByProject —— 统计项目下资产总数和已完成数量
func (s *AssetService) CountByProject(projectID uint64) (total int64, completed int64, err error) {
	return s.repo.CountByProject(projectID)
}

// UpdateStatus —— 更新资产状态和可选的错误信息
func (s *AssetService) UpdateStatus(id uint64, status string, errMsgs ...string) error {
	errMsg := ""
	if len(errMsgs) > 0 {
		errMsg = errMsgs[0]
	}
	asset, err := s.GetByID(id)
	if err != nil {
		return err
	}
	metadata, err := parseAssetMetadata(asset.Metadata)
	if err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}
	nextStatus := status
	nextErrorMsg := errMsg
	if status == "failed" {
		successImageURL := resolveAssetSuccessfulImageURL(asset, metadata)
		if successImageURL != "" {
			nextStatus = "completed"
			nextErrorMsg = ""
			if strings.TrimSpace(asset.ImageURL) == "" {
				asset.ImageURL = successImageURL
			}
			if strings.TrimSpace(stringValue(metadata["selected_generated_image_url"])) == "" {
				metadata["selected_generated_image_url"] = successImageURL
			}
			delete(metadata, "generation_progress")
		} else {
			progress := buildAssetGenerationProgress("failed", 100, "图片生成失败", errMsg, 0, "")
			if raw, ok := metadata["generation_progress"].(map[string]interface{}); ok {
				if startedAt := strings.TrimSpace(stringValue(raw["started_at"])); startedAt != "" {
					progress.StartedAt = startedAt
				}
			}
			metadata["generation_progress"] = progress
		}
	} else {
		delete(metadata, "generation_progress")
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	asset.Status = nextStatus
	asset.ErrorMsg = nextErrorMsg
	asset.Metadata = datatypes.JSON(metadataJSON)
	asset.UpdatedAt = time.Now()
	return s.repo.Update(asset)
}

// UpdateGenerated —— 更新资产的图像生成结果（图片URL和提示词），标记为 completed
func (s *AssetService) UpdateGenerated(id uint64, imageURL, prompt, modelName string) error {
	asset, err := s.GetByID(id)
	if err != nil {
		return err
	}

	metadata, err := parseAssetMetadata(asset.Metadata)
	if err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}
	versions := appendGeneratedImageVersion(metadata, asset.ImageURL, imageURL, prompt, modelName, asset.UpdatedAt)
	metadata["generated_images"] = versions
	metadata["selected_generated_image_url"] = imageURL
	delete(metadata, "generation_progress")
	delete(metadata, "panel_images")
	delete(metadata, "composite_image_url")
	delete(metadata, "seed")

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	history, err := normalizeAssetChatHistory(asset.AgentHistory)
	if err != nil {
		return fmt.Errorf("normalize agent history: %w", err)
	}
	history = append(history, map[string]interface{}{
		"role":      "assistant",
		"content":   "已生成新的候选图片，可在左侧候选列表中选择想保留的版本。",
		"timestamp": time.Now().Format(time.RFC3339),
		"image_url": imageURL,
	})
	historyJSON, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("marshal history: %w", err)
	}

	asset.ImageURL = imageURL
	asset.PromptUsed = prompt
	asset.Status = "completed"
	asset.ErrorMsg = ""
	asset.Metadata = datatypes.JSON(metadataJSON)
	asset.AgentHistory = datatypes.JSON(historyJSON)
	asset.PanelImages = nil
	asset.CompositeImageURL = ""
	asset.Seed = -1
	asset.UpdatedAt = time.Now()
	return s.repo.Update(asset)
}

// UpdateGeneratedPanels —— 在一次事务内写入四视图分栏图 URL + 拼接图 URL + 使用的 seed。
// 语义与 UpdateGenerated 等价（标记 completed、追加生成历史），但针对角色资产的
// "分 4 次生成 + 后端拼接" 流程，记录更完整的生成上下文。
// panelURLs 顺序固定为 [closeup, front, side, back]；缺位可传空字符串（此时状态置 partial）。
// compositeURL 为最终横向拼接后的图片 URL；ImageURL 指向正面视图（front），保持主资源缩略图为单张。
func (s *AssetService) UpdateGeneratedPanels(id uint64, panelURLs []string, compositeURL, prompt, modelName string, seed int64) error {
	asset, err := s.GetByID(id)
	if err != nil {
		return err
	}

	// 主资源图：优先正面视图（新顺序中 index 0 = front），fallback 到第一张非空分栏图。
	primaryImageURL := ""
	if len(panelURLs) > 0 && strings.TrimSpace(panelURLs[0]) != "" {
		primaryImageURL = panelURLs[0]
	} else {
		for _, u := range panelURLs {
			if strings.TrimSpace(u) != "" {
				primaryImageURL = u
				break
			}
		}
	}
	if primaryImageURL == "" {
		primaryImageURL = compositeURL
	}

	metadata, err := parseAssetMetadata(asset.Metadata)
	if err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}
	// 把正面视图视为"新生成"版本，复用已有历史结构。
	versions := appendGeneratedImageVersion(metadata, asset.ImageURL, primaryImageURL, prompt, modelName, asset.UpdatedAt)
	metadata["generated_images"] = versions
	metadata["selected_generated_image_url"] = primaryImageURL
	metadata["panel_images"] = panelURLs
	metadata["composite_image_url"] = compositeURL
	metadata["seed"] = seed
	delete(metadata, "generation_progress")

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	history, err := normalizeAssetChatHistory(asset.AgentHistory)
	if err != nil {
		return fmt.Errorf("normalize agent history: %w", err)
	}
	history = append(history, map[string]interface{}{
		"role":      "assistant",
		"content":   "已按四视图流程生成（特写/正面/侧面/背面），可在资源卡中展开查看或单栏重绘。",
		"timestamp": time.Now().Format(time.RFC3339),
		"image_url": compositeURL,
	})
	historyJSON, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("marshal history: %w", err)
	}

	// partial 判定：任一 panelURL 为空字符串 → partial；否则 completed。
	status := "completed"
	for _, u := range panelURLs {
		if strings.TrimSpace(u) == "" {
			status = "partial"
			break
		}
	}
	if compositeURL == "" {
		status = "partial"
	}

	asset.ImageURL = primaryImageURL
	asset.PromptUsed = prompt
	asset.Status = status
	asset.ErrorMsg = ""
	asset.Metadata = datatypes.JSON(metadataJSON)
	asset.AgentHistory = datatypes.JSON(historyJSON)
	// 持久化独立字段（供前端/单栏重绘直接读取）。
	asset.PanelImages = pq.StringArray(panelURLs)
	asset.CompositeImageURL = compositeURL
	asset.Seed = seed
	asset.UpdatedAt = time.Now()
	return s.repo.Update(asset)
}

// UpdateGenerationProgress —— 更新资产的生成进度元数据。
func (s *AssetService) UpdateGenerationProgress(id uint64, progress assetGenerationProgress) error {
	asset, err := s.GetByID(id)
	if err != nil {
		return err
	}
	metadata, err := parseAssetMetadata(asset.Metadata)
	if err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}
	if raw, ok := metadata["generation_progress"].(map[string]interface{}); ok {
		if progress.StartedAt == "" {
			progress.StartedAt = strings.TrimSpace(stringValue(raw["started_at"]))
		}
	}
	if progress.StartedAt == "" {
		progress.StartedAt = time.Now().Format(time.RFC3339)
	}
	metadata["generation_progress"] = progress
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	asset.Metadata = datatypes.JSON(metadataJSON)
	asset.UpdatedAt = time.Now()
	return s.repo.Update(asset)
}

func buildAssetGenerationProgress(stage string, percent int, label, detail string, taskID int64, modelName string) assetGenerationProgress {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return assetGenerationProgress{
		Stage:     stage,
		Label:     strings.TrimSpace(label),
		Detail:    strings.TrimSpace(detail),
		Percent:   percent,
		TaskID:    taskID,
		ModelName: strings.TrimSpace(modelName),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

func parseAssetMetadata(raw datatypes.JSON) (map[string]interface{}, error) {
	if len(raw) == 0 {
		return map[string]interface{}{}, nil
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, err
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	return metadata, nil
}

func appendGeneratedImageVersion(metadata map[string]interface{}, previousImageURL, latestImageURL, prompt, modelName string, previousUpdatedAt time.Time) []assetImageVersion {
	versions := extractGeneratedImageVersions(metadata)
	now := time.Now().Format(time.RFC3339)

	if strings.TrimSpace(previousImageURL) != "" && previousImageURL != latestImageURL {
		versions = append([]assetImageVersion{{
			URL:       previousImageURL,
			Prompt:    strings.TrimSpace(prompt),
			CreatedAt: previousUpdatedAt.Format(time.RFC3339),
			Source:    "previous",
		}}, versions...)
	}

	versions = append([]assetImageVersion{{
		URL:       latestImageURL,
		Prompt:    strings.TrimSpace(prompt),
		CreatedAt: now,
		Source:    "generated",
		ModelName: strings.TrimSpace(modelName),
	}}, versions...)

	deduped := make([]assetImageVersion, 0, len(versions))
	seen := make(map[string]struct{}, len(versions))
	for _, version := range versions {
		url := strings.TrimSpace(version.URL)
		if url == "" {
			continue
		}
		if _, exists := seen[url]; exists {
			continue
		}
		seen[url] = struct{}{}
		version.URL = url
		deduped = append(deduped, version)
		if len(deduped) >= 12 {
			break
		}
	}
	return deduped
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func numberValue(raw interface{}) float64 {
	switch value := raw.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case int32:
		return float64(value)
	case uint64:
		return float64(value)
	case uint32:
		return float64(value)
	case json.Number:
		parsed, _ := value.Float64()
		return parsed
	default:
		return 0
	}
}

func extractGeneratedImageVersions(metadata map[string]interface{}) []assetImageVersion {
	rawVersions, ok := metadata["generated_images"]
	if !ok {
		return nil
	}

	list, ok := rawVersions.([]interface{})
	if !ok {
		return nil
	}

	versions := make([]assetImageVersion, 0, len(list))
	for _, item := range list {
		data, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		url := strings.TrimSpace(stringValue(data["url"]))
		if url == "" {
			continue
		}
		versions = append(versions, assetImageVersion{
			URL:       url,
			Prompt:    strings.TrimSpace(stringValue(data["prompt"])),
			CreatedAt: strings.TrimSpace(stringValue(data["created_at"])),
			Source:    strings.TrimSpace(stringValue(data["source"])),
			ModelName: strings.TrimSpace(stringValue(data["model_name"])),
		})
	}
	return versions
}

func resolveAssetSuccessfulImageURL(asset *model.Asset, metadata map[string]interface{}) string {
	preferred := strings.TrimSpace(stringValue(metadata["selected_generated_image_url"]))
	versions := extractGeneratedImageVersions(metadata)
	if preferred != "" {
		for _, version := range versions {
			if strings.TrimSpace(version.URL) == preferred {
				return preferred
			}
		}
	}
	primaryImage := strings.TrimSpace(asset.ImageURL)
	if primaryImage != "" {
		return primaryImage
	}
	if preferred != "" {
		return preferred
	}
	if len(versions) > 0 {
		return strings.TrimSpace(versions[0].URL)
	}
	return ""
}

// StartStaleCleanup —— 定期检查并重置超时的 generating 状态资产
// StartStaleCleanup runs a periodic check that resets assets stuck in "generating"
// for longer than the given threshold. Call from main and pass the cancel context.
func (s *AssetService) StartStaleCleanup(ctx context.Context, interval, staleThreshold time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.repo.ResetStaleGenerating(staleThreshold)
			if err != nil {
				s.log.Error("stale generating cleanup failed", zap.Error(err))
			} else if n > 0 {
				s.log.Warn("reset stale generating assets", zap.Int64("count", n), zap.Duration("threshold", staleThreshold))
			}
		}
	}
}

var ErrAssetLocked = errors.New("asset is locked")

// ─── Vision QA ────────────────────────────────────────────────────────────────

// VisionValidatePanels calls the configured vision LLM to semantically verify all 4 character
// panels against their expected framing criteria. Non-fatal: returns nil report + error on failure.
func (s *AssetService) VisionValidatePanels(ctx context.Context, panelURLs []string, charName, charDesc string) (*PanelVisionReport, error) {
	if s.llmBaseURL == "" || s.llmAPIKey == "" {
		return nil, fmt.Errorf("vision validation: LLM not configured")
	}

	// Build content parts: text instruction + up to 4 image references.
	panels := []string{"closeup", "front", "side", "back"}
	contentParts := []map[string]interface{}{
		{
			"type": "text",
			"text": fmt.Sprintf(`You are a quality-control reviewer for AI-generated character asset images used in animation production.

Character name: %s
Character description: %s

The following images are 4 character panels generated separately, in this exact order:
  Panel 1 – CLOSEUP: head-and-shoulders portrait (face must be clearly visible, fills most of the frame; NOT a full-body shot)
  Panel 2 – FRONT: full-body front view from head to toe (character faces camera, both feet visible)
  Panel 3 – SIDE: full-body side profile at exactly 90° (not 3/4 view; facing left; no eye contact; head to toe)
  Panel 4 – BACK: full-body back view (character faces away; no front face visible; hairstyle and back of outfit clearly visible; head to toe)

For each panel, evaluate strictly whether it meets its panel-specific criteria.
Also note any consistency issues across panels (e.g., costume colour or hairstyle differs).

Return strictly valid JSON in this schema (no markdown, no prose outside the JSON):
{
  "panels": [
    { "panel": "closeup", "pass": true, "score": 8, "issues": [] },
    { "panel": "front",   "pass": true, "score": 7, "issues": ["feet slightly cropped"] },
    { "panel": "side",    "pass": false,"score": 4, "issues": ["appears to be 3/4 view, not strict 90° profile"] },
    { "panel": "back",    "pass": true, "score": 9, "issues": [] }
  ],
  "overall_pass": false,
  "summary": "One sentence overall assessment."
}`, charName, charDesc),
		},
	}
	for i, url := range panelURLs {
		if i >= 4 || strings.TrimSpace(url) == "" {
			break
		}
		contentParts = append(contentParts, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]string{
				"url": url,
			},
		})
		_ = panels[i] // suppress unused warning
	}

	messages := []map[string]interface{}{
		{
			"role":    "user",
			"content": contentParts,
		},
	}

	reqBody := map[string]interface{}{
		"model":           s.llmVisionModel,
		"messages":        messages,
		"temperature":     0.1,
		"max_tokens":      800,
		"response_format": map[string]string{"type": "json_object"},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal vision request: %w", err)
	}

	visionTimeout := 45 * time.Second
	if s.llmTimeout > 0 && s.llmTimeout < visionTimeout {
		visionTimeout = s.llmTimeout
	}
	vCtx, cancel := context.WithTimeout(ctx, visionTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(vCtx, http.MethodPost, s.llmBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	resp, err := (&http.Client{Timeout: visionTimeout + 5*time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("vision llm call: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vision llm error %d: %.200s", resp.StatusCode, string(respBody))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return nil, fmt.Errorf("parse vision response: %w", err)
	}
	if len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("vision llm returned no choices")
	}

	raw := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	// Strip ```json ``` fences if present
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		if len(lines) > 2 {
			raw = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var report PanelVisionReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		return nil, fmt.Errorf("parse vision json: %w (raw: %.200s)", err, raw)
	}
	report.Model = s.llmVisionModel
	report.CheckedAt = time.Now().Format(time.RFC3339)
	return &report, nil
}

// UpdatePanelValidation stores the AI vision QA report in the asset's metadata.
// Called after VisionValidatePanels succeeds; safe to call even if the asset is already "completed".
func (s *AssetService) UpdatePanelValidation(id uint64, report *PanelVisionReport) error {
	if report == nil {
		return nil
	}
	asset, err := s.GetByID(id)
	if err != nil {
		return err
	}
	metadata, err := parseAssetMetadata(asset.Metadata)
	if err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}
	metadata["panel_validation"] = report
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	asset.Metadata = datatypes.JSON(metadataJSON)
	asset.UpdatedAt = time.Now()
	return s.repo.Update(asset)
}
