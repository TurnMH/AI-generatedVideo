package repository

import (
	"context"

	"github.com/autovideo/model-service/internal/model"
	"gorm.io/gorm"
)

// ModelRepo handles persistence for Model and ModelHealth records.
type ModelRepo struct {
	db *gorm.DB
}

// NewModelRepo —— 创建 ModelRepo 实例
// NewModelRepo constructs a ModelRepo.
func NewModelRepo(db *gorm.DB) *ModelRepo {
	return &ModelRepo{db: db}
}

// List —— 查询模型列表，默认返回所有活跃模型，以及有 failure_reason 的已停用模型（供前端展示停用原因）
// List returns active models plus inactive models that have a failure_reason (so the UI can show why).
// modelTypes supports zero, one, or multiple types (mutilModelList).
func (r *ModelRepo) List(ctx context.Context, modelTypes ...string) ([]*model.Model, error) {
	var models []*model.Model
	q := r.db.WithContext(ctx).Where("is_active = ? OR (is_active = ? AND failure_reason IS NOT NULL)", true, false)
	switch len(modelTypes) {
	case 0:
		// no filter
	case 1:
		if modelTypes[0] != "" {
			q = q.Where("type = ?", modelTypes[0])
		}
	default:
		q = q.Where("type IN ?", modelTypes)
	}
	err := q.Order("is_active DESC, sort_order ASC, priority DESC").Find(&models).Error
	return models, err
}

// ListFiltered —— 按筛选条件（类型、供应商、速度等级等）查询模型列表
// ListFiltered returns models matching the given filter criteria.
func (r *ModelRepo) ListFiltered(ctx context.Context, f model.ListFilter) ([]*model.Model, error) {
	var models []*model.Model
	q := r.db.WithContext(ctx)

	if len(f.Types) > 0 {
		q = q.Where("type IN ?", f.Types)
	} else if f.Type != "" {
		q = q.Where("type = ?", f.Type)
	}
	if f.Provider != "" {
		q = q.Where("provider = ?", f.Provider)
	}
	if f.SpeedRating != "" {
		q = q.Where("speed_rating = ?", f.SpeedRating)
	}
	if f.Enabled != nil {
		q = q.Where("is_active = ?", *f.Enabled)
	}

	switch f.SortBy {
	case "input_price_asc":
		q = q.Order("input_price ASC NULLS LAST")
	case "input_price_desc":
		q = q.Order("input_price DESC NULLS LAST")
	case "sort_order":
		q = q.Order("sort_order ASC, priority DESC")
	default:
		q = q.Order("sort_order ASC, priority DESC")
	}

	err := q.Find(&models).Error
	return models, err
}

// GetByID —— 根据主键 ID 查询单个模型
// GetByID returns a model by primary key.
func (r *ModelRepo) GetByID(ctx context.Context, id uint64) (*model.Model, error) {
	var m model.Model
	err := r.db.WithContext(ctx).First(&m, id).Error
	return &m, err
}

// Create —— 将新模型记录写入数据库
// Create persists a new model.
func (r *ModelRepo) Create(ctx context.Context, m *model.Model) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// Update —— 更新模型的全部字段并保存到数据库
// Update persists all fields of an existing model.
func (r *ModelRepo) Update(ctx context.Context, m *model.Model) error {
	return r.db.WithContext(ctx).Save(m).Error
}

// Delete —— 根据主键 ID 删除模型记录
// Delete removes a model by primary key.
func (r *ModelRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Model{}, id).Error
}

// SetDefault —— 在事务中将指定模型设为同类型默认，同时取消原有默认
// SetDefault sets the given model as the default for its type within a transaction.
// It first unsets is_default for all models of the same type, then sets the target.
func (r *ModelRepo) SetDefault(ctx context.Context, id uint64, modelType string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Model{}).
			Where("type = ? AND is_default = ?", modelType, true).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.Model{}).Where("id = ?", id).Update("is_default", true).Error
	})
}

// GetActiveByType —— 查询指定类型的所有活跃模型，按优先级降序返回
// GetActiveByType returns active models for the given task type, ordered by priority desc.
func (r *ModelRepo) GetActiveByType(ctx context.Context, modelType string) ([]*model.Model, error) {
	var models []*model.Model
	err := r.db.WithContext(ctx).
		Where("is_active = ? AND type = ?", true, modelType).
		Order("priority DESC").
		Find(&models).Error
	return models, err
}

// GetAllActive —— 查询所有活跃模型（不限类型）
// GetAllActive returns every active model across all types.
func (r *ModelRepo) GetAllActive(ctx context.Context) ([]*model.Model, error) {
	var models []*model.Model
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&models).Error
	return models, err
}

// SaveHealth —— 将一条健康检查记录写入数据库
// SaveHealth inserts a new health-check record.
func (r *ModelRepo) SaveHealth(ctx context.Context, health *model.ModelHealth) error {
	return r.db.WithContext(ctx).Create(health).Error
}

// GetLatestHealth —— 查询每个模型的最新健康检查记录，返回关联的模型信息
// GetLatestHealth returns the most recent health record per model (joined with model info).
func (r *ModelRepo) GetLatestHealth(ctx context.Context) ([]*model.ModelHealth, error) {
	var healths []*model.ModelHealth

	// Subquery: latest id per model
	subQuery := r.db.Model(&model.ModelHealth{}).
		Select("MAX(id)").
		Group("model_id")

	err := r.db.WithContext(ctx).
		Where("id IN (?)", subQuery).
		Preload("Model").
		Find(&healths).Error
	return healths, err
}
