package generators

import "testing"

func TestPreferredRatioIntent(t *testing.T) {
	t.Parallel()

	cases := map[string]RatioIntent{
		"portrait":        RatioPortrait,
		"character-sheet": RatioPortrait,
		"poster":          RatioPortrait,
		"storyboard":      RatioLandscape,
		"scene-concept":   RatioLandscape,
		"general":         RatioSquare,
	}
	for taskType, want := range cases {
		if got := PreferredRatioIntent(taskType); got != want {
			t.Fatalf("PreferredRatioIntent(%q) = %q, want %q", taskType, got, want)
		}
	}
}

func TestNormalizeGenerateSizeSDXLDefaultsByTaskType(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("sdxl", "storyboard", 0, 0)
	if got.Width != 768 || got.Height != 512 {
		t.Fatalf("NormalizeGenerateSize(sdxl, storyboard, 0, 0) = %dx%d", got.Width, got.Height)
	}
	if !got.Changed {
		t.Fatalf("expected Changed=true for missing size")
	}
}

func TestNormalizeGenerateSizeSDXLRoundsToMultiple(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("sdxl", "portrait", 513, 769)
	if got.Width != 512 || got.Height != 768 {
		t.Fatalf("NormalizeGenerateSize(sdxl, portrait, 513, 769) = %dx%d", got.Width, got.Height)
	}
}

func TestNormalizeGenerateSizeOpenAIUsesApprovedSquareSize(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("dalle", "portrait", 1536, 1024)
	if got.Width != 1024 || got.Height != 1024 {
		t.Fatalf("NormalizeGenerateSize(dalle, portrait, 1536, 1024) = %dx%d", got.Width, got.Height)
	}
	if !got.Changed {
		t.Fatalf("expected Changed=true for unsupported enum size")
	}
}

func TestNormalizeGenerateSizeWanxV1UsesDocumentedSquareSize(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("wanx-v1", "poster", 1536, 1024)
	if got.Width != 1024 || got.Height != 1024 {
		t.Fatalf("NormalizeGenerateSize(wanx-v1, poster, 1536, 1024) = %dx%d", got.Width, got.Height)
	}
	if got.Policy.ProviderKey != "tongyi-wanx-v1" {
		t.Fatalf("unexpected policy provider key: %q", got.Policy.ProviderKey)
	}
}

func TestNormalizeGenerateSizeWanx21PreservesExplicitSize(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("wanx2.1-t2i-turbo", "portrait", 1280, 720)
	if got.Width != 1280 || got.Height != 720 {
		t.Fatalf("NormalizeGenerateSize(wanx2.1-t2i-turbo, portrait, 1280, 720) = %dx%d", got.Width, got.Height)
	}
	if got.Changed {
		t.Fatalf("expected Changed=false for passthrough policy")
	}
}

func TestNormalizeGenerateSizeDoubaoUsesVerifiedLandscape(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("doubao-image", "storyboard", 0, 0)
	if got.Width != 1280 || got.Height != 960 {
		t.Fatalf("NormalizeGenerateSize(doubao-image, storyboard, 0, 0) = %dx%d", got.Width, got.Height)
	}
	if got.Policy.ProviderKey != "doubao" {
		t.Fatalf("unexpected policy provider key: %q", got.Policy.ProviderKey)
	}
}

func TestNormalizeGenerateSizeDoubaoUsesVerifiedPortrait(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("doubao-seedream-4-0-250828", "portrait", 0, 0)
	if got.Width != 960 || got.Height != 1280 {
		t.Fatalf("NormalizeGenerateSize(doubao-seedream-4-0-250828, portrait, 0, 0) = %dx%d", got.Width, got.Height)
	}
}

func TestNormalizeGenerateSizeQianfanConservativeSquare(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("qianfan-flux.1-schnell", "storyboard", 0, 0)
	if got.Width != 1024 || got.Height != 1024 {
		t.Fatalf("NormalizeGenerateSize(qianfan-flux.1-schnell, storyboard, 0, 0) = %dx%d", got.Width, got.Height)
	}
	if got.Policy.Verification != VerificationPartial {
		t.Fatalf("expected verification=partial, got %q", got.Policy.Verification)
	}
}

// ─── CogView ─────────────────────────────────────────────────

func TestNormalizeGenerateSizeCogViewConservativeSquare(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("cogview-3-plus", "portrait", 0, 0)
	if got.Width != 1024 || got.Height != 1024 {
		t.Fatalf("NormalizeGenerateSize(cogview-3-plus, portrait, 0, 0) = %dx%d", got.Width, got.Height)
	}
	if got.Policy.ProviderKey != "cogview" {
		t.Fatalf("expected provider_key=cogview, got %q", got.Policy.ProviderKey)
	}
	if got.Policy.Mode != ImageSizeModeEnumSize {
		t.Fatalf("expected mode=enum_size, got %q", got.Policy.Mode)
	}
}

// ─── OpenAI variants ─────────────────────────────────────────

func TestNormalizeGenerateSizeGPTImageConservativeSquare(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"gpt-image-1", "gpt-4o-image", "dall-e-3"} {
		got := NormalizeGenerateSize(name, "storyboard", 1536, 1024)
		if got.Width != 1024 || got.Height != 1024 {
			t.Fatalf("NormalizeGenerateSize(%s, storyboard, 1536, 1024) = %dx%d", name, got.Width, got.Height)
		}
		if got.Policy.ProviderKey != "openai-image" {
			t.Fatalf("%s: expected provider_key=openai-image, got %q", name, got.Policy.ProviderKey)
		}
	}
}

// ─── SDXL boundary clamp ─────────────────────────────────────

func TestNormalizeGenerateSizeSDXLClampMin(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("sdxl", "general", 256, 256)
	if got.Width != 512 || got.Height != 512 {
		t.Fatalf("NormalizeGenerateSize(sdxl, general, 256, 256) = %dx%d, want 512x512", got.Width, got.Height)
	}
	if !got.Changed {
		t.Fatalf("expected Changed=true for clamped size")
	}
}

func TestNormalizeGenerateSizeSDXLClampMax(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("sdxl", "general", 2048, 2048)
	if got.Width != 1536 || got.Height != 1536 {
		t.Fatalf("NormalizeGenerateSize(sdxl, general, 2048, 2048) = %dx%d, want 1536x1536", got.Width, got.Height)
	}
}

// ─── Doubao enum validation ──────────────────────────────────

func TestNormalizeGenerateSizeDoubaoAcceptsLegalSize(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("doubao-image", "general", 1280, 960)
	if got.Width != 1280 || got.Height != 960 {
		t.Fatalf("expected 1280x960 passthrough, got %dx%d", got.Width, got.Height)
	}
	if got.Changed {
		t.Fatalf("expected Changed=false for legal enum size")
	}
}

func TestNormalizeGenerateSizeDoubaoRejectsIllegalSize(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("doubao-image", "storyboard", 1536, 1024)
	// 1536x1024 is not in allowed list, should fall back to landscape default 1280x960
	if got.Width != 1280 || got.Height != 960 {
		t.Fatalf("expected fallback 1280x960, got %dx%d", got.Width, got.Height)
	}
	if !got.Changed {
		t.Fatalf("expected Changed=true for rejected size")
	}
}

// ─── Passthrough default fallback ────────────────────────────

func TestNormalizeGenerateSizeWanx21DefaultsWhenOmitted(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("wanx2.1-t2i-turbo", "general", 0, 0)
	if got.Width != 1024 || got.Height != 1024 {
		t.Fatalf("NormalizeGenerateSize(wanx2.1, general, 0, 0) = %dx%d, want 1024x1024", got.Width, got.Height)
	}
	if !got.Changed {
		t.Fatalf("expected Changed=true for default fallback")
	}
}

// ─── Unknown model fallback ──────────────────────────────────

func TestNormalizeGenerateSizeUnknownModelFallback(t *testing.T) {
	t.Parallel()

	got := NormalizeGenerateSize("some-unknown-model-xyz", "portrait", 0, 0)
	if got.Width != 1024 || got.Height != 1024 {
		t.Fatalf("expected 1024x1024 fallback, got %dx%d", got.Width, got.Height)
	}
	if got.Policy.ProviderKey != "generic-image" {
		t.Fatalf("expected provider_key=generic-image, got %q", got.Policy.ProviderKey)
	}
	if got.Policy.Verification != VerificationAssumed {
		t.Fatalf("expected verification=assumed, got %q", got.Policy.Verification)
	}
}

// ─── TaskTypeRatioDefaults ───────────────────────────────────

func TestTaskTypeRatioDefaults(t *testing.T) {
	t.Parallel()

	defaults := TaskTypeRatioDefaults()
	expected := map[string]RatioIntent{
		"portrait":        RatioPortrait,
		"character-sheet": RatioPortrait,
		"poster":          RatioPortrait,
		"storyboard":      RatioLandscape,
		"scene-concept":   RatioLandscape,
		"general":         RatioSquare,
	}
	for taskType, want := range expected {
		got, ok := defaults[taskType]
		if !ok {
			t.Fatalf("TaskTypeRatioDefaults missing key %q", taskType)
		}
		if got != want {
			t.Fatalf("TaskTypeRatioDefaults[%q] = %q, want %q", taskType, got, want)
		}
	}
}

// ─── ResolveImageSizePolicy direct assertions ────────────────

func TestResolveImageSizePolicyFieldCoverage(t *testing.T) {
	t.Parallel()

	// SDXL — arbitrary_wh with full constraints
	sdxl := ResolveImageSizePolicy("sdxl")
	if sdxl.Mode != ImageSizeModeArbitraryWH {
		t.Fatalf("sdxl mode = %q", sdxl.Mode)
	}
	if sdxl.RequireMultiple != 64 {
		t.Fatalf("sdxl RequireMultiple = %d", sdxl.RequireMultiple)
	}
	if sdxl.MinWidth != 512 || sdxl.MaxWidth != 1536 {
		t.Fatalf("sdxl width range = %d-%d", sdxl.MinWidth, sdxl.MaxWidth)
	}

	// Doubao — enum_size with 3 allowed sizes
	doubao := ResolveImageSizePolicy("doubao-image")
	if doubao.Mode != ImageSizeModeEnumSize {
		t.Fatalf("doubao mode = %q", doubao.Mode)
	}
	if len(doubao.AllowedSizes) != 3 {
		t.Fatalf("doubao AllowedSizes count = %d, want 3", len(doubao.AllowedSizes))
	}

	// Tongyi wanx2.1 — passthrough
	tongyi := ResolveImageSizePolicy("wanx2.1-t2i-plus")
	if tongyi.Mode != ImageSizeModePassthrough {
		t.Fatalf("tongyi mode = %q", tongyi.Mode)
	}

	// Empty model name — should resolve to sdxl
	empty := ResolveImageSizePolicy("")
	if empty.ProviderKey != "sdxl" {
		t.Fatalf("empty model provider_key = %q", empty.ProviderKey)
	}
}
