package generators

import (
	"context"
	"strings"
)

type GenerateReq struct {
	Prompt            string
	NegativePrompt    string
	TaskType          string
	StylePreset       string
	StyleReferenceURL string
	Width             int
	Height            int
	Steps             int
	CfgScale          float64
	Seed              int64
	LoraModelID       string
	IPAdapterURL      string
	// ReferenceImageURLs carries additional reference images (e.g. every
	// character that appears in a storyboard panel) for models that support
	// multi-image conditioning such as Gemini (parts[]), Qwen-Image-Edit
	// (messages.content[]), Seedream (image[]) and gpt-image-1 (image[]).
	// StyleReferenceURL and IPAdapterURL remain the primary references for
	// back-compat with single-ref generators (SDXL, Tongyi, Flux, DALL-E gen).
	ReferenceImageURLs []string
	// IsCharacterSheet marks whether any attached reference image is a
	// 4-panel character turnaround sheet (close-up / front / side / back of
	// ONE person). When true, prompt builders inject explicit guidance so
	// generators don't interpret the strip as four unrelated subjects.
	// Callers should set this explicitly; as a fallback, a URL containing
	// "_composite." is also recognised as a character sheet.
	IsCharacterSheet bool
}

// AllReferenceImageURLs returns the ordered, de-duplicated union of the
// single-ref fields and the multi-ref slice. Callers that support multiple
// references should prefer this helper so they transparently pick up both
// legacy and new fields. Empty strings are skipped.
func (r GenerateReq) AllReferenceImageURLs() []string {
	out := make([]string, 0, 2+len(r.ReferenceImageURLs))
	seen := make(map[string]struct{}, 2+len(r.ReferenceImageURLs))
	add := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	add(r.StyleReferenceURL)
	add(r.IPAdapterURL)
	for _, u := range r.ReferenceImageURLs {
		add(u)
	}
	return out
}

type GenerateRes struct {
	ImageURL     string
	ThumbnailURL string
	Width        int
	Height       int
	Seed         int64
	ModelUsed    string
}

// RefMode 描述生成器使用参考图的方式。
type RefMode string

const (
	// RefModeT2I — 纯文生图，不接受参考图，人物一致性完全依赖文字描述。
	RefModeT2I RefMode = "t2i"
	// RefModeIPAdapter — 单图软注入（IP-Adapter / ref_image_url），参考图影响外观但非强依赖。
	RefModeIPAdapter RefMode = "ip-adapter"
	// RefModeI2I — 图生图模式，强参考依赖；无参考图时输出质量大幅下降。
	RefModeI2I RefMode = "i2i"
	// RefModeFusion — 多图融合（Gemini parts[] / gpt-image edits / Baidu messages），支持多角色参考。
	RefModeFusion RefMode = "fusion"
)

// RefCapability 描述生成器的参考图能力。
type RefCapability struct {
	// Mode 表示参考图接入方式。
	Mode RefMode `json:"mode"`
	// MaxRefs 为该生成器接受的最大参考图数量（0=不支持，1=单图，N=多图）。
	MaxRefs int `json:"max_refs"`
	// StrongRef 为 true 时表示缺少参考图会导致生成质量显著下降。
	StrongRef bool `json:"strong_ref"`
}

type ImageGenerator interface {
	Name() string
	Generate(ctx context.Context, req GenerateReq) (*GenerateRes, error)
	IsAvailable(ctx context.Context) bool
	// RefCapability 返回该生成器的参考图能力声明，供上游调用方在模型选择和分镜生成时使用。
	RefCapability() RefCapability
}
