package generators

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// VideoGenerateReq carries the parameters for a single clip generation.
type VideoGenerateReq struct {
	SourceImageURL     string
	TailImageURL       string   // end-frame for startEnd2video mode (Kling/Doubao/Vidu)
	CharacterImageURLs []string // reference images for subject/character consistency (reference2video)
	ClipIndex          int      // 0-based position in the sequence
	TotalClips         int      // total number of clips in the sequence
	Prompt             string
	NegativePrompt     string
	StylePreset        string
	MotionMode         string  // gentle / dynamic / cinematic
	DurationSec        float64 // desired duration
	VoiceText          string  // speech/TTS text for models that support native audio

	// Generation mode: "img2video" (default) | "startEnd2video" | "reference2video"
	GenerateMode string

	// GenerateAudio requests native audio generation from the model (doubao-seedance / suanneng).
	// When true, the downstream generator sets generate_audio=true in the API request.
	GenerateAudio bool

	// Model-specific optional params (read from task.RenderConfig):
	Resolution      string // e.g. "1080p" "720p" "360p"           — honoured by Vidu, Sora2, Doubao
	AspectRatio     string // e.g. "16:9" "9:16" "1:1"             — honoured by Vidu, Kling, Wan, Sora2, Doubao
	MotionAmplitude string // e.g. "auto" "small" "medium" "large" — honoured by Vidu
	VideoMode       string // e.g. "std" "pro"                     — honoured by Kling
	Count           int    // number of clips to generate per call  — honoured by Sora2 (n)
}

// VideoClip is the result of a successful generation.
type VideoClip struct {
	ClipURL     string
	DurationSec float64
	ModelUsed   string
}

// ── Model parameter capabilities ─────────────────────────────────────────────

// ParamValue is a single selectable value for a model parameter.
type ParamValue struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ModelParamOption describes one configurable parameter for a video model.
type ModelParamOption struct {
	Key     string       `json:"key"`     // "duration" | "resolution" | "aspect_ratio" | "motion_amplitude" | "video_mode" | "count"
	Label   string       `json:"label"`   // display label (Chinese-friendly)
	Default string       `json:"default"` // default value string
	Values  []ParamValue `json:"values"`  // allowed choices (empty = free-form / not exposed)
}

// ── VideoGenerator interface ───────────────────────────────────────────────────

// firstNonEmpty returns the first non-empty string from its arguments.
// Used by generators to prefer caller-supplied RenderConfig params over hardcoded defaults.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// RetrySubmit calls submitFn up to maxAttempts times, backing off on rate-limit
// (429) errors. The backoff sequence is 5s → 15s → 30s → 60s.
// Any non-rate-limit error (or context cancellation) returns immediately.
func RetrySubmit(ctx context.Context, maxAttempts int, submitFn func() error) error {
	return RetrySubmitWithBackoffs(ctx, maxAttempts, submitFn,
		[]time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second})
}

// RetrySubmitWithBackoffs is like RetrySubmit but uses the supplied backoff
// durations. Tests use this to avoid sleeping through real backoff intervals.
func RetrySubmitWithBackoffs(ctx context.Context, maxAttempts int, submitFn func() error, backoffs []time.Duration) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		err := submitFn()
		if err == nil {
			return nil
		}
		// Only retry on explicit rate-limit signals.
		if !isRateLimitErr(err.Error()) {
			return err
		}
		lastErr = err
		wait := backoffs[min429(i, len(backoffs)-1)]
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("rate-limited after %d attempts: %w", maxAttempts, lastErr)
}

func isRateLimitErr(msg string) bool {
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "rate limited") ||
		strings.Contains(msg, "too many requests")
}

func min429(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type VideoGenerator interface {
	// Name returns the canonical name of this generator (e.g. "kling").
	Name() string

	// Generate submits a clip generation job and blocks until it completes.
	Generate(ctx context.Context, req VideoGenerateReq) (*VideoClip, error)

	// IsAvailable performs a lightweight health / quota check.
	IsAvailable(ctx context.Context) bool

	// SupportsNativeAudio reports whether this generator produces clips with
	// embedded audio/speech from VoiceText. When true, the caller should skip
	// the separate ffmpeg dubbing-attachment step.
	SupportsNativeAudio() bool

	// ParamOptions returns the list of model-specific parameters the generator
	// accepts. The frontend uses this to show only relevant controls per model.
	ParamOptions() []ModelParamOption
}
