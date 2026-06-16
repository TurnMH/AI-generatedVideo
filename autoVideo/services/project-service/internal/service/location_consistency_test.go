package service

import "testing"

func TestInferLocationViewType_InteriorFromDescription(t *testing.T) {
	got := InferLocationViewType("王大发跪在店内堂中，向刘师傅苦苦哀求。", "刘师傅包子铺", "medium", "")
	if got != LocationViewInterior {
		t.Fatalf("expected interior, got %q", got)
	}
}

func TestInferLocationViewType_ExteriorFromEstablishing(t *testing.T) {
	got := InferLocationViewType("包子铺门面在街角，招牌醒目。", "刘师傅包子铺", "establishing", "")
	if got != LocationViewExterior {
		t.Fatalf("expected exterior, got %q", got)
	}
}

func TestParseLocationHubAndZone(t *testing.T) {
	hub, zone := ParseLocationHubAndZone("刘师傅包子铺·内景")
	if hub != "刘师傅包子铺" || zone != "内景" {
		t.Fatalf("unexpected hub/zone: %q / %q", hub, zone)
	}
}

func TestPickStoryboardSceneReference_SkipsMismatchedExterior(t *testing.T) {
	entries := []sceneAssetEntry{{
		Name:     "刘师傅包子铺",
		Hub:      "刘师傅包子铺",
		ViewType: LocationViewExterior,
		ImageURL: "https://example.com/outside.png",
	}}
	pick := pickStoryboardSceneReference(LocationViewInterior, "刘师傅包子铺", map[string]string{
		"刘师傅包子铺": "https://example.com/outside.png",
	}, entries)
	if !pick.Skipped || pick.Matched {
		t.Fatalf("expected skipped mismatch, got %+v", pick)
	}
}

func TestPickStoryboardSceneReference_MatchesInteriorAsset(t *testing.T) {
	entries := []sceneAssetEntry{{
		Name:     "刘师傅包子铺·内景",
		Hub:      "刘师傅包子铺",
		Zone:     "内景",
		ViewType: LocationViewInterior,
		ImageURL: "https://example.com/inside.png",
	}}
	pick := pickStoryboardSceneReference(LocationViewInterior, "刘师傅包子铺", map[string]string{
		"刘师傅包子铺·内景": "https://example.com/inside.png",
	}, entries)
	if !pick.Matched || pick.URL == "" {
		t.Fatalf("expected interior match, got %+v", pick)
	}
}
