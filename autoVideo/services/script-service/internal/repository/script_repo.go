package repository

import (
	"context"

	"github.com/autovideo/script-service/internal/model"
	"gorm.io/gorm"
)

type ScriptRepository interface {
	Create(ctx context.Context, script *model.Script) error
	GetByID(ctx context.Context, id int64) (*model.Script, error)
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateLLMResult(ctx context.Context, id int64, status string, result model.JSONMap) error
	ListByProjectID(ctx context.Context, projectID int64, page, size int) ([]*model.Script, int64, error)
	ListVersionsByProjectAndTitle(ctx context.Context, projectID int64, title string) ([]*model.Script, error)
	GetMaxVersion(ctx context.Context, projectID int64, title string) (int, error)
	TouchUpdatedAt(ctx context.Context, id int64) error
	FindIDsByProjectID(ctx context.Context, projectID int64) ([]int64, error)
	DeleteByProjectID(ctx context.Context, projectID int64) error
	DeleteByID(ctx context.Context, scriptID int64) error
}

type scriptRepository struct {
	db *gorm.DB
}

// NewScriptRepository —— 创建剧本仓储实例，返回 ScriptRepository 接口
func NewScriptRepository(db *gorm.DB) ScriptRepository {
	return &scriptRepository{db: db}
}

// Create —— 将剧本记录插入数据库
func (r *scriptRepository) Create(ctx context.Context, script *model.Script) error {
	return r.db.WithContext(ctx).Create(script).Error
}

// GetByID —— 根据 ID 查询剧本记录，返回剧本指针或错误
func (r *scriptRepository) GetByID(ctx context.Context, id int64) (*model.Script, error) {
	var script model.Script
	if err := r.db.WithContext(ctx).First(&script, id).Error; err != nil {
		return nil, err
	}
	return &script, nil
}

// UpdateStatus —— 更新剧本的解析状态字段
func (r *scriptRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	return r.db.WithContext(ctx).
		Model(&model.Script{}).
		Where("id = ?", id).
		Update("parse_status", status).Error
}

// UpdateLLMResult —— 更新剧本的解析状态和 LLM 分析结果
func (r *scriptRepository) UpdateLLMResult(ctx context.Context, id int64, status string, result model.JSONMap) error {
	return r.db.WithContext(ctx).
		Model(&model.Script{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"parse_status": status,
			"llm_result":   result,
		}).Error
}

// ListByProjectID —— 按项目 ID 分页查询剧本列表，返回剧本列表和总数
func (r *scriptRepository) ListByProjectID(ctx context.Context, projectID int64, page, size int) ([]*model.Script, int64, error) {
	var scripts []*model.Script
	var total int64

	db := r.db.WithContext(ctx).Model(&model.Script{}).Where("project_id = ?", projectID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * size
	if err := db.Offset(offset).Limit(size).Order("id DESC").Find(&scripts).Error; err != nil {
		return nil, 0, err
	}
	return scripts, total, nil
}

// ListVersionsByProjectAndTitle —— 查询同一项目和标题下的所有剧本版本，按版本号升序
func (r *scriptRepository) ListVersionsByProjectAndTitle(ctx context.Context, projectID int64, title string) ([]*model.Script, error) {
	var scripts []*model.Script
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND title = ?", projectID, title).
		Order("version ASC").
		Find(&scripts).Error; err != nil {
		return nil, err
	}
	return scripts, nil
}

// GetMaxVersion —— 查询同一项目和标题下的最大版本号
func (r *scriptRepository) GetMaxVersion(ctx context.Context, projectID int64, title string) (int, error) {
	var maxVersion int
	err := r.db.WithContext(ctx).
		Model(&model.Script{}).
		Where("project_id = ? AND title = ?", projectID, title).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error
	return maxVersion, err
}

// TouchUpdatedAt —— 将指定剧本的 updated_at 更新为当前时间，用于标记为最新使用的版本
func (r *scriptRepository) TouchUpdatedAt(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.Script{}).
		Where("id = ?", id).
		Update("updated_at", gorm.Expr("NOW()")).Error
}

// FindIDsByProjectID —— 查询项目下所有剧本的 ID 列表
func (r *scriptRepository) FindIDsByProjectID(ctx context.Context, projectID int64) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).Model(&model.Script{}).
		Where("project_id = ?", projectID).
		Pluck("id", &ids).Error
	return ids, err
}

// DeleteByProjectID —— 删除项目下所有剧本记录（硬删除）
func (r *scriptRepository) DeleteByProjectID(ctx context.Context, projectID int64) error {
	return r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Delete(&model.Script{}).Error
}

// DeleteByID —— 根据 ID 删除单条剧本记录（硬删除），子表应在此之前完成清理
func (r *scriptRepository) DeleteByID(ctx context.Context, scriptID int64) error {
return r.db.WithContext(ctx).
Where("id = ?", scriptID).
Delete(&model.Script{}).Error
}
