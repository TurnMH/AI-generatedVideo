package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/autovideo/character-service/internal/model"
)

type volcAssetSyncState struct {
	Provider    string
	GroupID     string
	AssetID     string
	Status      string
	AssetType   string
	ProjectName string
	URL         string
	Error       string
	SyncedAt    string
}

func (s *AssetService) SetVolcAssetClient(client *VolcAssetClient) {
	s.volcAsset = client
}

func syncCharacterAssetProviderMetadata(ctx context.Context, client *VolcAssetClient, asset *model.Asset, sourceURL string, metadata map[string]interface{}) (map[string]interface{}, error) {
	if asset == nil {
		return metadata, nil
	}
	if client == nil || !client.Enabled() {
		return metadata, nil
	}
	if !strings.EqualFold(strings.TrimSpace(asset.Type), "character") {
		return metadata, nil
	}
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return metadata, fmt.Errorf("empty source url for provider asset sync")
	}
	groupID := existingVolcAssetGroupID(metadata)
	if groupID == "" {
		groupName := buildVolcAssetGroupName(client.GroupNamePrefix(), asset)
		description := strings.TrimSpace(asset.Description)
		if description == "" {
			description = strings.TrimSpace(asset.Name)
		}
		group, err := client.CreateAssetGroup(ctx, groupName, description)
		if err != nil {
			return metadata, fmt.Errorf("create provider asset group: %w", err)
		}
		groupID = strings.TrimSpace(group.ID)
	}
	created, err := client.CreateAsset(ctx, groupID, sourceURL, "Image")
	if err != nil {
		return metadata, fmt.Errorf("create provider asset: %w", err)
	}
	active, err := client.WaitForAssetActive(ctx, created.ID)
	if err != nil {
		failedState := volcAssetSyncState{
			Provider:    "volcengine",
			GroupID:     groupID,
			AssetID:     strings.TrimSpace(created.ID),
			Status:      strings.TrimSpace(created.Status),
			AssetType:   firstNonEmptyString(created.AssetType, "Image"),
			ProjectName: firstNonEmptyString(created.ProjectName, client.ProjectName()),
			URL:         firstNonEmptyString(created.URL, sourceURL),
			Error:       err.Error(),
			SyncedAt:    time.Now().Format(time.RFC3339),
		}
		return mergeVolcAssetMetadata(metadata, failedState), fmt.Errorf("wait provider asset active: %w", err)
	}
	successState := volcAssetSyncState{
		Provider:    "volcengine",
		GroupID:     firstNonEmptyString(active.GroupID, groupID),
		AssetID:     strings.TrimSpace(active.ID),
		Status:      firstNonEmptyString(active.Status, "Active"),
		AssetType:   firstNonEmptyString(active.AssetType, "Image"),
		ProjectName: firstNonEmptyString(active.ProjectName, client.ProjectName()),
		URL:         firstNonEmptyString(active.URL, sourceURL),
		SyncedAt:    time.Now().Format(time.RFC3339),
	}
	return mergeVolcAssetMetadata(metadata, successState), nil
}

func (s *AssetService) syncCharacterAssetProviderMetadata(ctx context.Context, asset *model.Asset, sourceURL string, metadata map[string]interface{}) (map[string]interface{}, error) {
	return syncCharacterAssetProviderMetadata(ctx, s.volcAsset, asset, sourceURL, metadata)
}

func buildVolcAssetGroupName(prefix string, asset *model.Asset) string {
	prefix = firstNonEmptyString(prefix, "autovideo-character")
	if asset == nil {
		return prefix + "-upload"
	}
	return fmt.Sprintf("%s-p%d-a%d", prefix, asset.ProjectID, asset.ID)
}

func existingVolcAssetGroupID(metadata map[string]interface{}) string {
	for _, key := range []string{"provider_asset_group_id", "volcengine_asset_group_id", "doubao_asset_group_id", "seedance_asset_group_id"} {
		if value := strings.TrimSpace(stringValue(metadata[key])); value != "" {
			return value
		}
	}
	for _, key := range []string{"volc_asset", "provider_asset", "volcengine_private_asset"} {
		raw, ok := metadata[key].(map[string]interface{})
		if !ok {
			continue
		}
		if value := strings.TrimSpace(stringValue(raw["group_id"])); value != "" {
			return value
		}
		if value := strings.TrimSpace(stringValue(raw["provider_asset_group_id"])); value != "" {
			return value
		}
	}
	return ""
}

func mergeVolcAssetMetadata(metadata map[string]interface{}, state volcAssetSyncState) map[string]interface{} {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	nested := map[string]interface{}{
		"provider":     firstNonEmptyString(state.Provider, "volcengine"),
		"group_id":     strings.TrimSpace(state.GroupID),
		"asset_id":     strings.TrimSpace(state.AssetID),
		"status":       strings.TrimSpace(state.Status),
		"asset_type":   strings.TrimSpace(state.AssetType),
		"project_name": strings.TrimSpace(state.ProjectName),
		"url":          strings.TrimSpace(state.URL),
		"uri":          buildProviderAssetURI(state.AssetID),
		"synced_at":    strings.TrimSpace(state.SyncedAt),
		"error":        strings.TrimSpace(state.Error),
	}
	metadata["provider"] = nested["provider"]
	metadata["provider_asset_provider"] = nested["provider"]
	metadata["provider_asset_group_id"] = nested["group_id"]
	metadata["provider_asset_id"] = nested["asset_id"]
	metadata["provider_asset_status"] = nested["status"]
	metadata["provider_asset_type"] = nested["asset_type"]
	metadata["provider_asset_project_name"] = nested["project_name"]
	metadata["provider_asset_url"] = nested["url"]
	metadata["provider_asset_uri"] = nested["uri"]
	metadata["provider_asset_synced_at"] = nested["synced_at"]

	// Compatibility aliases for fengxi/dumps1-style top-level semantics.
	metadata["asset_group_id"] = nested["group_id"]
	metadata["asset_id"] = nested["asset_id"]
	metadata["asset_status"] = nested["status"]
	metadata["asset_type"] = nested["asset_type"]
	metadata["asset_project_name"] = nested["project_name"]
	metadata["asset_url"] = nested["url"]
	metadata["asset_uri"] = nested["uri"]
	metadata["volcengine_asset_group_id"] = nested["group_id"]
	metadata["volcengine_asset_id"] = nested["asset_id"]
	metadata["volcengine_asset_status"] = nested["status"]
	metadata["volcengine_asset_uri"] = nested["uri"]
	metadata["doubao_asset_group_id"] = nested["group_id"]
	metadata["doubao_asset_id"] = nested["asset_id"]
	metadata["doubao_asset_status"] = nested["status"]
	metadata["doubao_asset_uri"] = nested["uri"]
	metadata["seedance_asset_group_id"] = nested["group_id"]
	metadata["seedance_asset_id"] = nested["asset_id"]
	metadata["seedance_asset_status"] = nested["status"]
	metadata["seedance_asset_uri"] = nested["uri"]

	if strings.TrimSpace(state.Error) == "" {
		delete(metadata, "provider_asset_error")
	} else {
		metadata["provider_asset_error"] = strings.TrimSpace(state.Error)
	}
	metadata["volc_asset"] = nested
	metadata["provider_asset"] = nested
	return metadata
}

func buildProviderAssetURI(assetID string) string {
	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return ""
	}
	return "asset://" + assetID
}
