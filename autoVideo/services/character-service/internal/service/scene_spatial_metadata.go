package service

import (
	"encoding/json"
	"strings"

	"gorm.io/datatypes"
)

const (
	sceneViewExterior = "exterior"
	sceneViewInterior = "interior"
	sceneViewEntrance = "entrance"
	sceneViewAerial   = "aerial"
)

var sceneHubSeparators = []string{"·", "・", "-", "—", "|", "/", "：", ":"}

func parseSceneHubAndZone(name string) (hub, zone string) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ""
	}
	for _, sep := range sceneHubSeparators {
		if idx := strings.Index(trimmed, sep); idx > 0 {
			left := strings.TrimSpace(trimmed[:idx])
			right := strings.TrimSpace(trimmed[idx+len(sep):])
			if left != "" && right != "" {
				return left, right
			}
		}
	}
	return trimmed, ""
}

func inferSceneViewTypeFromZoneLabel(zone string) string {
	z := strings.ToLower(strings.TrimSpace(zone))
	switch {
	case strings.Contains(z, "内") || strings.Contains(z, "indoor") || strings.Contains(z, "interior"):
		return sceneViewInterior
	case strings.Contains(z, "外") || strings.Contains(z, "outdoor") || strings.Contains(z, "exterior") || strings.Contains(z, "street"):
		return sceneViewExterior
	case strings.Contains(z, "门口") || strings.Contains(z, "入口") || strings.Contains(z, "entrance") || strings.Contains(z, "door"):
		return sceneViewEntrance
	case strings.Contains(z, "俯视") || strings.Contains(z, "航拍") || strings.Contains(z, "aerial"):
		return sceneViewAerial
	default:
		return ""
	}
}

func inferSceneViewTypeFromText(name, description string) string {
	combined := strings.ToLower(strings.TrimSpace(name + " " + description))
	if strings.Contains(combined, "门口") || strings.Contains(combined, "入口") || strings.Contains(combined, "entrance") {
		return sceneViewEntrance
	}
	if strings.Contains(combined, "店内") || strings.Contains(combined, "室内") || strings.Contains(combined, "内景") || strings.Contains(combined, "interior") {
		return sceneViewInterior
	}
	if strings.Contains(combined, "店外") || strings.Contains(combined, "室外") || strings.Contains(combined, "外景") || strings.Contains(combined, "exterior") {
		return sceneViewExterior
	}
	return ""
}

// enrichSceneSpatialMetadata fills view_type / location_hub / location_zone for scene assets.
func enrichSceneSpatialMetadata(assetType, name, description string, metadata map[string]interface{}) map[string]interface{} {
	if !isSceneAssetType(assetType) {
		return metadata
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	hub, zone := parseSceneHubAndZone(name)
	if hub == "" {
		hub = strings.TrimSpace(name)
	}
	if _, ok := metadata["location_hub"]; !ok && hub != "" {
		metadata["location_hub"] = hub
	}
	if _, ok := metadata["location_zone"]; !ok && zone != "" {
		metadata["location_zone"] = zone
	}
	if _, ok := metadata["view_type"]; !ok {
		viewType := inferSceneViewTypeFromZoneLabel(zone)
		if viewType == "" {
			viewType = inferSceneViewTypeFromText(name, description)
		}
		if viewType != "" {
			metadata["view_type"] = viewType
		}
	}
	return metadata
}

func mergeSceneSpatialMetadataJSON(raw datatypes.JSON, assetType, name, description string) (datatypes.JSON, error) {
	metadata := map[string]interface{}{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return raw, err
		}
	}
	enriched := enrichSceneSpatialMetadata(assetType, name, description, metadata)
	b, err := json.Marshal(enriched)
	if err != nil {
		return raw, err
	}
	return datatypes.JSON(b), nil
}
