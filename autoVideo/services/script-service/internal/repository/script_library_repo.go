package repository

import (
	"context"

	"github.com/autovideo/script-service/internal/model"
	"gorm.io/gorm"
)

type ScriptLibraryRepository interface {
	Create(ctx context.Context, item *model.ScriptLibrary) error
	FindByID(ctx context.Context, id int64) (*model.ScriptLibrary, error)
	Update(ctx context.Context, item *model.ScriptLibrary) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, source string, userID int64) ([]model.ScriptLibrary, error)
}

type scriptLibraryRepo struct {
	db *gorm.DB
}

// NewScriptLibraryRepository —— 创建剧本库仓储实例，返回 ScriptLibraryRepository 接口
func NewScriptLibraryRepository(db *gorm.DB) ScriptLibraryRepository {
	return &scriptLibraryRepo{db: db}
}

// Create —— 将剧本库条目插入数据库
func (r *scriptLibraryRepo) Create(ctx context.Context, item *model.ScriptLibrary) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// FindByID —— 根据 ID 查询剧本库条目
func (r *scriptLibraryRepo) FindByID(ctx context.Context, id int64) (*model.ScriptLibrary, error) {
	var item model.ScriptLibrary
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

// Update —— 保存更新后的剧本库条目到数据库
func (r *scriptLibraryRepo) Update(ctx context.Context, item *model.ScriptLibrary) error {
	return r.db.WithContext(ctx).Save(item).Error
}

// Delete —— 根据 ID 删除剧本库条目
func (r *scriptLibraryRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ScriptLibrary{}, id).Error
}

// List —— 查询剧本库列表，支持按来源筛选，返回公开和用户自有的条目
func (r *scriptLibraryRepo) List(ctx context.Context, source string, userID int64) ([]model.ScriptLibrary, error) {
	var items []model.ScriptLibrary
	q := r.db.WithContext(ctx)
	if source != "" {
		q = q.Where("source = ?", source)
	}
	// Show public items + user's own items
	if userID > 0 {
		q = q.Where("is_public = true OR user_id = ?", userID)
	} else {
		q = q.Where("is_public = true")
	}
	err := q.Order("sort_order ASC, created_at DESC").Limit(1000).Find(&items).Error
	return items, err
}
