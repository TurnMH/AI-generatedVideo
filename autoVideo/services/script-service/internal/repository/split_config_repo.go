package repository

import (
	"context"

	"github.com/autovideo/script-service/internal/model"
	"gorm.io/gorm"
)

// SplitConfigRepository handles database operations for SplitConfig records.
type SplitConfigRepository interface {
	Create(ctx context.Context, config *model.SplitConfig) error
	GetByScriptID(ctx context.Context, scriptID int64) (*model.SplitConfig, error)
	Update(ctx context.Context, config *model.SplitConfig) error
	DeleteByScriptID(ctx context.Context, scriptID int64) error
}

type splitConfigRepository struct {
	db *gorm.DB
}

// NewSplitConfigRepository —— 创建拆分配置仓储实例，返回 SplitConfigRepository 接口
func NewSplitConfigRepository(db *gorm.DB) SplitConfigRepository {
	return &splitConfigRepository{db: db}
}

// Create —— 将拆分配置记录插入数据库
func (r *splitConfigRepository) Create(ctx context.Context, config *model.SplitConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

// GetByScriptID —— 根据剧本 ID 查询拆分配置
func (r *splitConfigRepository) GetByScriptID(ctx context.Context, scriptID int64) (*model.SplitConfig, error) {
	var config model.SplitConfig
	if err := r.db.WithContext(ctx).Where("script_id = ?", scriptID).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// Update —— 保存更新后的拆分配置到数据库
func (r *splitConfigRepository) Update(ctx context.Context, config *model.SplitConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

// DeleteByScriptID —— 根据剧本 ID 删除拆分配置
func (r *splitConfigRepository) DeleteByScriptID(ctx context.Context, scriptID int64) error {
	return r.db.WithContext(ctx).Where("script_id = ?", scriptID).Delete(&model.SplitConfig{}).Error
}
