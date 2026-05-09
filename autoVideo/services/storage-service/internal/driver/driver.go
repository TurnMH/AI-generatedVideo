package driver

import (
	"context"
	"io"
	"time"
)

// StorageDriver abstracts the underlying object storage backend.
type StorageDriver interface {
	Upload(ctx context.Context, bucket, objectKey string, reader io.Reader, size int64, contentType string) error
	GetURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, bucket, objectKey string) error
	GetPresignedPutURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
	ListObjects(ctx context.Context, bucket, prefix string) ([]ObjectInfo, error)
}

type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	LastModified time.Time
}
