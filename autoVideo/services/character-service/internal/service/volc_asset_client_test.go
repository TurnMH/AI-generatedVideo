package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/autovideo/character-service/internal/model"
	"go.uber.org/zap"
)

func TestMergeVolcAssetMetadata(t *testing.T) {
	metadata := map[string]interface{}{
		"selected_generated_image_url": "https://cdn.example.com/source.png",
	}
	merged := mergeVolcAssetMetadata(metadata, volcAssetSyncState{
		Provider:    "volcengine",
		GroupID:     "group-123",
		AssetID:     "asset-456",
		Status:      "Active",
		AssetType:   "Image",
		ProjectName: "default",
		URL:         "https://asset.example.com/final.png",
		SyncedAt:    "2026-06-01T09:00:00Z",
	})

	if got := strings.TrimSpace(stringValue(merged["provider_asset_id"])); got != "asset-456" {
		t.Fatalf("provider_asset_id = %q, want asset-456", got)
	}
	if got := strings.TrimSpace(stringValue(merged["provider_asset_status"])); got != "Active" {
		t.Fatalf("provider_asset_status = %q, want Active", got)
	}
	if got := strings.TrimSpace(stringValue(merged["asset_id"])); got != "asset-456" {
		t.Fatalf("asset_id = %q, want asset-456", got)
	}
	if got := strings.TrimSpace(stringValue(merged["asset_status"])); got != "Active" {
		t.Fatalf("asset_status = %q, want Active", got)
	}
	if got := strings.TrimSpace(stringValue(merged["seedance_asset_uri"])); got != "asset://asset-456" {
		t.Fatalf("seedance_asset_uri = %q, want asset://asset-456", got)
	}
	nested, ok := merged["volc_asset"].(map[string]interface{})
	if !ok {
		t.Fatalf("volc_asset missing or invalid: %#v", merged["volc_asset"])
	}
	if got := strings.TrimSpace(stringValue(nested["group_id"])); got != "group-123" {
		t.Fatalf("nested group_id = %q, want group-123", got)
	}
	if got := strings.TrimSpace(stringValue(nested["uri"])); got != "asset://asset-456" {
		t.Fatalf("nested uri = %q, want asset://asset-456", got)
	}
}

func TestVolcAssetClientCreateAndPoll(t *testing.T) {
	var getAssetCalls int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("Action")
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "HMAC-SHA256 Credential=test-ak/") {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		if got := r.Header.Get("X-Date"); got == "" {
			t.Fatal("missing X-Date header")
		}
		_ = json.NewDecoder(r.Body).Decode(&map[string]interface{}{})
		w.Header().Set("Content-Type", "application/json")
		switch action {
		case "CreateAssetGroup":
			_, _ = w.Write([]byte(`{"ResponseMetadata":{"RequestId":"req-group"},"Result":{"Id":"group-123","Status":"Active","ProjectName":"default"}}`))
		case "CreateAsset":
			_, _ = w.Write([]byte(`{"ResponseMetadata":{"RequestId":"req-create"},"Result":{"Id":"asset-456","GroupId":"group-123","Status":"Processing","ProjectName":"default","AssetType":"Image","URL":"https://cdn.example.com/input.png"}}`))
		case "GetAsset":
			call := atomic.AddInt32(&getAssetCalls, 1)
			if call < 2 {
				_, _ = w.Write([]byte(`{"ResponseMetadata":{"RequestId":"req-get-1"},"Result":{"Id":"asset-456","GroupId":"group-123","Status":"Processing","ProjectName":"default","AssetType":"Image","URL":"https://cdn.example.com/input.png"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"ResponseMetadata":{"RequestId":"req-get-2"},"Result":{"Id":"asset-456","GroupId":"group-123","Status":"Active","ProjectName":"default","AssetType":"Image","URL":"https://cdn.example.com/final.png"}}`))
		default:
			t.Fatalf("unexpected action: %s", action)
		}
	}))
	defer server.Close()

	client := NewVolcAssetClient(VolcAssetClientConfig{
		Enabled:         true,
		AccessKey:       "test-ak",
		SecretKey:       "test-sk",
		Host:            strings.TrimPrefix(server.URL, "https://"),
		ProjectName:     "default",
		PollInterval:    5 * time.Millisecond,
		PollTimeout:     time.Second,
		HTTPClient:      server.Client(),
		GroupNamePrefix: "autovideo-character",
	}, zap.NewNop())
	if client == nil {
		t.Fatal("expected volc asset client")
	}

	group, err := client.CreateAssetGroup(context.Background(), "demo-group", "demo")
	if err != nil {
		t.Fatalf("CreateAssetGroup() error = %v", err)
	}
	if group.ID != "group-123" {
		t.Fatalf("group.ID = %q, want group-123", group.ID)
	}
	created, err := client.CreateAsset(context.Background(), group.ID, "https://cdn.example.com/input.png", "Image")
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	if created.ID != "asset-456" {
		t.Fatalf("created.ID = %q, want asset-456", created.ID)
	}
	active, err := client.WaitForAssetActive(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("WaitForAssetActive() error = %v", err)
	}
	if active.Status != "Active" {
		t.Fatalf("active.Status = %q, want Active", active.Status)
	}
}

func TestBuildVolcAssetGroupName(t *testing.T) {
	name := buildVolcAssetGroupName("autovideo-character", &model.Asset{ID: 35825, ProjectID: 176})
	if name != "autovideo-character-p176-a35825" {
		t.Fatalf("group name = %q", name)
	}
}
