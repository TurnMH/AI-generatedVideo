package generators

import (
	"testing"
)

// ─── KlingGenerator Name / WithName / WithModel / WithOmniModel ───────────────

func TestKlingGenerator_DefaultName(t *testing.T) {
	t.Parallel()
	gen := NewKlingGeneratorWithKeys("https://api.klingai.com", "ak-test")
	if gen.Name() != "kling" {
		t.Errorf("expected default name 'kling', got %q", gen.Name())
	}
}

func TestKlingGenerator_WithName_Aiping(t *testing.T) {
	t.Parallel()
	gen := NewKlingGeneratorWithKeys("https://aiping.cn", "QC-testkey")
	gen.WithName("aiping")
	if gen.Name() != "aiping" {
		t.Errorf("expected 'aiping', got %q", gen.Name())
	}
}

func TestKlingGenerator_WithModel_SetsModelName(t *testing.T) {
	t.Parallel()
	gen := NewKlingGeneratorWithKeys("https://api.klingai.com", "ak-test")
	result := gen.WithModel("kling-v3")
	if gen.ModelName != "kling-v3" {
		t.Errorf("expected ModelName = 'kling-v3', got %q", gen.ModelName)
	}
	// WithModel should return the same generator for chaining
	if result != gen {
		t.Error("WithModel() should return the receiver for chaining")
	}
}

func TestKlingGenerator_WithOmniModel_SetsOmniModel(t *testing.T) {
	t.Parallel()
	gen := NewKlingGeneratorWithKeys("https://api.klingai.com", "ak-test")
	gen.WithOmniModel("kling-v3-omni")
	if gen.OmniModel != "kling-v3-omni" {
		t.Errorf("expected OmniModel = 'kling-v3-omni', got %q", gen.OmniModel)
	}
}

func TestKlingGenerator_DefaultModels(t *testing.T) {
	t.Parallel()
	gen := NewKlingGeneratorWithKeys("https://api.klingai.com", "ak-test")
	if gen.ModelName != "kling-v1-6" {
		t.Errorf("expected default ModelName = 'kling-v1-6', got %q", gen.ModelName)
	}
	if gen.OmniModel != "kling-v1-6" {
		t.Errorf("expected default OmniModel = 'kling-v1-6', got %q", gen.OmniModel)
	}
}

func TestKlingGenerator_WithModel_IgnoresEmpty(t *testing.T) {
	t.Parallel()
	gen := NewKlingGeneratorWithKeys("https://api.klingai.com", "ak-test")
	gen.WithModel("")
	// Empty string should not override the default
	if gen.ModelName != "kling-v1-6" {
		t.Errorf("expected ModelName to stay 'kling-v1-6' on empty override, got %q", gen.ModelName)
	}
}

// ─── Aiping channel configuration tests ─────────────────────────────────────

func TestAipingChannel_RegistrationPattern(t *testing.T) {
	t.Parallel()

	// This mirrors what video-service cmd/main.go does for aiping registration
	aipingBase := "https://aiping.cn"
	aipingKey := "QC-3c33b1ea5b5ae6dc8bb36e2df351fd22-0c092c6bc6c400606e6dc3b3bed891ff"
	klingModel := "kling-v3"

	gen := NewKlingGeneratorWithKeys(aipingBase, aipingKey)
	gen.WithModel(klingModel)
	gen.WithName("aiping")

	if gen.Name() != "aiping" {
		t.Errorf("expected name 'aiping', got %q", gen.Name())
	}
	if gen.ModelName != "kling-v3" {
		t.Errorf("expected ModelName 'kling-v3', got %q", gen.ModelName)
	}
	if gen.BaseURL != aipingBase {
		t.Errorf("expected BaseURL %q, got %q", aipingBase, gen.BaseURL)
	}
}

// ─── Kling v3 upgrade configuration tests ────────────────────────────────────

func TestKlingV3_FullConfiguration(t *testing.T) {
	t.Parallel()

	// Mirrors cmd/main.go Kling registration with config.local.yaml v3 values
	gen := NewKlingGeneratorWithKeys("https://api.klingai.com", "a5c8UfgYEhz5xe9WuNsRI8wa2HJ8k9KH")
	gen.WithModel("kling-v3")
	gen.WithOmniModel("kling-v3-omni")

	if gen.Name() != "kling" {
		t.Errorf("expected name 'kling', got %q", gen.Name())
	}
	if gen.ModelName != "kling-v3" {
		t.Errorf("expected ModelName 'kling-v3', got %q", gen.ModelName)
	}
	if gen.OmniModel != "kling-v3-omni" {
		t.Errorf("expected OmniModel 'kling-v3-omni', got %q", gen.OmniModel)
	}
}

// ─── KlingGenerator IsAvailable ──────────────────────────────────────────────

func TestKlingGenerator_IsAvailable_WithKey(t *testing.T) {
	t.Parallel()
	gen := NewKlingGeneratorWithKeys("https://api.klingai.com", "sk-test")
	if !gen.IsAvailable(nil) {
		t.Error("expected IsAvailable() = true with non-empty key")
	}
}

func TestKlingGenerator_IsAvailable_NoKey(t *testing.T) {
	t.Parallel()
	gen := NewKlingGeneratorWithKeys("https://api.klingai.com") // no keys
	if gen.IsAvailable(nil) {
		t.Error("expected IsAvailable() = false with no keys")
	}
}
