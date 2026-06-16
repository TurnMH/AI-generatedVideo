package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

type optimizeVideoPromptRequest struct {
	Prompt        string `json:"prompt"`
	TargetModel   string `json:"target_model"`
	TextModel     string `json:"text_model"`
	Mode          string `json:"mode"`
	StylePreset   string `json:"style_preset"`
	AspectRatio   string `json:"aspect_ratio"`
	Duration      string `json:"duration"`
	GenerateAudio bool   `json:"generate_audio"`
}

// TranslatePrompt translates prompt text for UI or generation use.
// POST /api/v1/utils/translate  { "text": "...", "to": "en" | "zh" }
// - to=en (default): Chinese → English for image-generation prompts
// - to=zh: English/long prompt → readable Chinese for UI display
func (h *UtilsHandler) TranslatePrompt(c *gin.Context) {
	var req struct {
		Text string `json:"text"`
		To   string `json:"to"`
	}
	if err := c.BindJSON(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing text"})
		return
	}

	target := strings.ToLower(strings.TrimSpace(req.To))
	if target == "zh" {
		if isPrimarilyChinese(req.Text) {
			c.JSON(http.StatusOK, gin.H{"translated": strings.TrimSpace(req.Text)})
			return
		}
		translated, err := h.callLLMTranslateToChinese(c.Request.Context(), req.Text)
		if err != nil || translated == "" {
			c.JSON(http.StatusOK, gin.H{"translated": req.Text, "warning": "translation failed, original returned"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"translated": translated})
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

// isPrimarilyChinese returns true when text is mostly Chinese (not EN prompt + a few CN names).
func isPrimarilyChinese(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	han, latin, compact := 0, 0, 0
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r):
			han++
			compact++
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			latin++
			compact++
		case r != ' ' && r != '\n' && r != '\t' && r != '\r':
			compact++
		}
	}
	if han == 0 || latin > han || compact == 0 {
		return false
	}
	return float64(han)/float64(compact) >= 0.18
}

const (
	translateChineseChunkMax = 3200
	translateChineseTimeout  = 90 * time.Second
	translateChineseMaxTok   = 4096
)

// OptimizeVideoPrompt rewrites a manual video prompt for the selected video model and mode.
// POST /api/v1/utils/optimize-video-prompt
func (h *UtilsHandler) OptimizeVideoPrompt(c *gin.Context) {
	var req optimizeVideoPromptRequest
	if err := c.BindJSON(&req); err != nil || strings.TrimSpace(req.Prompt) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing prompt"})
		return
	}
	optimized, err := h.callLLMOptimizeVideoPrompt(c.Request.Context(), req)
	if err != nil || strings.TrimSpace(optimized) == "" {
		warning := "optimization failed, original returned"
		if err != nil {
			warning = "optimization failed: " + err.Error()
		}
		c.JSON(http.StatusOK, gin.H{"optimized": req.Prompt, "warning": warning})
		return
	}
	if strings.TrimSpace(optimized) == strings.TrimSpace(req.Prompt) {
		c.JSON(http.StatusOK, gin.H{"optimized": optimized, "warning": "optimizer returned same content"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"optimized": optimized})
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

func (h *UtilsHandler) callLLMTranslateToChinese(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("empty text")
	}
	if len(text) <= translateChineseChunkMax {
		return h.callLLMTranslateToChineseChunk(ctx, text)
	}
	chunks := splitTextForTranslation(text, translateChineseChunkMax)
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		translated, err := h.callLLMTranslateToChineseChunk(ctx, chunk)
		if err != nil {
			return "", err
		}
		out = append(out, translated)
	}
	return strings.TrimSpace(strings.Join(out, "\n\n")), nil
}

func splitTextForTranslation(text string, maxLen int) []string {
	text = strings.TrimSpace(text)
	if len(text) <= maxLen {
		return []string{text}
	}
	parts := strings.Split(text, "\n\n")
	var chunks []string
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		chunks = append(chunks, current.String())
		current.Reset()
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if len(part) > maxLen {
			flush()
			for start := 0; start < len(part); {
				end := start + maxLen
				if end > len(part) {
					end = len(part)
				} else if idx := strings.LastIndex(part[start:end], ". "); idx > maxLen/3 {
					end = start + idx + 1
				}
				chunks = append(chunks, strings.TrimSpace(part[start:end]))
				start = end
			}
			continue
		}
		extra := len(part)
		if current.Len() > 0 {
			extra += 2
		}
		if current.Len()+extra > maxLen {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(part)
	}
	flush()
	if len(chunks) == 0 {
		return []string{text}
	}
	return chunks
}

func (h *UtilsHandler) callLLMTranslateToChineseChunk(ctx context.Context, text string) (string, error) {
	tCtx, cancel := context.WithTimeout(ctx, translateChineseTimeout)
	defer cancel()

	body, _ := json.Marshal(map[string]interface{}{
		"model": h.llmModel,
		"messages": []map[string]string{
			{
				"role": "system",
				"content": "You are a professional translator for AI image-generation prompts. " +
					"Translate the user's English prompt into clear, readable Chinese for creators reviewing what the model received. " +
					"Preserve ALL constraints: style, character lock, camera, mood, continuity, negative instructions, and technical tokens. " +
					"Do NOT summarize or shorten — keep equivalent detail and length. " +
					"Use natural Chinese prose; you may use line breaks between sections. " +
					"Output only the Chinese translation, no explanation, no quotes, no preamble.",
			},
			{"role": "user", "content": text},
		},
		"temperature": 0.3,
		"max_tokens":  translateChineseMaxTok,
	})

	req, err := http.NewRequestWithContext(tCtx, http.MethodPost, h.llmBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.llmAPIKey)

	client := &http.Client{Timeout: translateChineseTimeout}
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

func (h *UtilsHandler) callLLMOptimizeVideoPrompt(ctx context.Context, reqBody optimizeVideoPromptRequest) (string, error) {
	tCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	textModel := sanitizeRequestedTextModel(reqBody.TextModel, h.llmModel)
	styleHint := "动漫风格"
	if strings.EqualFold(strings.TrimSpace(reqBody.StylePreset), "realistic") {
		styleHint = "真实环境 / 写实风格"
	}
	audioHint := "关闭"
	if reqBody.GenerateAudio {
		audioHint = "开启"
	}

	systemPrompt := "你是资深视频生成提示词导演与镜头设计师。你的任务不是点评，而是把用户的原始提示词强制重写成一段更完整、更可执行、更适合视频模型直接生成的最终提示词。输出只保留最终提示词正文，不要标题、不要分点、不要解释、不要引号。必须显式补足以下维度：主体身份与数量、主体在画面中的空间方位、场景环境、时间天气、镜头景别、机位与运镜、动作起止、表情姿态、光线氛围、材质细节、连续性约束。若原文过短，必须主动补全；若原文已较完整，也必须在不改变核心意图的前提下显著增强细节密度与镜头可执行性。除非原文已经是极高质量成片级提示词，否则不要原样返回。"
	userPrompt := fmt.Sprintf("请基于以下信息，直接重写出一版更强的视频生成提示词，目标是让结果比原文更具体、更有镜头感、更容易生成稳定画面。\n目标视频模型：%s\n文本优化模型：%s\n生成模式：%s\n风格：%s\n画幅比例：%s\n时长：%s\n原生音频：%s\n\n重写要求：\n1. 保留用户核心意图，不要偏题。\n2. 明确主体、环境、空间关系、动作顺序、镜头运动和画面氛围。\n3. 强化首尾动作衔接、连续性、时间推进与画面可执行性。\n4. 避免空泛词、堆砌形容词、互相冲突的描述。\n5. 如果原文太短，必须主动补足细节。\n6. 最终结果应明显不同于原文措辞，不要简单同义复述。\n\n用户原始提示词：\n%s", strings.TrimSpace(reqBody.TargetModel), textModel, strings.TrimSpace(reqBody.Mode), styleHint, strings.TrimSpace(reqBody.AspectRatio), strings.TrimSpace(reqBody.Duration), audioHint, strings.TrimSpace(reqBody.Prompt))

	body, _ := json.Marshal(map[string]interface{}{
		"model": textModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.5,
		"max_tokens":  700,
	})

	httpReq, err := http.NewRequestWithContext(tCtx, http.MethodPost, h.llmBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+h.llmAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
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

func sanitizeRequestedTextModel(requested, fallback string) string {
	candidate := strings.TrimSpace(requested)
	if candidate == "" {
		return fallback
	}
	lowered := strings.ToLower(candidate)
	if strings.ContainsAny(lowered, " \t\r\n\"'") {
		return fallback
	}
	if strings.Contains(lowered, "/") || strings.Contains(lowered, "\\") {
		return fallback
	}
	return candidate
}
