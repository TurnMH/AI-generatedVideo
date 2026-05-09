package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioDriver implements StorageDriver using MinIO.
type MinioDriver struct {
	client     *minio.Client
	endpoint   string
	useSSL     bool
	publicRead bool // when true, each uploaded object gets x-amz-acl: public-read
}

// NewMinioDriver —— 创建 MinIO 存储驱动实例，连接指定端点，返回 *MinioDriver
// publicRead: set true when a CDN serves the bucket so each object is uploaded
// with x-amz-acl=public-read and is immediately accessible without signed URLs.
func NewMinioDriver(endpoint, accessKey, secretKey string, useSSL, pathStyle, publicRead bool) (*MinioDriver, error) {
	lookup := minio.BucketLookupAuto
	if pathStyle {
		lookup = minio.BucketLookupPath
	}
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:       useSSL,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("minio.New: %w", err)
	}
	return &MinioDriver{client: mc, endpoint: endpoint, useSSL: useSSL, publicRead: publicRead}, nil
}

// Upload —— 通过 MinIO 客户端将文件上传到指定 bucket/objectKey。
// 当 publicRead=true 时通过 x-amz-acl:public-read 使对象可被 CDN 无认证访问。
func (d *MinioDriver) Upload(ctx context.Context, bucket, objectKey string, reader io.Reader, size int64, contentType string) error {
	opts := minio.PutObjectOptions{ContentType: contentType}
	if d.publicRead {
		// minio-go v7 passes UserMetadata keys matching isAmzHeader() directly as
		// request headers; x-amz-acl is recognised and forwarded unchanged.
		opts.UserMetadata = map[string]string{"x-amz-acl": "public-read"}
	}
	_, err := d.client.PutObject(ctx, bucket, objectKey, reader, size, opts)
	if err != nil {
		return fmt.Errorf("minio PutObject: %w", err)
	}
	return nil
}

// GetURL —— 生成 MinIO 对象的预签名下载 URL，返回带有效期的访问地址
func (d *MinioDriver) GetURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
	reqParams := make(url.Values)
	u, err := d.client.PresignedGetObject(ctx, bucket, objectKey, expiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("minio PresignedGetObject: %w", err)
	}
	return u.String(), nil
}

// Delete —— 从 MinIO 中删除指定 bucket/objectKey 对应的对象
func (d *MinioDriver) Delete(ctx context.Context, bucket, objectKey string) error {
	err := d.client.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("minio RemoveObject: %w", err)
	}
	return nil
}

// GetPresignedPutURL —— 生成 MinIO 对象的预签名上传 URL
func (d *MinioDriver) GetPresignedPutURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
	u, err := d.client.PresignedPutObject(ctx, bucket, objectKey, expiry)
	if err != nil {
		return "", fmt.Errorf("minio PresignedPutObject: %w", err)
	}
	return u.String(), nil
}

// SetBucketPublicRead —— 在现有 bucket policy 中追加公开只读条目（不覆盖已有策略），
// 使 CDN 可以无认证访问所有对象。已含 s3:GetObject for "*" 的策略不重复添加。
func (d *MinioDriver) SetBucketPublicRead(ctx context.Context, bucket string) error {
	const sid = "PublicReadGetObject"
	resource := fmt.Sprintf("arn:aws-cn:s3:::%s/*", bucket)

	existing, _ := d.client.GetBucketPolicy(ctx, bucket)

	var doc map[string]interface{}
	if existing != "" {
		if err := json.Unmarshal([]byte(existing), &doc); err != nil {
			doc = nil
		}
	}
	if doc == nil {
		doc = map[string]interface{}{"Version": "2012-10-17", "Statement": []interface{}{}}
	}

	// Check if public-read statement already present
	stmts, _ := doc["Statement"].([]interface{})
	for _, s := range stmts {
		sm, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		if sm["Sid"] == sid {
			return nil // already set
		}
	}

	// Append the public-read statement
	newStmt := map[string]interface{}{
		"Sid":       sid,
		"Effect":    "Allow",
		"Principal": map[string]interface{}{"AWS": "*"},
		"Action":    "s3:GetObject",
		"Resource":  resource,
	}
	doc["Statement"] = append(stmts, newStmt)

	policyBytes, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("SetBucketPublicRead marshal: %w", err)
	}
	if err := d.client.SetBucketPolicy(ctx, bucket, string(policyBytes)); err != nil {
		return fmt.Errorf("SetBucketPublicRead %s: %w", bucket, err)
	}
	return nil
}

// ListObjects —— 列出 MinIO 中指定 bucket/prefix 下的所有对象信息
func (d *MinioDriver) ListObjects(ctx context.Context, bucket, prefix string) ([]ObjectInfo, error) {
	var results []ObjectInfo
	for obj := range d.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("minio ListObjects: %w", obj.Err)
		}
		results = append(results, ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			ContentType:  obj.ContentType,
			LastModified: obj.LastModified,
		})
	}
	return results, nil
}
