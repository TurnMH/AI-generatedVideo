package handler

import (
	"strconv"

	"github.com/autovideo/task-service/internal/service"
	"github.com/autovideo/task-service/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TaskHandler handles HTTP requests for task management.
type TaskHandler struct {
	svc    *service.TaskService
	logger *zap.Logger
}

// NewTaskHandler —— 创建 TaskHandler 实例
// NewTaskHandler creates a TaskHandler.
func NewTaskHandler(svc *service.TaskService, logger *zap.Logger) *TaskHandler {
	return &TaskHandler{svc: svc, logger: logger}
}

// CreateTask —— 处理 POST /tasks，创建新任务并发布到 Kafka
// CreateTask handles POST /api/v1/tasks.
// It creates a new task and publishes it to Kafka.
func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req service.CreateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// allow callers to pass user_id via header for service-to-service calls
	if req.UserID == 0 {
		if uid, err := strconv.ParseUint(c.GetHeader("X-User-ID"), 10, 64); err == nil {
			req.UserID = uid
		}
	}
	if req.UserID == 0 {
		response.BadRequest(c, "user_id is required")
		return
	}

	task, err := h.svc.Create(req)
	if err != nil {
		h.logger.Error("CreateTask error", zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}

	response.Created(c, task)
}

// GetTask —— 处理 GET /tasks/:id，根据 ID 返回任务详情
// GetTask handles GET /api/v1/tasks/:id.
func (h *TaskHandler) GetTask(c *gin.Context) {
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}

	task, err := h.svc.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "task not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, task)
}

// ListTasks —— 处理 GET /tasks，按用户、状态、类型、项目分页查询任务列表
// ListTasks handles GET /api/v1/tasks.
// Query params: status, type, project_id, page, page_size, user_id.
func (h *TaskHandler) ListTasks(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 64)
	if userID == 0 {
		if uid, err := strconv.ParseUint(c.GetHeader("X-User-ID"), 10, 64); err == nil {
			userID = uid
		}
	}
	if userID == 0 {
		response.BadRequest(c, "user_id is required")
		return
	}

	status := c.Query("status")
	taskType := c.Query("type")
	projectID, _ := strconv.ParseUint(c.Query("project_id"), 10, 64)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	tasks, total, err := h.svc.List(userID, status, taskType, projectID, page, pageSize)
	if err != nil {
		h.logger.Error("ListTasks error", zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}

	response.Page(c, tasks, total, page, pageSize)
}

// CancelTask —— 处理 POST /tasks/:id/cancel，取消指定的待执行任务
// CancelTask handles POST /api/v1/tasks/:id/cancel.
func (h *TaskHandler) CancelTask(c *gin.Context) {
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}

	var userID uint64
	if uid, err := strconv.ParseUint(c.GetHeader("X-User-ID"), 10, 64); err == nil {
		userID = uid
	}
	if userID == 0 {
		response.BadRequest(c, "X-User-ID header required")
		return
	}

	if err := h.svc.Cancel(id, userID); err != nil {
		switch err.Error() {
		case "forbidden":
			response.Forbidden(c, "access denied")
		default:
			response.BadRequest(c, err.Error())
		}
		return
	}

	response.OK(c, gin.H{"cancelled": true})
}

// GetProgress —— 处理 GET /tasks/:id/progress，返回任务的进度历史记录
// GetProgress handles GET /api/v1/tasks/:id/progress.
func (h *TaskHandler) GetProgress(c *gin.Context) {
	id, err := parseUint64Param(c, "id")
	if err != nil {
		response.BadRequest(c, "invalid task id")
		return
	}

	records, err := h.svc.GetProgress(id)
	if err != nil {
		h.logger.Error("GetProgress error", zap.Error(err))
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, records)
}

// parseUint64Param —— 从路由参数中解析指定名称的 uint64 值
// parseUint64Param extracts a named route parameter as uint64.
func parseUint64Param(c *gin.Context, name string) (uint64, error) {
	return strconv.ParseUint(c.Param(name), 10, 64)
}
