package repository

import (
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"github.com/autovideo/project-service/internal/model"
)

type ProjectRepo struct {
	db *gorm.DB
}

// NewProjectRepo —— 创建项目仓储实例
func NewProjectRepo(db *gorm.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

// Create —— 新增一条项目记录到数据库
func (r *ProjectRepo) Create(p *model.Project) error {
	return r.db.Create(p).Error
}

// FindByID —— 按 ID 和用户 ID 查询项目（含权限校验），预加载剧集
// FindByID retrieves a project by ID and verifies ownership via userID.
func (r *ProjectRepo) FindByID(id, userID uint64) (*model.Project, error) {
	var project model.Project
	err := r.db.Preload("Episodes").
		Where("id = ? AND user_id = ?", id, userID).
		First(&project).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &project, nil
}

// FindByIDNoAuth —— 按 ID 查询项目，不校验用户归属
// FindByIDNoAuth returns a project by ID without user ownership check.
func (r *ProjectRepo) FindByIDNoAuth(id uint64) (*model.Project, error) {
	var project model.Project
	err := r.db.First(&project, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &project, nil
}

// List —— 分页查询用户的项目列表，支持关键词和状态过滤
// List returns paginated projects for a user with optional keyword and status filters.
func (r *ProjectRepo) List(userID uint64, keyword, status, projectType string, page, pageSize int) ([]model.Project, int64, error) {
	var projects []model.Project
	var total int64

	query := r.db.Model(&model.Project{}).Where("user_id = ?", userID)

	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title ILIKE ? OR description ILIKE ?", like, like)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if projectType != "" {
		query = query.Where("project_type = ?", projectType)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Find(&projects).Error
	if err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

// FindAutoPreparationCandidates returns projects that were left in the
// post-split auto-preparation stage and should be resumed after a restart.
func (r *ProjectRepo) FindAutoPreparationCandidates(limit int) ([]model.Project, error) {
	var projects []model.Project
	query := r.db.Model(&model.Project{}).
		Where("status IN ?", []string{"script_processing", "asset_generating"}).
		Where("COALESCE(progress ->> 'stage', '') = ?", "script_prepping").
		Order("updated_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&projects).Error
	return projects, err
}

// FindEpisodeGenerationCandidates returns projects that were interrupted before
// episode splitting completed and should be restarted on service boot.
func (r *ProjectRepo) FindEpisodeGenerationCandidates(limit int) ([]model.Project, error) {
	var projects []model.Project
	query := r.db.Model(&model.Project{}).
		Where("status = ?", "script_processing").
		Where("COALESCE(progress ->> 'stage', '') IN ?", []string{"", "episode_splitting"}).
		Order("updated_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&projects).Error
	return projects, err
}

// Update —— 保存项目记录的全部字段更新
func (r *ProjectRepo) Update(p *model.Project) error {
	return r.db.Save(p).Error
}

// SoftDelete —— 按 ID 和用户 ID 软删除项目
// SoftDelete soft-deletes a project by ID with ownership check.
func (r *ProjectRepo) SoftDelete(id, userID uint64) error {
	result := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Project{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateStatus —— 仅更新项目的状态字段
// UpdateStatus updates only the status column for a project.
func (r *ProjectRepo) UpdateStatus(id, userID uint64, newStatus string) error {
	result := r.db.Model(&model.Project{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("status", newStatus)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateProgress —— 原子更新项目的进度 JSONB 字段
// UpdateProgress atomically updates the progress JSONB field for a project.
func (r *ProjectRepo) UpdateProgress(id uint64, progress json.RawMessage) error {
	return r.db.Model(&model.Project{}).
		Where("id = ?", id).
		Update("progress", progress).Error
}

// UpdateStatusAndProgress —— 原子更新项目的状态和进度
// UpdateStatusAndProgress atomically updates both status and progress.
func (r *ProjectRepo) UpdateStatusAndProgress(id, userID uint64, status string, progress json.RawMessage) error {
	result := r.db.Model(&model.Project{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{
			"status":   status,
			"progress": progress,
		})
	if result.Error != nil {
		return result.Error
	}
	return nil
}

// ScriptVersionRepo

type ScriptVersionRepo struct {
	db *gorm.DB
}

// NewScriptVersionRepo —— 创建剧本版本仓储实例
func NewScriptVersionRepo(db *gorm.DB) *ScriptVersionRepo {
	return &ScriptVersionRepo{db: db}
}

// Create —— 新增一条剧本版本记录
func (r *ScriptVersionRepo) Create(sv *model.ScriptVersion) error {
	return r.db.Create(sv).Error
}

// ListByProjectID —— 按项目 ID 查询所有剧本版本，按版本号倒序
func (r *ScriptVersionRepo) ListByProjectID(projectID uint64) ([]model.ScriptVersion, error) {
	var versions []model.ScriptVersion
	err := r.db.Where("project_id = ?", projectID).
		Order("version_number DESC").
		Find(&versions).Error
	return versions, err
}

// FindByID —— 按主键查询单条剧本版本记录
func (r *ScriptVersionRepo) FindByID(id uint64) (*model.ScriptVersion, error) {
	var sv model.ScriptVersion
	err := r.db.First(&sv, id).Error
	if err != nil {
		return nil, err
	}
	return &sv, nil
}

// SetCurrent —— 在事务中将指定版本设为当前版本并取消其他版本
// SetCurrent marks a version as current and un-marks all others for that project.
func (r *ScriptVersionRepo) SetCurrent(projectID, versionID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ScriptVersion{}).
			Where("project_id = ?", projectID).
			Update("is_current", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.ScriptVersion{}).
			Where("id = ? AND project_id = ?", versionID, projectID).
			Update("is_current", true).Error
	})
}

// SetAllNotCurrent —— 将项目所有剧本版本标记为非当前
// SetAllNotCurrent marks all versions of a project as not current.
func (r *ScriptVersionRepo) SetAllNotCurrent(projectID uint64) error {
	return r.db.Model(&model.ScriptVersion{}).
		Where("project_id = ?", projectID).
		Update("is_current", false).Error
}

// NextVersionNumber —— 返回项目的下一个可用版本号
// NextVersionNumber returns the next available version number for a project.
func (r *ScriptVersionRepo) NextVersionNumber(projectID uint64) int {
	var max int
	r.db.Model(&model.ScriptVersion{}).
		Where("project_id = ?", projectID).
		Select("COALESCE(MAX(version_number), 0)").
		Scan(&max)
	return max + 1
}

// DeleteByProjectID —— 删除项目下所有剧本版本记录
func (r *ScriptVersionRepo) DeleteByProjectID(projectID uint64) error {
	return r.db.Where("project_id = ?", projectID).Delete(&model.ScriptVersion{}).Error
}
