package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/autovideo/video-service/internal/model"
	"github.com/autovideo/video-service/internal/repository"
	"github.com/autovideo/video-service/internal/service/generators"
)

type testVideoGenerator struct{}

func (testVideoGenerator) Name() string { return "test" }
func (testVideoGenerator) Generate(context.Context, generators.VideoGenerateReq) (*generators.VideoClip, error) {
	return nil, nil
}
func (testVideoGenerator) IsAvailable(context.Context) bool { return true }
func (testVideoGenerator) SupportsNativeAudio() bool       { return false }
func (testVideoGenerator) ParamOptions() []generators.ModelParamOption {
	return nil
}

func TestPerClipCharacterAssetReferenceImages(t *testing.T) {
	sceneAssetIDs := [][]int64{{11, 12, 13}}
	anchors := map[int64]videoAssetPromptAnchor{
		11: {ID: 11, Type: "character", ImageURL: "https://example.com/char-main.png"},
		12: {ID: 12, Type: "scene", ImageURL: "https://example.com/scene.png"},
		13: {ID: 13, Type: "character", ImageURL: "https://example.com/char-side.png"},
	}
	got := perClipCharacterAssetReferenceImages(sceneAssetIDs, anchors, 0)
	if len(got) != 2 {
		t.Fatalf("len(got)=%d, want 2", len(got))
	}
	if got[0] != "https://example.com/char-main.png" {
		t.Fatalf("got[0]=%q", got[0])
	}
	if got[1] != "https://example.com/char-side.png" {
		t.Fatalf("got[1]=%q", got[1])
	}
}

func TestMergeReferenceURLsKeepsCharacterAssetRefsFirst(t *testing.T) {
	assetRefs := []string{"https://example.com/char-main.png"}
	nameRefs := []string{"https://example.com/name-match.png", "https://example.com/char-main.png"}
	got := mergeReferenceURLs(assetRefs, nameRefs)
	if len(got) != 2 {
		t.Fatalf("len(got)=%d, want 2", len(got))
	}
	if got[0] != "https://example.com/char-main.png" {
		t.Fatalf("got[0]=%q, want char asset ref first", got[0])
	}
	if got[1] != "https://example.com/name-match.png" {
		t.Fatalf("got[1]=%q", got[1])
	}
}

func TestNormalizeVideoGenerateReqDoesNotMixSceneAssetRefsIntoCharacterRefs(t *testing.T) {
	req := generators.VideoGenerateReq{
		CharacterImageURLs: []string{"https://example.com/identity-anchor.png"},
	}
	got := normalizeVideoGenerateReq(testVideoGenerator{}, "vidu", req, []string{"https://example.com/scene-ref.png"})
	if len(got.CharacterImageURLs) != 1 {
		t.Fatalf("len(got.CharacterImageURLs)=%d, want 1", len(got.CharacterImageURLs))
	}
	if got.CharacterImageURLs[0] != "https://example.com/identity-anchor.png" {
		t.Fatalf("got character ref %q, want identity anchor only", got.CharacterImageURLs[0])
	}
}

func TestIdentityAnchorReferencesPreferProjectLevelAnchor(t *testing.T) {
	rc := model.RenderConfig{
		"character_anchor_image_url":     "https://example.com/project-anchor.png",
		"approved_first_frame_image_url": "https://example.com/approved-first-frame.png",
		"start_image_url":                "https://example.com/start-image.png",
	}
	got := identityAnchorReferences(rc)
	if len(got) != 3 {
		t.Fatalf("len(got)=%d, want 3", len(got))
	}
	if got[0] != "https://example.com/project-anchor.png" {
		t.Fatalf("got[0]=%q, want project anchor first", got[0])
	}
	if got[1] != "https://example.com/approved-first-frame.png" {
		t.Fatalf("got[1]=%q", got[1])
	}
	if got[2] != "https://example.com/start-image.png" {
		t.Fatalf("got[2]=%q", got[2])
	}
}

func TestFetchAssetPromptAnchorsUsesServiceJWT(t *testing.T) {
	var gotAuth string
	var gotInternal string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotInternal = r.Header.Get("X-Internal-Service")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{
				"id":        35653,
				"name":      "李恩泽",
				"type":      "character",
				"image_url": "https://example.com/char.png",
			}},
		})
	}))
	defer server.Close()

	svc := NewVideoService(nil, nil, nil, "", server.URL, "test-secret", nil, 1, 1)
	anchors := svc.fetchAssetPromptAnchors(context.Background(), 172, []int64{35653})
	if gotAuth == "" {
		t.Fatalf("expected Authorization header")
	}
	if gotInternal != "video-service" {
		t.Fatalf("got internal service header %q", gotInternal)
	}
	anchor, ok := anchors[35653]
	if !ok {
		t.Fatalf("expected anchor for 35653")
	}
	if anchor.ImageURL != "https://example.com/char.png" {
		t.Fatalf("anchor.ImageURL=%q", anchor.ImageURL)
	}
}

var _ repository.VideoTaskRepo
