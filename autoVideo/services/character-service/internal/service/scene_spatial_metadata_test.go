package service

import "testing"

func TestEnrichSceneSpatialMetadataFromName(t *testing.T) {
	meta := enrichSceneSpatialMetadata("scene", "刘师傅包子铺·内景", "店内蒸汽弥漫", map[string]interface{}{})
	if meta["location_hub"] != "刘师傅包子铺" {
		t.Fatalf("hub = %v", meta["location_hub"])
	}
	if meta["view_type"] != sceneViewInterior {
		t.Fatalf("view_type = %v", meta["view_type"])
	}
}

func TestEnrichSceneSpatialMetadataPreservesExisting(t *testing.T) {
	meta := enrichSceneSpatialMetadata("scene", "刘师傅包子铺", "", map[string]interface{}{
		"view_type": "exterior",
	})
	if meta["view_type"] != "exterior" {
		t.Fatalf("expected existing view_type preserved, got %v", meta["view_type"])
	}
}
