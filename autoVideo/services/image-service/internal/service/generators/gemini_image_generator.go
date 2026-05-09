package generators

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// sanitizePromptForGemini softens explicit emotional-distress language for child
// characters to reduce Gemini safety-filter false positives.  Gemini tends to
// refuse requests that combine minors with strong fear/crying imagery, so we
// replace the most triggering phrases with milder equivalents while preserving
// the visual intent.
func sanitizePromptForGemini(prompt string) string {
	replacements := [][2]string{
		{"crying in fear", "with a worried, teary expression"},
		{"crying out of fear", "with a worried, teary expression"},
		{"crying with fear", "with a worried, teary expression"},
		{"crying and trembling", "with a sorrowful expression and tense posture"},
		{"crying, trembling", "with a teary expression and tense posture"},
		{"trembling in fear", "with a tense, uneasy posture"},
		{"trembling with fear", "with a tense, uneasy posture"},
		{"tears streaming down her face", "with tears in her eyes"},
		{"tears streaming down his face", "with tears in his eyes"},
		{"tears streaming down their face", "with tears in their eyes"},
		{"tears streaming down the face", "with tears in the eyes"},
		{"tears stream down", "with tears forming"},
		{"terrified expression", "frightened expression"},
		{"terrified and crying", "upset and teary-eyed"},
		{"in terror", "in distress"},
		{"filled with terror", "filled with anxiety"},
		{"paralyzed with fear", "frozen with anxiety"},
		{"screaming in fear", "calling out in distress"},
		{"screaming in terror", "calling out in distress"},
		{"sobbing uncontrollably", "crying softly"},
		{"crying uncontrollably", "crying softly"},
		{"wailing", "crying softly"},
	}
	result := prompt
	lowerResult := strings.ToLower(result)
	for _, pair := range replacements {
		from, to := pair[0], pair[1]
		idx := strings.Index(strings.ToLower(lowerResult), strings.ToLower(from))
		if idx == -1 {
			continue
		}
		result = result[:idx] + to + result[idx+len(from):]
		lowerResult = strings.ToLower(result)
	}
	return result
}

// geminiChannel holds a (baseURL, apiKey) pair for one Gemini proxy channel.
type geminiChannel struct {
	base string
	key  string
}

// geminiImageGenerator calls the Gemini generateContent API (or compatible proxy)
// for text-to-image and image-to-image generation.
// Multiple (base, key) pairs are round-robined for load balancing.
// chanSems limits concurrent in-flight requests per channel to 2 to avoid 429 storms.
type geminiImageGenerator struct {
	channels   []geminiChannel
	chanSems   []chan struct{} // per-channel concurrency limiter (2 slots each)
	counter    atomic.Int64
	model      string // e.g. "gemini-3.1-flash-image-preview"
	genKey     string // registry key e.g. "banana2.1"
	client     *http.Client
	logger     *zap.Logger
	authBearer bool // true: "Authorization: Bearer {key}", false: "Authorization: {key}"
}

// geminiPart is one element in the Gemini contents[0].parts array.
type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inline_data,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"` // base64-encoded
}

type geminiGenerateReq struct {
	Contents         []geminiContent         `json:"contents"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiGenerationConfig struct {
	ResponseModalities []string `json:"responseModalities"`
}

type geminiGenerateResp struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text       string `json:"text"`
				InlineData *struct {
					MimeType string `json:"mimeType"`
					Data     string `json:"data"` // base64
				} `json:"inlineData"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// NewGeminiImageGenerator builds a generator with explicit (base, key) channel slices.
// If bearerAuth is true, adds "Bearer " prefix; otherwise uses the key verbatim.
// perChannelConcurrency controls max in-flight requests per (base,key) pair (default 2).
func NewGeminiImageGenerator(bases, keys []string, model, genKey string, bearerAuth bool, logger *zap.Logger, perChannelConcurrency ...int) ImageGenerator {
	if model == "" {
		model = "gemini-3.1-flash-image-preview"
	}
	if genKey == "" {
		genKey = model
	}
	var channels []geminiChannel
	for i, key := range keys {
		if key == "" {
			continue
		}
		base := ""
		if i < len(bases) {
			base = bases[i]
		} else if len(bases) > 0 {
			base = bases[len(bases)-1]
		}
		if base == "" {
			base = "https://generativelanguage.googleapis.com"
		}
		base = strings.TrimRight(base, "/")
		channels = append(channels, geminiChannel{base: base, key: key})
	}
	// Per-channel concurrency slots; default 2 to stay within standard Gemini rate limits.
	perChan := 2
	if len(perChannelConcurrency) > 0 && perChannelConcurrency[0] > 0 {
		perChan = perChannelConcurrency[0]
	}
	chanSems := make([]chan struct{}, len(channels))
	for i := range channels {
		chanSems[i] = make(chan struct{}, perChan)
	}
	return &geminiImageGenerator{
		channels:   channels,
		chanSems:   chanSems,
		model:      model,
		genKey:     genKey,
		client:     &http.Client{Timeout: 120 * time.Second},
		logger:     logger,
		authBearer: bearerAuth,
	}
}

// Name returns the registry key.
func (g *geminiImageGenerator) Name() string { return g.genKey }

// IsAvailable returns true when at least one channel is configured.
func (g *geminiImageGenerator) IsAvailable(_ context.Context) bool {
	return len(g.channels) > 0
}

// RefCapability —— Gemini 支持 parts[] 内联多图（最多 6 张），属于多图融合模式。
func (g *geminiImageGenerator) RefCapability() RefCapability {
	return RefCapability{Mode: RefModeFusion, MaxRefs: 6, StrongRef: false}
}

func (g *geminiImageGenerator) nextChannelIdx() (int, geminiChannel) {
	n := g.counter.Add(1)
	idx := int(n-1) % len(g.channels)
	return idx, g.channels[idx]
}

// Generate calls the Gemini generateContent endpoint and returns the first image.
func (g *geminiImageGenerator) Generate(ctx context.Context, req GenerateReq) (*GenerateRes, error) {
	// Route through the shared structured prompt builder so Gemini receives
	// the same task-type framing, character-sheet interpretation guide, and
	// negative instructions that the other enhanced generators (DALL-E /
	// Flux) get. Without this, a raw req.Prompt would bypass the 4-panel
	// reference-sheet explanation and Nano Banana would often merge four
	// separate characters into the output.
	prompt := buildNaturalLanguagePrompt(req)
	if strings.TrimSpace(prompt) == "" {
		prompt = strings.TrimSpace(req.Prompt)
	}
	if prompt == "" {
		prompt = "Generate an image based on the reference."
	}
	prompt = sanitizePromptForGemini(prompt)

	// Retry strategy: try each channel once with short waits for non-429 errors,
	// longer waits for 429s. Fast-fail after 2 minutes total to allow upstream
	// fallback to other generators.
	maxAttempts := len(g.channels) + 1
	if maxAttempts > 5 {
		maxAttempts = 5
	}
	if maxAttempts < 2 {
		maxAttempts = 2
	}

	start := time.Now()
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Fast-fail: if we've been retrying for over 2 minutes, give up so the
		// caller can fall back to another generator instead of blocking the slot.
		if attempt > 1 && time.Since(start) > 2*time.Minute {
			g.logger.Warn("gemini: fast-fail after timeout",
				zap.String("generator", g.genKey),
				zap.Int("attempts", attempt),
				zap.Duration("elapsed", time.Since(start)))
			break
		}

		if attempt > 0 {
			isRateLimit := lastErr != nil && strings.Contains(lastErr.Error(), "http 429")
			var baseWait time.Duration
			if isRateLimit {
				// 429: escalating wait 15/30/60s
				baseWait = time.Duration(15*(1<<min429(attempt-1, 2))) * time.Second
			} else {
				// Other errors: quick rotate to next channel
				baseWait = 3 * time.Second
			}
			jitter := time.Duration(rand.Intn(5000)) * time.Millisecond
			wait := baseWait + jitter
			g.logger.Warn("gemini image call failed, retrying",
				zap.String("generator", g.genKey),
				zap.Int("attempt", attempt+1),
				zap.Duration("wait", wait),
				zap.Error(lastErr))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}
		chanIdx, ch := g.nextChannelIdx()
		// Acquire per-channel rate-limit slot (blocks if channel is busy, respects ctx).
		select {
		case g.chanSems[chanIdx] <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		res, err := g.callAPI(ctx, ch, prompt, req.AllReferenceImageURLs())
		<-g.chanSems[chanIdx]
		if err != nil {
			lastErr = err
			continue
		}
		return res, nil
	}
	return nil, fmt.Errorf("gemini image generator %q: all attempts failed: %w", g.genKey, lastErr)
}

func (g *geminiImageGenerator) callAPI(ctx context.Context, ch geminiChannel, prompt string, refImageURLs []string) (*GenerateRes, error) {
	// Build parts
	parts := []geminiPart{{Text: prompt}}

	// Cap at 6 reference images to avoid oversized request bodies and keep
	// the model within documented multi-image fusion guidance for
	// gemini-2.5-flash-image (Nano Banana).
	const maxRefs = 6
	refs := refImageURLs
	if len(refs) > maxRefs {
		refs = refs[:maxRefs]
	}

	// Add reference images for img2img / fusion
	for _, imgURL := range refs {
		if imgURL == "" {
			continue
		}
		inlineData, err := g.fetchAndEncode(ctx, imgURL)
		if err != nil {
			g.logger.Warn("failed to fetch reference image; skipping", zap.String("url", imgURL), zap.Error(err))
			continue
		}
		parts = append(parts, geminiPart{InlineData: inlineData})
	}

	payload := geminiGenerateReq{
		Contents: []geminiContent{{Parts: parts}},
		GenerationConfig: &geminiGenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
		},
	}
	body, _ := json.Marshal(payload)

	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent", ch.base, g.model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if g.authBearer {
		httpReq.Header.Set("Authorization", "Bearer "+ch.key)
	} else {
		httpReq.Header.Set("x-goog-api-key", ch.key)
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(raw))
	}

	var result geminiGenerateResp
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("api error: %s", result.Error.Message)
	}
	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	// Find the first image part in the response
	for _, part := range result.Candidates[0].Content.Parts {
		if part.InlineData != nil && part.InlineData.Data != "" {
			mimeType := part.InlineData.MimeType
			if mimeType == "" {
				mimeType = "image/png"
			}
			dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, part.InlineData.Data)
			return &GenerateRes{
				ImageURL:  dataURI,
				ModelUsed: g.genKey,
			}, nil
		}
	}
	return nil, fmt.Errorf("no image part in gemini response")
}

func min429(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// fetchAndEncode downloads an image URL and returns it as a geminiInlineData.
func (g *geminiImageGenerator) fetchAndEncode(ctx context.Context, imageURL string) (*geminiInlineData, error) {
	if strings.HasPrefix(imageURL, "data:") {
		// Already a data URI — decode and re-encode
		parts := strings.SplitN(imageURL, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid data URI")
		}
		header := parts[0] // e.g. "data:image/png;base64"
		mimeType := "image/png"
		if semicolon := strings.Index(header, ";"); semicolon > 5 {
			mimeType = header[5:semicolon]
		}
		return &geminiInlineData{MimeType: mimeType, Data: parts[1]}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	imgBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" || !strings.HasPrefix(mimeType, "image/") {
		mimeType = detectMimeType(imgBytes)
	}
	// Strip any parameters (e.g. "image/jpeg; charset=utf-8")
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	return &geminiInlineData{
		MimeType: mimeType,
		Data:     base64.StdEncoding.EncodeToString(imgBytes),
	}, nil
}

// detectMimeType inspects the first bytes of an image to determine MIME type.
func detectMimeType(data []byte) string {
	if len(data) < 4 {
		return "image/png"
	}
	switch {
	case data[0] == 0xFF && data[1] == 0xD8:
		return "image/jpeg"
	case data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47:
		return "image/png"
	case data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46:
		return "image/gif"
	case data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46:
		return "image/webp"
	default:
		return "image/png"
	}
}
