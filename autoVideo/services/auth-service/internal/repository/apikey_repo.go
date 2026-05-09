package repository

import (
	"fmt"

	"github.com/autovideo/auth-service/internal/model"
	"gorm.io/gorm"
)

type APIKeyRepository interface {
	Create(key *model.UserAPIKey) error
	FindByUserID(userID uint64) ([]model.UserAPIKey, error)
	FindByID(id uint64) (*model.UserAPIKey, error)
	Delete(id, userID uint64) error
}

type apiKeyRepository struct {
	db *gorm.DB
}

// NewAPIKeyRepository —— 创建 APIKeyRepository 实例，注入数据库连接
func NewAPIKeyRepository(db *gorm.DB) APIKeyRepository {
	return &apiKeyRepository{db: db}
}

// Create —— 将用户 API Key 记录插入数据库
func (r *apiKeyRepository) Create(key *model.UserAPIKey) error {
	if err := r.db.Create(key).Error; err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	return nil
}

// FindByUserID —— 根据用户 ID 查询所有有效的 API Key 列表
func (r *apiKeyRepository) FindByUserID(userID uint64) ([]model.UserAPIKey, error) {
	var keys []model.UserAPIKey
	if err := r.db.Where("user_id = ? AND is_active = true", userID).Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("find api keys by user_id: %w", err)
	}
	return keys, nil
}

// FindByID —— 根据主键 ID 查询单条 API Key 记录
func (r *apiKeyRepository) FindByID(id uint64) (*model.UserAPIKey, error) {
	var key model.UserAPIKey
	if err := r.db.First(&key, id).Error; err != nil {
		return nil, fmt.Errorf("find api key by id: %w", err)
	}
	return &key, nil
}

// Delete —— 根据 ID 和用户 ID 删除 API Key，校验归属后删除
func (r *apiKeyRepository) Delete(id, userID uint64) error {
	result := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.UserAPIKey{})
	if result.Error != nil {
		return fmt.Errorf("delete api key: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("api key not found or not owned by user")
	}
	return nil
}
