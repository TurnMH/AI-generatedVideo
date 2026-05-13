package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/autovideo/storage-service/internal/driver"
	"github.com/autovideo/storage-service/internal/model"
	"github.com/google/uuid"
)

// UploadReq contains all fields needed to upload a file.
type UploadReq struct {
	UserID      uint64
	Bucket      string
	Filename    string
	ContentType string
	Size        int64
	Reader      io.Reader
	ProjectID   *uint64
	Category    string
}

// UploadRes holds the result of a successful upload.
type UploadRes struct {
	ObjectKey string `json:"object_key"`
	CdnURL    string `json:"cdn_url"`
	FileID    uint64 `json:"file_id"`
}

// fileRepo is the minimal interface StorageService needs from the repository.
// *repository.FileRepo satisfies this interface.
type fileRepo interface {
	Create(ctx context.Context, f *model.File) error
	GetByObjectKey(ctx context.Context, objectKey string) (*model.File, error)
	ListByUserID(ctx context.Context, userID uint64) ([]model.File, error)
	DeleteByObjectKey(ctx context.Context, objectKey string) error
}

// StorageService orchestrates uploads, URL generation, and deletions.
type StorageService struct {
	driver           driver.StorageDriver
	repo             fileRepo
	cdnBase          string
	bucketMap        map[string]string // logical name → physical bucket name
	includeBucketCDN bool              // true=本地MinIO(CDN URL含桶名); false=生产CDN(桶名由CDN层绑定)
}

// NewStorageService —— 创建存储服务实例，注入驱动、仓库、CDN 地址和桶名映射表
// bucketMap maps logical bucket names (e.g. "videos") to physical bucket names
// (e.g. "jrl" for TOS single-bucket). Pass nil or empty map to use names as-is.
// includeBucketCDN: set true for local MinIO (path-style URLs include bucket),
// false for production CDN (CDN domain is already bound to a specific bucket).
func NewStorageService(d driver.StorageDriver, repo fileRepo, cdnBase string, bucketMap map[string]string, includeBucketCDN bool) *StorageService {
	if bucketMap == nil {
		bucketMap = make(map[string]string)
	}
	return &StorageService{driver: d, repo: repo, cdnBase: cdnBase, bucketMap: bucketMap, includeBucketCDN: includeBucketCDN}
}

// resolveBucket —— 将逻辑桶名解析为配置中的物理桶名
// Falls back to the logical name if no mapping is found.
func (s *StorageService) resolveBucket(logical string) string {
	if physical, ok := s.bucketMap[logical]; ok && physical != "" {
		return physical
	}
	return logical
}

// Upload —— 上传文件到存储驱动并持久化元数据，返回对象 key、CDN URL 和文件 ID
// Upload stores the file and persists its metadata.
func (s *StorageService) Upload(ctx context.Context, req UploadReq) (*UploadRes, error) {
	ext := filepath.Ext(req.Filename)
	// objectKey format: ai-video/{YYYY}/{MM}/{DD}/{uuid}{ext}
	// e.g. ai-video/2026/04/17/c560b392-f3bf-47da-849d-ea09197e2a26.png
	dateDir := time.Now().UTC().Format("2006/01/02")
	objectKey := fmt.Sprintf("ai-video/%s/%s%s", dateDir, uuid.NewString(), ext)

	physical := s.resolveBucket(req.Bucket)
	if err := s.driver.Upload(ctx, physical, objectKey, req.Reader, req.Size, req.ContentType); err != nil {
		return nil, fmt.Errorf("driver.Upload: %w", err)
	}

	// CDN URL:
	//   production (cdn_include_bucket=false): https://cdn.jurilu.com/ai-video/2026/04/17/uuid.png
	//   local MinIO (cdn_include_bucket=true):  http://localhost:9000/images/ai-video/2026/04/17/uuid.png
	base := strings.TrimRight(s.cdnBase, "/")
	var cdnURL string
	if s.includeBucketCDN {
		cdnURL = base + "/" + physical + "/" + objectKey
	} else {
		cdnURL = base + "/" + objectKey
	}

	f := &model.File{
		UserID:      req.UserID,
		Bucket:      physical, // store physical name for correct retrieval
		ObjectKey:   objectKey,
		Filename:    req.Filename,
		ContentType: req.ContentType,
		SizeBytes:   req.Size,
		CdnURL:      cdnURL,
		ProjectID:   req.ProjectID,
		Category:    req.Category,
	}
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, fmt.Errorf("repo.Create: %w", err)
	}

	return &UploadRes{
		ObjectKey: objectKey,
		CdnURL:    cdnURL,
		FileID:    f.ID,
	}, nil
}

// GetURL —— 根据 objectKey 获取文件的（预签名）访问 URL
// GetURL returns a (possibly pre-signed) URL for the given object key.
func (s *StorageService) GetURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	f, err := s.repo.GetByObjectKey(ctx, objectKey)
	if err != nil {
		return "", fmt.Errorf("repo.GetByObjectKey: %w", err)
	}
	if strings.TrimSpace(f.CdnURL) != "" {
		return f.CdnURL, nil
	}
	u, err := s.driver.GetURL(ctx, f.Bucket, objectKey, expiry)
	if err != nil {
		return "", fmt.Errorf("driver.GetURL: %w", err)
	}
	return u, nil
}

// Delete —— 校验权限后从存储驱动和数据库中删除指定文件
// Delete removes the file from the driver and the database.
func (s *StorageService) Delete(ctx context.Context, objectKey string, userID uint64) error {
	f, err := s.repo.GetByObjectKey(ctx, objectKey)
	if err != nil {
		return fmt.Errorf("repo.GetByObjectKey: %w", err)
	}
	if f.UserID != userID {
		return fmt.Errorf("permission denied")
	}
	if err := s.driver.Delete(ctx, f.Bucket, objectKey); err != nil {
		return fmt.Errorf("driver.Delete: %w", err)
	}
	return s.repo.DeleteByObjectKey(ctx, objectKey)
}

// GetPresignedPutURL —— 生成预签名上传 URL 和对应的 objectKey
// GetPresignedPutURL returns a pre-signed PUT URL and the generated object key.
func (s *StorageService) GetPresignedPutURL(ctx context.Context, bucket, filename string, userID uint64) (string, string, error) {
	ext := filepath.Ext(filename)
	dateDir := time.Now().UTC().Format("2006/01/02")
	objectKey := fmt.Sprintf("ai-video/%s/%s%s", dateDir, uuid.NewString(), ext)

	physical := s.resolveBucket(bucket)
	presignURL, err := s.driver.GetPresignedPutURL(ctx, physical, objectKey, 15*time.Minute)
	if err != nil {
		return "", "", fmt.Errorf("driver.GetPresignedPutURL: %w", err)
	}
	return presignURL, objectKey, nil
}

// ListFiles —— 查询并返回指定用户的所有文件列表
// ListFiles returns all files belonging to a user.
func (s *StorageService) ListFiles(ctx context.Context, userID uint64) ([]model.File, error) {
	return s.repo.ListByUserID(ctx, userID)
}
