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

// MotionPromptService uses an OpenAI-compatible LLM to generate per-clip motion
// descriptions that are cinematically coherent across the full sequence.
// It replaces the rule-based clipMotionPrompt() output with holistic, scene-aware
// camera + action directions. Falls back gracefully to the rule-based path on any
// LLM error.
type MotionPromptService struct {
	llmKey   string
	llmBase  string
	llmModel string
	auditor  *MotionPromptAuditor
	logger   *zap.Logger
}

// NewMotionPromptService creates a MotionPromptService. Returns nil if llmKey is
// empty — callers should check for nil before calling RefineBatch.
func NewMotionPromptService(llmKey, llmBase, llmModel string, logger *zap.Logger) *MotionPromptService {
	if llmKey == "" {
		return nil
	}
	if llmBase == "" {
		llmBase = "https://api.openai.com"
	}
	if llmModel == "" {
		llmModel = "gpt-4.1-mini"
	}
	// Normalize base URL: accept both "https://host" and "https://host/v1" forms.
	// The endpoint path already includes /v1, so a trailing /v1 would produce
	// "/v1/v1/chat/completions" and 404.
	normalized := strings.TrimRight(llmBase, "/")
	normalized = strings.TrimSuffix(normalized, "/v1")
	return &MotionPromptService{
		llmKey:   llmKey,
		llmBase:  normalized,
		llmModel: llmModel,
		logger:   logger,
		auditor:  newMotionPromptAuditor(logger),
	}
}

// RefineBatch takes per-clip scene descriptions (already LLM-refined image prompts
// from the storyboard step) and returns per-clip VIDEO motion descriptions that
// consider continuity across the whole sequence.
//
// modelFamily should be the canonical family string returned by videoModelFamily().
// Chinese output is generated for "kling", "wan", "doubao", "vidu", "suanneng" families.
// On any error the function returns nil and the caller should fall back to clipMotionPrompt().
func (s *MotionPromptService) RefineBatch(
	ctx context.Context,
	perClipDescs []string,
	modelFamily string,
	motionMode string,
	stylePreset string,
	charDescriptions string,
) []string {
	if len(perClipDescs) == 0 {
		return nil
	}

	useChinese := modelFamily == "kling" || modelFamily == "wan" ||
		modelFamily == "doubao" || modelFamily == "vidu" || modelFamily == "suanneng"

	systemPrompt := s.buildSystemPrompt(useChinese, motionMode, stylePreset)
	userPrompt := s.buildUserPrompt(perClipDescs, charDescriptions, useChinese)

	reqBody := map[string]any{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  2048,
		"temperature": 0.6,
	}
	data, _ := json.Marshal(reqBody)

	llmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(llmCtx, http.MethodPost,
		s.llmBase+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		s.logger.Warn("motion_prompt: build request failed", zap.Error(err))
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.llmKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		s.logger.Warn("motion_prompt: LLM call failed", zap.Error(err))
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != http.StatusOK {
		s.logger.Warn("motion_prompt: LLM non-200",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)))
		return nil
	}

	content := extractLLMContent(body)
	if content == "" {
		s.logger.Warn("motion_prompt: empty LLM response")
		return nil
	}

	results, err := parseMotionPromptJSON(content, len(perClipDescs))
	if err != nil {
		preview := content
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		s.logger.Warn("motion_prompt: parse failed",
			zap.Error(err),
			zap.String("raw", preview))
		return nil
	}

	s.logger.Info("motion_prompt: LLM refined",
		zap.Int("clips", len(results)),
		zap.String("family", modelFamily))

	// Audit: sensitive word replacement + deduplication variation.
	if s.auditor != nil {
		results = s.auditor.Audit(results)
	}
	return results
}

func (s *MotionPromptService) buildSystemPrompt(useChinese bool, motionMode, stylePreset string) string {
	if useChinese {
		mode := motionModeZH(motionMode)
		return fmt.Sprintf(`你是一位AI视频生成序列的摄影指导，专攻镜头运动设计与画面连贯性。

你的核心任务是：为每个分镜片段生成精准的镜头运动描述，确保整个视频序列如同一部真实电影般流畅自然。

━━━━━━━━━━━━━━━━━━━━━━━━
【输出结构 — 每条描述必须覆盖以下层次】
━━━━━━━━━━━━━━━━━━━━━━━━
① 主体（画面中的核心人物或主体，简要说明）
② 景别（特写/近景/中景/全景/远景）+ 视角（平视/仰拍/俯拍/POV）
③ 运镜（推/拉/摇/跟/固定/环绕/手持等，描述具体机器运动轨迹）
④ 主体动作节拍（人物/主体在这段时间内具体做了什么，动词+结果）
⑤ 氛围基调（当前片段的情绪/视觉氛围，如"压抑紧张""温暖柔和"）
⑥ 构图提示（三分法/对称/斜线引导/框架，如有明确要求则写入）
⑦ 动态控制（动作速度/幅度/运动轨迹的节奏控制说明）
⑧ 衔接过渡方式（本镜头如何进入下一镜头：动作连续/切换/视线引导/匹配切）

━━━━━━━━━━━━━━━━━━━━━━━━
【流畅性法则 — 必须严格执行】
━━━━━━━━━━━━━━━━━━━━━━━━
1. 轴线法则：同一场景内，人物朝向不得在相邻镜头间无缘由反转；
2. 动作连续：若前镜头人物抬手，下一镜头应延续此动作（切入同动作的不同景别），不能突然静止；
3. 能量渐变：相邻分镜的运动强度不能突变（激烈动作后必须接一个静止或慢镜特写作为喘息）；
4. 视觉重心锚定：描述人物的画面位置（左/右/居中），避免相邻镜头人物位置无规律跳跃；
5. 景别衔接节奏：不能连续超过2个相同景别（特写/全景/中景需交替使用）；
6. 光线意识：运动描述中若涉及转身/移位，考虑光源方向变化对主体面部的影响。

━━━━━━━━━━━━━━━━━━━━━━━━
【内联摄影标注 — 最高优先级执行】
━━━━━━━━━━━━━━━━━━━━━━━━
场景描述中可能包含 [摄影:xxx] [景别:xxx] [运镜:xxx] [氛围:xxx] [构图:xxx] 格式的标注，必须忠实转化为运动指令（不得重新创作）：
  [摄影:低角度仰拍缓推] → "低角度仰拍，镜头以缓速向前推进，主体由小变大逐渐充满画面下半区"
  [摄影:跟焦特写] → "跟焦特写，随人物细微动作保持面部清晰，背景持续虚化柔化"

【当前运动模式】%s（%s）
【画面风格】%s

━━━━━━━━━━━━━━━━━━━━━━━━
【禁止清单】
━━━━━━━━━━━━━━━━━━━━━━━━
✗ 画面闪烁/帧抖动/身份漂移（服装/面貌跨镜头突变）
✗ 扭曲肢体/穿模（两人同框物理接触时）
✗ 相邻镜头运动强度突变超过3级（0-10分制）
✗ 连续超过2个完全静止镜头
✗ 镜头运动方向与人物动作方向完全对抗
✗ 在视觉描述中加入台词或内心独白

每条描述不超过80字，现在时态，动词开头。
只输出合法JSON数组，每个片段一个字符串，不附加任何说明文字：
["第0片段运动描述", "第1片段运动描述", ...]`, mode, motionMode, stylePreset)
	}

	mode := motionModeEN(motionMode)
	return fmt.Sprintf(`You are a cinematographer and motion director for an AI video generation pipeline, specializing in camera movement design and cross-clip visual continuity.

Your core task: write precise camera motion descriptions for each clip so the full sequence flows like a real film — no jarring cuts, no identity drift, no physics glitches.

━━━━━━━━━━━━━━━━━━━━━━━━
OUTPUT STRUCTURE — each description must cover these layers:
━━━━━━━━━━━━━━━━━━━━━━━━
① Subject — who/what is the focal element in the frame
② Shot size (close-up/medium/wide/extreme-wide) + Camera angle (eye-level/low/high/POV)
③ Camera movement (push/pull/pan/track/static/orbit — describe the specific motion trajectory)
④ Subject action beat (what the character/element physically does, verb + outcome)
⑤ Atmosphere/mood (emotional tone of the clip, e.g. "tense and oppressive", "warm and gentle")
⑥ Composition note (rule-of-thirds/symmetry/diagonal/frame-within-frame — if relevant)
⑦ Motion dynamics (speed / scale / trajectory rhythm control)
⑧ Transition cue to next clip (how this clip hands off: action continues / match cut / eyeline bridge / environmental cut)

━━━━━━━━━━━━━━━━━━━━━━━━
CONTINUITY LAWS — must be strictly followed:
━━━━━━━━━━━━━━━━━━━━━━━━
1. 180° Rule: within the same scene, character facing direction must not reverse between adjacent clips without a bridging cut;
2. Action Continuity: if a character raises their arm in clip N, clip N+1 should continue or resolve that gesture (not cut to a new unrelated action);
3. Energy Gradient: motion intensity must NOT jump more than 3 points (0-10 scale); high-action clips require a pause/close-up breath beat before the next intense action;
4. Frame Anchor: always specify where the subject is in the frame (left/right/center) to prevent random position jumps;
5. Shot Size Rhythm: avoid more than 2 consecutive clips at the same shot size; alternate wide/medium/close;
6. Light Consistency: if a character moves or turns, note how the lighting relationship to their face changes.

━━━━━━━━━━━━━━━━━━━━━━━━
INLINE CINEMATOGRAPHY ANNOTATIONS — HIGHEST PRIORITY:
━━━━━━━━━━━━━━━━━━━━━━━━
Scene descriptions may contain [摄影:xxx] director annotations — translate faithfully, do not reinvent:
  [摄影:低角度仰拍缓推] → "low-angle upward tilt, camera slowly pushes forward, subject grows to fill lower frame"
  [摄影:跟焦特写] → "tight tracking close-up, follow focus on micro-expressions, background progressively softens"

Motion mode: %s (%s) — calibrate energy accordingly
Visual style: %s

━━━━━━━━━━━━━━━━━━━━━━━━
FORBIDDEN:
━━━━━━━━━━━━━━━━━━━━━━━━
✗ Frame flicker / temporal jitter / identity drift (costume/face change mid-sequence)
✗ Anatomical distortion / mesh intersection (two characters in close physical contact in same frame)
✗ Adjacent clip motion intensity jump > 3 points
✗ More than 2 consecutive fully static clips
✗ Camera motion direction directly opposing subject movement direction
✗ Including dialogue or inner monologue in motion descriptions

Max 60 words per clip. Present tense, verb-first. Clips form a coherent narrative.
Output ONLY a valid JSON array of strings, one entry per clip, no additional text:
["motion for clip 0", "motion for clip 1", ...]`, mode, motionMode, stylePreset)
}

func (s *MotionPromptService) buildUserPrompt(descs []string, charDescriptions string, useChinese bool) string {
	var sb strings.Builder
	if strings.TrimSpace(charDescriptions) != "" {
		if useChinese {
			sb.WriteString("【角色身份锁定】（镜头运动不得造成身份漂移，服饰/年代/外貌特征必须贯穿全部片段保持一致）：\n")
			sb.WriteString(strings.TrimSpace(charDescriptions))
			sb.WriteString("\n\n")
		} else {
			sb.WriteString("[CHARACTER IDENTITY LOCK] (motion must NOT cause identity drift; wardrobe/era/appearance must remain consistent across all clips):\n")
			sb.WriteString(strings.TrimSpace(charDescriptions))
			sb.WriteString("\n\n")
		}
	}
	if useChinese {
		sb.WriteString(fmt.Sprintf("总片段数：%d\n\n场景序列：\n", len(descs)))
	} else {
		sb.WriteString(fmt.Sprintf("Total clips: %d\n\nScene sequence:\n", len(descs)))
	}
	for i, d := range descs {
		if useChinese {
			sb.WriteString(fmt.Sprintf("[片段%d] %s\n", i, d))
		} else {
			sb.WriteString(fmt.Sprintf("[Clip %d] %s\n", i, d))
		}
	}
	return sb.String()
}

// extractLLMContent pulls the assistant message content from an OpenAI-format response body.
func extractLLMContent(body []byte) string {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || len(resp.Choices) == 0 {
		return ""
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content)
}

// parseMotionPromptJSON extracts a []string JSON array from the LLM response,
// tolerating markdown code fences and minor surrounding text.
func parseMotionPromptJSON(content string, expected int) ([]string, error) {
	// Strip markdown code fences if present.
	if idx := strings.Index(content, "["); idx >= 0 {
		content = content[idx:]
	}
	if idx := strings.LastIndex(content, "]"); idx >= 0 {
		content = content[:idx+1]
	}

	var result []string
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("empty array")
	}

	// Pad or trim to match expected length.
	for len(result) < expected {
		result = append(result, result[len(result)-1])
	}
	return result[:expected], nil
}

func motionModeZH(mode string) string {
	switch mode {
	case "dynamic":
		return "动感模式 — 快节奏、高能量镜头运动"
	case "cinematic":
		return "电影模式 — 缓慢克制、优雅镜头语言"
	default:
		return "自然模式 — 平滑自然、均衡运动"
	}
}

func motionModeEN(mode string) string {
	switch mode {
	case "dynamic":
		return "dynamic — fast-paced, energetic camera, dramatic action"
	case "cinematic":
		return "cinematic — slow, deliberate, controlled lens language"
	default:
		return "normal — smooth, natural, balanced movement"
	}
}
