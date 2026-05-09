package service

import (
	"context"

	"github.com/autovideo/model-service/internal/model"
	"github.com/autovideo/model-service/internal/repository"
	"go.uber.org/zap"
)

// ModelService encapsulates business logic for model CRUD and health queries.
type ModelService struct {
	modelRepo *repository.ModelRepo
	usageRepo *repository.UsageRepo
	logger    *zap.Logger
}

// NewModelService —— 创建 ModelService 实例，注入仓库和日志依赖
// NewModelService constructs a ModelService.
func NewModelService(
	modelRepo *repository.ModelRepo,
	usageRepo *repository.UsageRepo,
	logger *zap.Logger,
) *ModelService {
	return &ModelService{
		modelRepo: modelRepo,
		usageRepo: usageRepo,
		logger:    logger,
	}
}

// List —— 查询活跃模型列表，可按类型筛选；types 支持传入多个类型（mutilModelList 级联查询）
// List returns active models, optionally filtered by one or more types.
func (s *ModelService) List(ctx context.Context, modelTypes ...string) ([]*model.Model, error) {
	return s.modelRepo.List(ctx, modelTypes...)
}

// ListFiltered —— 按筛选条件查询模型列表
// ListFiltered returns models matching the given filter criteria.
func (s *ModelService) ListFiltered(ctx context.Context, f model.ListFilter) ([]*model.Model, error) {
	return s.modelRepo.ListFiltered(ctx, f)
}

// GetByID —— 根据主键 ID 查询单个模型
// GetByID returns a model by primary key.
func (s *ModelService) GetByID(ctx context.Context, id uint64) (*model.Model, error) {
	return s.modelRepo.GetByID(ctx, id)
}

// Create —— 创建新模型并记录日志
// Create inserts a new model record.
func (s *ModelService) Create(ctx context.Context, m *model.Model) error {
	if err := s.modelRepo.Create(ctx, m); err != nil {
		s.logger.Error("create model failed", zap.String("name", m.Name), zap.Error(err))
		return err
	}
	s.logger.Info("model created", zap.Uint64("id", m.ID), zap.String("name", m.Name))
	return nil
}

// Update —— 更新模型全部字段并记录日志
// Update saves all fields of the given model.
func (s *ModelService) Update(ctx context.Context, m *model.Model) error {
	if err := s.modelRepo.Update(ctx, m); err != nil {
		s.logger.Error("update model failed", zap.Uint64("id", m.ID), zap.Error(err))
		return err
	}
	s.logger.Info("model updated", zap.Uint64("id", m.ID))
	return nil
}

// Delete —— 删除模型，日志提醒检查引用关系
// Delete removes a model. Logs a warning if the model might be referenced.
func (s *ModelService) Delete(ctx context.Context, id uint64) error {
	s.logger.Warn("deleting model – ensure no active projects reference it", zap.Uint64("id", id))
	if err := s.modelRepo.Delete(ctx, id); err != nil {
		s.logger.Error("delete model failed", zap.Uint64("id", id), zap.Error(err))
		return err
	}
	s.logger.Info("model deleted", zap.Uint64("id", id))
	return nil
}

// ToggleActive —— 切换模型的启用/禁用状态，返回更新后的模型
// ToggleActive flips the is_active flag of a model and returns the updated record.
func (s *ModelService) ToggleActive(ctx context.Context, id uint64) (*model.Model, error) {
	m, err := s.modelRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	m.IsActive = !m.IsActive
	if err := s.modelRepo.Update(ctx, m); err != nil {
		s.logger.Error("toggle active failed", zap.Uint64("id", id), zap.Error(err))
		return nil, err
	}
	s.logger.Info("model toggled", zap.Uint64("id", id), zap.Bool("is_active", m.IsActive))
	return m, nil
}

// SetDefault —— 将指定模型设为其类型的默认模型
// SetDefault marks a model as the default for its type, unsetting any previous default.
func (s *ModelService) SetDefault(ctx context.Context, id uint64) (*model.Model, error) {
	m, err := s.modelRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.modelRepo.SetDefault(ctx, id, m.Type); err != nil {
		s.logger.Error("set default failed", zap.Uint64("id", id), zap.Error(err))
		return nil, err
	}
	m.IsDefault = true
	s.logger.Info("model set as default", zap.Uint64("id", id), zap.String("type", m.Type))
	return m, nil
}

// GetHealthStatus —— 返回所有活跃模型的最新健康检查记录
// GetHealthStatus returns the latest health record for every active model.
func (s *ModelService) GetHealthStatus(ctx context.Context) ([]*model.ModelHealth, error) {
	return s.modelRepo.GetLatestHealth(ctx)
}
