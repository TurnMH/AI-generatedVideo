package service

import (
	"fmt"

	"github.com/autovideo/auth-service/internal/model"
	"github.com/autovideo/auth-service/internal/repository"
	"github.com/autovideo/auth-service/pkg/config"
	"github.com/autovideo/auth-service/pkg/crypto"
)

type AddAPIKeyReq struct {
	Provider   string
	Alias      string
	PlainKey   string
	BaseURL    string
	ModelScope string
}

type APIKeyService interface {
	AddAPIKey(userID uint64, provider, alias, plainKey, baseURL, modelScope string) error
	ListAPIKeys(userID uint64) ([]model.UserAPIKey, error)
	ListSystemAPIKeys() ([]model.SystemAPIKey, error)
	ListRuntimeAPIKeys() ([]model.SystemAPIKey, error)
	SyncRuntimeAPIKeys(configFile string) error
	DeleteAPIKey(id, userID uint64) error
}

type apiKeyService struct {
	repo    repository.APIKeyRepository
	sysRepo repository.SystemAPIKeyRepository
	cfg     *config.Config
}

// NewAPIKeyService —— 创建 APIKeyService 实例，注入用户/系统 API Key 仓储和配置
func NewAPIKeyService(repo repository.APIKeyRepository, sysRepo repository.SystemAPIKeyRepository, cfg *config.Config) APIKeyService {
	return &apiKeyService{repo: repo, sysRepo: sysRepo, cfg: cfg}
}

// AddAPIKey 加密 plainKey 后存库
func (s *apiKeyService) AddAPIKey(userID uint64, provider, alias, plainKey, baseURL, modelScope string) error {
	encrypted, err := crypto.Encrypt(plainKey, s.cfg.Crypto.EncryptionKey)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	key := &model.UserAPIKey{
		UserID:       userID,
		Provider:     provider,
		KeyAlias:     alias,
		EncryptedKey: encrypted,
		BaseURL:      baseURL,
		ModelScope:   modelScope,
		IsActive:     true,
	}

	if err := s.repo.Create(key); err != nil {
		return fmt.Errorf("save api key: %w", err)
	}
	return nil
}

// ListAPIKeys 列出用户所有有效 API Key（不返回密文）
func (s *apiKeyService) ListAPIKeys(userID uint64) ([]model.UserAPIKey, error) {
	keys, err := s.repo.FindByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	return keys, nil
}

// ListSystemAPIKeys 列出系统级 API Key（不返回密文）
func (s *apiKeyService) ListSystemAPIKeys() ([]model.SystemAPIKey, error) {
	keys, err := s.sysRepo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("list system api keys: %w", err)
	}
	return keys, nil
}

// ListRuntimeAPIKeys 列出 runtime.* 前缀的系统级 API Key（供服务间调用，返回明文）
func (s *apiKeyService) ListRuntimeAPIKeys() ([]model.SystemAPIKey, error) {
	keys, err := s.sysRepo.FindRuntimeKeys()
	if err != nil {
		return nil, fmt.Errorf("list runtime api keys: %w", err)
	}
	return keys, nil
}

// SyncRuntimeAPIKeys 从统一配置文件提取运行时渠道，并覆盖数据库中的 runtime.* 镜像。
func (s *apiKeyService) SyncRuntimeAPIKeys(configFile string) error {
	runtimeKeys, err := buildRuntimeSystemKeys(configFile)
	if err != nil {
		return fmt.Errorf("build runtime api keys: %w", err)
	}
	if err := s.sysRepo.ReplaceRuntimeKeys(runtimeKeys); err != nil {
		return fmt.Errorf("replace runtime api keys: %w", err)
	}
	return nil
}

// DeleteAPIKey 删除指定 API Key，校验归属
func (s *apiKeyService) DeleteAPIKey(id, userID uint64) error {
	if err := s.repo.Delete(id, userID); err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	return nil
}
