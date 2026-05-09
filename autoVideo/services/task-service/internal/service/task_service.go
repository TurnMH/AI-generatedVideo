package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/autovideo/task-service/internal/hub"
	"github.com/autovideo/task-service/internal/model"
	"github.com/autovideo/task-service/internal/repository"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"gorm.io/datatypes"
)

// CreateTaskReq carries the fields needed to create a new task.
type CreateTaskReq struct {
	TaskType   string          `json:"task_type" binding:"required"`
	Payload    json.RawMessage `json:"payload"`
	Priority   int             `json:"priority"`
	UserID     uint64          `json:"user_id"`
	MaxRetries int             `json:"max_retries"`
}

// TaskService implements business logic for task management.
type TaskService struct {
	taskRepo *repository.TaskRepo
	progRepo *repository.ProgressRepo
	kafka    *KafkaService
	wsHub    *hub.Hub
	logger   *zap.Logger
}

// NewTaskService —— 创建 TaskService 实例，注入所有依赖
// NewTaskService creates a TaskService with all required dependencies.
func NewTaskService(
	taskRepo *repository.TaskRepo,
	progRepo *repository.ProgressRepo,
	kafka *KafkaService,
	wsHub *hub.Hub,
	logger *zap.Logger,
) *TaskService {
	return &TaskService{
		taskRepo: taskRepo,
		progRepo: progRepo,
		kafka:    kafka,
		wsHub:    wsHub,
		logger:   logger,
	}
}

// Create —— 创建新任务并发布到对应的 Kafka topic，返回任务实体
// Create stores a new task and dispatches it to the appropriate Kafka topic.
func (s *TaskService) Create(req CreateTaskReq) (*model.Task, error) {
	if req.MaxRetries == 0 {
		req.MaxRetries = 3
	}

	task := &model.Task{
		TaskType:   req.TaskType,
		Payload:    datatypes.JSON(req.Payload),
		Priority:   req.Priority,
		Status:     model.TaskPending,
		MaxRetries: req.MaxRetries,
		UserID:     req.UserID,
	}

	if err := s.taskRepo.Create(task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	topic := taskTypeTopic(req.TaskType)
	payload, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("marshal task: %w", err)
	}

	if err := s.kafka.Publish(
		context.Background(),
		topic,
		fmt.Sprintf("%d", task.ID),
		payload,
		req.Priority,
	); err != nil {
		// mark as failed if we cannot enqueue
		s.logger.Error("kafka publish failed", zap.Uint64("task_id", task.ID), zap.Error(err))
	}

	return task, nil
}

// CreateBatch —— 批量创建任务并一次性发布到 Kafka，减少网络往返
// CreateBatch stores multiple tasks and dispatches them in a single Kafka write.
func (s *TaskService) CreateBatch(reqs []CreateTaskReq) ([]*model.Task, error) {
	if len(reqs) == 0 {
		return nil, nil
	}

	tasks := make([]*model.Task, 0, len(reqs))
	for _, req := range reqs {
		if req.MaxRetries == 0 {
			req.MaxRetries = 3
		}
		task := &model.Task{
			TaskType:   req.TaskType,
			Payload:    datatypes.JSON(req.Payload),
			Priority:   req.Priority,
			Status:     model.TaskPending,
			MaxRetries: req.MaxRetries,
			UserID:     req.UserID,
		}
		if err := s.taskRepo.Create(task); err != nil {
			return nil, fmt.Errorf("create task: %w", err)
		}
		tasks = append(tasks, task)
	}

	// Group by topic and batch-publish.
	byTopic := make(map[string][]kafka.Message)
	for _, task := range tasks {
		topic := taskTypeTopic(task.TaskType)
		payload, err := json.Marshal(task)
		if err != nil {
			continue
		}
		byTopic[topic] = append(byTopic[topic], kafka.Message{
			Key:   []byte(fmt.Sprintf("%d", task.ID)),
			Value: payload,
			Headers: []kafka.Header{
				{Key: "X-Priority", Value: []byte(fmt.Sprintf("%d", task.Priority))},
			},
		})
	}
	for topic, msgs := range byTopic {
		if err := s.kafka.PublishBatch(context.Background(), topic, msgs); err != nil {
			s.logger.Error("batch kafka publish failed", zap.String("topic", topic), zap.Error(err))
		}
	}

	return tasks, nil
}

// GetByID —— 根据 ID 查询单个任务
// GetByID retrieves a single task.
func (s *TaskService) GetByID(id uint64) (*model.Task, error) {
	return s.taskRepo.GetByID(id)
}

// List —— 按用户、状态、类型、项目分页查询任务列表
// List returns a paginated, optionally filtered list of tasks for a user.
func (s *TaskService) List(userID uint64, status, taskType string, projectID uint64, page, pageSize int) ([]model.Task, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.taskRepo.List(userID, status, taskType, projectID, page, pageSize)
}

// Cancel —— 将待执行任务标记为已取消，仅任务所属用户可操作
// Cancel transitions a pending task to cancelled. Only pending tasks may be cancelled.
func (s *TaskService) Cancel(id, userID uint64) error {
	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		return err
	}
	if task.UserID != userID {
		return fmt.Errorf("forbidden")
	}
	if task.Status != model.TaskPending {
		return fmt.Errorf("only pending tasks can be cancelled, current status: %s", task.Status)
	}
	task.Status = model.TaskCancelled
	return s.taskRepo.Save(task)
}

// UpdateStatus —— 更新任务状态、记录时间戳并通过 WebSocket 广播状态变更
// UpdateStatus updates a task's status, recording timestamps and broadcasting via WebSocket.
func (s *TaskService) UpdateStatus(id uint64, status model.TaskStatus, errMsg string) error {
	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		return err
	}

	now := time.Now()
	task.Status = status
	task.ErrorMsg = errMsg

	switch status {
	case model.TaskRunning:
		task.StartedAt = &now
	case model.TaskSucceeded, model.TaskFailed, model.TaskCancelled:
		task.FinishedAt = &now
		if task.StartedAt == nil {
			task.StartedAt = &now
		}
	}

	if err := s.taskRepo.Save(task); err != nil {
		return err
	}

	// broadcast to WebSocket subscribers
	msgType := "task_status"
	switch status {
	case model.TaskSucceeded:
		msgType = "task_complete"
	case model.TaskFailed:
		msgType = "task_failed"
	}

	s.wsHub.BroadcastToTask(id, hub.WSMessage{
		Type:    msgType,
		TaskID:  id,
		Status:  string(status),
		Message: errMsg,
	})

	return nil
}

// UpdateResult —— 将任务结果写入 result 字段，更新状态为 succeeded 并广播
// UpdateResult stores a JSON result on the task, marks it succeeded, and broadcasts via WebSocket.
func (s *TaskService) UpdateResult(id uint64, result json.RawMessage) error {
	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		return err
	}
	now := time.Now()
	task.Status = model.TaskSucceeded
	task.Result = datatypes.JSON(result)
	task.FinishedAt = &now
	if task.StartedAt == nil {
		task.StartedAt = &now
	}
	if err := s.taskRepo.Save(task); err != nil {
		return err
	}
	s.wsHub.BroadcastToTask(id, hub.WSMessage{
		Type:   "task_complete",
		TaskID: id,
		Status: string(model.TaskSucceeded),
	})
	return nil
}

// UpdateProgress —— 记录任务进度并通过 WebSocket 广播进度更新
// UpdateProgress records a progress entry and broadcasts it via WebSocket.
func (s *TaskService) UpdateProgress(id uint64, progress int, message string) error {
	p := &model.TaskProgress{
		TaskID:   id,
		Progress: progress,
		Message:  message,
	}
	if err := s.progRepo.Create(p); err != nil {
		return err
	}

	s.wsHub.BroadcastToTask(id, hub.WSMessage{
		Type:     "task_progress",
		TaskID:   id,
		Progress: progress,
		Message:  message,
	})

	return nil
}

// GetProgress —— 查询指定任务的全部进度历史
// GetProgress returns the full progress history for a task.
func (s *TaskService) GetProgress(taskID uint64) ([]model.TaskProgress, error) {
	return s.progRepo.ListByTaskID(taskID)
}

// RetryFailedTasks —— 重试失败且仍有重试次数的任务，使用指数退避策略
// RetryFailedTasks re-enqueues failed tasks that still have retries remaining.
// It uses exponential back-off: a task is only retried after 2^retry_count seconds have elapsed
// since it finished.
func (s *TaskService) RetryFailedTasks(ctx context.Context) {
	tasks, err := s.taskRepo.FindFailedRetryable()
	if err != nil {
		s.logger.Error("RetryFailedTasks query error", zap.Error(err))
		return
	}

	for _, task := range tasks {
		task := task // capture
		// exponential back-off guard
		if task.FinishedAt != nil {
			backoff := time.Duration(math.Pow(2, float64(task.RetryCount))) * time.Second
			if time.Since(*task.FinishedAt) < backoff {
				continue
			}
		}

		task.Status = model.TaskPending
		task.RetryCount++
		task.ErrorMsg = ""
		task.StartedAt = nil
		task.FinishedAt = nil

		if err := s.taskRepo.Save(&task); err != nil {
			s.logger.Error("RetryFailedTasks save error", zap.Uint64("task_id", task.ID), zap.Error(err))
			continue
		}

		payload, _ := json.Marshal(task)
		topic := taskTypeTopic(task.TaskType)
		if err := s.kafka.Publish(ctx, topic, fmt.Sprintf("%d", task.ID), payload, task.Priority); err != nil {
			s.logger.Error("RetryFailedTasks publish error", zap.Uint64("task_id", task.ID), zap.Error(err))
		}

		s.logger.Info("task retried", zap.Uint64("task_id", task.ID), zap.Int("retry_count", task.RetryCount))
	}
}

// TimeoutCheck —— 检测运行超过 30 分钟的任务并标记为失败
// TimeoutCheck marks running tasks that have exceeded 30 minutes as failed.
func (s *TaskService) TimeoutCheck(ctx context.Context) {
	tasks, err := s.taskRepo.FindRunningOlderThanSeconds(30 * 60)
	if err != nil {
		s.logger.Error("TimeoutCheck query error", zap.Error(err))
		return
	}

	for _, task := range tasks {
		task := task
		if err := s.UpdateStatus(task.ID, model.TaskFailed, "timeout"); err != nil {
			s.logger.Error("TimeoutCheck update error", zap.Uint64("task_id", task.ID), zap.Error(err))
		} else {
			s.logger.Warn("task timed out", zap.Uint64("task_id", task.ID))
		}
	}
}

// taskTypeTopic —— 将任务类型字符串映射到对应的 Kafka 请求 topic
// taskTypeTopic maps a task type string to the corresponding Kafka request topic.
func taskTypeTopic(taskType string) string {
	switch taskType {
	case "script_analyze":
		return TopicScriptAnalyzeReq
	case "script_quick_generate":
		return TopicScriptQuickGenerateReq
	case "image_generate":
		return TopicImageGenerateReq
	case "video_generate":
		return TopicVideoGenerateReq
	case "music_generate":
		return TopicMusicGenerateReq
	default:
		return fmt.Sprintf("%s.request", taskType)
	}
}
