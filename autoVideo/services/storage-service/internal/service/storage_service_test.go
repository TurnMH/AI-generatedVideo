package service

import (
	"testing"
)

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
