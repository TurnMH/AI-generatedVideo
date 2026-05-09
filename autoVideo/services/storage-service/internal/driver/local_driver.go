package driver

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalDriver stores files on the local filesystem for development.
type LocalDriver struct {
	basePath string
	port     string
}

// NewLocalDriver —— 创建以 basePath 为根目录的本地存储驱动，返回 *LocalDriver
// NewLocalDriver creates a LocalDriver rooted at basePath.
func NewLocalDriver(basePath, port string) *LocalDriver {
	return &LocalDriver{basePath: basePath, port: port}
}

// Upload —— 将文件写入本地文件系统，按 bucket/objectKey 路径存储，返回 error
func (d *LocalDriver) Upload(ctx context.Context, bucket, objectKey string, reader io.Reader, size int64, contentType string) error {
	dest := filepath.Join(d.basePath, bucket, objectKey)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// GetURL —— 返回本地文件的可访问 URL（localhost 静态路径）
func (d *LocalDriver) GetURL(_ context.Context, bucket, objectKey string, _ time.Duration) (string, error) {
	return fmt.Sprintf("http://localhost:%s/static/%s/%s", d.port, bucket, objectKey), nil
}

// Delete —— 删除本地文件系统中指定 bucket/objectKey 对应的文件
func (d *LocalDriver) Delete(_ context.Context, bucket, objectKey string) error {
	path := filepath.Join(d.basePath, bucket, objectKey)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove file: %w", err)
	}
	return nil
}

// GetPresignedPutURL —— 返回用于上传的本地模拟预签名 URL
func (d *LocalDriver) GetPresignedPutURL(_ context.Context, bucket, objectKey string, _ time.Duration) (string, error) {
	return fmt.Sprintf("http://localhost:%s/api/v1/storage/upload?bucket=%s&key=%s", d.port, bucket, objectKey), nil
}

// ListObjects —— 遍历本地目录，返回指定 bucket/prefix 下所有文件的信息列表
func (d *LocalDriver) ListObjects(_ context.Context, bucket, prefix string) ([]ObjectInfo, error) {
	root := filepath.Join(d.basePath, bucket, prefix)
	var results []ObjectInfo
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(filepath.Join(d.basePath, bucket), path)
		results = append(results, ObjectInfo{
			Key:          rel,
			Size:         info.Size(),
			LastModified: info.ModTime(),
		})
		return nil
	})
	return results, err
}
