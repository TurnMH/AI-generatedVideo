package service

// SceneContinuityAuditor reviews groups of storyboards using an LLM and
// identifies continuity gaps (missing characters, empty locations, thin scene
// descriptions, abrupt transitions). It also automatically patches these gaps
// by inferring context from neighbouring storyboards in the same scene group.

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

// ── Public data types ─────────────────────────────────────────────────────────

// StoryboardForAudit is the lightweight view of a storyboard passed to the LLM.
type StoryboardForAudit struct {
	ID               uint64   `json:"id"`
	Seq              int      `json:"seq"`
	SceneDescription string   `json:"scene_description"`
	Characters       []string `json:"characters"`
	Location         string   `json:"location"`
	Mood             string   `json:"mood,omitempty"`
	CameraMovement   string   `json:"camera_movement,omitempty"`
}

// ContinuityPatch carries the suggested corrections for a single storyboard.
// Only fields with non-zero values should be applied; the caller must check
// Changed before writing to the database.
type ContinuityPatch struct {
	ID               uint64   `json:"id"`
	SceneDescription string   `json:"scene_description,omitempty"`
	Characters       []string `json:"characters,omitempty"`
	Location         string   `json:"location,omitempty"`
	Mood             string   `json:"mood,omitempty"`
	Issues           []string `json:"issues"` // human-readable explanations
	Changed          bool     `json:"changed"`
}

// ContinuityAuditResult summarises the outcome of auditing one project/episode.
type ContinuityAuditResult struct {
	TotalGroups  int               `json:"total_groups"`
	TotalPatched int               `json:"total_patched"`
	Patches      []ContinuityPatch `json:"patches"`
}

// ── Auditor ───────────────────────────────────────────────────────────────────

// SceneContinuityAuditor calls an LLM to review storyboard scene groups.
type SceneContinuityAuditor struct {
	llmBaseURL string
	llmAPIKey  string
	llmModel   string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewSceneContinuityAuditor creates an auditor.
func NewSceneContinuityAuditor(llmBaseURL, llmAPIKey, llmModel string, logger *zap.Logger) *SceneContinuityAuditor {
	if llmBaseURL == "" {
		llmBaseURL = "https://api.easyart.cc/v1"
	}
	if llmModel == "" {
		llmModel = "gpt-4.1-mini"
	}
	l := logger
	if l == nil {
		l = zap.NewNop()
	}
	return &SceneContinuityAuditor{
		llmBaseURL: strings.TrimRight(llmBaseURL, "/"),
		llmAPIKey:  llmAPIKey,
		llmModel:   llmModel,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		logger:     l,
	}
}

// ── System prompt ─────────────────────────────────────────────────────────────

const continuityAuditSystemPrompt = `你是一名资深分镜脚本顾问，专门负责检查 AI 影视项目的分镜连贯性并自动补全缺失信息。

你收到的是同一场景（scene_group）内的若干分镜，按顺序排列。你的任务：

**连贯性审查清单**
1. **角色缺失**：某帧 characters 为空，但上下文分镜中有角色出现 → 补充相同角色（除非场景描述明确说明无人）。
2. **地点缺失**：location 为空，但同组其他分镜有地点 → 从上下文推断并填入。
3. **描述过于单薄**：scene_description 少于 15 个字，或仅有"对话""走路"等无视觉信息的词 → 基于上下文扩充为 25-60 字的具体视觉描述（保留原意）。
4. **角色名拼写/称呼不一致**：同一角色在不同分镜中用了不同名字（全名/简称/外号）→ 统一为出现频率最高的称呼。
5. **情绪/氛围缺失**：mood 为空且描述中有明显情绪词 → 从 [tense, calm, joyful, sad, mysterious, romantic, epic, dramatic, fearful, hopeful] 中选一个合适的填入。
6. **叙事断层**：相邻两帧之间发生了明显的场景跳转但没有过渡描述 → 在 issues 中注明，不自动插帧（只标记）。

**修改原则**
- 只补充/更正有明确依据的信息，不凭空发明情节。
- 不修改 scene_description 的叙事核心，只在周边细节上扩充。
- 若一帧没有任何问题，设 changed: false，其他字段留空。

**输出格式（严格遵守）**：
{
  "patches": [
    {
      "id": <分镜ID>,
      "scene_description": "<仅在需要修改时填写，否则留空字符串>",
      "characters": [<仅在需要修改时填写，否则留空数组>],
      "location": "<仅在需要修改时填写，否则留空字符串>",
      "mood": "<仅在需要修改时填写，否则留空字符串>",
      "issues": ["<问题描述1>", "<问题描述2>"],
      "changed": true/false
    }
  ]
}`

// ── Core method ───────────────────────────────────────────────────────────────

// AuditGroup sends one scene group to the LLM and returns patches.
// groupKey is used only for logging. sbs must be sorted by sequence_number ASC.
func (a *SceneContinuityAuditor) AuditGroup(ctx context.Context, groupKey string, sbs []StoryboardForAudit) ([]ContinuityPatch, error) {
	if len(sbs) == 0 {
		return nil, nil
	}

	type auditPayload struct {
		SceneGroup  string               `json:"scene_group"`
		Storyboards []StoryboardForAudit `json:"storyboards"`
	}
	payload := auditPayload{
		SceneGroup:  groupKey,
		Storyboards: sbs,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal audit payload: %w", err)
	}

	userContent := fmt.Sprintf(
		"请审查以下场景组（%s）的 %d 帧分镜，检查连贯性并输出修补建议（JSON）：\n\n%s",
		groupKey, len(sbs), string(payloadJSON),
	)

	reqBody := map[string]any{
		"model": a.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": continuityAuditSystemPrompt},
			{"role": "user", "content": userContent},
		},
		"temperature":     0.2,
		"max_tokens":      4096,
		"response_format": map[string]string{"type": "json_object"},
	}
	data, _ := json.Marshal(reqBody)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build llm request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.llmAPIKey)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap
	if err != nil {
		return nil, fmt.Errorf("read llm response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm returned status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	// Unwrap OpenAI-compatible chat completion envelope.
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse llm envelope: %w", err)
	}
	if len(envelope.Choices) == 0 || envelope.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("empty llm response")
	}

	var result struct {
		Patches []ContinuityPatch `json:"patches"`
	}
	if err := json.Unmarshal([]byte(envelope.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("parse patches JSON: %w", err)
	}

	// Filter to only truly changed patches.
	var out []ContinuityPatch
	for _, p := range result.Patches {
		if p.Changed {
			out = append(out, p)
		}
	}

	a.logger.Info("continuity_auditor: group done",
		zap.String("group_key", groupKey),
		zap.Int("total", len(sbs)),
		zap.Int("patched", len(out)),
	)
	return out, nil
}

// ── Grouping helpers ──────────────────────────────────────────────────────────

// maxGroupBatchSize is the maximum number of storyboards sent to the LLM in one
// call to avoid exceeding context window / token limits.
const maxGroupBatchSize = 20

// GroupStoryboards partitions a flat ordered slice of storyboards into named
// groups suitable for continuity auditing.
//
// Strategy:
//  1. If any storyboard has a non-empty scene_group_key → group by that key.
//  2. Otherwise, group by common location (consecutive same-location runs).
//  3. If still ungrouped (no locations), emit a single group "episode".
//
// Large groups are split into batches of maxGroupBatchSize with a 2-frame overlap
// so the LLM retains context across batch boundaries.
func GroupStoryboards(sbs []StoryboardForAudit) map[string][]StoryboardForAudit {
	groups := make(map[string][]StoryboardForAudit)
	order := []string{}
	seen := make(map[string]bool)

	for _, sb := range sbs {
		key := sb.Location
		if key == "" {
			key = "default"
		}
		if !seen[key] {
			seen[key] = true
			order = append(order, key)
		}
		groups[key] = append(groups[key], sb)
	}

	// Split large groups into overlapping batches.
	result := make(map[string][]StoryboardForAudit)
	for _, key := range order {
		batch := groups[key]
		if len(batch) <= maxGroupBatchSize {
			result[key] = batch
			continue
		}
		// Split with 2-frame overlap.
		batchIdx := 0
		for start := 0; start < len(batch); start += maxGroupBatchSize - 2 {
			end := start + maxGroupBatchSize
			if end > len(batch) {
				end = len(batch)
			}
			batchKey := fmt.Sprintf("%s#%d", key, batchIdx)
			result[batchKey] = batch[start:end]
			batchIdx++
			if end == len(batch) {
				break
			}
		}
	}
	return result
}

// GroupStoryboardsBySceneKey groups storyboards using scene_group_key when available,
// falling back to location-based grouping for storyboards without a key.
// The caller must have set Location = SceneGroupKey for storyboards that have one.
func GroupStoryboardsBySceneKey(sbs []StoryboardForAudit) map[string][]StoryboardForAudit {
	hasSGKey := false
	for _, sb := range sbs {
		// AuditSceneContinuity sets Location = SceneGroupKey when the storyboard has one.
		// A non-empty Location that matches a scene_group_key pattern signals serial grouping.
		if sb.Location != "" {
			hasSGKey = true
			break
		}
	}
	if hasSGKey {
		// Use the Location field (which carries scene_group_key) directly for grouping.
		groups := make(map[string][]StoryboardForAudit)
		for _, sb := range sbs {
			key := sb.Location
			if key == "" {
				key = "default"
			}
			groups[key] = append(groups[key], sb)
		}
		return groups
	}
	return GroupStoryboards(sbs)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
