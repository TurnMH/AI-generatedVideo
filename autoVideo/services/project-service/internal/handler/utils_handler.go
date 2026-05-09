package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
)

// UtilsHandler provides utility endpoints (translation, etc.)
type UtilsHandler struct {
	llmBaseURL string
	llmAPIKey  string
	llmModel   string
}

func NewUtilsHandler(llmBaseURL, llmAPIKey, llmModel string) *UtilsHandler {
	if llmBaseURL == "" {
		llmBaseURL = "https://api.easyart.cc/v1"
	}
	if llmModel == "" {
		llmModel = "gpt-5.4-mini"
	}
	return &UtilsHandler{
		llmBaseURL: strings.TrimRight(llmBaseURL, "/"),
		llmAPIKey:  llmAPIKey,
		llmModel:   llmModel,
	}
}

// TranslatePrompt translates Chinese text to English for image-generation prompts.
// POST /api/v1/utils/translate  { "text": "..." }
func (h *UtilsHandler) TranslatePrompt(c *gin.Context) {
	var req struct {
		Text string `json:"text"`
	}
	if err := c.BindJSON(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing text"})
		return
	}

	if !containsChineseChars(req.Text) {
		c.JSON(http.StatusOK, gin.H{"translated": req.Text})
		return
	}

	translated, err := h.callLLMTranslate(c.Request.Context(), req.Text)
	if err != nil || translated == "" {
		// Fallback: return original text so UI doesn't break
		c.JSON(http.StatusOK, gin.H{"translated": req.Text, "warning": "translation failed, original returned"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"translated": translated})
}

func containsChineseChars(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func (h *UtilsHandler) callLLMTranslate(ctx context.Context, text string) (string, error) {
	tCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]interface{}{
		"model": h.llmModel,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a professional translator and prompt engineer specializing in AI image-generation prompts. If the input contains Chinese, translate it to English first. Then optimize the result into a vivid, descriptive English prompt: add relevant style cues, lighting descriptions, and quality keywords where appropriate. Output only the final English prompt, no explanation, no quotes, no preamble.",
			},
			{"role": "user", "content": text},
		},
		"temperature": 0.4,
		"max_tokens":  512,
	})

	req, err := http.NewRequestWithContext(tCtx, http.MethodPost, h.llmBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.llmAPIKey)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var llmResp struct {
		Choices []struct {
			Message struct{ Content string `json:"content"` } `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return "", err
	}
	return strings.TrimSpace(llmResp.Choices[0].Message.Content), nil
}
