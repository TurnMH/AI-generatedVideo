package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
)

// StorageClient 调用 storage-service 上传文件
type StorageClient struct {
	baseURL string
}

// NewStorageClient —— 创建存储客户端实例，返回 *StorageClient
func NewStorageClient(baseURL string) *StorageClient {
	return &StorageClient{baseURL: baseURL}
}

type storageResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		CDNURL   string `json:"cdn_url"`
		FileSize int64  `json:"file_size"`
	} `json:"data"`
}

// Upload 把文件内容上传到 storage-service，返回 cdn_url
func (c *StorageClient) Upload(filename string, content io.Reader) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err = io.Copy(part, content); err != nil {
		return "", fmt.Errorf("copy file: %w", err)
	}
	_ = writer.WriteField("bucket", "characters")
	_ = writer.WriteField("user_id", "0")
	writer.Close()

	url := c.baseURL + "/api/v1/storage/upload"
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	var sr storageResponse
	if err = json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	// storage-service 成功时返回 code=0，错误时返回 code=HTTP状态码
	if sr.Code != 0 && sr.Code != 200 {
		return "", fmt.Errorf("storage-service error: %s", sr.Message)
	}
	return sr.Data.CDNURL, nil
}
