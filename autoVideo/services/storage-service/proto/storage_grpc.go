// Code generated manually — mirrors storage.proto definitions.
// Run `protoc` to regenerate from storage.proto when needed.
package proto

import (
	"bytes"
	"context"
	"time"

	"github.com/autovideo/storage-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- Minimal hand-written pb types (avoids protoc toolchain requirement) ---

type UploadRequest struct {
	Data        []byte `json:"data"`
	Bucket      string `json:"bucket"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	UserId      uint64 `json:"user_id"`
}

type UploadResponse struct {
	ObjectKey string `json:"object_key"`
	CdnUrl    string `json:"cdn_url"`
	FileId    uint64 `json:"file_id"`
}

type GetURLRequest struct {
	ObjectKey     string `json:"object_key"`
	ExpirySeconds int64  `json:"expiry_seconds"`
}

type GetURLResponse struct {
	Url string `json:"url"`
}

type DeleteRequest struct {
	ObjectKey string `json:"object_key"`
	UserId    uint64 `json:"user_id"`
}

type DeleteResponse struct {
	Success bool `json:"success"`
}

// StorageServiceServer is implemented by the gRPC server.
type StorageServiceServer interface {
	Upload(context.Context, *UploadRequest) (*UploadResponse, error)
	GetURL(context.Context, *GetURLRequest) (*GetURLResponse, error)
	Delete(context.Context, *DeleteRequest) (*DeleteResponse, error)
}

// GRPCServer implements StorageServiceServer.
type GRPCServer struct {
	svc *service.StorageService
}

// NewGRPCServer —— 创建 gRPC 服务器实例，注入存储服务
// NewGRPCServer creates a GRPCServer.
func NewGRPCServer(svc *service.StorageService) *GRPCServer {
	return &GRPCServer{svc: svc}
}

// Upload —— gRPC 上传接口，将请求数据转发给存储服务并返回上传结果
func (s *GRPCServer) Upload(ctx context.Context, req *UploadRequest) (*UploadResponse, error) {
	res, err := s.svc.Upload(ctx, service.UploadReq{
		UserID:      req.UserId,
		Bucket:      req.Bucket,
		Filename:    req.Filename,
		ContentType: req.ContentType,
		Size:        int64(len(req.Data)),
		Reader:      bytes.NewReader(req.Data),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upload failed: %v", err)
	}
	return &UploadResponse{
		ObjectKey: res.ObjectKey,
		CdnUrl:    res.CdnURL,
		FileId:    res.FileID,
	}, nil
}

// GetURL —— gRPC 获取文件访问 URL 接口，返回预签名或 CDN 地址
func (s *GRPCServer) GetURL(ctx context.Context, req *GetURLRequest) (*GetURLResponse, error) {
	expiry := time.Duration(req.ExpirySeconds) * time.Second
	if expiry <= 0 {
		expiry = time.Hour
	}
	u, err := s.svc.GetURL(ctx, req.ObjectKey, expiry)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get url failed: %v", err)
	}
	return &GetURLResponse{Url: u}, nil
}

// Delete —— gRPC 删除文件接口，校验权限后删除文件
func (s *GRPCServer) Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error) {
	if err := s.svc.Delete(ctx, req.ObjectKey, req.UserId); err != nil {
		if err.Error() == "permission denied" {
			return nil, status.Errorf(codes.PermissionDenied, "permission denied")
		}
		return nil, status.Errorf(codes.Internal, "delete failed: %v", err)
	}
	return &DeleteResponse{Success: true}, nil
}


