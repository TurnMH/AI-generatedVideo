package repository

import (
	"errors"
	"strconv"

	"github.com/autovideo/task-service/internal/model"
	"gorm.io/gorm"
)

// TaskRepo handles persistence for Task entities.
type TaskRepo struct {
	db *gorm.DB
}

// NewTaskRepo —— 创建 TaskRepo 实例
// NewTaskRepo creates a new TaskRepo.
func NewTaskRepo(db *gorm.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

// Create —— 将新任务记录写入数据库
// Create inserts a new task record.
func (r *TaskRepo) Create(task *model.Task) error {
	return r.db.Create(task).Error
}

// GetByID —— 根据主键 ID 查询单个任务
// GetByID retrieves a task by primary key.
func (r *TaskRepo) GetByID(id uint64) (*model.Task, error) {
	var task model.Task
	if err := r.db.First(&task, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &task, nil
}

// List —— 按用户、状态、类型、项目分页查询任务列表，返回数据和总数
// List retrieves tasks with optional filters and pagination.
func (r *TaskRepo) List(userID uint64, status, taskType string, projectID uint64, page, pageSize int) ([]model.Task, int64, error) {
	var tasks []model.Task
	var total int64

	q := r.db.Model(&model.Task{}).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if taskType != "" {
		q = q.Where("task_type = ?", taskType)
	}
	if projectID > 0 {
		q = q.Where("payload->>'project_id' = ?", strconv.FormatUint(projectID, 10))
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// UpdateStatus —— 更新任务状态及相关字段并保存
// UpdateStatus updates the status (and optional timestamps / error message) of a task.
func (r *TaskRepo) UpdateStatus(task *model.Task) error {
	return r.db.Save(task).Error
}

// Save —— 保存任务的全部字段到数据库
// Save persists all fields of a task.
func (r *TaskRepo) Save(task *model.Task) error {
	return r.db.Save(task).Error
}

// FindFailedRetryable —— 查询所有失败且仍有剩余重试次数的任务
// FindFailedRetryable returns failed tasks that still have retries remaining.
func (r *TaskRepo) FindFailedRetryable() ([]model.Task, error) {
	var tasks []model.Task
	err := r.db.
		Where("status = ? AND retry_count < max_retries", model.TaskFailed).
		Find(&tasks).Error
	return tasks, err
}

// FindRunningOlderThanSeconds —— 查询运行超过指定秒数的任务（用于超时检测）
// FindRunningOlderThan returns running tasks whose started_at is older than the given duration in seconds.
func (r *TaskRepo) FindRunningOlderThanSeconds(seconds int) ([]model.Task, error) {
	var tasks []model.Task
	err := r.db.
		Where("status = ? AND started_at < NOW() - INTERVAL '1 second' * ?", model.TaskRunning, seconds).
		Find(&tasks).Error
	return tasks, err
}
