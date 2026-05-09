package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/autovideo/storage-service/internal/driver"
	"github.com/autovideo/storage-service/internal/model"
	"github.com/autovideo/storage-service/internal/service"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// ── Mocks ──────────────────────────────────────────────────────────────────

type stubDriver struct {
	bucket string
	key    string
	body   []byte
	presignURL string
}

func (d *stubDriver) Upload(_ context.Context, bucket, key string, r io.Reader, _ int64, _ string) error {
	d.bucket = bucket
	d.key = key
	d.body, _ = io.ReadAll(r)
	return nil
}
func (d *stubDriver) GetURL(_ context.Context, _, key string, _ time.Duration) (string, error) {
	return "https://cdn.example.com/bucket/" + key, nil
}
func (d *stubDriver) Delete(_ context.Context, _, _ string) error { return nil }
func (d *stubDriver) GetPresignedPutURL(_ context.Context, bucket, key string, _ time.Duration) (string, error) {
	d.bucket = bucket
	d.key = key
	return "https://minio.example.com/" + bucket + "/" + key + "?X-Presign=token", nil
}

func (d *stubDriver) ListObjects(_ context.Context, bucket, _ string) ([]driver.ObjectInfo, error) {
	return nil, nil
}

type stubRepo struct{ last *model.File }

func (r *stubRepo) Create(_ context.Context, f *model.File) error {
	f.ID = 99
	r.last = f
	return nil
}
func (r *stubRepo) GetByObjectKey(_ context.Context, key string) (*model.File, error) {
	if r.last != nil && r.last.ObjectKey == key {
		return r.last, nil
	}
	return nil, nil
}
func (r *stubRepo) ListByUserID(_ context.Context, _ uint64) ([]model.File, error) { return nil, nil }
func (r *stubRepo) DeleteByObjectKey(_ context.Context, _ string) error             { return nil }

// ── Helpers ────────────────────────────────────────────────────────────────

func newRouter(bucketMap map[string]string, includeBucket bool) (*gin.Engine, *stubDriver, *stubRepo) {
	d := &stubDriver{}
	r := &stubRepo{}
	svc := service.NewStorageService(d, r, "https://cdn.example.com", bucketMap, includeBucket)
	h := NewStorageHandler(svc)

	engine := gin.New()
	engine.Use(gin.Recovery())
	api := engine.Group("/api/v1/storage")
	h.RegisterRoutes(api)
	return engine, d, r
}

// buildMultipart builds a multipart/form-data body with the given fields and a file.
func buildMultipart(fields map[string]string, fileContent string) (*bytes.Buffer, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	fw, _ := w.CreateFormFile("file", "test.mp4")
	_, _ = io.WriteString(fw, fileContent)
	_ = w.Close()
	return &buf, w.FormDataContentType()
}

// ── Tests ──────────────────────────────────────────────────────────────────

// TestUploadHandler_Success verifies a normal upload returns 200 with new path format.
func TestUploadHandler_Success(t *testing.T) {
	// includeBucket=false simulates production CDN (no bucket in URL)
	engine, drv, repo := newRouter(nil, false)

	body, ct := buildMultipart(map[string]string{
		"bucket":  "videos",
		"user_id": "1",
	}, "fake-video-bytes")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/upload", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	if drv.bucket != "videos" {
		t.Errorf("driver bucket = %q, want %q", drv.bucket, "videos")
	}
	if string(drv.body) != "fake-video-bytes" {
		t.Errorf("driver body = %q", string(drv.body))
	}
	if repo.last == nil {
		t.Fatal("repo.Create was not called")
	}
	if repo.last.Bucket != "videos" {
		t.Errorf("DB bucket = %q, want %q", repo.last.Bucket, "videos")
	}

	var resp struct {
		Data struct {
			CdnURL    string `json:"cdn_url"`
			ObjectKey string `json:"object_key"`
			FileID    uint64 `json:"file_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal("bad JSON:", err)
	}
	if resp.Data.FileID == 0 {
		t.Error("file_id missing in response")
	}
	// New format: ai-video/YYYY/MM/DD/uuid.ext — no bucket segment in CDN URL
	if !strings.HasPrefix(resp.Data.ObjectKey, "ai-video/") {
		t.Errorf("object_key %q should start with ai-video/", resp.Data.ObjectKey)
	}
	if !strings.Contains(resp.Data.CdnURL, "/ai-video/") {
		t.Errorf("cdn_url %q does not contain /ai-video/", resp.Data.CdnURL)
	}
	if strings.Contains(resp.Data.CdnURL, "/videos/") {
		t.Errorf("production cdn_url %q should NOT contain bucket name", resp.Data.CdnURL)
	}
}

// TestUploadHandler_LocalMinIO verifies bucket IS in CDN URL when cdn_include_bucket=true.
func TestUploadHandler_LocalMinIO(t *testing.T) {
	engine, _, _ := newRouter(nil, true)

	body, ct := buildMultipart(map[string]string{
		"bucket":  "images",
		"user_id": "2",
	}, "img")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/upload", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct{ CdnURL string `json:"cdn_url"` } `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	// Local: https://cdn.example.com/images/ai-video/YYYY/MM/DD/uuid.ext
	if !strings.Contains(resp.Data.CdnURL, "/images/") {
		t.Errorf("local cdn_url %q should contain bucket /images/", resp.Data.CdnURL)
	}
	if !strings.Contains(resp.Data.CdnURL, "/ai-video/") {
		t.Errorf("local cdn_url %q should contain /ai-video/", resp.Data.CdnURL)
	}
}

// TestUploadHandler_BucketMapped verifies TOS single-bucket mapping at the HTTP level.
func TestUploadHandler_BucketMapped(t *testing.T) {
	bm := map[string]string{
		"videos": "jrl", "images": "jrl", "scripts": "jrl",
		"characters": "jrl", "dubbing": "jrl", "audios": "jrl",
	}
	// Production mode: no bucket in CDN URL
	engine, drv, repo := newRouter(bm, false)

	for _, logical := range []string{"videos", "images", "dubbing", "audios", "scripts", "characters"} {
		t.Run(logical+"→jrl", func(t *testing.T) {
			body, ct := buildMultipart(map[string]string{
				"bucket":  logical,
				"user_id": "5",
			}, "data")

			req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/upload", body)
			req.Header.Set("Content-Type", ct)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
			}
			if drv.bucket != "jrl" {
				t.Errorf("driver bucket = %q, want jrl (logical=%s)", drv.bucket, logical)
			}
			if repo.last.Bucket != "jrl" {
				t.Errorf("DB bucket = %q, want jrl (logical=%s)", repo.last.Bucket, logical)
			}

			var resp struct {
				Data struct{ CdnURL string `json:"cdn_url"` } `json:"data"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			// Production: bucket NOT in CDN URL
			if strings.Contains(resp.Data.CdnURL, "/jrl/") {
				t.Errorf("production cdn_url %q should NOT contain /jrl/", resp.Data.CdnURL)
			}
			if !strings.Contains(resp.Data.CdnURL, "/ai-video/") {
				t.Errorf("cdn_url %q should contain /ai-video/", resp.Data.CdnURL)
			}
		})
	}
}

// TestUploadHandler_MissingBucket verifies 400 when bucket field is absent.
func TestUploadHandler_MissingBucket(t *testing.T) {
	engine, _, _ := newRouter(nil, false)
	body, ct := buildMultipart(map[string]string{"user_id": "1"}, "data")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/upload", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUploadHandler_MissingFile verifies 400 when file field is absent.
func TestUploadHandler_MissingFile(t *testing.T) {
	engine, _, _ := newRouter(nil, false)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("bucket", "videos")
	_ = mw.WriteField("user_id", "1")
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPresignHandler_BucketMapped verifies presign endpoint uses bucket mapping.
func TestPresignHandler_BucketMapped(t *testing.T) {
	bm := map[string]string{"images": "jrl"}
	engine, drv, _ := newRouter(bm, false)

	payload := `{"bucket":"images","filename":"photo.png","user_id":3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/presign",
		strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if drv.bucket != "jrl" {
		t.Errorf("presign driver bucket = %q, want jrl", drv.bucket)
	}
	var resp struct {
		Data struct {
			PresignedURL string `json:"presigned_url"`
			ObjectKey    string `json:"object_key"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Data.PresignedURL, "/jrl/") {
		t.Errorf("presigned_url %q does not contain /jrl/", resp.Data.PresignedURL)
	}
}
