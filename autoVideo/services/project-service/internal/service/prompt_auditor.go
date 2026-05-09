package service

// PromptAuditorService audits AI-generated storyboard and motion prompts
// before they are committed to the database. It runs three passes:
//  1. Rule-based sensitive word detection and safe replacement.
//  2. Jaccard similarity deduplication — flags near-duplicate prompts.
//  3. LLM reviewer pass — rewrites flagged prompts to fix sensitive content
//     and diversify duplicates, ensuring every prompt still produces output.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode"

	"go.uber.org/zap"
)

// AuditedPrompt is the result of auditing a single generated prompt.
type AuditedPrompt struct {
	Index    int      `json:"index"`
	Original string   `json:"original"`
	Final    string   `json:"final"`
	Flags    []string `json:"flags,omitempty"` // e.g. ["sensitive:blood","near_dup:3"]
	Changed  bool     `json:"changed"`
}

// PromptAuditorService centralises prompt quality control.
type PromptAuditorService struct {
	llmBaseURL     string
	llmAPIKey      string
	llmModel       string
	sensitiveWords []SensitiveEntry
	dupThreshold   float64 // Jaccard similarity threshold; default 0.72
	logger         *zap.Logger
}

// NewPromptAuditorService creates an auditor.
// Sensitive words default to defaultSensitiveEntries; pass nil to use defaults.
func NewPromptAuditorService(llmBaseURL, llmAPIKey, llmModel string, sensitiveWords []SensitiveEntry, logger *zap.Logger) *PromptAuditorService {
	sw := sensitiveWords
	if len(sw) == 0 {
		sw = defaultSensitiveEntries
	}
	if llmModel == "" {
		llmModel = "gpt-4.1-mini"
	}
	return &PromptAuditorService{
		llmBaseURL:     strings.TrimRight(llmBaseURL, "/"),
		llmAPIKey:      llmAPIKey,
		llmModel:       llmModel,
		sensitiveWords: sw,
		dupThreshold:   0.72,
		logger:         logger,
	}
}

// AuditBatch audits a batch of prompts and returns one AuditedPrompt per input.
// promptType is "image" or "motion" (used for logging only).
// Even if sensitive words are found, output is always produced — never blocked.
func (a *PromptAuditorService) AuditBatch(ctx context.Context, prompts []string, promptType string) []AuditedPrompt {
	n := len(prompts)
	results := make([]AuditedPrompt, n)
	for i, p := range prompts {
		results[i] = AuditedPrompt{Index: i, Original: p, Final: p}
	}

	// ── Pass 1: rule-based sensitive word replacement ────────────────────────
	for i := range results {
		cleaned, flags := a.scanAndReplace(results[i].Final)
		if len(flags) > 0 {
			results[i].Final = cleaned
			results[i].Flags = append(results[i].Flags, flags...)
			results[i].Changed = true
		}
	}

	// ── Pass 2: deduplication (Jaccard similarity) ───────────────────────────
	dupFlags := a.checkDeduplication(results)
	for i, flag := range dupFlags {
		if flag != "" {
			results[i].Flags = append(results[i].Flags, flag)
		}
	}

	// ── Pass 3: LLM reviewer — fix flagged prompts ───────────────────────────
	var flaggedIdx []int
	for i := range results {
		if len(results[i].Flags) > 0 {
			flaggedIdx = append(flaggedIdx, i)
		}
	}
	if len(flaggedIdx) > 0 {
		a.llmReviewFlagged(ctx, results, flaggedIdx, promptType)
	}

	// Log summary
	changed := 0
	for _, r := range results {
		if r.Changed {
			changed++
		}
	}
	if a.logger != nil && changed > 0 {
		a.logger.Info("prompt_auditor: audit complete",
			zap.String("type", promptType),
			zap.Int("total", n),
			zap.Int("changed", changed),
			zap.Int("flagged", len(flaggedIdx)),
		)
	}
	return results
}

// ── Pass 1: Sensitive word scan and replacement ─────────────────────────────

// scanAndReplace performs case-insensitive substring replacement of all sensitive
// entries. Returns the cleaned prompt and a list of flag strings.
func (a *PromptAuditorService) scanAndReplace(prompt string) (string, []string) {
	lower := strings.ToLower(prompt)
	result := prompt
	var flags []string
	for _, entry := range a.sensitiveWords {
		lw := strings.ToLower(entry.Word)
		if strings.Contains(lower, lw) {
			// Replace case-insensitively while preserving surrounding text.
			result = replaceCI(result, entry.Word, entry.Replacement)
			lower = strings.ToLower(result)
			flags = append(flags, "sensitive:"+entry.Category+":"+entry.Word)
		}
	}
	return result, flags
}

// replaceCI replaces all case-insensitive occurrences of old with new in s.
func replaceCI(s, old, replacement string) string {
	lowerS := strings.ToLower(s)
	lowerOld := strings.ToLower(old)
	var b strings.Builder
	for {
		idx := strings.Index(lowerS, lowerOld)
		if idx < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:idx])
		b.WriteString(replacement)
		s = s[idx+len(old):]
		lowerS = strings.ToLower(s)
	}
	return b.String()
}

// ── Pass 2: Deduplication ────────────────────────────────────────────────────

// checkDeduplication returns a flag string per prompt index.
// When prompt[i] is highly similar to an earlier prompt[j], it is flagged
// "near_dup:j" so the LLM reviewer knows to diversify it.
func (a *PromptAuditorService) checkDeduplication(results []AuditedPrompt) []string {
	flags := make([]string, len(results))
	for i := 1; i < len(results); i++ {
		for j := 0; j < i; j++ {
			sim := jaccardSimilarity(results[i].Final, results[j].Final)
			if sim >= a.dupThreshold {
				flags[i] = fmt.Sprintf("near_dup:%d:%.2f", j, sim)
				break
			}
		}
	}
	return flags
}

// jaccardSimilarity returns the Jaccard index (0–1) between the word-token sets
// of two strings. Tokens are lower-cased alphanumeric words.
func jaccardSimilarity(a, b string) float64 {
	setA := tokenSet(a)
	setB := tokenSet(b)
	if len(setA) == 0 && len(setB) == 0 {
		return 1.0
	}
	var inter int
	for w := range setA {
		if setB[w] {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return math.Round(float64(inter)/float64(union)*100) / 100
}

// tokenSet splits text into a set of lower-case alphanumeric words.
func tokenSet(text string) map[string]bool {
	set := map[string]bool{}
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			set[strings.ToLower(string(cur))] = true
			cur = cur[:0]
		}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
	return set
}

// ── Pass 3: LLM reviewer ─────────────────────────────────────────────────────

type llmReviewRequest struct {
	Index    int      `json:"index"`
	Prompt   string   `json:"prompt"`
	Issues   []string `json:"issues"`
}

type llmReviewResponse struct {
	Results []struct {
		Index  int    `json:"index"`
		Fixed  string `json:"fixed"`
		Action string `json:"action"` // "rewritten" | "diversified" | "unchanged"
	} `json:"results"`
}

// llmReviewFlagged calls the LLM to rewrite prompts that were flagged in passes 1–2.
// The LLM acts as a "senior visual content reviewer" persona.
func (a *PromptAuditorService) llmReviewFlagged(ctx context.Context, results []AuditedPrompt, idxs []int, promptType string) {
	var requests []llmReviewRequest
	for _, i := range idxs {
		requests = append(requests, llmReviewRequest{
			Index:  i,
			Prompt: results[i].Final,
			Issues: results[i].Flags,
		})
	}

	reqJSON, _ := json.Marshal(requests)

	systemPrompt := fmt.Sprintf(`你是一位资深影视内容审核员（AI提示词专员），负责对AI生成的%s提示词进行最终审核和修订。

你的职责：
1. 对标记了敏感词（sensitive:*）的提示词：用保留视觉意图的安全表达重写相关部分，**绝不能直接拒绝或返回空内容**，必须产出可用的视觉描述。
2. 对标记了近似重复（near_dup:*）的提示词：在保持该场景核心视觉元素的前提下，改变构图、光线、角度或时间细节，使其与原始相似提示词形成差异化。
3. 如果提示词既有敏感词又有重复问题，同时处理两个问题。
4. 未标记任何问题的提示词原样返回（action: "unchanged"）。

输出严格遵守以下JSON格式：
{
  "results": [
    {"index": <原始index>, "fixed": "<修订后的完整提示词>", "action": "rewritten|diversified|unchanged"}
  ]
}`, promptType)

	userContent := fmt.Sprintf("请审核以下 %d 条%s提示词并修订：\n\n%s\n\n返回JSON格式结果。",
		len(requests), promptType, string(reqJSON))

	reqBody := map[string]any{
		"model": a.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userContent},
		},
		"temperature":     0.3,
		"max_tokens":      8192,
		"response_format": map[string]string{"type": "json_object"},
	}

	data, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		if a.logger != nil {
			a.logger.Warn("prompt_auditor: build LLM request failed", zap.Error(err))
		}
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.llmAPIKey)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		if a.logger != nil {
			a.logger.Warn("prompt_auditor: LLM call failed", zap.Error(err))
		}
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != http.StatusOK {
		if a.logger != nil {
			a.logger.Warn("prompt_auditor: LLM non-200",
				zap.Int("status", resp.StatusCode),
				zap.String("body", string(body[:minInt(200, len(body))])),
			)
		}
		return
	}

	// Extract "content" from OpenAI-compatible response.
	var wrapper struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil || len(wrapper.Choices) == 0 {
		return
	}
	rawContent := wrapper.Choices[0].Message.Content

	var reviewResp llmReviewResponse
	if err := json.Unmarshal([]byte(rawContent), &reviewResp); err != nil {
		if a.logger != nil {
			a.logger.Warn("prompt_auditor: parse LLM response failed", zap.Error(err))
		}
		return
	}

	// Apply LLM fixes back to results.
	for _, r := range reviewResp.Results {
		if r.Index < 0 || r.Index >= len(results) {
			continue
		}
		if r.Fixed != "" && r.Action != "unchanged" {
			results[r.Index].Final = r.Fixed
			results[r.Index].Changed = true
			results[r.Index].Flags = append(results[r.Index].Flags, "llm_reviewed:"+r.Action)
		}
	}
}

// minInt is a local helper for integer minimum (avoids conflict with existing min helper).
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
