package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/autovideo/agent-service/internal/model"
)

type Tool interface {
	Definition() model.ToolDefinition
	Execute(ctx context.Context, input map[string]any) (map[string]any, error)
}

type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry(tools ...Tool) *ToolRegistry {
	m := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		m[tool.Definition().Name] = tool
	}
	return &ToolRegistry{tools: m}
}

func (r *ToolRegistry) Definitions() []model.ToolDefinition {
	defs := make([]model.ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition())
	}
	return defs
}

func (r *ToolRegistry) Execute(ctx context.Context, name string, input map[string]any) (map[string]any, error) {
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not registered", name)
	}
	return tool.Execute(ctx, input)
}

type HTTPTool struct {
	def          ToolDefinitionFactory
	client       *http.Client
	exec         func(ctx context.Context, client *http.Client, endpoint string, input map[string]any) (map[string]any, error)
	fallbackStub bool
}

type ToolDefinitionFactory struct {
	Name        string
	Description string
	Inputs      []string
	Endpoint    string
}

func NewStubTool(name, desc string, inputs []string, endpoint string) *HTTPTool {
	return &HTTPTool{
		def:          ToolDefinitionFactory{Name: name, Description: desc, Inputs: inputs, Endpoint: endpoint},
		client:       &http.Client{Timeout: 15 * time.Second},
		exec:         nil,
		fallbackStub: true,
	}
}

func NewProjectGetTool(endpoint string) *HTTPTool {
	return &HTTPTool{
		def:          ToolDefinitionFactory{Name: "project.get", Description: "Load project state and metadata.", Inputs: []string{"project_id", "episode_id"}, Endpoint: endpoint},
		client:       &http.Client{Timeout: 15 * time.Second},
		exec:         executeProjectGet,
		fallbackStub: true,
	}
}

func NewVideoGenerateBatchTool(endpoint string) *HTTPTool {
	return &HTTPTool{
		def:          ToolDefinitionFactory{Name: "video.generate_batch", Description: "Submit batched video generation task.", Inputs: []string{"project_id", "episodes", "style_preset", "motion_mode", "model_name"}, Endpoint: endpoint},
		client:       &http.Client{Timeout: 30 * time.Second},
		exec:         executeVideoGenerateBatch,
		fallbackStub: true,
	}
}

func NewScriptGenerateTool(endpoint string) *HTTPTool {
	return &HTTPTool{
		def:          ToolDefinitionFactory{Name: "script.generate", Description: "Generate or refine script/storyboard draft.", Inputs: []string{"mode", "title", "genre", "premise", "source_text", "project_context"}, Endpoint: endpoint},
		client:       &http.Client{Timeout: 60 * time.Second},
		exec:         executeScriptGenerate,
		fallbackStub: true,
	}
}

func NewImageGenerateBatchTool(endpoint string) *HTTPTool {
	return &HTTPTool{
		def:          ToolDefinitionFactory{Name: "image.generate_batch", Description: "Generate storyboard image tasks in batch.", Inputs: []string{"project_id", "user_id", "storyboard", "style_preset", "model_name"}, Endpoint: endpoint},
		client:       &http.Client{Timeout: 60 * time.Second},
		exec:         executeImageGenerateBatch,
		fallbackStub: true,
	}
}

func NewDubbingGenerateTool(endpoint string) *HTTPTool {
	return &HTTPTool{
		def:          ToolDefinitionFactory{Name: "dubbing.generate", Description: "Generate narration dubbing payload for commentary comic mode.", Inputs: []string{"project_id", "user_id", "script_content", "script_dialogues", "voice_style", "language"}, Endpoint: endpoint},
		client:       &http.Client{Timeout: 60 * time.Second},
		exec:         executeDubbingGenerate,
		fallbackStub: true,
	}
}

func NewShotPlanGenerateTool() *HTTPTool {
	return &HTTPTool{
		def:          ToolDefinitionFactory{Name: "shot_plan.generate", Description: "Generate shot-level plan from structured storyboard/script for commentary comic mode.", Inputs: []string{"script_storyboard", "script_dialogues", "target_shot_count", "shot_density"}},
		client:       &http.Client{Timeout: 15 * time.Second},
		exec:         executeShotPlanGenerate,
		fallbackStub: false,
	}
}

func NewTaskGetStatusTool(endpoint string) *HTTPTool {
	return &HTTPTool{
		def:          ToolDefinitionFactory{Name: "task.get_status", Description: "Fetch downstream task status.", Inputs: []string{"task_id", "user_id"}, Endpoint: endpoint},
		client:       &http.Client{Timeout: 15 * time.Second},
		exec:         executeTaskGetStatus,
		fallbackStub: true,
	}
}

func (t *HTTPTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Name:        t.def.Name,
		Description: t.def.Description,
		InputSchema: t.def.Inputs,
	}
}

func (t *HTTPTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	if t.exec == nil || strings.TrimSpace(t.def.Endpoint) == "" {
		return t.stub(input), nil
	}
	output, err := t.exec(ctx, t.client, t.def.Endpoint, input)
	if err == nil {
		output["mode"] = "live"
		output["tool"] = t.def.Name
		output["endpoint"] = t.def.Endpoint
		return output, nil
	}
	if !t.fallbackStub {
		return nil, err
	}
	stub := t.stub(input)
	stub["fallback_reason"] = err.Error()
	return stub, nil
}

func (t *HTTPTool) stub(input map[string]any) map[string]any {
	return map[string]any{
		"accepted":    true,
		"tool":        t.def.Name,
		"endpoint":    t.def.Endpoint,
		"input":       input,
		"executed_at": time.Now().UTC().Format(time.RFC3339),
		"mode":        "stub",
	}
}

func executeProjectGet(ctx context.Context, client *http.Client, endpoint string, input map[string]any) (map[string]any, error) {
	projectID, err := int64Input(input, "project_id")
	if err != nil || projectID <= 0 {
		return nil, fmt.Errorf("project_id is required")
	}
	base := strings.TrimRight(endpoint, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/projects/%d", base, projectID), nil)
	if err != nil {
		return nil, err
	}
	applyServiceHeaders(req, input)
	var body envelope
	if err := doJSON(client, req, &body); err != nil {
		return nil, err
	}
	return map[string]any{
		"project": body.Data,
	}, nil
}

func executeVideoGenerateBatch(ctx context.Context, client *http.Client, endpoint string, input map[string]any) (map[string]any, error) {
	projectID, err := int64Input(input, "project_id")
	if err != nil || projectID <= 0 {
		return nil, fmt.Errorf("project_id is required")
	}
	episodes, ok := input["episodes"]
	if !ok {
		return nil, fmt.Errorf("episodes is required")
	}
	payload := map[string]any{}
	for k, v := range input {
		if k == "project_id" || k == "user_id" || k == "authorization" {
			continue
		}
		payload[k] = v
	}
	payload["episodes"] = episodes
	if _, ok := payload["style_preset"]; !ok {
		payload["style_preset"] = "anime-2d"
	}
	if _, ok := payload["motion_mode"]; !ok {
		payload["motion_mode"] = "gentle"
	}
	if _, ok := payload["model_name"]; !ok {
		payload["model_name"] = "kling"
	}
	buf, _ := json.Marshal(payload)
	base := strings.TrimRight(endpoint, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/projects/%d/videos/generate-batch", base, projectID), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyServiceHeaders(req, input)
	var body envelope
	if err := doJSON(client, req, &body); err != nil {
		return nil, err
	}
	return map[string]any{
		"submission": body.Data,
	}, nil
}

func executeScriptGenerate(ctx context.Context, client *http.Client, endpoint string, input map[string]any) (map[string]any, error) {
	payload := map[string]any{}
	for k, v := range input {
		if k == "user_id" || k == "authorization" {
			continue
		}
		payload[k] = v
	}
	if _, ok := payload["mode"]; !ok {
		payload["mode"] = "script"
	}
	if _, ok := payload["premise"]; !ok {
		if goal, ok := payload["goal"]; ok {
			payload["premise"] = goal
		}
	}
	if _, ok := payload["requirements"]; !ok {
		if constraints, ok := payload["constraints"]; ok {
			if buf, err := json.Marshal(constraints); err == nil {
				payload["requirements"] = string(buf)
			}
		}
	}
	buf, _ := json.Marshal(payload)
	base := strings.TrimRight(endpoint, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/script-library/generate", base), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyServiceHeaders(req, input)
	var body envelope
	if err := doJSON(client, req, &body); err != nil {
		return nil, err
	}
	scriptData, _ := body.Data.(map[string]any)
	return map[string]any{
		"script":                   body.Data,
		"script_structured_content": extractScriptStructuredContent(scriptData),
		"script_storyboard":         extractScriptStoryboard(scriptData),
		"script_dialogues":          extractScriptDialogues(scriptData),
	}, nil
}

func extractScriptStructuredContent(scriptData map[string]any) any {
	if len(scriptData) == 0 {
		return nil
	}
	if storyboard := extractScriptStoryboard(scriptData); len(storyboard) > 0 {
		return storyboard
	}
	if dialogues := extractScriptDialogues(scriptData); len(dialogues) > 0 {
		return dialogues
	}
	if content := stringValue(scriptData["content"]); content != "" {
		return content
	}
	return nil
}

func extractScriptStoryboard(scriptData map[string]any) []map[string]any {
	if len(scriptData) == 0 {
		return nil
	}
	storyboard := make([]map[string]any, 0)
	if outline, ok := scriptData["outline"].([]any); ok {
		for idx, item := range outline {
			if text := stringValue(item); text != "" {
				storyboard = append(storyboard, map[string]any{
					"index":              idx,
					"scene_description":  text,
					"dialogue":           text,
				})
			}
		}
	}
	if len(storyboard) == 0 {
		if content := stringValue(scriptData["content"]); content != "" {
			for idx, chunk := range splitScriptContent(content) {
				storyboard = append(storyboard, map[string]any{
					"index":             idx,
					"scene_description": chunk,
					"dialogue":          chunk,
				})
			}
		}
	}
	return storyboard
}

func extractScriptDialogues(scriptData map[string]any) []string {
	if len(scriptData) == 0 {
		return nil
	}
	if outline, ok := scriptData["outline"].([]any); ok {
		out := make([]string, 0, len(outline))
		for _, item := range outline {
			if text := stringValue(item); text != "" {
				out = append(out, text)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	if content := stringValue(scriptData["content"]); content != "" {
		return splitScriptContent(content)
	}
	return nil
}

func executeImageGenerateBatch(ctx context.Context, client *http.Client, endpoint string, input map[string]any) (map[string]any, error) {
	projectID, err := int64Input(input, "project_id")
	if err != nil || projectID <= 0 {
		return nil, fmt.Errorf("project_id is required")
	}
	userID, err := int64Input(input, "user_id")
	if err != nil || userID <= 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	storyboardRaw, ok := input["storyboard"]
	if !ok {
		storyboardRaw = input["script_storyboard"]
	}
	storyboard, ok := storyboardRaw.([]any)
	if !ok || len(storyboard) == 0 {
		return nil, fmt.Errorf("storyboard is required")
	}
	base := strings.TrimRight(endpoint, "/")
	stylePreset := firstString(stringValue(input["style_preset"]), "anime-2d")
	modelName := firstString(stringValue(input["model_name"]), "gpt-image-1")
	waitForResult := boolValue(input["wait_for_result"])
	pollTimeout := durationFromSeconds(input["poll_timeout_sec"], 90*time.Second)
	pollInterval := durationFromSeconds(input["poll_interval_sec"], 3*time.Second)
	results := make([]map[string]any, 0, len(storyboard))
	for idx, item := range storyboard {
		entry, _ := item.(map[string]any)
		prompt := buildImagePrompt(entry)
		if prompt == "" {
			continue
		}
		payload := map[string]any{
			"project_id":   projectID,
			"user_id":      userID,
			"prompt":       prompt,
			"task_type":    "storyboard",
			"style_preset": stylePreset,
			"model_name":   modelName,
		}
		buf, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/images/generate", base), bytes.NewReader(buf))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		applyServiceHeaders(req, input)
		var body envelope
		if err := doJSON(client, req, &body); err != nil {
			return nil, fmt.Errorf("storyboard image %d: %w", idx, err)
		}
		generation, _ := body.Data.(map[string]any)
		result := map[string]any{
			"index":      idx,
			"prompt":     prompt,
			"generation": body.Data,
		}
		if waitForResult {
			if taskID, err := int64Input(generation, "id"); err == nil && taskID > 0 {
				finalTask, pollErr := pollImageTaskResult(ctx, client, base, input, taskID, pollTimeout, pollInterval)
				if pollErr != nil {
					result["poll_error"] = pollErr.Error()
				} else {
					result["generation"] = finalTask
					if url := firstString(stringValue(finalTask["result_url"]), stringValue(finalTask["image_url"]), stringValue(finalTask["url"])); url != "" {
						result["image_url"] = url
					}
				}
			}
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no valid storyboard prompts extracted")
	}
	return map[string]any{
		"images": results,
		"count":  len(results),
	}, nil
}

func pollImageTaskResult(ctx context.Context, client *http.Client, endpoint string, input map[string]any, taskID int64, timeout time.Duration, interval time.Duration) (map[string]any, error) {
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		select {
		case <-deadlineCtx.Done():
			return nil, fmt.Errorf("poll image task %d timeout: %w", taskID, deadlineCtx.Err())
		case <-time.After(interval):
			req, err := http.NewRequestWithContext(deadlineCtx, http.MethodGet, fmt.Sprintf("%s/api/v1/images/tasks/%d", strings.TrimRight(endpoint, "/"), taskID), nil)
			if err != nil {
				return nil, err
			}
			applyServiceHeaders(req, input)
			var body envelope
			if err := doJSON(client, req, &body); err != nil {
				return nil, err
			}
			task, _ := body.Data.(map[string]any)
			status := strings.ToLower(stringValue(task["status"]))
			switch status {
			case "succeeded", "completed":
				return task, nil
			case "failed", "cancelled":
				return task, fmt.Errorf("image task %d ended with status %s", taskID, status)
			}
		}
	}
}

func boolValue(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.EqualFold(strings.TrimSpace(val), "true")
	default:
		return false
	}
}

func durationFromSeconds(v any, fallback time.Duration) time.Duration {
	switch val := v.(type) {
	case int:
		if val > 0 {
			return time.Duration(val) * time.Second
		}
	case int64:
		if val > 0 {
			return time.Duration(val) * time.Second
		}
	case float64:
		if val > 0 {
			return time.Duration(val) * time.Second
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return fallback
}

func buildImagePrompt(entry map[string]any) string {
	base := firstString(
		stringValue(entry["prompt"]),
		stringValue(entry["scene_description"]),
		stringValue(entry["visual_desc"]),
		stringValue(entry["description"]),
	)
	if len(entry) == 0 {
		return base
	}
	parts := make([]string, 0, 5)
	if base != "" {
		parts = append(parts, base)
	}
	if panelType := stringValue(entry["panel_type"]); panelType != "" {
		parts = append(parts, "panel type: "+panelType)
	}
	if camera := stringValue(entry["camera"]); camera != "" {
		parts = append(parts, "camera framing: "+camera)
	}
	if focus := stringValue(entry["visual_focus"]); focus != "" {
		parts = append(parts, "visual focus: "+focus)
	}
	if pose := stringValue(entry["character_pose"]); pose != "" {
		parts = append(parts, "character pose: "+pose)
	}
	if emotion := stringValue(entry["emotion"]); emotion != "" {
		parts = append(parts, "emotion tone: "+emotion)
	}
	if transition := stringValue(entry["transition"]); transition != "" {
		parts = append(parts, "transition cue: "+transition)
	}
	if subtitle := firstString(stringValue(entry["subtitle_candidate"]), stringValue(entry["dialogue"])); subtitle != "" {
		parts = append(parts, "narration context: "+subtitle)
	}
	if duration := stringValue(entry["duration_sec"]); duration != "" {
		parts = append(parts, "shot duration around "+duration+" seconds")
	}
	return strings.Join(parts, ", ")
}

func executeDubbingGenerate(ctx context.Context, client *http.Client, endpoint string, input map[string]any) (map[string]any, error) {
	segments := buildDubbingSegments(input)
	if len(segments) == 0 {
		return nil, fmt.Errorf("script content is required for dubbing")
	}

	payload := map[string]any{}
	for k, v := range input {
		if k == "authorization" {
			continue
		}
		payload[k] = v
	}
	payload["segments"] = segments
	if _, ok := payload["voice_style"]; !ok {
		payload["voice_style"] = "calm_narration"
	}
	if _, ok := payload["language"]; !ok {
		payload["language"] = "zh-CN"
	}

	buf, _ := json.Marshal(payload)
	base := strings.TrimRight(endpoint, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/dubbing/generate", base), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyServiceHeaders(req, input)
	var body envelope
	if err := doJSON(client, req, &body); err != nil {
		return nil, err
	}
	data, _ := body.Data.(map[string]any)
	return map[string]any{
		"dubbing":            body.Data,
		"audio_url":          extractAudioURL(data),
		"subtitle_timeline":  extractSubtitleTimeline(data, segments),
		"narration_segments": segments,
	}, nil
}

func buildDubbingSegments(input map[string]any) []map[string]any {
	if dialogues, ok := input["script_dialogues"].([]string); ok && len(dialogues) > 0 {
		segments := make([]map[string]any, 0, len(dialogues))
		for idx, line := range dialogues {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			segments = append(segments, map[string]any{"index": idx, "text": line})
		}
		if len(segments) > 0 {
			return segments
		}
	}
	if raw, ok := input["script_dialogues"].([]any); ok && len(raw) > 0 {
		segments := make([]map[string]any, 0, len(raw))
		for idx, item := range raw {
			if text := stringValue(item); text != "" {
				segments = append(segments, map[string]any{"index": idx, "text": text})
			}
		}
		if len(segments) > 0 {
			return segments
		}
	}
	content := firstString(stringValue(input["script_content"]), stringValue(input["subtitle_text"]), stringValue(input["source_text"]))
	if content == "" {
		return nil
	}
	parts := splitScriptContent(content)
	segments := make([]map[string]any, 0, len(parts))
	for idx, part := range parts {
		segments = append(segments, map[string]any{"index": idx, "text": part})
	}
	return segments
}

func extractAudioURL(data map[string]any) string {
	if len(data) == 0 {
		return ""
	}
	if audio, ok := data["audio"].(map[string]any); ok {
		if url := firstString(stringValue(audio["audio_url"]), stringValue(audio["url"])); url != "" {
			return url
		}
	}
	return firstString(stringValue(data["audio_url"]), stringValue(data["url"]))
}

func extractSubtitleTimeline(data map[string]any, fallback []map[string]any) []map[string]any {
	if len(data) > 0 {
		if subtitles, ok := data["subtitles"].([]any); ok && len(subtitles) > 0 {
			out := make([]map[string]any, 0, len(subtitles))
			for idx, item := range subtitles {
				if entry, ok := item.(map[string]any); ok {
					text := firstString(stringValue(entry["text"]), stringValue(entry["subtitle"]), stringValue(entry["content"]))
					if text == "" {
						continue
					}
					out = append(out, map[string]any{
						"index":     idx,
						"text":      text,
						"start_sec": entry["start_sec"],
						"end_sec":   entry["end_sec"],
					})
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	out := make([]map[string]any, 0, len(fallback))
	for _, item := range fallback {
		text := stringValue(item["text"])
		if text == "" {
			continue
		}
		out = append(out, map[string]any{
			"index": item["index"],
			"text":  text,
		})
	}
	return out
}

func executeShotPlanGenerate(_ context.Context, _ *http.Client, _ string, input map[string]any) (map[string]any, error) {
	characterName := firstString(stringValue(input["character_name"]), stringValue(input["character_focus"]), stringValue(input["main_character"]))
	costumeHint := firstString(stringValue(input["costume_hint"]), stringValue(input["character_costume"]), stringValue(input["wardrobe_hint"]))
	locationTag := firstString(stringValue(input["location_tag"]), stringValue(input["world_location"]), stringValue(input["scene_location"]))
	storyboardRaw, ok := input["script_storyboard"]
	if !ok {
		storyboardRaw = input["storyboard"]
	}
	storyboard, _ := storyboardRaw.([]any)
	if len(storyboard) == 0 {
		if typed, ok := storyboardRaw.([]map[string]any); ok {
			storyboard = make([]any, 0, len(typed))
			for _, item := range typed {
				storyboard = append(storyboard, item)
			}
		}
	}
	dialogues := extractDialogueSlice(input["script_dialogues"])
	shotDensity := firstString(stringValue(input["shot_density"]), "medium")
	targetShotCount, _ := int64InputLoose(input, "target_shot_count")
	shotsPerScene := normalizeShotsPerScene(shotDensity, targetShotCount, len(storyboard))

	shots := make([]map[string]any, 0)
	for idx, item := range storyboard {
		entry, _ := item.(map[string]any)
		sceneDesc := firstString(
			stringValue(entry["scene_description"]),
			stringValue(entry["visual_desc"]),
			stringValue(entry["description"]),
			stringValue(entry["dialogue"]),
		)
		dialogue := ""
		if idx < len(dialogues) {
			dialogue = dialogues[idx]
		}
		for shotIdx := 0; shotIdx < shotsPerScene; shotIdx++ {
			panelType := suggestPanelType(shotIdx, shotsPerScene)
			camera := suggestShotCamera(shotIdx, shotsPerScene)
			emotion := suggestShotEmotion(sceneDesc, dialogue, shotIdx, shotsPerScene)
			visualFocus := suggestVisualFocus(sceneDesc, dialogue, shotIdx, shotsPerScene)
			characterPose := suggestCharacterPose(dialogue, shotIdx, shotsPerScene)
			shots = append(shots, map[string]any{
				"shot_id":            fmt.Sprintf("scene-%d-shot-%d", idx+1, shotIdx+1),
				"scene_index":        idx,
				"shot_index":         shotIdx,
				"scene_description":  sceneDesc,
				"dialogue":           dialogue,
				"prompt":             buildShotPrompt(sceneDesc, dialogue, visualFocus, characterPose, emotion, panelType, shotIdx, shotsPerScene),
				"camera":             camera,
				"duration_sec":       suggestShotDuration(shotIdx, shotsPerScene),
				"transition":         suggestShotTransition(shotIdx),
				"subtitle_candidate": firstString(dialogue, sceneDesc),
				"panel_type":         panelType,
				"visual_focus":       visualFocus,
				"character_pose":     characterPose,
				"emotion":            emotion,
				"character_name":     characterName,
				"character_focus":    firstString(characterName, "protagonist"),
				"costume_hint":       costumeHint,
				"location_tag":       firstString(locationTag, summarizeLocation(sceneDesc)),
			})
		}
	}
	if len(shots) == 0 {
		return nil, fmt.Errorf("script_storyboard is required")
	}
	return map[string]any{
		"shot_plan": map[string]any{
			"mode":            "commentary_comic",
			"shot_density":    shotDensity,
			"scene_count":     len(storyboard),
			"shot_count":      len(shots),
			"shots_per_scene": shotsPerScene,
			"character_name":  characterName,
			"costume_hint":    costumeHint,
			"location_tag":    locationTag,
		},
		"shots": shots,
	}, nil
}

func summarizeLocation(sceneDesc string) string {
	sceneDesc = strings.TrimSpace(sceneDesc)
	if sceneDesc == "" {
		return ""
	}
	parts := splitScriptContent(sceneDesc)
	if len(parts) > 0 {
		return firstString(parts[0])
	}
	return sceneDesc
}

func extractDialogueSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return trimStringSlice(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if text := stringValue(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func normalizeShotsPerScene(shotDensity string, targetShotCount int64, sceneCount int) int {
	if sceneCount <= 0 {
		return 0
	}
	if targetShotCount > 0 {
		perScene := int(targetShotCount) / sceneCount
		if int(targetShotCount)%sceneCount != 0 {
			perScene++
		}
		if perScene < 1 {
			perScene = 1
		}
		return perScene
	}
	switch strings.ToLower(strings.TrimSpace(shotDensity)) {
	case "low":
		return 1
	case "high":
		return 3
	default:
		return 2
	}
}

func buildShotPrompt(sceneDesc, dialogue, visualFocus, characterPose, emotion, panelType string, shotIdx, shotsPerScene int) string {
	phase := "establishing"
	switch {
	case shotsPerScene == 1:
		phase = "single framed narrative panel"
	case shotIdx == 0:
		phase = "establishing shot"
	case shotIdx == shotsPerScene-1:
		phase = "close emotional beat"
	default:
		phase = "medium action beat"
	}
	parts := []string{phase}
	if panelType != "" {
		parts = append(parts, "panel type: "+panelType)
	}
	if sceneDesc != "" {
		parts = append(parts, sceneDesc)
	}
	if visualFocus != "" {
		parts = append(parts, "visual focus: "+visualFocus)
	}
	if characterPose != "" {
		parts = append(parts, "character pose: "+characterPose)
	}
	if emotion != "" {
		parts = append(parts, "emotion: "+emotion)
	}
	if dialogue != "" {
		parts = append(parts, "narration context: "+dialogue)
	}
	return strings.Join(parts, ", ")
}

func suggestPanelType(shotIdx, shotsPerScene int) string {
	if shotsPerScene <= 1 {
		return "hero_panel"
	}
	if shotIdx == 0 {
		return "establishing_panel"
	}
	if shotIdx == shotsPerScene-1 {
		return "impact_closeup"
	}
	return "narrative_panel"
}

func suggestShotCamera(shotIdx, shotsPerScene int) string {
	if shotsPerScene <= 1 {
		return "medium"
	}
	if shotIdx == 0 {
		return "wide"
	}
	if shotIdx == shotsPerScene-1 {
		return "close_up"
	}
	return "medium"
}

func suggestShotDuration(shotIdx, shotsPerScene int) float64 {
	if shotsPerScene <= 1 {
		return 3.5
	}
	if shotIdx == 0 {
		return 2.5
	}
	return 2.0
}

func suggestVisualFocus(sceneDesc, dialogue string, shotIdx, shotsPerScene int) string {
	if shotIdx == 0 {
		return firstString(sceneDesc, "environment and spatial context")
	}
	if shotIdx == shotsPerScene-1 {
		return firstString(dialogue, "facial expression and key reveal")
	}
	return firstString(dialogue, sceneDesc, "interaction beat")
}

func suggestCharacterPose(dialogue string, shotIdx, shotsPerScene int) string {
	if strings.TrimSpace(dialogue) == "" {
		if shotIdx == 0 {
			return "observing the scene cautiously"
		}
		return "moving through the scene"
	}
	if shotIdx == shotsPerScene-1 {
		return "intense reaction with expressive face"
	}
	if shotIdx == 0 {
		return "entering frame and surveying surroundings"
	}
	return "mid-action storytelling pose"
}

func suggestShotEmotion(sceneDesc, dialogue string, shotIdx, shotsPerScene int) string {
	text := strings.ToLower(sceneDesc + " " + dialogue)
	switch {
	case strings.Contains(text, "danger") || strings.Contains(text, "危") || strings.Contains(text, "险") || strings.Contains(text, "紧张"):
		return "tense"
	case strings.Contains(text, "mystery") || strings.Contains(text, "神秘") || strings.Contains(text, "遗迹") || strings.Contains(text, "unknown"):
		return "mysterious"
	case strings.Contains(text, "battle") || strings.Contains(text, "fight") || strings.Contains(text, "战"):
		return "intense"
	case shotIdx == shotsPerScene-1:
		return "dramatic"
	default:
		return "cinematic"
	}
}

func suggestShotTransition(shotIdx int) string {
	if shotIdx == 0 {
		return "cut"
	}
	return "dissolve"
}

func int64InputLoose(input map[string]any, key string) (int64, error) {
	raw, ok := input[key]
	if !ok || raw == nil {
		return 0, fmt.Errorf("missing %s", key)
	}
	switch v := raw.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case json.Number:
		return v.Int64()
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, fmt.Errorf("empty %s", key)
		}
		return strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	default:
		return 0, fmt.Errorf("invalid %s", key)
	}
}

func executeTaskGetStatus(ctx context.Context, client *http.Client, endpoint string, input map[string]any) (map[string]any, error) {
	taskID, err := int64Input(input, "task_id")
	if err != nil || taskID <= 0 {
		return nil, fmt.Errorf("task_id is required")
	}
	base := strings.TrimRight(endpoint, "/")
	u, err := url.Parse(fmt.Sprintf("%s/api/v1/tasks/%d", base, taskID))
	if err != nil {
		return nil, err
	}
	if userID, err := int64Input(input, "user_id"); err == nil && userID > 0 {
		q := u.Query()
		q.Set("user_id", strconv.FormatInt(userID, 10))
		u.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	applyServiceHeaders(req, input)
	var body envelope
	if err := doJSON(client, req, &body); err != nil {
		return nil, err
	}
	return map[string]any{
		"task": body.Data,
	}, nil
}

type envelope struct {
	Code    any `json:"code"`
	Message string `json:"message"`
	Data    any `json:"data"`
}

func doJSON(client *http.Client, req *http.Request, out *envelope) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func applyServiceHeaders(req *http.Request, input map[string]any) {
	if auth, ok := input["authorization"].(string); ok && strings.TrimSpace(auth) != "" {
		req.Header.Set("Authorization", auth)
	}
	if userID, err := int64Input(input, "user_id"); err == nil && userID > 0 {
		req.Header.Set("X-User-ID", strconv.FormatInt(userID, 10))
	}
}

func int64Input(input map[string]any, key string) (int64, error) {
	raw, ok := input[key]
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	switch v := raw.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case json.Number:
		return v.Int64()
	case string:
		return strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	default:
		return 0, fmt.Errorf("invalid %s", key)
	}
}
