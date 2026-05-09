package repository

import (
	"context"
	"time"

	"github.com/autovideo/image-service/internal/model"
	"gorm.io/gorm"
)

type ImageRepo interface {
	Create(ctx context.Context, task *model.ImageTask) error
	GetByID(ctx context.Context, id int64) (*model.ImageTask, error)
	List(ctx context.Context, filter ListFilter) ([]*model.ImageTask, int64, error)
	UpdateStatus(ctx context.Context, id int64, status, resultURL, thumbnailURL, errMsg string) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	DeleteByProjectID(ctx context.Context, projectID int64) error
	FindStaleRunningAndPending(ctx context.Context, runningThreshold int) ([]*model.ImageTask, error)
	FindPendingAfterID(ctx context.Context, afterID int64, limit int) ([]*model.ImageTask, error)
	ResetStaleRunning(ctx context.Context) (int64, error)
	ResetStaleRunningOlderThan(ctx context.Context, threshold time.Duration) (int64, error)
}

type ListFilter struct {
	ProjectID int64
	SceneID   int64
	TaskType  string
	Status    string
	Page      int
	PageSize  int
}

type imageRepo struct {
	db *gorm.DB
}

// NewImageRepo —— 创建 ImageRepo 实例，返回 ImageRepo 接口
func NewImageRepo(db *gorm.DB) ImageRepo {
	return &imageRepo{db: db}
}

// Create —— 将图片任务记录插入数据库，返回错误信息
func (r *imageRepo) Create(ctx context.Context, task *model.ImageTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

// GetByID —— 根据 ID 查询图片任务，返回任务指针或错误
func (r *imageRepo) GetByID(ctx context.Context, id int64) (*model.ImageTask, error) {
	var task model.ImageTask
	err := r.db.WithContext(ctx).First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// List —— 按筛选条件分页查询图片任务列表，返回任务列表和总数
func (r *imageRepo) List(ctx context.Context, filter ListFilter) ([]*model.ImageTask, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.ImageTask{})

	if filter.ProjectID > 0 {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	if filter.SceneID > 0 {
		query = query.Where("scene_id = ?", filter.SceneID)
	}
	if filter.TaskType != "" {
		query = query.Where("task_type = ?", filter.TaskType)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var tasks []*model.ImageTask
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error
	return tasks, total, err
}

// UpdateStatus —— 更新图片任务的状态、结果 URL、缩略图 URL 和错误信息
func (r *imageRepo) UpdateStatus(ctx context.Context, id int64, status, resultURL, thumbnailURL, errMsg string) error {
	fields := map[string]interface{}{"status": status}
	if resultURL != "" {
		fields["result_url"] = resultURL
	}
	if thumbnailURL != "" {
		fields["thumbnail_url"] = thumbnailURL
	}
	if errMsg != "" {
		fields["error_msg"] = errMsg
	}
	return r.db.WithContext(ctx).Model(&model.ImageTask{}).Where("id = ?", id).Updates(fields).Error
}

// UpdateFields —— 根据字段映射批量更新图片任务的指定字段
func (r *imageRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.ImageTask{}).Where("id = ?", id).Updates(fields).Error
}

func (r *imageRepo) DeleteByProjectID(ctx context.Context, projectID int64) error {
	return r.db.WithContext(ctx).Where("project_id = ?", projectID).Delete(&model.ImageTask{}).Error
}

// ResetStaleRunning —— 将所有卡在 running 状态的任务重置为 pending 以便重新调度，返回影响行数
// Used at startup to clear all running tasks (orphaned from previous crash).
func (r *imageRepo) ResetStaleRunning(ctx context.Context) (int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.ImageTask{}).
		Where("status = ?", "running").
		Update("status", "pending")
	return tx.RowsAffected, tx.Error
}

// ResetStaleRunningOlderThan —— 将超过指定时长仍为 running 的任务重置为 pending，返回影响行数
// Used by periodic cleanup to avoid resetting tasks that are legitimately in progress.
func (r *imageRepo) ResetStaleRunningOlderThan(ctx context.Context, threshold time.Duration) (int64, error) {
	cutoff := time.Now().Add(-threshold)
	tx := r.db.WithContext(ctx).Model(&model.ImageTask{}).
		Where("status = ? AND updated_at < ?", "running", cutoff).
		Update("status", "pending")
	return tx.RowsAffected, tx.Error
}

// FindStaleRunningAndPending —— 查询状态为 pending 的任务列表（按 ID 升序，限制数量），用于重新调度
func (r *imageRepo) FindStaleRunningAndPending(ctx context.Context, limit int) ([]*model.ImageTask, error) {
	var tasks []*model.ImageTask
	err := r.db.WithContext(ctx).
		Where("status = ?", "pending").
		Order("id ASC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

// FindPendingAfterID —— 查询指定 ID 之后状态为 pending 的任务列表，用于分批恢复
func (r *imageRepo) FindPendingAfterID(ctx context.Context, afterID int64, limit int) ([]*model.ImageTask, error) {
	var tasks []*model.ImageTask
	err := r.db.WithContext(ctx).
		Where("status = ? AND id > ?", "pending", afterID).
		Order("id ASC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}
