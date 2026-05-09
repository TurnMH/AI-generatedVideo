package repository

import (
	"fmt"

	"github.com/autovideo/auth-service/internal/model"
	"gorm.io/gorm"
)

type SystemAPIKeyRepository interface {
	FindAll() ([]model.SystemAPIKey, error)
	FindRuntimeKeys() ([]model.SystemAPIKey, error)
	FindByProvider(provider string) ([]model.SystemAPIKey, error)
	FindByID(id uint64) (*model.SystemAPIKey, error)
	Create(key *model.SystemAPIKey) error
	ReplaceRuntimeKeys(keys []model.SystemAPIKey) error
	Delete(id uint64) error
}

type systemAPIKeyRepository struct {
	db *gorm.DB
}

// NewSystemAPIKeyRepository —— 创建 SystemAPIKeyRepository 实例，注入数据库连接
func NewSystemAPIKeyRepository(db *gorm.DB) SystemAPIKeyRepository {
	return &systemAPIKeyRepository{db: db}
}

// FindAll —— 查询所有有效的系统级 API Key，按 provider 和 id 排序返回
func (r *systemAPIKeyRepository) FindAll() ([]model.SystemAPIKey, error) {
	var keys []model.SystemAPIKey
	if err := r.db.Where("is_active = true").Order("provider, id").Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("find all system api keys: %w", err)
	}
	return keys, nil
}

// FindRuntimeKeys —— 查询所有 runtime.* 前缀的系统级 API Key，按 provider 和 id 排序返回
func (r *systemAPIKeyRepository) FindRuntimeKeys() ([]model.SystemAPIKey, error) {
	var keys []model.SystemAPIKey
	if err := r.db.Where("is_active = true AND provider LIKE ?", "runtime.%").Order("provider, id").Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("find runtime system api keys: %w", err)
	}
	return keys, nil
}

// FindByProvider —— 根据提供商名称查询对应的系统级 API Key 列表
func (r *systemAPIKeyRepository) FindByProvider(provider string) ([]model.SystemAPIKey, error) {
	var keys []model.SystemAPIKey
	if err := r.db.Where("provider = ? AND is_active = true", provider).Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("find system api keys by provider: %w", err)
	}
	return keys, nil
}

// FindByID —— 根据主键 ID 查询单条系统级 API Key 记录
func (r *systemAPIKeyRepository) FindByID(id uint64) (*model.SystemAPIKey, error) {
	var key model.SystemAPIKey
	if err := r.db.First(&key, id).Error; err != nil {
		return nil, fmt.Errorf("find system api key by id: %w", err)
	}
	return &key, nil
}

// Create —— 将系统级 API Key 记录插入数据库
func (r *systemAPIKeyRepository) Create(key *model.SystemAPIKey) error {
	if err := r.db.Create(key).Error; err != nil {
		return fmt.Errorf("create system api key: %w", err)
	}
	return nil
}

// ReplaceRuntimeKeys —— 用最新的 runtime.* 配置覆盖数据库中的 runtime 系统渠道镜像
func (r *systemAPIKeyRepository) ReplaceRuntimeKeys(keys []model.SystemAPIKey) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("provider LIKE ?", "runtime.%").Delete(&model.SystemAPIKey{}).Error; err != nil {
			return fmt.Errorf("delete runtime system api keys: %w", err)
		}
		if len(keys) == 0 {
			return nil
		}
		for i := range keys {
			keys[i].ID = 0
		}
		if err := tx.Create(&keys).Error; err != nil {
			return fmt.Errorf("create runtime system api keys: %w", err)
		}
		return nil
	})
}

// Delete —— 根据 ID 删除系统级 API Key 记录
func (r *systemAPIKeyRepository) Delete(id uint64) error {
	result := r.db.Delete(&model.SystemAPIKey{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete system api key: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("system api key not found")
	}
	return nil
}
