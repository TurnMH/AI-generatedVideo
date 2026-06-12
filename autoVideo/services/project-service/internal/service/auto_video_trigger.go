package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/autovideo/project-service/internal/model"
	"github.com/autovideo/project-service/internal/productionmode"
	"github.com/autovideo/project-service/internal/speechtext"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/datatypes"
)

const autoVideoTriggerDebounce = 3 * time.Second

// SetVideoService configures the video-service URL for automatic video batch submission.
func (s *StoryboardService) SetVideoService(baseURL string) {
	s.videoBaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func (s *StoryboardService) isAdWorkbenchProject(project *model.Project) bool {
	return productionmode.IsAd(project)
}

func parseStoryboardConfigMap(raw datatypes.JSON) map[string]interface{} {
	out := map[string]interface{}{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func storyboardConfigBool(raw datatypes.JSON, key string, defaultVal bool) bool {
	cfg := parseStoryboardConfigMap(raw)
	val, ok := cfg[key]
	if !ok {
		return defaultVal
	}
	switch typed := val.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return defaultVal
}

func storyboardConfigString(raw datatypes.JSON, key string) string {
	cfg := parseStoryboardConfigMap(raw)
	if val, ok := cfg[key]; ok {
		if text, ok := val.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func storyboardEligibleForVideo(sb model.Storyboard, isSerial bool) bool {
	if sb.Status != "completed" || sb.IsVoided {
		return false
	}
	if strings.TrimSpace(sb.ImageURL) != "" {
		return true
	}
	return isSerial && strings.TrimSpace(sb.SceneGroupKey) != ""
}

func episodeHasRenderableImage(storyboards []model.Storyboard) bool {
	for _, sb := range storyboards {
		if strings.TrimSpace(sb.ImageURL) != "" {
			return true
		}
	}
	return false
}

func int64SliceFromPQ(values pq.Int64Array) []int64 {
	if len(values) == 0 {
		return nil
	}
	out := make([]int64, len(values))
	copy(out, values)
	return out
}

func stringSliceFromPQ(values pq.StringArray) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

// TriggerAutoVideoIfReady submits generate-batch when all storyboards are image-ready.
// Skips ad-workbench projects and projects that already auto-triggered video generation.
func (s *StoryboardService) TriggerAutoVideoIfReady(ctx context.Context, projectID uint64) error {
	if s.videoBaseURL == "" || s.projectRepo == nil {
		return nil
	}

	s.autoVideoMu.Lock()
	if s.autoVideoInflight == nil {
		s.autoVideoInflight = make(map[uint64]time.Time)
	}
	if last, ok := s.autoVideoInflight[projectID]; ok && time.Since(last) < autoVideoTriggerDebounce {
		s.autoVideoMu.Unlock()
		return nil
	}
	s.autoVideoInflight[projectID] = time.Now()
	s.autoVideoMu.Unlock()

	project, err := s.projectRepo.FindByIDNoAuth(projectID)
	if err != nil {
		return err
	}
	if s.isAdWorkbenchProject(project) {
		return nil
	}
	if s.isProjectGenerationPaused(projectID) {
		return nil
	}
	if !storyboardConfigBool(project.StoryboardConfig, "auto_generate_video", true) {
		return nil
	}
	if storyboardConfigBool(project.StoryboardConfig, "auto_video_triggered", false) {
		return nil
	}

	for _, status := range []string{"pending", "generating", "failed"} {
		count, countErr := s.repo.CountByProjectAndStatus(projectID, status)
		if countErr != nil {
			return countErr
		}
		if count > 0 {
			return nil
		}
	}

	storyboards, err := s.repo.FindAllActive(projectID, nil)
	if err != nil {
		return err
	}
	if len(storyboards) == 0 {
		return nil
	}

	isSerial := strings.TrimSpace(project.ProjectType) == "video_serial"
	byEpisode := map[uint64][]model.Storyboard{}
	for _, sb := range storyboards {
		if !storyboardEligibleForVideo(sb, isSerial) {
			continue
		}
		if sb.EpisodeID == nil || *sb.EpisodeID == 0 {
			continue
		}
		epID := *sb.EpisodeID
		byEpisode[epID] = append(byEpisode[epID], sb)
	}
	if len(byEpisode) == 0 {
		return nil
	}

	type episodePayload struct {
		EpisodeID         uint64     `json:"episode_id"`
		ImageURLs         []string   `json:"image_urls"`
		SceneDescriptions []string   `json:"scene_descriptions"`
		Dialogues         []string   `json:"dialogues,omitempty"`
		Durations         []float64  `json:"durations,omitempty"`
		CameraMovements   []string   `json:"camera_movements,omitempty"`
		Moods             []string   `json:"moods,omitempty"`
		SceneCharacters   [][]string `json:"scene_characters,omitempty"`
		SceneAssetIDs     [][]int64  `json:"scene_asset_ids,omitempty"`
		SceneDescription  string     `json:"scene_description,omitempty"`
		SceneGroupKeys    []string   `json:"scene_group_keys,omitempty"`
	}
	type batchRequest struct {
		Episodes        []episodePayload       `json:"episodes"`
		StylePreset     string                 `json:"style_preset,omitempty"`
		MotionMode      string                 `json:"motion_mode,omitempty"`
		ModelName       string                 `json:"model_name,omitempty"`
		VideoMode       string                 `json:"video_mode,omitempty"`
		ClipDurationSec float64                `json:"clip_duration_sec,omitempty"`
		SerialScene     bool                   `json:"serial_scene,omitempty"`
		RenderConfig    map[string]interface{} `json:"render_config,omitempty"`
	}

	episodes := make([]episodePayload, 0, len(byEpisode))
	for epID, rows := range byEpisode {
		if !episodeHasRenderableImage(rows) {
			continue
		}
		payload := episodePayload{EpisodeID: epID}
		for _, sb := range rows {
			payload.ImageURLs = append(payload.ImageURLs, sb.ImageURL)
			desc := strings.TrimSpace(sb.PromptUsed)
			if desc == "" {
				desc = strings.TrimSpace(sb.SceneDescription)
			}
			payload.SceneDescriptions = append(payload.SceneDescriptions, desc)
			payload.Dialogues = append(payload.Dialogues, strings.TrimSpace(speechtext.SanitizeForSpeech(sb.Dialogue)))
			if sb.Duration > 0 {
				payload.Durations = append(payload.Durations, float64(sb.Duration))
			} else {
				payload.Durations = append(payload.Durations, 0)
			}
			payload.CameraMovements = append(payload.CameraMovements, strings.TrimSpace(sb.CameraMovement))
			payload.Moods = append(payload.Moods, strings.TrimSpace(sb.Mood))
			payload.SceneCharacters = append(payload.SceneCharacters, stringSliceFromPQ(sb.Characters))
			payload.SceneAssetIDs = append(payload.SceneAssetIDs, int64SliceFromPQ(sb.AssetIDs))
			payload.SceneGroupKeys = append(payload.SceneGroupKeys, strings.TrimSpace(sb.SceneGroupKey))
		}
		payload.SceneDescription = strings.TrimSpace(strings.Join(payload.SceneDescriptions, " "))
		episodes = append(episodes, payload)
	}
	if len(episodes) == 0 {
		return nil
	}

	modelName := storyboardConfigString(project.StoryboardConfig, "video_model")
	if modelName == "" {
		modelName = "kling"
	}
	stylePreset := storyboardConfigString(project.StoryboardConfig, "style_preset")
	if stylePreset == "" {
		stylePreset = "anime-2d"
	}
	motionMode := storyboardConfigString(project.StoryboardConfig, "motion_mode")
	if motionMode == "" {
		motionMode = "gentle"
	}
	clipDuration := 5.0
	if raw := storyboardConfigString(project.StoryboardConfig, "duration"); raw != "" {
		if parsed, parseErr := parsePositiveFloat(raw); parseErr == nil {
			clipDuration = parsed
		}
	}

	reqBody := batchRequest{
		Episodes:        episodes,
		ModelName:       modelName,
		StylePreset:     stylePreset,
		MotionMode:      motionMode,
		VideoMode:       strings.TrimSpace(project.VideoMode),
		ClipDurationSec: clipDuration,
		SerialScene:     isSerial,
		RenderConfig: map[string]interface{}{
			"allow_incomplete_compose": true,
			"auto_triggered":           true,
		},
	}
	if storyboardConfigBool(project.StoryboardConfig, "generate_audio", false) {
		reqBody.RenderConfig["generate_audio"] = true
	}

	if err := s.submitAutoVideoBatch(ctx, project, reqBody); err != nil {
		return err
	}

	cfg := parseStoryboardConfigMap(project.StoryboardConfig)
	cfg["auto_video_triggered"] = true
	cfg["auto_video_triggered_at"] = time.Now().UTC().Format(time.RFC3339)
	if data, marshalErr := json.Marshal(cfg); marshalErr == nil {
		project.StoryboardConfig = datatypes.JSON(data)
		_ = s.projectRepo.Update(project)
	}
	_ = s.projectRepo.UpdateStatus(projectID, project.UserID, "video_generating")

	if s.logger != nil {
		s.logger.Info("auto video generation triggered",
			zap.Uint64("project_id", projectID),
			zap.Int("episode_count", len(episodes)),
			zap.String("model_name", modelName),
			zap.Bool("serial_scene", isSerial),
		)
	}
	return nil
}

func parsePositiveFloat(raw string) (float64, error) {
	var value float64
	_, err := fmt.Sscanf(strings.TrimSpace(raw), "%f", &value)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid positive float")
	}
	return value, nil
}

func (s *StoryboardService) submitAutoVideoBatch(ctx context.Context, project *model.Project, body interface{}) error {
	token, err := s.buildProjectServiceToken(project)
	if err != nil {
		return err
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	url := fmt.Sprintf("%s/api/v1/projects/%d/videos/generate-batch", s.videoBaseURL, project.ID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("auto video batch failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (s *StoryboardService) buildProjectServiceToken(project *model.Project) (string, error) {
	secret := strings.TrimSpace(s.jwtSecret)
	if secret == "" {
		return "", errors.New("jwt secret not configured")
	}
	userID := project.UserID
	if userID == 0 {
		userID = 1
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := map[string]interface{}{
		"user_id":    userID,
		"project_id": project.ID,
		"role":       "service",
		"token_type": "access",
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(10 * time.Minute).Unix(),
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + sig, nil
}
