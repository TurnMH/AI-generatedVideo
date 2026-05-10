package generators

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// containsCJK returns true when s contains at least one CJK (Chinese / Japanese / Korean)
// Unicode codepoint. Used to decide whether translation is needed.
func containsCJK(s string) bool {
	for _, r := range s {
		if r >= 0x2E80 {
			return true
		}
	}
	return false
}

// NewPromptTranslator builds a context-aware translation function that converts
// Chinese/CJK image-generation prompts to English via an OpenAI-compatible
// /chat/completions endpoint (e.g. ppai, poloai).
//
// If keys or baseURL are empty, or if the prompt contains no CJK characters,
// the returned function is a no-op that returns the original text unchanged.
//
// On any translation error the function falls back to stripCJKCharacters so
// Qianfan at least receives something valid rather than rejecting the request.
func NewPromptTranslator(keys []string, baseURL string, logger *zap.Logger) func(ctx context.Context, text string) string {
	if len(keys) == 0 || baseURL == "" {
		return nil
	}
	pool := newSmartKeyPool(keys)
	client := &http.Client{Timeout: 15 * time.Second}

	// Ensure the base URL ends at /v1 (do not add /v1 if it's already there).
	base := strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(base, "/v1") {
		base = base + "/v1"
	}
	chatURL := base + "/chat/completions"

	return func(ctx context.Context, text string) string {
		if !containsCJK(text) {
			return text
		}
		apiKey := pool.nextKey()
		if apiKey == "" {
			return stripCJKCharacters(text)
		}

		body, _ := json.Marshal(map[string]interface{}{
			"model": "gpt-4o-mini",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": "You are an image prompt translator. Translate the user's image generation prompt to English. Output only the translated prompt text, nothing else.",
				},
				{
					"role":    "user",
					"content": text,
				},
			},
			"max_tokens":  200,
			"temperature": 0,
		})

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(body))
		if err != nil {
			logger.Warn("prompt translator: build request failed", zap.Error(err))
			return stripCJKCharacters(text)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			logger.Warn("prompt translator: http error", zap.Error(err))
			return stripCJKCharacters(text)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logger.Warn("prompt translator: non-200 response", zap.Int("status", resp.StatusCode))
			return stripCJKCharacters(text)
		}

		raw, _ := io.ReadAll(resp.Body)
		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &result); err != nil || len(result.Choices) == 0 {
			logger.Warn("prompt translator: decode failed", zap.Error(err))
			return stripCJKCharacters(text)
		}

		translated := strings.TrimSpace(result.Choices[0].Message.Content)
		if translated == "" {
			return stripCJKCharacters(text)
		}
		logger.Debug("prompt translator: translated", zap.String("original", text), zap.String("translated", translated))
		return translated
	}
}
