package service

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/autovideo/storage-service/internal/driver"
	"github.com/autovideo/storage-service/internal/model"
)

type testDriver struct{}

func (testDriver) Upload(context.Context, string, string, io.Reader, int64, string) error {
	return nil
}

func (testDriver) GetURL(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return "https://signed.example.com/object", nil
}

func (testDriver) Delete(context.Context, string, string) error { return nil }

func (testDriver) GetPresignedPutURL(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

func (testDriver) ListObjects(context.Context, string, string) ([]driver.ObjectInfo, error) {
	return nil, nil
}

type testRepo struct {
	file *model.File
}

func (testRepo) Create(context.Context, *model.File) error { return nil }

func (r testRepo) GetByObjectKey(context.Context, string) (*model.File, error) {
	return r.file, nil
}

func (testRepo) ListByUserID(context.Context, uint64) ([]model.File, error) { return nil, nil }

func (testRepo) DeleteByObjectKey(context.Context, string) error { return nil }

// TestResolveBucket verifies logical-to-physical bucket name mapping.
func TestResolveBucket(t *testing.T) {
	cases := []struct {
		name     string
		bmap     map[string]string
		logical  string
		expected string
	}{
		{"nil map → passthrough", nil, "videos", "videos"},
		{"empty value → passthrough", map[string]string{"videos": ""}, "videos", "videos"},
		{"mapped to physical", map[string]string{"videos": "jrl"}, "videos", "jrl"},
		{"unmapped key → passthrough", map[string]string{"images": "jrl"}, "videos", "videos"},
		{"multi-bucket single physical", map[string]string{
			"videos": "jrl", "images": "jrl", "audios": "jrl",
		}, "audios", "jrl"},
		{"scripts mapped", map[string]string{"scripts": "jrl"}, "scripts", "jrl"},
		{"dubbing mapped", map[string]string{"dubbing": "jrl"}, "dubbing", "jrl"},
		{"characters mapped", map[string]string{"characters": "jrl"}, "characters", "jrl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bm := map[string]string{}
			if tc.bmap != nil {
				bm = tc.bmap
			}
			svc := &StorageService{bucketMap: bm}
			got := svc.resolveBucket(tc.logical)
			if got != tc.expected {
				t.Errorf("resolveBucket(%q) = %q, want %q", tc.logical, got, tc.expected)
			}
		})
	}
}

// TestNewStorageServiceBucketMap verifies nil map is initialised safely.
func TestNewStorageServiceBucketMap(t *testing.T) {
	svc := NewStorageService(nil, nil, "https://cdn.example.com", nil, false)
	if svc.bucketMap == nil {
		t.Fatal("bucketMap should not be nil after NewStorageService with nil map")
	}
	// passthrough when no mapping
	if got := svc.resolveBucket("videos"); got != "videos" {
		t.Errorf("expected passthrough, got %q", got)
	}
}

// TestAllLogicalBucketsCovered verifies the 8 known logical bucket names can all
// be resolved (passthrough or mapped) without panic.
func TestAllLogicalBucketsCovered(t *testing.T) {
	// TOS single-bucket scenario: all logical names → "jrl"
	bm := map[string]string{
		"images": "jrl", "videos": "jrl", "scripts": "jrl",
		"characters": "jrl", "uploads": "jrl", "exports": "jrl",
		"dubbing": "jrl", "audios": "jrl",
	}
	svc := &StorageService{bucketMap: bm}
	for logical := range bm {
		if got := svc.resolveBucket(logical); got != "jrl" {
			t.Errorf("bucket %q → %q, want jrl", logical, got)
		}
	}
}

func TestGetURLPrefersCDNURL(t *testing.T) {
	svc := NewStorageService(testDriver{}, testRepo{file: &model.File{
		Bucket:    "j1-common-bucket",
		ObjectKey: "ai-video/2026/05/12/example.png",
		CdnURL:    "https://cdn.example.com/ai-video/2026/05/12/example.png",
	}}, "https://cdn.example.com", nil, false)

	got, err := svc.GetURL(context.Background(), "ai-video/2026/05/12/example.png", time.Minute)
	if err != nil {
		t.Fatalf("GetURL returned error: %v", err)
	}
	if got != "https://cdn.example.com/ai-video/2026/05/12/example.png" {
		t.Fatalf("GetURL = %q, want CDN URL", got)
	}
}
