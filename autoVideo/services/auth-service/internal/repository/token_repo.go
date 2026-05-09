package repository

import (
	"fmt"

	"github.com/autovideo/auth-service/internal/model"
	"gorm.io/gorm"
)

type TokenRepository interface {
	Create(token *model.RefreshToken) error
	FindByHash(hash string) (*model.RefreshToken, error)
	Delete(id uint64) error
	DeleteByUserID(userID uint64) error
}

type tokenRepository struct {
	db *gorm.DB
}

// NewTokenRepository —— 创建 TokenRepository 实例，注入数据库连接
func NewTokenRepository(db *gorm.DB) TokenRepository {
	return &tokenRepository{db: db}
}

// Create —— 将刷新令牌记录插入数据库
func (r *tokenRepository) Create(token *model.RefreshToken) error {
	if err := r.db.Create(token).Error; err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

// FindByHash —— 根据令牌哈希值查询刷新令牌记录
func (r *tokenRepository) FindByHash(hash string) (*model.RefreshToken, error) {
	var token model.RefreshToken
	if err := r.db.Where("token_hash = ?", hash).First(&token).Error; err != nil {
		return nil, fmt.Errorf("find token by hash: %w", err)
	}
	return &token, nil
}

// Delete —— 根据 ID 删除单条刷新令牌记录
func (r *tokenRepository) Delete(id uint64) error {
	if err := r.db.Delete(&model.RefreshToken{}, id).Error; err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}
	return nil
}

// DeleteByUserID —— 根据用户 ID 删除该用户的所有刷新令牌
func (r *tokenRepository) DeleteByUserID(userID uint64) error {
	if err := r.db.Where("user_id = ?", userID).Delete(&model.RefreshToken{}).Error; err != nil {
		return fmt.Errorf("delete refresh tokens by user_id: %w", err)
	}
	return nil
}
