package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/autovideo/agent-service/internal/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AgentService struct {
	logger       *zap.Logger
	defaultModel string
	systemPrompt string
	tools        *ToolRegistry
	executions   ExecutionStore
}

func NewAgentService(logger *zap.Logger, defaultModel, systemPrompt string, tools *ToolRegistry, executions ExecutionStore) *AgentService {
	if executions == nil {
		executions = NewExecutionStore()
	}
	return &AgentService{
		logger:       logger,
		defaultModel: defaultModel,
		systemPrompt: systemPrompt,
		tools:        tools,
		executions:   executions,
	}
}

func (s *AgentService) ListTools() []model.ToolDefinition {
	return s.tools.Definitions()
}

func (s *AgentService) BuildPlan(_ context.Context, req model.PlanRequest) (*model.Plan, error) {
	goal := strings.TrimSpace(req.Goal)
	if goal == "" {
		return nil, fmt.Errorf("goal is required")
	}
	planID := uuid.NewString()
	mode := firstString(stringConstraint(req.Constraints, "mode"), stringContext(req.Context, "mode"), "story_video")
	stylePreset := firstString(
		stringConstraint(req.Constraints, "style_preset"),
		stringConstraint(req.Constraints, "style"),
		stringContext(req.Context, "style_preset"),
		"anime-2d",
	)
	motionMode := firstString(
		stringConstraint(req.Constraints, "motion_mode"),
		stringContext(req.Context, "motion_mode"),
		"gentle",
	)
	modelName := firstString(
		stringConstraint(req.Constraints, "model_name"),
		stringContext(req.Context, "model_name"),
		"kling",
	)
	videoEpisodes := buildVideoEpisodes(req)
	if mode == "commentary_comic" {
		applyCommentaryEpisodes(videoEpisodes, req)
	}
	steps := buildPlanSteps(req, goal, mode, stylePreset, motionMode, modelName, videoEpisodes)
	plan := &model.Plan{
		PlanID:      planID,
		Goal:        goal,
		ProjectID:   req.ProjectID,
		EpisodeID:   req.EpisodeID,
		Status:      "draft",
		Agent:       "orchestrator-agent",
		Model:       s.defaultModel,
		CreatedAt:   time.Now().UTC(),
		Constraints: req.Constraints,
		Context: map[string]any{
			"system_prompt": s.systemPrompt,
			"user_context":  req.Context,
		},
		Steps: steps,
	}
	s.logger.Info("agent plan built",
		zap.String("plan_id", planID),
		zap.Int64("project_id", req.ProjectID),
		zap.Int("steps", len(steps)))
	return plan, nil
}

func (s *AgentService) ExecutePlan(ctx context.Context, req model.ExecutePlanRequest) (*model.ExecutePlanResponse, error) {
	return s.executePlanWithSeed(ctx, req.Plan, nil)
}

func (s *AgentService) GetExecution(executionID string) (*ExecutionRecord, bool) {
	return s.executions.Get(executionID)
}

func (s *AgentService) ListExecutions(planID string, filter ExecutionListFilter) []ExecutionRecord {
	return s.executions.List(planID, filter)
}

func (s *AgentService) RetryExecution(ctx context.Context, executionID string) (*model.RetryExecutionResponse, error) {
	record, ok := s.executions.Get(executionID)
	if !ok {
		return nil, fmt.Errorf("execution %q not found", executionID)
	}
	resp, err := s.ExecutePlan(ctx, model.ExecutePlanRequest{Plan: clonePlan(record.Plan)})
	if err != nil {
		return nil, err
	}
	return &model.RetryExecutionResponse{
		SourceExecutionID: executionID,
		Retried:           resp,
	}, nil
}

func (s *AgentService) ResumeExecution(ctx context.Context, executionID string) (*model.ResumeExecutionResponse, error) {
	record, ok := s.executions.Get(executionID)
	if !ok {
		return nil, fmt.Errorf("execution %q not found", executionID)
	}
	seedResults, resultByStep, resumedFromStepID, skippedStepIDs := buildResumeSeed(record.Plan, record.Results)
	resp, err := s.executePlanWithSeed(ctx, clonePlan(record.Plan), seedResults)
	if err != nil {
		return nil, err
	}
	if resumedFromStepID == "" {
		resumedFromStepID = firstPendingStepID(record.Plan, resultByStep)
	}
	return &model.ResumeExecutionResponse{
		SourceExecutionID: executionID,
		ResumedFromStepID: resumedFromStepID,
		SkippedStepIDs:    skippedStepIDs,
		Resumed:           resp,
	}, nil
}

func (s *AgentService) ReplayFromStep(ctx context.Context, executionID, stepID string) (*model.ReplayStepExecutionResponse, error) {
	record, ok := s.executions.Get(executionID)
	if !ok {
		return nil, fmt.Errorf("execution %q not found", executionID)
	}
	seedResults, skippedStepIDs, err := buildReplaySeed(record.Plan, record.Results, stepID)
	if err != nil {
		return nil, err
	}
	resp, err := s.executePlanWithSeed(ctx, clonePlan(record.Plan), seedResults)
	if err != nil {
		return nil, err
	}
	return &model.ReplayStepExecutionResponse{
		SourceExecutionID: executionID,
		ReplayFromStepID:  stepID,
		SkippedStepIDs:    skippedStepIDs,
		Replayed:          resp,
	}, nil
}

func (s *AgentService) executePlanWithSeed(ctx context.Context, plan model.Plan, seedResults []model.StepResult) (*model.ExecutePlanResponse, error) {
	record := s.executions.Start(plan)
	results := cloneStepResults(seedResults)
	resultByStep := make(map[string]model.StepResult, len(plan.Steps))
	for _, result := range results {
		resultByStep[result.StepID] = result
	}
	status := "succeeded"

	for _, step := range plan.Steps {
		if existing, ok := resultByStep[step.ID]; ok && existing.Status == "succeeded" {
			continue
		}
		started := time.Now().UTC()
		resolvedInput := cloneMap(step.Input)
		dependencyInfo, depErr := resolveDependencies(step, resultByStep, resolvedInput)
		result := model.StepResult{
			StepID:         step.ID,
			Tool:           step.Tool,
			ResolvedInput:  resolvedInput,
			DependencyInfo: dependencyInfo,
			StartedAt:      started,
		}
		if depErr != nil {
			result.Status = "failed"
			result.Message = depErr.Error()
			result.EndedAt = time.Now().UTC()
			status = "failed"
			results = append(results, result)
			resultByStep[step.ID] = result
			s.executions.Update(record.ExecutionID, status, results)
			break
		}

		output, err := s.tools.Execute(ctx, step.Tool, resolvedInput)
		result.EndedAt = time.Now().UTC()
		if err != nil {
			result.Status = "failed"
			result.Message = err.Error()
			status = "failed"
			results = append(results, result)
			resultByStep[step.ID] = result
			s.executions.Update(record.ExecutionID, status, results)
			break
		}
		result.Status = "succeeded"
		result.Message = fmt.Sprintf("tool %s executed", step.Tool)
		result.Output = output
		results = append(results, result)
		resultByStep[step.ID] = result
		s.executions.Update(record.ExecutionID, status, results)
	}

	stored := s.executions.Update(record.ExecutionID, status, results)
	resp := &model.ExecutePlanResponse{
		PlanID:      plan.PlanID,
		ExecutionID: record.ExecutionID,
		Status:      status,
		Results:     results,
	}
	if stored != nil {
		resp.ExecutionMeta = map[string]any{
			"created_at": stored.CreatedAt,
			"updated_at": stored.UpdatedAt,
			"step_count": len(stored.Plan.Steps),
			"seeded_steps": len(seedResults),
		}
	}
	return resp, nil
}

func buildResumeSeed(plan model.Plan, previousResults []model.StepResult) ([]model.StepResult, map[string]model.StepResult, string, []string) {
	previousByStep := make(map[string]model.StepResult, len(previousResults))
	for _, result := range previousResults {
		previousByStep[result.StepID] = result
	}
	seedResults := make([]model.StepResult, 0, len(previousResults))
	resultByStep := make(map[string]model.StepResult, len(previousResults))
	skippedStepIDs := make([]string, 0, len(previousResults))
	resumedFromStepID := ""
	for _, step := range plan.Steps {
		prev, ok := previousByStep[step.ID]
		if !ok || prev.Status != "succeeded" {
			resumedFromStepID = step.ID
			break
		}
		seedResults = append(seedResults, prev)
		resultByStep[step.ID] = prev
		skippedStepIDs = append(skippedStepIDs, step.ID)
	}
	return seedResults, resultByStep, resumedFromStepID, skippedStepIDs
}

func buildReplaySeed(plan model.Plan, previousResults []model.StepResult, replayFromStepID string) ([]model.StepResult, []string, error) {
	replayFromStepID = strings.TrimSpace(replayFromStepID)
	if replayFromStepID == "" {
		return nil, nil, fmt.Errorf("step id is required")
	}
	previousByStep := make(map[string]model.StepResult, len(previousResults))
	for _, result := range previousResults {
		previousByStep[result.StepID] = result
	}
	seedResults := make([]model.StepResult, 0, len(previousResults))
	skippedStepIDs := make([]string, 0, len(previousResults))
	found := false
	for _, step := range plan.Steps {
		if step.ID == replayFromStepID {
			found = true
			break
		}
		prev, ok := previousByStep[step.ID]
		if !ok || prev.Status != "succeeded" {
			return nil, nil, fmt.Errorf("cannot replay from step %q: prerequisite step %q is not succeeded in source execution", replayFromStepID, step.ID)
		}
		seedResults = append(seedResults, prev)
		skippedStepIDs = append(skippedStepIDs, step.ID)
	}
	if !found {
		return nil, nil, fmt.Errorf("step %q not found in plan", replayFromStepID)
	}
	return seedResults, skippedStepIDs, nil
}

func firstPendingStepID(plan model.Plan, resultByStep map[string]model.StepResult) string {
	for _, step := range plan.Steps {
		if _, ok := resultByStep[step.ID]; !ok {
			return step.ID
		}
	}
	return ""
}

func resolveDependencies(step model.PlanStep, resultByStep map[string]model.StepResult, resolvedInput map[string]any) ([]string, error) {
	info := make([]string, 0, len(step.DependsOn)+len(step.UseResultFields))
	for _, dep := range step.DependsOn {
		res, ok := resultByStep[dep]
		if !ok {
			return info, fmt.Errorf("dependency %q has not executed", dep)
		}
		if res.Status != "succeeded" {
			return info, fmt.Errorf("dependency %q did not succeed", dep)
		}
		info = append(info, fmt.Sprintf("dependency %s satisfied", dep))
	}
	for inputKey, refRaw := range step.UseResultFields {
		ref, ok := refRaw.(string)
		if !ok || strings.TrimSpace(ref) == "" {
			return info, fmt.Errorf("invalid result mapping for %q", inputKey)
		}
		value, err := resolveResultReference(ref, resultByStep)
		if err != nil {
			return info, err
		}
		value = adaptResolvedValue(step.Tool, inputKey, value, resolvedInput)
		if shouldSkipResolvedValue(value) {
			info = append(info, fmt.Sprintf("skipped mapping %s <- %s (empty adaptation)", inputKey, ref))
			continue
		}
		resolvedInput[inputKey] = value
		info = append(info, fmt.Sprintf("mapped %s <- %s", inputKey, ref))
	}
	return info, nil
}

func resolveResultReference(ref string, resultByStep map[string]model.StepResult) (any, error) {
	parts := strings.Split(strings.TrimSpace(ref), ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("invalid result reference %q", ref)
	}
	res, ok := resultByStep[parts[0]]
	if !ok {
		return nil, fmt.Errorf("referenced step %q not found", parts[0])
	}
	if res.Status != "succeeded" {
		return nil, fmt.Errorf("referenced step %q did not succeed", parts[0])
	}
	if len(parts) == 1 {
		return res.Output, nil
	}
	current := any(res.Output)
	for _, key := range parts[1:] {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("reference %q is not a map at %q", ref, key)
		}
		value, ok := obj[key]
		if !ok {
			return nil, fmt.Errorf("reference %q missing field %q", ref, key)
		}
		current = value
	}
	return current, nil
}

func adaptResolvedValue(toolName, inputKey string, value any, resolvedInput map[string]any) any {
	switch toolName {
	case "video.generate_batch":
		return adaptVideoGenerateBatchInput(inputKey, value, resolvedInput)
	default:
		return value
	}
}

func adaptVideoGenerateBatchInput(inputKey string, value any, resolvedInput map[string]any) any {
	switch inputKey {
	case "scene_descriptions":
		if scenes := buildSceneDescriptions(value); len(scenes) > 0 {
			ensureEpisodePayloadShape(resolvedInput, len(scenes))
			applyToEpisodeEntries(resolvedInput, func(ep map[string]any) {
				ep["scene_descriptions"] = scenes
			})
			return scenes
		}
	case "dialogues":
		if dialogues := buildDialogues(value); len(dialogues) > 0 {
			ensureEpisodePayloadShape(resolvedInput, len(dialogues))
			applyToEpisodeEntries(resolvedInput, func(ep map[string]any) {
				ep["dialogues"] = dialogues
			})
			return dialogues
		}
	case "image_batch":
		if imageURLs := buildImageURLs(value); len(imageURLs) > 0 {
			ensureEpisodePayloadShape(resolvedInput, len(imageURLs))
			applyToEpisodeEntries(resolvedInput, func(ep map[string]any) {
				ep["image_urls"] = imageURLs
			})
			return imageURLs
		}
	case "subtitle_text":
		if subtitle := buildSubtitleText(value); subtitle != "" {
			ensureEpisodePayloadShape(resolvedInput, 1)
			applyToEpisodeEntries(resolvedInput, func(ep map[string]any) {
				ep["subtitle_text"] = subtitle
			})
			return subtitle
		}
	case "subtitle_timeline":
		if timeline := buildSubtitleTimeline(value); len(timeline) > 0 {
			ensureEpisodePayloadShape(resolvedInput, len(timeline))
			applyToEpisodeEntries(resolvedInput, func(ep map[string]any) {
				ep["subtitle_timeline"] = timeline
			})
			return timeline
		}
	case "audio_url":
		if audioURL := stringValue(value); audioURL != "" {
			ensureEpisodePayloadShape(resolvedInput, 1)
			applyToEpisodeEntries(resolvedInput, func(ep map[string]any) {
				ep["audio_url"] = audioURL
			})
			return audioURL
		}
	case "camera_movements":
		if cameras := buildShotMetadataStrings(value, "camera"); len(cameras) > 0 {
			ensureEpisodePayloadShape(resolvedInput, len(cameras))
			applyToEpisodeEntries(resolvedInput, func(ep map[string]any) {
				ep["camera_movements"] = cameras
			})
			return cameras
		}
	case "moods":
		if moods := buildShotMetadataStrings(value, "emotion"); len(moods) > 0 {
			ensureEpisodePayloadShape(resolvedInput, len(moods))
			applyToEpisodeEntries(resolvedInput, func(ep map[string]any) {
				ep["moods"] = moods
			})
			return moods
		}
	case "transition_plan":
		if transitions := buildShotMetadataStrings(value, "transition"); len(transitions) > 0 {
			ensureEpisodePayloadShape(resolvedInput, len(transitions))
			applyToEpisodeEntries(resolvedInput, func(ep map[string]any) {
				ep["transition_plan"] = transitions
			})
			return transitions
		}
	case "character_focus":
		if characterFocus := buildShotMetadataStrings(value, "character_focus"); len(characterFocus) > 0 {
			ensureEpisodePayloadShape(resolvedInput, len(characterFocus))
			applyToEpisodeEntries(resolvedInput, func(ep map[string]any) {
				ep["character_focus"] = characterFocus
			})
			return characterFocus
		}
	case "costume_hints":
		if costumeHints := buildShotMetadataStrings(value, "costume_hint"); len(costumeHints) > 0 {
			ensureEpisodePayloadShape(resolvedInput, len(costumeHints))
			applyToEpisodeEntries(resolvedInput, func(ep map[string]any) {
				ep["costume_hints"] = costumeHints
			})
			return costumeHints
		}
	case "location_tags":
		if locationTags := buildShotMetadataStrings(value, "location_tag"); len(locationTags) > 0 {
			ensureEpisodePayloadShape(resolvedInput, len(locationTags))
			applyToEpisodeEntries(resolvedInput, func(ep map[string]any) {
				ep["location_tags"] = locationTags
			})
			return locationTags
		}
	}
	return value
}

func buildSceneDescriptions(value any) []string {
	switch v := value.(type) {
	case string:
		return splitScriptContent(v)
	case []string:
		return trimStringSlice(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if entry, ok := item.(map[string]any); ok {
				if text := firstString(stringValue(entry["scene_description"]), stringValue(entry["prompt"]), stringValue(entry["visual_desc"]), stringValue(entry["description"])); text != "" {
					out = append(out, text)
				}
				continue
			}
			if text := stringValue(item); text != "" {
				out = append(out, text)
			}
		}
		return trimStringSlice(out)
	default:
		return nil
	}
}

func buildDialogues(value any) []string {
	switch v := value.(type) {
	case string:
		return splitScriptContent(v)
	case []string:
		return trimStringSlice(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if entry, ok := item.(map[string]any); ok {
				if text := firstString(stringValue(entry["dialogue"]), stringValue(entry["subtitle_candidate"]), stringValue(entry["subtitle"]), stringValue(entry["text"])); text != "" {
					out = append(out, text)
				}
				continue
			}
			if text := stringValue(item); text != "" {
				out = append(out, text)
			}
		}
		return trimStringSlice(out)
	default:
		return nil
	}
}

func buildShotMetadataStrings(value any, key string) []string {
	switch v := value.(type) {
	case []string:
		return trimStringSlice(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			entry, ok := item.(map[string]any)
			if !ok {
				if text := stringValue(item); text != "" {
					out = append(out, text)
				}
				continue
			}
			if text := stringValue(entry[key]); text != "" {
				out = append(out, text)
			}
		}
		return trimStringSlice(out)
	default:
		return nil
	}
}

func buildImageURLs(value any) []string {
	switch v := value.(type) {
	case []string:
		return trimStringSlice(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if entry, ok := item.(map[string]any); ok {
				if generation, ok := entry["generation"].(map[string]any); ok {
					if url := firstString(stringValue(generation["image_url"]), stringValue(generation["url"]), stringValue(generation["result_url"])); url != "" {
						out = append(out, url)
						continue
					}
				}
				if url := firstString(stringValue(entry["image_url"]), stringValue(entry["url"]), stringValue(entry["result_url"])); url != "" {
					out = append(out, url)
				}
				continue
			}
			if text := stringValue(item); text != "" {
				out = append(out, text)
			}
		}
		return trimStringSlice(out)
	default:
		return nil
	}
}

func buildSubtitleTimeline(value any) []map[string]any {
	raw, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]map[string]any); ok {
			return typed
		}
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for idx, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			if text := stringValue(item); text != "" {
				out = append(out, map[string]any{"index": idx, "text": text})
			}
			continue
		}
		text := firstString(stringValue(entry["text"]), stringValue(entry["subtitle"]), stringValue(entry["content"]))
		if text == "" {
			continue
		}
		out = append(out, map[string]any{
			"index":     firstString(stringValue(entry["index"]), fmt.Sprintf("%d", idx)),
			"text":      text,
			"start_sec": entry["start_sec"],
			"end_sec":   entry["end_sec"],
		})
	}
	return out
}

func buildSubtitleText(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []string:
		return strings.Join(trimStringSlice(v), "\n")
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := stringValue(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func splitScriptContent(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	chunks := strings.Split(content, "\n\n")
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk != "" {
			out = append(out, chunk)
		}
	}
	if len(out) > 0 {
		return out
	}
	return trimStringSlice(strings.Split(content, "\n"))
}

func trimStringSlice(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func ensureEpisodePayloadShape(resolvedInput map[string]any, sceneCount int) {
	episodes, ok := resolvedInput["episodes"].([]map[string]any)
	if ok && len(episodes) > 0 {
		return
	}
	episode := map[string]any{}
	if projectID, ok := resolvedInput["episode_id"]; ok {
		episode["episode_id"] = projectID
	}
	if prompt := stringValue(resolvedInput["goal"]); prompt != "" {
		episode["prompt"] = prompt
	}
	if sceneCount > 0 {
		episode["image_urls"] = make([]string, sceneCount)
	}
	resolvedInput["episodes"] = []map[string]any{episode}
}

func applyToEpisodeEntries(resolvedInput map[string]any, fn func(map[string]any)) {
	episodes, ok := resolvedInput["episodes"].([]map[string]any)
	if !ok {
		return
	}
	for _, ep := range episodes {
		fn(ep)
	}
}

func shouldSkipResolvedValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case []string:
		return len(v) == 0
	}
	return false
}

func boolConstraint(m map[string]any, key string) bool {
	return boolValue(m[key])
}

func boolContext(m map[string]any, key string) bool {
	return boolValue(m[key])
}

func int64Constraint(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	default:
		return 0
	}
}

func stringConstraint(m map[string]any, key string) string {
	return stringValue(m[key])
}

func stringContext(m map[string]any, key string) string {
	return stringValue(m[key])
}

func stringValue(v any) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case fmt.Stringer:
		return strings.TrimSpace(val.String())
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	default:
		return ""
	}
}

func firstString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func buildVideoEpisodes(req model.PlanRequest) []map[string]any {
	episode := map[string]any{}
	if req.EpisodeID > 0 {
		episode["episode_id"] = req.EpisodeID
	}
	episode["title"] = firstString(stringContext(req.Context, "episode_title"), stringContext(req.Context, "title"), fmt.Sprintf("Episode %d", req.EpisodeID))
	episode["prompt"] = firstString(stringContext(req.Context, "video_prompt"), req.Goal)
	if duration := firstString(stringConstraint(req.Constraints, "duration_sec"), stringConstraint(req.Constraints, "episode_duration"), stringContext(req.Context, "episode_duration")); duration != "" {
		episode["duration"] = duration
	}
	return []map[string]any{episode}
}

func buildPlanSteps(req model.PlanRequest, goal, mode, stylePreset, motionMode, modelName string, videoEpisodes []map[string]any) []model.PlanStep {
	authorization := firstString(stringContext(req.Context, "authorization"))
	scriptMode := firstString(stringContext(req.Context, "script_mode"), "script")
	platform := firstString(stringContext(req.Context, "platform"), "short_drama")
	if mode == "commentary_comic" {
		scriptMode = firstString(stringContext(req.Context, "script_mode"), "narration_script")
		platform = firstString(stringContext(req.Context, "platform"), "commentary_comic")
	}

	steps := []model.PlanStep{
		{
			ID:          "step-1",
			Agent:       "orchestrator-agent",
			Tool:        "project.get",
			Description: "Load project context and current production state.",
			Status:      "pending",
			Input: map[string]any{
				"project_id": req.ProjectID,
				"episode_id": req.EpisodeID,
				"user_id":    req.UserID,
			},
			Expected: []string{"project metadata", "existing episodes", "current task state"},
		},
		{
			ID:          "step-2",
			Agent:       "storyboard-agent",
			Tool:        "script.generate",
			Description: "Generate or refine script/storyboard draft from the goal.",
			Status:      "pending",
			DependsOn:   []string{"step-1"},
			Input: map[string]any{
				"mode":             scriptMode,
				"goal":             goal,
				"title":            firstString(stringContext(req.Context, "title"), fmt.Sprintf("Project %d Episode %d", req.ProjectID, req.EpisodeID)),
				"genre":            firstString(stringConstraint(req.Constraints, "genre"), stringContext(req.Context, "genre"), stylePreset),
				"platform":         platform,
				"delivery_format":  firstString(stringContext(req.Context, "delivery_format"), "video"),
				"episode_duration": firstString(stringConstraint(req.Constraints, "episode_duration"), stringConstraint(req.Constraints, "duration_sec"), stringContext(req.Context, "episode_duration"), "30s"),
				"reference_style":  stylePreset,
				"premise":          goal,
				"chapter_brief":    firstString(stringContext(req.Context, "chapter_brief"), goal),
				"source_text":      firstString(stringContext(req.Context, "source_text"), stringContext(req.Context, "script_text"), goal),
				"audience":         stringContext(req.Context, "audience"),
				"tone":             firstString(stringConstraint(req.Constraints, "tone"), stringContext(req.Context, "tone")),
				"constraints":      req.Constraints,
				"context":          req.Context,
			},
			UseResultFields: map[string]any{
				"project_context": "step-1.project",
			},
			Expected: []string{"script draft", "scene list", "storyboard direction"},
		},
	}

	if mode == "commentary_comic" {
		steps = append(steps, model.PlanStep{
			ID:          "step-3",
			Agent:       "narration-agent",
			Tool:        "dubbing.generate",
			Description: "Generate narration dubbing metadata for commentary comic mode.",
			Status:      "pending",
			DependsOn:   []string{"step-2"},
			Input: map[string]any{
				"project_id":    req.ProjectID,
				"episode_id":    req.EpisodeID,
				"user_id":       req.UserID,
				"authorization": authorization,
				"voice_style":   firstString(stringConstraint(req.Constraints, "voice_style"), stringContext(req.Context, "voice_style"), "calm_narration"),
				"language":      firstString(stringConstraint(req.Constraints, "language"), stringContext(req.Context, "language"), "zh-CN"),
			},
			UseResultFields: map[string]any{
				"script_dialogues": "step-2.script_dialogues",
				"script_content":   "step-2.script.content",
			},
			Expected: []string{"audio url", "subtitle timeline", "narration segments"},
		})
		steps = append(steps, model.PlanStep{
			ID:          "step-4",
			Agent:       "shot-planning-agent",
			Tool:        "shot_plan.generate",
			Description: "Expand scene-level script/storyboard into shot-level plan for commentary comic mode.",
			Status:      "pending",
			DependsOn:   []string{"step-2"},
			Input: map[string]any{
				"shot_density":      firstString(stringConstraint(req.Constraints, "shot_density"), stringContext(req.Context, "shot_density"), "medium"),
				"target_shot_count": int64Constraint(req.Constraints, "target_shot_count"),
				"character_name":    firstString(stringConstraint(req.Constraints, "character_name"), stringContext(req.Context, "character_name"), stringContext(req.Context, "main_character")),
				"character_focus":   firstString(stringConstraint(req.Constraints, "character_focus"), stringContext(req.Context, "character_focus")),
				"costume_hint":      firstString(stringConstraint(req.Constraints, "costume_hint"), stringContext(req.Context, "costume_hint")),
				"location_tag":      firstString(stringConstraint(req.Constraints, "location_tag"), stringContext(req.Context, "location_tag"), stringContext(req.Context, "scene_location")),
			},
			UseResultFields: map[string]any{
				"script_storyboard": "step-2.script_storyboard",
				"script_dialogues":  "step-2.script_dialogues",
			},
			Expected: []string{"shot plan", "shot prompts", "camera suggestions"},
		})
		steps = append(steps, model.PlanStep{
			ID:          "step-5",
			Agent:       "image-agent",
			Tool:        "image.generate_batch",
			Description: "Generate storyboard images from the shot-level plan.",
			Status:      "pending",
			DependsOn:   []string{"step-4"},
			Input: map[string]any{
				"project_id":        req.ProjectID,
				"user_id":           req.UserID,
				"style_preset":      stylePreset,
				"model_name":        firstString(stringConstraint(req.Constraints, "image_model_name"), stringContext(req.Context, "image_model_name"), "gpt-image-1"),
				"authorization":     authorization,
				"wait_for_result":   boolConstraint(req.Constraints, "wait_for_image_result") || boolContext(req.Context, "wait_for_image_result"),
				"poll_timeout_sec":  int64Constraint(req.Constraints, "image_poll_timeout_sec"),
				"poll_interval_sec": int64Constraint(req.Constraints, "image_poll_interval_sec"),
			},
			UseResultFields: map[string]any{
				"storyboard": "step-4.shots",
			},
			Expected: []string{"image task ids", "shot prompt list"},
		})
		steps = append(steps, model.PlanStep{
			ID:          "step-6",
			Agent:       "video-compose-agent",
			Tool:        "video.generate_batch",
			Description: "Submit commentary comic video generation with narration-aware payload.",
			Status:      "pending",
			DependsOn:   []string{"step-2", "step-3", "step-4", "step-5"},
			Input: map[string]any{
				"project_id":    req.ProjectID,
				"goal":          goal,
				"user_id":       req.UserID,
				"episodes":      videoEpisodes,
				"style_preset":  stylePreset,
				"motion_mode":   motionMode,
				"model_name":    modelName,
				"episode_id":    req.EpisodeID,
				"authorization": authorization,
			},
			UseResultFields: map[string]any{
				"script_context":     "step-2",
				"scene_descriptions": "step-4.shots",
				"dialogues":          "step-2.script_dialogues",
				"subtitle_text":      "step-2.script.content",
				"subtitle_timeline":  "step-3.subtitle_timeline",
				"audio_url":          "step-3.audio_url",
				"image_batch":        "step-5.images",
				"camera_movements":   "step-4.shots",
				"moods":              "step-4.shots",
				"transition_plan":    "step-4.shots",
				"character_focus":    "step-4.shots",
				"costume_hints":      "step-4.shots",
				"location_tags":      "step-4.shots",
			},
			Expected: []string{"video task id", "batch submission status"},
			FailureHints: []string{
				"If dubbing is unavailable, fall back to subtitle-only composition.",
				"If provider rate limits, reduce parallelism or switch model.",
			},
		})
		return steps
	}

	steps = append(steps,
		model.PlanStep{
			ID:          "step-3",
			Agent:       "image-agent",
			Tool:        "image.generate_batch",
			Description: "Generate storyboard images from the structured script output.",
			Status:      "pending",
			DependsOn:   []string{"step-2"},
			Input: map[string]any{
				"project_id":        req.ProjectID,
				"user_id":           req.UserID,
				"style_preset":      stylePreset,
				"model_name":        firstString(stringConstraint(req.Constraints, "image_model_name"), stringContext(req.Context, "image_model_name"), "gpt-image-1"),
				"authorization":     authorization,
				"wait_for_result":   boolConstraint(req.Constraints, "wait_for_image_result") || boolContext(req.Context, "wait_for_image_result"),
				"poll_timeout_sec":  int64Constraint(req.Constraints, "image_poll_timeout_sec"),
				"poll_interval_sec": int64Constraint(req.Constraints, "image_poll_interval_sec"),
			},
			UseResultFields: map[string]any{
				"script_storyboard": "step-2.script_storyboard",
			},
			Expected: []string{"image task ids", "storyboard prompt list"},
		},
		model.PlanStep{
			ID:          "step-4",
			Agent:       "video-planning-agent",
			Tool:        "video.generate_batch",
			Description: "Submit video generation batch after storyboard and asset preparation are ready.",
			Status:      "pending",
			DependsOn:   []string{"step-2", "step-3"},
			Input: map[string]any{
				"project_id":    req.ProjectID,
				"goal":          goal,
				"user_id":       req.UserID,
				"episodes":      videoEpisodes,
				"style_preset":  stylePreset,
				"motion_mode":   motionMode,
				"model_name":    modelName,
				"episode_id":    req.EpisodeID,
				"authorization": authorization,
			},
			UseResultFields: map[string]any{
				"script_context":     "step-2",
				"scene_descriptions": "step-2.script_storyboard",
				"dialogues":          "step-2.script_dialogues",
				"subtitle_text":      "step-2.script.content",
				"image_batch":        "step-3.images",
			},
			Expected: []string{"video task id", "batch submission status"},
			FailureHints: []string{
				"If provider rate limits, reduce parallelism or switch model.",
				"If clip consistency fails, retry with stronger character references.",
			},
		},
	)
	return steps
}

func applyCommentaryEpisodes(videoEpisodes []map[string]any, req model.PlanRequest) {
	for _, episode := range videoEpisodes {
		episode["mode"] = "commentary_comic"
		episode["narration_mode"] = true
		if speaker := firstString(stringConstraint(req.Constraints, "voice_style"), stringContext(req.Context, "voice_style")); speaker != "" {
			episode["voice_style"] = speaker
		}
		if subtitleStyle := firstString(stringConstraint(req.Constraints, "subtitle_style"), stringContext(req.Context, "subtitle_style")); subtitleStyle != "" {
			episode["subtitle_style"] = subtitleStyle
		}
	}
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
