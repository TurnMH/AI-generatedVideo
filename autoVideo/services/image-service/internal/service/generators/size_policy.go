package generators

import (
	"fmt"
	"math"
	"strings"
)

type ImageSizeMode string

type VerificationLevel string

type RatioIntent string

const (
	ImageSizeModeArbitraryWH ImageSizeMode = "arbitrary_wh"
	ImageSizeModeEnumSize    ImageSizeMode = "enum_size"
	ImageSizeModePassthrough ImageSizeMode = "passthrough"
)

const (
	VerificationVerified VerificationLevel = "verified"
	VerificationPartial  VerificationLevel = "partial"
	VerificationAssumed  VerificationLevel = "assumed"
)

const (
	RatioSquare    RatioIntent = "square"
	RatioPortrait  RatioIntent = "portrait"
	RatioLandscape RatioIntent = "landscape"
)

type ImageSizePolicy struct {
	ProviderKey      string
	Mode             ImageSizeMode
	Verification     VerificationLevel
	AllowedSizes     []string
	DefaultSquare    string
	DefaultPortrait  string
	DefaultLandscape string
	MinWidth         int
	MaxWidth         int
	MinHeight        int
	MaxHeight        int
	RequireMultiple  int
	Notes            string
}

type NormalizedSizeResult struct {
	Width   int
	Height  int
	Changed bool
	Reason  string
	Policy  ImageSizePolicy
}

func NormalizeGenerateSize(modelName, taskType string, width, height int) NormalizedSizeResult {
	policy := ResolveImageSizePolicy(modelName)

	switch policy.Mode {
	case ImageSizeModeArbitraryWH:
		if width <= 0 || height <= 0 {
			width, height = defaultDimensionsForPolicy(policy, taskType)
			return NormalizedSizeResult{
				Width:   width,
				Height:  height,
				Changed: true,
				Reason:  fmt.Sprintf("applied %s default size for task_type=%s", policy.ProviderKey, NormalizeImageTaskType(taskType, "")),
				Policy:  policy,
			}
		}
		nw, nh := normalizeArbitrarySize(policy, width, height)
		return NormalizedSizeResult{
			Width:   nw,
			Height:  nh,
			Changed: nw != width || nh != height,
			Reason:  reasonForResize(width, height, nw, nh, policy.ProviderKey),
			Policy:  policy,
		}
	case ImageSizeModeEnumSize:
		if width > 0 && height > 0 {
			size := fmt.Sprintf("%dx%d", width, height)
			if containsString(policy.AllowedSizes, size) {
				return NormalizedSizeResult{Width: width, Height: height, Policy: policy}
			}
		}
		fallbackW, fallbackH := defaultDimensionsForPolicy(policy, taskType)
		return NormalizedSizeResult{
			Width:   fallbackW,
			Height:  fallbackH,
			Changed: width != fallbackW || height != fallbackH,
			Reason:  fmt.Sprintf("mapped to %s approved size set", policy.ProviderKey),
			Policy:  policy,
		}
	default:
		if width > 0 && height > 0 {
			return NormalizedSizeResult{Width: width, Height: height, Policy: policy}
		}
		fallbackW, fallbackH := defaultDimensionsForPolicy(policy, taskType)
		return NormalizedSizeResult{
			Width:   fallbackW,
			Height:  fallbackH,
			Changed: true,
			Reason:  fmt.Sprintf("applied conservative default size for %s", policy.ProviderKey),
			Policy:  policy,
		}
	}
}

func ResolveImageSizePolicy(modelName string) ImageSizePolicy {
	name := strings.ToLower(strings.TrimSpace(modelName))
	switch {
	case name == "" || name == "sdxl":
		return ImageSizePolicy{
			ProviderKey:      "sdxl",
			Mode:             ImageSizeModeArbitraryWH,
			Verification:     VerificationVerified,
			DefaultSquare:    "1024x1024",
			DefaultPortrait:  "512x768",
			DefaultLandscape: "768x512",
			MinWidth:         512,
			MaxWidth:         1536,
			MinHeight:        512,
			MaxHeight:        1536,
			RequireMultiple:  64,
			Notes:            "Local ComfyUI workflow supports explicit width/height; use safe VRAM-oriented defaults.",
		}
	case isCogViewModel(name):
		return enumSquarePolicy("cogview", VerificationPartial, "CogView size schema still pending final official confirmation; keep conservative square default.")
	case isDoubaoModelName(name):
		return ImageSizePolicy{
			ProviderKey:      "doubao",
			Mode:             ImageSizeModeEnumSize,
			Verification:     VerificationPartial,
			AllowedSizes:     []string{"1024x1024", "1280x960", "960x1280"},
			DefaultSquare:    "1024x1024",
			DefaultPortrait:  "960x1280",
			DefaultLandscape: "1280x960",
			Notes:            "Ark doubao-seedream verified sizes from repo integration: 1024x1024, 1280x960; portrait 960x1280 inferred from landscape.",
		}
	case isOpenAIImageModel(name):
		return enumSquarePolicy("openai-image", VerificationPartial, "OpenAI official docs were not fetchable in-tool (403); keep conservative square default until official size matrix is verified offline.")
	case name == "tongyi" || name == "wanx-v1":
		return enumSquarePolicy("tongyi-wanx-v1", VerificationPartial, "DashScope wanx-v1 official examples explicitly document size=1024*1024; other sizes remain unverified in current doc access.")
	case isTongyiModelName(name):
		return ImageSizePolicy{
			ProviderKey:      "tongyi",
			Mode:             ImageSizeModePassthrough,
			Verification:     VerificationPartial,
			DefaultSquare:    "1024x1024",
			DefaultPortrait:  "1024x1024",
			DefaultLandscape: "1024x1024",
			Notes:            "DashScope wanx2.x model-level size whitelist still needs final official verification; preserve explicit width/height for now.",
		}
	case isQianfanModelName(name):
		return enumSquarePolicy("qianfan", VerificationPartial, "Qianfan verified models: flux.1-schnell, stable-diffusion-xl; size whitelist not yet confirmed beyond 1024x1024.")
	default:
		return enumSquarePolicy("generic-image", VerificationAssumed, "Unknown model; conservative square fallback.")
	}
}

func PreferredRatioIntent(taskType string) RatioIntent {
	switch NormalizeImageTaskType(taskType, "") {
	case "portrait", "character-view", "character-sheet", "poster":
		return RatioPortrait
	case "storyboard", "scene-concept":
		return RatioLandscape
	default:
		return RatioSquare
	}
}

func TaskTypeRatioDefaults() map[string]RatioIntent {
	return map[string]RatioIntent{
		"portrait":        RatioPortrait,
		"character-view":  RatioPortrait,
		"character-sheet": RatioPortrait,
		"poster":          RatioPortrait,
		"storyboard":      RatioLandscape,
		"scene-concept":   RatioLandscape,
		"general":         RatioSquare,
	}
}

func defaultDimensionsForPolicy(policy ImageSizePolicy, taskType string) (int, int) {
	var size string
	switch PreferredRatioIntent(taskType) {
	case RatioPortrait:
		size = policy.DefaultPortrait
	case RatioLandscape:
		size = policy.DefaultLandscape
	default:
		size = policy.DefaultSquare
	}
	if size == "" {
		size = policy.DefaultSquare
	}
	w, h := parseDimensions(size)
	if w <= 0 || h <= 0 {
		return 1024, 1024
	}
	return w, h
}

func normalizeArbitrarySize(policy ImageSizePolicy, width, height int) (int, int) {
	w, h := width, height
	if policy.RequireMultiple > 0 {
		w = roundToNearestMultiple(w, policy.RequireMultiple)
		h = roundToNearestMultiple(h, policy.RequireMultiple)
	}
	if policy.MinWidth > 0 && w < policy.MinWidth {
		w = policy.MinWidth
	}
	if policy.MaxWidth > 0 && w > policy.MaxWidth {
		w = policy.MaxWidth
	}
	if policy.MinHeight > 0 && h < policy.MinHeight {
		h = policy.MinHeight
	}
	if policy.MaxHeight > 0 && h > policy.MaxHeight {
		h = policy.MaxHeight
	}
	return w, h
}

func enumSquarePolicy(providerKey string, verification VerificationLevel, notes string) ImageSizePolicy {
	return ImageSizePolicy{
		ProviderKey:      providerKey,
		Mode:             ImageSizeModeEnumSize,
		Verification:     verification,
		AllowedSizes:     []string{"1024x1024"},
		DefaultSquare:    "1024x1024",
		DefaultPortrait:  "1024x1024",
		DefaultLandscape: "1024x1024",
		Notes:            notes,
	}
}

func parseDimensions(size string) (int, int) {
	var width, height int
	_, _ = fmt.Sscanf(strings.TrimSpace(size), "%dx%d", &width, &height)
	return width, height
}

func roundToNearestMultiple(n, multiple int) int {
	if multiple <= 0 || n <= 0 {
		return n
	}
	return int(math.Round(float64(n)/float64(multiple))) * multiple
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func reasonForResize(oldW, oldH, newW, newH int, providerKey string) string {
	if oldW == newW && oldH == newH {
		return ""
	}
	return fmt.Sprintf("normalized size for %s from %dx%d to %dx%d", providerKey, oldW, oldH, newW, newH)
}

func isOpenAIImageModel(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	return name == "dalle" ||
		strings.HasPrefix(name, "dall-e") ||
		strings.HasPrefix(name, "gpt-image") ||
		strings.HasPrefix(name, "gpt-4o-image")
}

func isTongyiModelName(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	return name == "tongyi" || strings.HasPrefix(name, "wanx") || strings.HasPrefix(name, "wan2.")
}

func isDoubaoModelName(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	return name == "doubao-image" || strings.HasPrefix(name, "doubao-")
}

func isQianfanModelName(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(name, "qianfan-")
}
