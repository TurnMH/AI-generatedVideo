package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// SerialFailureAnalysis holds the LLM's diagnosis of why a serial clip failed
// and actionable suggestions for the user to improve their storyboard.
type SerialFailureAnalysis struct {
	Reason      string   `json:"reason"`
	Suggestions []string `json:"suggestions"`
}

// SerialFailureAnalyzer uses an OpenAI-compatible LLM to diagnose serial clip
// generation failures and return user-friendly optimization suggestions.
// Returns nil from constructor when llmKey is empty; callers should nil-check.
type SerialFailureAnalyzer struct {
	llmKey   string
	llmBase  string
	llmModel string
	client   *http.Client
	logger   *zap.Logger
}

// NewSerialFailureAnalyzer creates an analyzer. Returns nil if llmKey is empty.
func NewSerialFailureAnalyzer(llmKey, llmBase, llmModel string, logger *zap.Logger) *SerialFailureAnalyzer {
	if llmKey == "" {
		return nil
	}
	if llmBase == "" {
		llmBase = "https://api.openai.com"
	}
	if llmModel == "" {
		llmModel = "gpt-4.1-mini"
	}
	normalized := strings.TrimRight(llmBase, "/")
	normalized = strings.TrimSuffix(normalized, "/v1")
	return &SerialFailureAnalyzer{
		llmKey:   llmKey,
		llmBase:  normalized,
		llmModel: llmModel,
		client:   &http.Client{Timeout: 60 * time.Second},
		logger:   logger,
	}
}

// Analyze diagnoses a serial clip failure and returns an optimization plan.
//
//   - groupKey:   scene group identifier (e.g. normalised location key)
//   - failedSeq:  0-based index of the failed clip within the group
//   - groupDescs: per-clip motion/scene descriptions for all clips in the group
//   - rawError:   the generation error message returned by the model API
//   - modelName:  video generation model name (for model-specific advice)
func (a *SerialFailureAnalyzer) Analyze(
	ctx context.Context,
	groupKey string,
	failedSeq int,
	groupDescs []string,
	rawError string,
	modelName string,
) (*SerialFailureAnalysis, error) {
	systemPrompt := `你是一位专业的 AI 视频生成工程师。
用户在使用「串行场景视频生成」功能时，某个分镜片段在重试3次后仍然失败，导致本场景后续分镜无法生成。
请根据场景描述和错误信息分析失败原因，并给出可操作的优化建议。

要求：
1. 失败原因：简明扼要，1-2句话，聚焦在内容/描述/模型限制层面
2. 优化建议：3-5条具体建议，包括分镜描述修改方向、拆分/合并策略或参数调整方向

请严格以 JSON 格式回复，结构如下（不含注释）：
{
  "reason": "...",
  "suggestions": ["...", "...", "..."]
}`

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("场景组标识：%s\n", groupKey))
	sb.WriteString(fmt.Sprintf("视频生成模型：%s\n", modelName))
	sb.WriteString(fmt.Sprintf("本场景共 %d 个分镜，失败分镜序号（0-based）：%d\n\n", len(groupDescs), failedSeq))
	sb.WriteString("各分镜描述：\n")
	for i, desc := range groupDescs {
		tag := ""
		if i == failedSeq {
			tag = " ← 失败分镜"
		}
		sb.WriteString(fmt.Sprintf("[%d]%s %s\n", i, tag, sfaTruncate(desc, 200)))
	}
	sb.WriteString(fmt.Sprintf("\n原始错误信息：\n%s", sfaTruncate(rawError, 500)))

	reqBody := map[string]any{
		"model": a.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": sb.String()},
		},
		"max_tokens":      1024,
		"temperature":     0.3,
		"response_format": map[string]string{"type": "json_object"},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal llm request: %w", err)
	}

	llmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(llmCtx, http.MethodPost,
		a.llmBase+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build llm request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.llmKey)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm http request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm status %d: %s", resp.StatusCode, sfaTruncate(string(body), 200))
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &completion); err != nil {
		return nil, fmt.Errorf("unmarshal llm response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("llm returned empty choices")
	}

	var result SerialFailureAnalysis
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("unmarshal analysis json: %w", err)
	}
	if result.Reason == "" {
		return nil, fmt.Errorf("llm returned empty reason")
	}
	return &result, nil
}

// sfaTruncate truncates s to at most maxLen runes.
func sfaTruncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
