package service

import (
	"errors"
	"io"

	"github.com/autovideo/character-service/internal/model"
	"github.com/autovideo/character-service/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CharacterService struct {
	repo      *repository.CharacterRepo
	styleR    *repository.StyleRepo
	storage   *StorageClient
	assetRepo *repository.AssetRepo
	log       *zap.Logger
}

// NewCharacterService —— 创建角色服务实例，返回 *CharacterService
func NewCharacterService(
	repo *repository.CharacterRepo,
	styleR *repository.StyleRepo,
	storage *StorageClient,
	assetRepo *repository.AssetRepo,
	log *zap.Logger,
) *CharacterService {
	return &CharacterService{repo: repo, styleR: styleR, storage: storage, assetRepo: assetRepo, log: log}
}

// ListByProject —— 按项目 ID 查询该项目下的所有角色
func (s *CharacterService) ListByProject(projectID int64) ([]*model.Character, error) {
	return s.repo.ListByProject(projectID)
}

// Create —— 创建新角色，默认风格为 anime
func (s *CharacterService) Create(c *model.Character) error {
	// default style preset
	if c.StylePreset == "" {
		c.StylePreset = "anime"
	}
	return s.repo.Create(c)
}

// GetByID —— 根据 ID 获取角色，不存在则返回 ErrNotFound
func (s *CharacterService) GetByID(id int64) (*model.Character, error) {
	c, err := s.repo.GetByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return c, err
}

// Update —— 更新指定角色信息，返回更新后的角色
func (s *CharacterService) Update(id int64, updates *model.Character) (*model.Character, error) {
	c, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	updates.ID = c.ID
	updates.ProjectID = c.ProjectID
	updates.CreatedAt = c.CreatedAt
	if err = s.repo.Update(updates); err != nil {
		return nil, err
	}
	return updates, nil
}

// Delete —— 删除指定角色及其关联资产，不存在则返回 ErrNotFound
func (s *CharacterService) Delete(id int64) error {
	if _, err := s.GetByID(id); err != nil {
		return err
	}
	// 级联删除角色关联的资产
	if err := s.assetRepo.DeleteByCharacterID(id); err != nil {
		s.log.Error("delete character assets failed", zap.Int64("character_id", id), zap.Error(err))
		return err
	}
	return s.repo.Delete(id)
}

// UploadReference 上传参考图并更新 reference_image_url
func (s *CharacterService) UploadReference(id int64, filename string, content io.Reader) (string, error) {
	if _, err := s.GetByID(id); err != nil {
		return "", err
	}
	cdnURL, err := s.storage.Upload(filename, content)
	if err != nil {
		s.log.Error("upload reference image failed", zap.Int64("id", id), zap.Error(err))
		return "", err
	}
	if err = s.repo.UpdateReferenceImage(id, cdnURL); err != nil {
		return "", err
	}
	return cdnURL, nil
}

// SetStyle 设置角色风格
func (s *CharacterService) SetStyle(id int64, preset, styleRefURL string) (*model.Character, error) {
	if _, err := s.GetByID(id); err != nil {
		return nil, err
	}
	// validate preset exists
	if _, err := s.styleR.GetByName(preset); err != nil {
		return nil, ErrInvalidStylePreset
	}
	if err := s.repo.UpdateStyle(id, preset, styleRefURL); err != nil {
		return nil, err
	}
	return s.repo.GetByID(id)
}

// GetCharacterConfig 给 gRPC（内部接口）用，返回含 style prompt 的完整配置
func (s *CharacterService) GetCharacterConfig(id int64) (*CharacterConfig, error) {
	c, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	style, _ := s.styleR.GetByName(c.StylePreset)
	cfg := &CharacterConfig{
		ID:                id,
		Name:              c.Name,
		AppearanceDesc:    c.AppearanceDesc,
		VoiceModel:        c.VoiceModel,
		StylePreset:       c.StylePreset,
		StyleReferenceURL: c.StyleReferenceURL,
		LoraModelID:       c.LoraModelID,
		IPAdapterConfig:   map[string]interface{}(c.IPAdapterConfig),
		FixedSeed:         c.FixedSeed,
	}
	if style != nil {
		cfg.PromptSuffix = style.PromptSuffix
		cfg.NegativePrompt = style.NegativePrompt
	}
	return cfg, nil
}

// GetStylePreset 根据名称返回风格配置（gRPC 用）
func (s *CharacterService) ListStylePresets() ([]*model.StylePreset, error) {
	return s.styleR.List()
}

// GetStylePreset —— 根据名称获取风格预设，不存在返回 ErrNotFound
func (s *CharacterService) GetStylePreset(name string) (*model.StylePreset, error) {
	sp, err := s.styleR.GetByName(name)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return sp, err
}

// CharacterConfig 是内部服务间交换的完整角色配置
type CharacterConfig struct {
	ID                int64                  `json:"id"`
	Name              string                 `json:"name"`
	AppearanceDesc    string                 `json:"appearance_desc"`
	VoiceModel        string                 `json:"voice_model"`
	StylePreset       string                 `json:"style_preset"`
	StyleReferenceURL string                 `json:"style_reference_url"`
	PromptSuffix      string                 `json:"prompt_suffix"`
	NegativePrompt    string                 `json:"negative_prompt"`
	LoraModelID       string                 `json:"lora_model_id"`
	IPAdapterConfig   map[string]interface{} `json:"ip_adapter_config"`
	FixedSeed         int64                  `json:"fixed_seed"`
}

var (
	ErrNotFound           = errors.New("record not found")
	ErrInvalidStylePreset = errors.New("invalid style preset")
)
