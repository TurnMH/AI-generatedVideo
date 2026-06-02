package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
func (testVideoGenerator) SupportsNativeAudio() bool        { return false }
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

func TestIdentityAnchorReferencesPreferApprovedFirstFrameWhenPresent(t *testing.T) {
	rc := model.RenderConfig{
		"character_anchor_image_url":     "https://example.com/project-anchor.png",
		"approved_first_frame_image_url": "https://example.com/approved-first-frame.png",
		"start_image_url":                "https://example.com/start-image.png",
	}
	got := identityAnchorReferences(rc)
	if len(got) != 3 {
		t.Fatalf("len(got)=%d, want 3", len(got))
	}
	if got[0] != "https://example.com/approved-first-frame.png" {
		t.Fatalf("got[0]=%q, want approved first frame first", got[0])
	}
	if got[1] != "https://example.com/project-anchor.png" {
		t.Fatalf("got[1]=%q", got[1])
	}
	if got[2] != "https://example.com/start-image.png" {
		t.Fatalf("got[2]=%q", got[2])
	}
}

func TestPreferredIdentitySourceImageURLUsesApprovedFirstFrameFirst(t *testing.T) {
	rc := model.RenderConfig{
		"character_anchor_image_url":     "https://example.com/project-anchor.png",
		"approved_first_frame_image_url": "https://example.com/approved-first-frame.png",
	}
	if got := preferredIdentitySourceImageURL(rc); got != "https://example.com/approved-first-frame.png" {
		t.Fatalf("got %q", got)
	}
}

func TestIdentityCharacterReferencesPreferActiveProviderAssetRef(t *testing.T) {
	rc := model.RenderConfig{
		"character_anchor_asset_id":      11,
		"character_anchor_image_url":     "https://example.com/project-anchor.png",
		"approved_first_frame_image_url": "https://example.com/approved-first-frame.png",
	}
	anchors := map[int64]videoAssetPromptAnchor{
		11: {
			ID:                  11,
			Type:                "character",
			ImageURL:            "https://example.com/project-anchor.png",
			Provider:            "volcengine",
			ProviderAssetID:     "asset-123",
			ProviderAssetStatus: "Active",
		},
	}
	got := identityCharacterReferences(rc, anchors)
	if len(got) != 3 {
		t.Fatalf("len(got)=%d, want 3", len(got))
	}
	if got[0] != "asset://asset-123" {
		t.Fatalf("got[0]=%q, want asset ref first", got[0])
	}
	if got[1] != "https://example.com/approved-first-frame.png" {
		t.Fatalf("got[1]=%q", got[1])
	}
}

func TestBuildReferenceImageBindingsKeepsIdentityAnchorFirst(t *testing.T) {
	rc := model.RenderConfig{
		"character_anchor_asset_id":      11,
		"character_anchor_image_url":     "https://example.com/project-anchor.png",
		"approved_first_frame_image_url": "https://example.com/approved-first-frame.png",
	}
	anchors := map[int64]videoAssetPromptAnchor{
		11: {
			ID:                  11,
			Type:                "character",
			Name:                "林夏",
			ImageURL:            "https://example.com/project-anchor.png",
			Provider:            "seedance",
			ProviderAssetURI:    "asset://asset-123",
			ProviderAssetStatus: "Active",
		},
		12: {
			ID:       12,
			Type:     "character",
			Name:     "阿杰",
			ImageURL: "https://example.com/ajie.png",
		},
	}
	got := buildReferenceImageBindings(rc, anchors, [][]int64{{11, 12}}, 0, []string{"林夏", "阿杰"}, map[string]string{"林夏": "https://example.com/linxia-sheet.png", "阿杰": "https://example.com/ajie.png"})
	if len(got) < 3 {
		t.Fatalf("len(got)=%d, want at least 3", len(got))
	}
	if got[0].URL != "asset://asset-123" || got[0].Label != "林夏" {
		t.Fatalf("got[0]=%+v", got[0])
	}
	if got[1].URL != "https://example.com/approved-first-frame.png" {
		t.Fatalf("got[1]=%+v", got[1])
	}
}

func TestBuildCharacterAndAudioMediaReferencesFromAnchors(t *testing.T) {
	rc := model.RenderConfig{"character_anchor_asset_id": 11}
	anchors := map[int64]videoAssetPromptAnchor{
		11: {ID: 11, Type: "character", Name: "林夏", ProviderAssetURI: "asset://asset-123", ProviderAssetStatus: "Active", ReferenceAudioURL: "https://example.com/linxia.wav"},
		12: {ID: 12, Type: "character", Name: "阿杰", ImageURL: "https://example.com/ajie.png", ReferenceAudioURL: "https://example.com/ajie.wav"},
	}
	bindings := buildReferenceImageBindings(rc, anchors, [][]int64{{11, 12}}, 0, []string{"林夏", "阿杰"}, map[string]string{"林夏": "https://example.com/linxia.png", "阿杰": "https://example.com/ajie.png"})
	charRefs := buildCharacterMediaReferences(bindings)
	if len(charRefs) < 2 {
		t.Fatalf("len(charRefs)=%d, want at least 2", len(charRefs))
	}
	if charRefs[0].Text != "林夏" || charRefs[0].Index != 1 || charRefs[0].URL != "asset://asset-123" {
		t.Fatalf("charRefs[0]=%+v", charRefs[0])
	}
	audioRefs := buildAudioMediaReferences(charRefs, rc, anchors, [][]int64{{11, 12}}, 0, []string{"林夏", "阿杰"})
	if len(audioRefs) != 2 {
		t.Fatalf("len(audioRefs)=%d, want 2", len(audioRefs))
	}
	if audioRefs[0].Text != "林夏" || audioRefs[0].URL != "https://example.com/linxia.wav" {
		t.Fatalf("audioRefs[0]=%+v", audioRefs[0])
	}
}

func TestAppendReferenceImageBindingHintForDoubao(t *testing.T) {
	prompt := appendReferenceImageBindingHint("主讲人口播产品卖点", []referenceImageBinding{
		{Label: "林夏", URL: "asset://asset-123", Note: "主角色身份锚点"},
		{Label: "阿杰", URL: "https://example.com/ajie.png", Note: "角色参考图"},
	}, "doubao-seedance")
	if !strings.Contains(prompt, "@图1=林夏") {
		t.Fatalf("prompt=%q", prompt)
	}
	if !strings.Contains(prompt, "@图2=阿杰") {
		t.Fatalf("prompt=%q", prompt)
	}
}

func TestAppendReferenceImageBindingHintReplacesCharacterNamesInsidePrompt(t *testing.T) {
	prompt := appendReferenceImageBindingHint("林夏走向镜头，阿杰在后景看向林夏。", []referenceImageBinding{
		{Label: "林夏", URL: "asset://asset-123", Note: "主角色身份锚点"},
		{Label: "阿杰", URL: "https://example.com/ajie.png", Note: "角色参考图"},
	}, "doubao-seedance")
	if !strings.Contains(prompt, "林夏(@图1)走向镜头") {
		t.Fatalf("prompt=%q", prompt)
	}
	if !strings.Contains(prompt, "阿杰(@图2)在后景") {
		t.Fatalf("prompt=%q", prompt)
	}
	if strings.Contains(prompt, "【参考图绑定】") {
		t.Fatalf("prompt should prefer inline replacement once @图 markers are present: %q", prompt)
	}
}

func TestBindReferenceImageMentionsPrefersLongerNamesFirst(t *testing.T) {
	got := bindReferenceImageMentions("小林夏站在林夏身后。", []referenceImageBinding{
		{Label: "林夏", URL: "asset://asset-1"},
		{Label: "小林夏", URL: "asset://asset-2"},
	})
	if !strings.Contains(got, "小林夏(@图2)站在林夏(@图1)身后") {
		t.Fatalf("got=%q", got)
	}
}

func TestValidateSameCharacterBindingsRequiresRefsForDoubao(t *testing.T) {
	err := validateSameCharacterBindings(model.RenderConfig{"require_same_character": true}, "doubao-seedance", generators.VideoGenerateReq{}, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "same-character preflight failed") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidateSameCharacterBindingsAllowsBoundRefsForDoubao(t *testing.T) {
	err := validateSameCharacterBindings(model.RenderConfig{"require_same_character": true}, "doubao-seedance", generators.VideoGenerateReq{}, []referenceImageBinding{{Label: "林夏", URL: "asset://asset-123"}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestValidateSameCharacterBindingsSkipsNonDoubaoFamilies(t *testing.T) {
	err := validateSameCharacterBindings(model.RenderConfig{"require_same_character": true}, "vidu", generators.VideoGenerateReq{}, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestFormatReferenceBindingsIncludesSlotAndURL(t *testing.T) {
	got := formatReferenceBindings([]referenceImageBinding{
		{Label: "林夏", URL: "asset://asset-123", Note: "主角色身份锚点"},
		{Label: "阿杰", URL: "https://example.com/ajie.png", Note: "角色参考图"},
	})
	if len(got) != 2 {
		t.Fatalf("len(got)=%d", len(got))
	}
	if got[0] != "@图1=林夏(主角色身份锚点) -> asset://asset-123" {
		t.Fatalf("got[0]=%q", got[0])
	}
	if got[1] != "@图2=阿杰(角色参考图) -> https://example.com/ajie.png" {
		t.Fatalf("got[1]=%q", got[1])
	}
}

func TestBuildClipIdentityTraceCarriesReferenceBindings(t *testing.T) {
	trace := buildClipIdentityTrace("doubao-seedance", "", generators.VideoGenerateReq{SourceImageURL: "https://example.com/source.png"}, false, []string{"asset://asset-123"}, []string{"asset://asset-123", "https://example.com/ajie.png"}, nil, []referenceImageBinding{
		{Label: "林夏", URL: "asset://asset-123", Note: "主角色身份锚点"},
		{Label: "阿杰", URL: "https://example.com/ajie.png", Note: "角色参考图"},
	}, false, false)
	if len(trace.ReferenceBindings) != 2 {
		t.Fatalf("len(trace.ReferenceBindings)=%d", len(trace.ReferenceBindings))
	}
	if trace.ReferenceBindings[0] != "@图1=林夏(主角色身份锚点) -> asset://asset-123" {
		t.Fatalf("trace.ReferenceBindings[0]=%q", trace.ReferenceBindings[0])
	}
}

func TestAppendReferenceImageBindingHintSkipsNonDoubaoFamily(t *testing.T) {
	prompt := appendReferenceImageBindingHint("plain prompt", []referenceImageBinding{{Label: "林夏", URL: "asset://asset-123"}}, "vidu")
	if prompt != "plain prompt" {
		t.Fatalf("prompt=%q", prompt)
	}
}

func TestReferenceURLForVideoAnchorAcceptsProviderAssetURICompatibilityAlias(t *testing.T) {
	anchor := videoAssetPromptAnchor{
		Provider:            "seedance",
		ProviderAssetStatus: "Active",
		ProviderAssetURI:    "asset://asset-789",
	}
	if got := referenceURLForVideoAnchor(anchor); got != "asset://asset-789" {
		t.Fatalf("got %q, want asset://asset-789", got)
	}
}

func TestPerClipCharacterAssetReferenceImagesPreferActiveProviderAssetRef(t *testing.T) {
	sceneAssetIDs := [][]int64{{11}}
	anchors := map[int64]videoAssetPromptAnchor{
		11: {
			ID:                  11,
			Type:                "character",
			ImageURL:            "https://example.com/char-main.png",
			Provider:            "doubao-seedance",
			ProviderAssetID:     "asset-456",
			ProviderAssetStatus: "Active",
		},
	}
	got := perClipCharacterAssetReferenceImages(sceneAssetIDs, anchors, 0)
	if len(got) != 1 {
		t.Fatalf("len(got)=%d, want 1", len(got))
	}
	if got[0] != "asset://asset-456" {
		t.Fatalf("got[0]=%q", got[0])
	}
}

func TestShouldUseSeedanceIdentityAnchorSourceFallbackAfterSecondLiveActionClip(t *testing.T) {
	task := &model.VideoTask{
		SerialScene: true,
		StylePreset: "live-action-short",
		RenderConfig: model.RenderConfig{
			"require_same_character":         true,
			"approved_first_frame_image_url": "https://example.com/approved-first-frame.png",
		},
	}
	if !shouldUseSeedanceIdentityAnchorSourceFallback("doubao-seedance", task, 2, "group-a", "") {
		t.Fatalf("expected Seedance live-action clip 3+ to fall back to identity anchor source")
	}
	if shouldUseSeedanceIdentityAnchorSourceFallback("doubao-seedance", task, 1, "group-a", "") {
		t.Fatalf("did not expect clip 2 to use identity anchor source fallback")
	}
	if shouldUseSeedanceIdentityAnchorSourceFallback("doubao", task, 2, "group-a", "") {
		t.Fatalf("did not expect plain doubao route to change here")
	}
}

func TestShouldChainSerialSourceWeakensLiveActionSameCharacterAfterSecondClip(t *testing.T) {
	task := &model.VideoTask{
		SerialScene: true,
		StylePreset: "live-action-short",
		RenderConfig: model.RenderConfig{
			"require_same_character": true,
		},
	}
	if !shouldChainSerialSource(task, 1, "group-a") {
		t.Fatalf("expected clip 2 in live-action same-character chain to keep serial source")
	}
	if shouldChainSerialSource(task, 2, "group-a") {
		t.Fatalf("expected clip 3+ in live-action same-character chain to stop inheriting previous tail frame")
	}
}

func TestShouldChainSerialSourceKeepsNonLiveActionBehavior(t *testing.T) {
	task := &model.VideoTask{
		SerialScene: true,
		StylePreset: "anime-2d",
		RenderConfig: model.RenderConfig{
			"require_same_character": true,
		},
	}
	if !shouldChainSerialSource(task, 2, "group-a") {
		t.Fatalf("expected non-live-action chain policy to remain unchanged")
	}
}

func TestShouldAddSerialContinuityPromptStopsAfterSecondLiveActionSameCharacterClip(t *testing.T) {
	task := &model.VideoTask{
		SerialScene: true,
		StylePreset: "live-action-short",
		RenderConfig: model.RenderConfig{
			"require_same_character": true,
		},
	}
	if !shouldAddSerialContinuityPrompt(task, 1, 1, "group-a", []string{"", "主讲人继续自然口播产品卖点"}, nil) {
		t.Fatalf("expected clip 2 continuity prompt to remain enabled when chain is active")
	}
	if shouldAddSerialContinuityPrompt(task, 2, 2, "group-a", []string{"", "", "主讲人继续自然口播产品卖点"}, nil) {
		t.Fatalf("expected clip 3+ continuity prompt to stop when live-action same-character chain is disabled")
	}
}

func TestShouldPreferStartEndIdentityModeForDoubaoSameCharacter(t *testing.T) {
	rc := model.RenderConfig{
		"require_same_character":         true,
		"approved_first_frame_image_url": "https://example.com/approved-first-frame.png",
	}
	req := generators.VideoGenerateReq{
		SourceImageURL:     "https://example.com/current-first.png",
		TailImageURL:       "https://example.com/current-last.png",
		CharacterImageURLs: []string{"https://example.com/identity-anchor.png"},
	}
	if !shouldPreferStartEndIdentityMode("doubao-seedance", rc, req) {
		t.Fatalf("expected doubao same-character clip to prefer startEnd2video")
	}
	if shouldPreferStartEndIdentityMode("vidu", rc, req) {
		t.Fatalf("did not expect non-doubao model to force startEnd2video here")
	}
}

func TestNormalizeVideoGenerateReqKeepsImg2VideoForDoubaoWhenSourcePresent(t *testing.T) {
	req := generators.VideoGenerateReq{
		SourceImageURL:     "https://example.com/current-first.png",
		CharacterImageURLs: []string{"https://example.com/identity-anchor.png"},
		GenerateMode:       "",
	}
	got := normalizeVideoGenerateReq(testVideoGenerator{}, "doubao-seedance", req, nil)
	if got.GenerateMode != "" {
		t.Fatalf("got.GenerateMode=%q, want empty so downstream doubao img2video path can carry source + reference_image", got.GenerateMode)
	}
	if len(got.CharacterImageURLs) != 1 || got.CharacterImageURLs[0] != "https://example.com/identity-anchor.png" {
		t.Fatalf("got.CharacterImageURLs=%v", got.CharacterImageURLs)
	}
}

func TestClipMotionPromptChineseFamilyForDoubaoIncludesReferenceSections(t *testing.T) {
	prompt := clipMotionPromptChineseFamily(1, 3, "人物从门口快步走向镜头", "cinematic", "live-action-short", "doubao", nil, "")
	for _, want := range []string{
		"参考人物参考图中的同一主体生成当前镜头",
		"参考上一镜头中的动作衔接、运镜方向与节奏",
		"参考场景与道具素材中的环境氛围和空间关系",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt %q does not contain %q", prompt, want)
		}
	}
}

func TestBuildClipIdentityTraceSummarizesSources(t *testing.T) {
	trace := buildClipIdentityTrace(
		"doubao-seedance",
		"startEnd2video",
		generators.VideoGenerateReq{
			GenerateMode:       "startEnd2video",
			SourceImageURL:     "https://example.com/source.png",
			TailImageURL:       "https://example.com/tail.png",
			CharacterImageURLs: []string{"https://example.com/char-a.png", "https://example.com/char-b.png"},
		},
		true,
		[]string{"https://example.com/project-anchor.png", "https://example.com/approved-first.png"},
		[]string{"https://example.com/char-a.png", "https://example.com/char-b.png"},
		[]string{"https://example.com/scene-ref.png"},
		[]referenceImageBinding{{Label: "林夏", URL: "asset://asset-123", Note: "主角色身份锚点"}},
		true,
		true,
	)
	if trace.ModelFamily != "doubao" {
		t.Fatalf("trace.ModelFamily=%q", trace.ModelFamily)
	}
	if trace.RequestedGenerateMode != "startEnd2video" || trace.FinalGenerateMode != "startEnd2video" {
		t.Fatalf("unexpected modes: %#v", trace)
	}
	if !trace.PreferredStartEndIdentity || !trace.HasSourceImage || !trace.HasTailImage {
		t.Fatalf("expected source/tail/preferred flags to be true: %#v", trace)
	}
	if len(trace.ProjectIdentityRefs) != 2 || len(trace.CharacterRefs) != 2 || len(trace.AssetRefs) != 1 {
		t.Fatalf("unexpected ref counts: %#v", trace)
	}
	if !trace.SerialContinuityPromptAdded || !trace.SerialChainingSourceActive {
		t.Fatalf("expected serial flags to be true: %#v", trace)
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
