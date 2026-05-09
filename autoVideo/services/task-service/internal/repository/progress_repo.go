package repository

import (
	"github.com/autovideo/task-service/internal/model"
	"gorm.io/gorm"
)

// ProgressRepo handles persistence for TaskProgress records.
type ProgressRepo struct {
	db *gorm.DB
}

// NewProgressRepo —— 创建 ProgressRepo 实例
// NewProgressRepo creates a new ProgressRepo.
func NewProgressRepo(db *gorm.DB) *ProgressRepo {
	return &ProgressRepo{db: db}
}

// Create —— 将一条任务进度记录写入数据库
// Create inserts a new progress record.
func (r *ProgressRepo) Create(p *model.TaskProgress) error {
	return r.db.Create(p).Error
}

// ListByTaskID —— 查询指定任务的全部进度记录，按时间正序返回
// ListByTaskID returns all progress records for a task ordered by creation time.
func (r *ProgressRepo) ListByTaskID(taskID uint64) ([]model.TaskProgress, error) {
	var records []model.TaskProgress
	err := r.db.
		Where("task_id = ?", taskID).
		Order("created_at ASC").
		Find(&records).Error
	return records, err
}

// LatestByTaskID —— 查询指定任务的最新一条进度记录
// LatestByTaskID returns the most recent progress entry for a task.
func (r *ProgressRepo) LatestByTaskID(taskID uint64) (*model.TaskProgress, error) {
	var p model.TaskProgress
	err := r.db.
		Where("task_id = ?", taskID).
		Order("created_at DESC").
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}
