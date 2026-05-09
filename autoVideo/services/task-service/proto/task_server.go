// Package proto contains hand-written gRPC server implementation.
// Run `protoc` to regenerate the pb.go files from task.proto.
// For now we define the service interface and a concrete server inline
// so the service compiles without a protoc step.
package proto

import (
	"context"
	"fmt"
	"time"

	"github.com/autovideo/task-service/internal/model"
	"github.com/autovideo/task-service/internal/service"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---- hand-written protobuf message structs (avoids protoc dependency) ----

type CreateTaskRequest struct {
	TaskType   string `json:"task_type"`
	Payload    []byte `json:"payload"`
	Priority   int32  `json:"priority"`
	UserID     uint64 `json:"user_id"`
	MaxRetries int32  `json:"max_retries"`
}

type TaskResponse struct {
	Id        uint64 `json:"id"`
	TaskType  string `json:"task_type"`
	Status    string `json:"status"`
	Priority  int32  `json:"priority"`
	CreatedAt string `json:"created_at"`
}

type UpdateStatusRequest struct {
	TaskId   uint64 `json:"task_id"`
	Status   string `json:"status"`
	ErrorMsg string `json:"error_msg"`
}

type UpdateStatusResponse struct {
	Success bool `json:"success"`
}

type UpdateProgressRequest struct {
	TaskId   uint64 `json:"task_id"`
	Progress int32  `json:"progress"`
	Message  string `json:"message"`
}

type UpdateProgressResponse struct {
	Success bool `json:"success"`
}

type GetTaskRequest struct {
	TaskId uint64 `json:"task_id"`
}

// ---- gRPC service descriptor ----

// TaskServiceServer is the interface that gRPC clients call.
type TaskServiceServer interface {
	CreateTask(context.Context, *CreateTaskRequest) (*TaskResponse, error)
	UpdateStatus(context.Context, *UpdateStatusRequest) (*UpdateStatusResponse, error)
	UpdateProgress(context.Context, *UpdateProgressRequest) (*UpdateProgressResponse, error)
	GetTask(context.Context, *GetTaskRequest) (*TaskResponse, error)
}

// RegisterTaskServiceServer —— 将 TaskServiceServer 实现注册到 gRPC 服务器
// RegisterTaskServiceServer registers srv onto s.
func RegisterTaskServiceServer(s *grpc.Server, srv TaskServiceServer) {
	s.RegisterService(&_TaskService_serviceDesc, srv)
}

var _TaskService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "task.v1.TaskService",
	HandlerType: (*TaskServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateTask",
			Handler:    _TaskService_CreateTask_Handler,
		},
		{
			MethodName: "UpdateStatus",
			Handler:    _TaskService_UpdateStatus_Handler,
		},
		{
			MethodName: "UpdateProgress",
			Handler:    _TaskService_UpdateProgress_Handler,
		},
		{
			MethodName: "GetTask",
			Handler:    _TaskService_GetTask_Handler,
		},
	},
	Streams: []grpc.StreamDesc{},
}

// _TaskService_CreateTask_Handler —— gRPC CreateTask 方法的服务端处理器
func _TaskService_CreateTask_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateTaskRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TaskServiceServer).CreateTask(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/task.v1.TaskService/CreateTask"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TaskServiceServer).CreateTask(ctx, req.(*CreateTaskRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// _TaskService_UpdateStatus_Handler —— gRPC UpdateStatus 方法的服务端处理器
func _TaskService_UpdateStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TaskServiceServer).UpdateStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/task.v1.TaskService/UpdateStatus"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TaskServiceServer).UpdateStatus(ctx, req.(*UpdateStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// _TaskService_UpdateProgress_Handler —— gRPC UpdateProgress 方法的服务端处理器
func _TaskService_UpdateProgress_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateProgressRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TaskServiceServer).UpdateProgress(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/task.v1.TaskService/UpdateProgress"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TaskServiceServer).UpdateProgress(ctx, req.(*UpdateProgressRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// _TaskService_GetTask_Handler —— gRPC GetTask 方法的服务端处理器
func _TaskService_GetTask_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetTaskRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TaskServiceServer).GetTask(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/task.v1.TaskService/GetTask"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TaskServiceServer).GetTask(ctx, req.(*GetTaskRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ---- concrete server ----

// GRPCServer implements TaskServiceServer backed by TaskService.
type GRPCServer struct {
	svc    *service.TaskService
	logger *zap.Logger
}

// NewGRPCServer —— 创建 gRPC 服务端实例，注入 TaskService 和日志
// NewGRPCServer creates a GRPCServer.
func NewGRPCServer(svc *service.TaskService, logger *zap.Logger) *GRPCServer {
	return &GRPCServer{svc: svc, logger: logger}
}

// CreateTask —— gRPC 接口：创建新任务并返回任务信息
func (g *GRPCServer) CreateTask(ctx context.Context, req *CreateTaskRequest) (*TaskResponse, error) {
	svcReq := service.CreateTaskReq{
		TaskType:   req.TaskType,
		Payload:    req.Payload,
		Priority:   int(req.Priority),
		UserID:     req.UserID,
		MaxRetries: int(req.MaxRetries),
	}
	task, err := g.svc.Create(svcReq)
	if err != nil {
		g.logger.Error("grpc CreateTask error", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return taskToResponse(task), nil
}

// UpdateStatus —— gRPC 接口：更新指定任务的状态
func (g *GRPCServer) UpdateStatus(ctx context.Context, req *UpdateStatusRequest) (*UpdateStatusResponse, error) {
	ts := model.TaskStatus(req.Status)
	if err := g.svc.UpdateStatus(req.TaskId, ts, req.ErrorMsg); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &UpdateStatusResponse{Success: true}, nil
}

// UpdateProgress —— gRPC 接口：更新指定任务的进度
func (g *GRPCServer) UpdateProgress(ctx context.Context, req *UpdateProgressRequest) (*UpdateProgressResponse, error) {
	if err := g.svc.UpdateProgress(req.TaskId, int(req.Progress), req.Message); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &UpdateProgressResponse{Success: true}, nil
}

// GetTask —— gRPC 接口：根据 ID 查询任务信息
func (g *GRPCServer) GetTask(ctx context.Context, req *GetTaskRequest) (*TaskResponse, error) {
	task, err := g.svc.GetByID(req.TaskId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "task %d not found: %v", req.TaskId, err)
	}
	return taskToResponse(task), nil
}

// taskToResponse —— 将 Task 模型转换为 gRPC TaskResponse
func taskToResponse(t *model.Task) *TaskResponse {
	return &TaskResponse{
		Id:        t.ID,
		TaskType:  t.TaskType,
		Status:    string(t.Status),
		Priority:  int32(t.Priority),
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
}

// Ensure GRPCServer implements TaskServiceServer at compile time.
var _ TaskServiceServer = (*GRPCServer)(nil)

// Keep fmt import used.
var _ = fmt.Sprintf
