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
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/autovideo/project-service/internal/model"
	"github.com/autovideo/project-service/internal/productionmode"
	"github.com/autovideo/project-service/internal/repository"
	"github.com/autovideo/project-service/internal/scriptpreserve"
	"github.com/autovideo/project-service/internal/scriptsplit"
	"github.com/autovideo/project-service/internal/speechtext"
	"github.com/autovideo/project-service/internal/stylepreset"
)

type CreateEpisodeReq struct {
	EpisodeNumber int    `json:"episode_number" binding:"required,min=1"`
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	ScriptExcerpt string `json:"script_excerpt"` // optional scene content for storyboard generation
}

type EpisodeService struct {
	episodeRepo         *repository.EpisodeRepo
	projectRepo         *repository.ProjectRepo
	storyboardSvc       *StoryboardService
	characterBaseURL    string
	scriptBaseURL       string // for fetching PromptTemplates
	videoBaseURL        string // for cascade-deleting video tasks on episode delete
	modelServiceBaseURL string
	authServiceBaseURL  string
	jwtSecret           string
	httpClient          *http.Client
	llmBaseURL          string
	llmAPIKey           string
	llmModel            string
	storageBaseURL      string
	auditor             *PromptAuditorService // prompt dedup + sensitive word + LLM review
	logger              *zap.Logger

	storyboardRunMu sync.Mutex
	storyboardRuns  map[uint64]storyboardRunState
}

type storyboardRunState struct {
	Scope     string
	StartedAt time.Time
}

type projectLLMConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Source  string
}

type remoteModelConfig struct {
	ID          uint64  `json:"id"`
	Provider    string  `json:"provider"`
	Type        string  `json:"type"`
	APIEndpoint string  `json:"api_endpoint"`
	ModelKey    string  `json:"model_key"`
	APIKeyRef   *string `json:"api_key_ref,omitempty"`
	Name        string  `json:"name"`
}

type serviceRuntimeAPIKey struct {
	Provider   string `json:"provider"`
	KeyAlias   string `json:"key_alias"`
	PlainKey   string `json:"plain_key"`
	BaseURL    string `json:"base_url"`
	ModelScope string `json:"model_scope"`
}

type episodeContextKey string

const (
	skipEpisodeAssetRefreshContextKey      episodeContextKey = "skipEpisodeAssetRefresh"
	skipEpisodeStoryboardTriggerContextKey episodeContextKey = "skipEpisodeStoryboardTrigger"
	skipEpisodeAssetExtractionContextKey   episodeContextKey = "skipEpisodeAssetExtraction"
)

func WithSkipEpisodeAssetRefresh(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipEpisodeAssetRefreshContextKey, true)
}

func shouldSkipEpisodeAssetRefresh(ctx context.Context) bool {
	skip, _ := ctx.Value(skipEpisodeAssetRefreshContextKey).(bool)
	return skip
}

func WithSkipEpisodeStoryboardTrigger(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipEpisodeStoryboardTriggerContextKey, true)
}

func shouldSkipEpisodeStoryboardTrigger(ctx context.Context) bool {
	skip, _ := ctx.Value(skipEpisodeStoryboardTriggerContextKey).(bool)
	return skip
}

func WithSkipEpisodeAssetExtraction(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipEpisodeAssetExtractionContextKey, true)
}

func shouldSkipEpisodeAssetExtraction(ctx context.Context) bool {
	skip, _ := ctx.Value(skipEpisodeAssetExtractionContextKey).(bool)
	return skip
}

func shouldEnableSceneSerial(projectType string) bool {
	return strings.TrimSpace(projectType) == "video_serial"
}

func storyboardRunScope(episodeID *uint64) string {
	if episodeID == nil {
		return "project"
	}
	return fmt.Sprintf("episode:%d", *episodeID)
}

func (s *EpisodeService) acquireStoryboardRun(projectID uint64, episodeID *uint64) error {
	s.storyboardRunMu.Lock()
	defer s.storyboardRunMu.Unlock()
	if s.storyboardRuns == nil {
		s.storyboardRuns = make(map[uint64]storyboardRunState)
	}
	scope := storyboardRunScope(episodeID)
	if current, ok := s.storyboardRuns[projectID]; ok {
		return fmt.Errorf("storyboard extraction already running for project %d (active_scope=%s, requested_scope=%s, started_at=%s)", projectID, current.Scope, scope, current.StartedAt.Format(time.RFC3339))
	}
	s.storyboardRuns[projectID] = storyboardRunState{Scope: scope, StartedAt: time.Now()}
	return nil
}

func (s *EpisodeService) releaseStoryboardRun(projectID uint64) {
	s.storyboardRunMu.Lock()
	defer s.storyboardRunMu.Unlock()
	delete(s.storyboardRuns, projectID)
}

// NewEpisodeService —— 创建剧集服务实例，初始化 LLM 及存储配置
func NewEpisodeService(
	episodeRepo *repository.EpisodeRepo,
	projectRepo *repository.ProjectRepo,
	llmBaseURL, llmAPIKey, llmModel, storageBaseURL string,
) *EpisodeService {
	if llmBaseURL == "" {
		llmBaseURL = "https://api.easyart.cc/v1"
	}
	if llmModel == "" {
		llmModel = "gpt-5.4-mini"
	}
	if storageBaseURL == "" {
		storageBaseURL = "http://localhost:8009"
	}
	base := strings.TrimRight(llmBaseURL, "/")
	return &EpisodeService{
		episodeRepo:    episodeRepo,
		projectRepo:    projectRepo,
		llmBaseURL:     base,
		llmAPIKey:      llmAPIKey,
		llmModel:       llmModel,
		storageBaseURL: storageBaseURL,
		httpClient:     &http.Client{Timeout: 5 * time.Minute},
		auditor:        NewPromptAuditorService(base, llmAPIKey, llmModel, nil, nil),
		storyboardRuns: make(map[uint64]storyboardRunState),
	}
}

// SetLogger —— 设置剧集服务的日志记录器
func (s *EpisodeService) SetLogger(l *zap.Logger) {
	s.logger = l
	if s.auditor != nil {
		s.auditor.logger = l
	}
}

// SetStoryboardService —— 注入分镜服务依赖，用于自动创建分镜
func (s *EpisodeService) SetStoryboardService(svc *StoryboardService) { s.storyboardSvc = svc }

func (s *EpisodeService) SetCharacterService(baseURL, jwtSecret string) {
	s.characterBaseURL = strings.TrimRight(baseURL, "/")
	s.jwtSecret = jwtSecret
}

// SetScriptService configures the optional script-service URL so that
// the episode service can fetch PromptTemplates for storyboard creation.
func (s *EpisodeService) SetScriptService(baseURL string) {
	s.scriptBaseURL = strings.TrimRight(baseURL, "/")
}

// SetVideoService configures the video-service URL so that episode deletion
// can cascade-delete associated VideoTask/DubbingTask records.
func (s *EpisodeService) SetVideoService(baseURL string) {
	s.videoBaseURL = strings.TrimRight(baseURL, "/")
}

func (s *EpisodeService) SetServiceEndpoints(modelBaseURL, authBaseURL, jwtSecret string) {
	s.modelServiceBaseURL = strings.TrimRight(modelBaseURL, "/")
	s.authServiceBaseURL = strings.TrimRight(authBaseURL, "/")
	if strings.TrimSpace(jwtSecret) != "" {
		s.jwtSecret = jwtSecret
	}
}

func (s *EpisodeService) resolveProjectLLMConfig(ctx context.Context, project *model.Project) projectLLMConfig {
	cfg := projectLLMConfig{BaseURL: s.llmBaseURL, APIKey: s.llmAPIKey, Model: s.llmModel, Source: "default"}
	if project == nil || project.TextModelID == nil || *project.TextModelID == 0 || s.modelServiceBaseURL == "" || s.authServiceBaseURL == "" || s.jwtSecret == "" {
		return cfg
	}
	modelMeta, err := s.fetchRemoteModel(ctx, *project.TextModelID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("resolve project text model failed", zap.Uint64("project_id", project.ID), zap.Uint64("text_model_id", *project.TextModelID), zap.Error(err))
		}
		return cfg
	}
	if strings.TrimSpace(modelMeta.Type) != "" && strings.TrimSpace(modelMeta.Type) != "llm" {
		return cfg
	}
	keys, err := s.fetchRuntimeAPIKeys(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("fetch runtime api keys for project text model failed", zap.Uint64("project_id", project.ID), zap.Error(err))
		}
		return cfg
	}
	selected := matchRuntimeKeyForModel(keys, modelMeta)
	if selected == nil {
		if s.logger != nil {
			s.logger.Warn("no runtime key matched project text model", zap.Uint64("project_id", project.ID), zap.Uint64("text_model_id", *project.TextModelID), zap.String("provider", modelMeta.Provider), zap.String("model_key", modelMeta.ModelKey))
		}
		return cfg
	}
	if strings.TrimSpace(modelMeta.APIEndpoint) != "" {
		cfg.BaseURL = strings.TrimRight(strings.TrimSpace(modelMeta.APIEndpoint), "/")
	} else if strings.TrimSpace(selected.BaseURL) != "" {
		cfg.BaseURL = strings.TrimRight(strings.TrimSpace(selected.BaseURL), "/")
	}
	if strings.TrimSpace(modelMeta.ModelKey) != "" {
		cfg.Model = strings.TrimSpace(modelMeta.ModelKey)
	}
	cfg.APIKey = strings.TrimSpace(selected.PlainKey)
	cfg.Source = fmt.Sprintf("project.text_model_id=%d", *project.TextModelID)
	return cfg
}

func (s *EpisodeService) fetchRemoteModel(ctx context.Context, id uint64) (*remoteModelConfig, error) {
	token, err := s.buildServiceToken(0)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/models/%d", s.modelServiceBaseURL, id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("model service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Data remoteModelConfig `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return &payload.Data, nil
}

func (s *EpisodeService) fetchRuntimeAPIKeys(ctx context.Context) ([]serviceRuntimeAPIKey, error) {
	token, err := s.buildServiceToken(0)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.authServiceBaseURL+"/internal/runtime-api-keys", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Internal-Service", "project-service")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("auth service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Data []serviceRuntimeAPIKey `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func matchRuntimeKeyForModel(keys []serviceRuntimeAPIKey, modelMeta *remoteModelConfig) *serviceRuntimeAPIKey {
	if modelMeta == nil {
		return nil
	}
	ref := ""
	if modelMeta.APIKeyRef != nil {
		ref = strings.TrimSpace(*modelMeta.APIKeyRef)
	}
	provider := strings.ToLower(strings.TrimSpace(modelMeta.Provider))
	for i := range keys {
		key := &keys[i]
		if ref != "" && strings.Contains(strings.ToLower(strings.TrimSpace(key.KeyAlias)), strings.ToLower(ref)) {
			return key
		}
	}
	for i := range keys {
		key := &keys[i]
		kp := strings.ToLower(strings.TrimSpace(key.Provider))
		if provider == "" {
			continue
		}
		if strings.Contains(kp, "."+provider) || strings.HasSuffix(kp, provider) || providerMatchesEndpoint(provider, modelMeta.APIEndpoint, key.BaseURL) {
			return key
		}
	}
	for i := range keys {
		key := &keys[i]
		if strings.TrimSpace(key.PlainKey) != "" {
			return key
		}
	}
	return nil
}

func providerMatchesEndpoint(provider, modelEndpoint, runtimeBase string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return false
	}
	for _, raw := range []string{modelEndpoint, runtimeBase} {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err == nil && strings.Contains(strings.ToLower(u.Host), provider) {
			return true
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(raw)), provider) {
			return true
		}
	}
	return false
}

func (s *EpisodeService) autoAssetPipelineReady() bool {
	return s.characterBaseURL != "" && s.jwtSecret != ""
}

func (s *EpisodeService) buildServiceToken(projectID uint64) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := map[string]interface{}{
		"user_id":    1,
		"project_id": projectID,
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
	mac := hmac.New(sha256.New, []byte(s.jwtSecret))
	mac.Write([]byte(unsigned))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + sig, nil
}

type assetReference struct {
	ID          int64
	Type        string
	Name        string
	Description string
	ImageURL    string
	EpisodeIDs  []int64
}

func (s *EpisodeService) deleteExistingAssets(ctx context.Context, projectID uint64) error {
	if !s.autoAssetPipelineReady() {
		return nil
	}
	token, err := s.buildServiceToken(projectID)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/api/v1/projects/%d/assets", s.characterBaseURL, projectID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete assets failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func (s *EpisodeService) extractAssetsForEpisode(ctx context.Context, projectID, episodeID uint64) error {
	if !s.autoAssetPipelineReady() {
		if s.logger != nil {
			s.logger.Warn("skip episode asset extraction because character service is not configured",
				zap.Uint64("project_id", projectID),
				zap.Uint64("episode_id", episodeID),
				zap.Bool("has_character_base_url", s.characterBaseURL != ""),
				zap.Bool("has_jwt_secret", s.jwtSecret != ""),
			)
		}
		return nil
	}
	token, err := s.buildServiceToken(projectID)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/v1/projects/%d/assets/extract-episode/%d", s.characterBaseURL, projectID, episodeID)
	const maxAttempts = 5
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
		if reqErr != nil {
			return reqErr
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		if shouldSkipEpisodeStoryboardTrigger(ctx) {
			req.Header.Set("X-Autovideo-Skip-Storyboard-Trigger", "true")
		}

		resp, doErr := s.httpClient.Do(req)
		if doErr != nil {
			lastErr = doErr
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode < http.StatusBadRequest {
				lastErr = nil
				break
			}
			lastErr = fmt.Errorf("extract episode assets failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
			if resp.StatusCode < http.StatusInternalServerError {
				return lastErr
			}
		}

		if attempt == maxAttempts {
			break
		}
		backoff := time.Duration(attempt) * 2 * time.Second
		if s.logger != nil {
			s.logger.Warn("episode asset extraction dispatch failed; retrying",
				zap.Uint64("project_id", projectID),
				zap.Uint64("episode_id", episodeID),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(lastErr),
			)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	if lastErr != nil {
		return lastErr
	}
	if s.logger != nil {
		s.logger.Info("triggered episode asset extraction",
			zap.Uint64("project_id", projectID),
			zap.Uint64("episode_id", episodeID),
			zap.String("character_service_url", url),
		)
	}
	return nil
}

func (s *EpisodeService) triggerEpisodeAssetGeneration(ctx context.Context, projectID, episodeID uint64) error {
	if !s.autoAssetPipelineReady() {
		return nil
	}
	token, err := s.buildServiceToken(projectID)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/v1/projects/%d/assets/generate-all?episode_id=%d", s.characterBaseURL, projectID, episodeID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("trigger episode asset generation failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if s.logger != nil {
		s.logger.Info("triggered episode asset generation",
			zap.Uint64("project_id", projectID),
			zap.Uint64("episode_id", episodeID),
		)
	}
	return nil
}

type episodeAssetSnapshot struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (s *EpisodeService) listEpisodeAssetSnapshots(ctx context.Context, projectID, episodeID uint64) ([]episodeAssetSnapshot, error) {
	if !s.autoAssetPipelineReady() {
		return nil, nil
	}
	token, err := s.buildServiceToken(projectID)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/api/v1/projects/%d/assets?page=1&page_size=500&episode_id=%d", s.characterBaseURL, projectID, episodeID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("list episode assets failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var paged struct {
		Data struct {
			Items []episodeAssetSnapshot `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &paged); err == nil && len(paged.Data.Items) > 0 {
		return paged.Data.Items, nil
	}
	var legacy struct {
		Data []episodeAssetSnapshot `json:"data"`
	}
	if err := json.Unmarshal(body, &legacy); err == nil {
		return legacy.Data, nil
	}
	return nil, nil
}

func episodeAssetExtractionInFlight(assets []episodeAssetSnapshot) bool {
	for _, asset := range assets {
		if asset.Name == "__extracting__" || asset.Status == "extracting" {
			return true
		}
	}
	return false
}

func countSettledEpisodeAssets(assets []episodeAssetSnapshot) int {
	count := 0
	for _, asset := range assets {
		if asset.Name == "__extracting__" || asset.Status == "extracting" {
			continue
		}
		count++
	}
	return count
}

func (s *EpisodeService) listProjectAssetSnapshots(ctx context.Context, projectID uint64) ([]episodeAssetSnapshot, error) {
	if !s.autoAssetPipelineReady() {
		return nil, nil
	}
	token, err := s.buildServiceToken(projectID)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/api/v1/projects/%d/assets?page=1&page_size=500", s.characterBaseURL, projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("list project assets failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var paged struct {
		Data struct {
			Items []episodeAssetSnapshot `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &paged); err == nil && len(paged.Data.Items) > 0 {
		return paged.Data.Items, nil
	}
	var legacy struct {
		Data []episodeAssetSnapshot `json:"data"`
	}
	if err := json.Unmarshal(body, &legacy); err == nil {
		return legacy.Data, nil
	}
	return nil, nil
}

// waitForProjectAssetExtraction blocks until all async episode/project extractions finish.
func (s *EpisodeService) waitForProjectAssetExtraction(ctx context.Context, projectID uint64, expectDispatch bool) error {
	if !s.autoAssetPipelineReady() {
		return nil
	}
	const dispatchGrace = 45 * time.Second
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	started := time.Now()

	for {
		assets, err := s.listProjectAssetSnapshots(ctx, projectID)
		if err != nil {
			return err
		}
		if episodeAssetExtractionInFlight(assets) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
			continue
		}
		if !expectDispatch || countSettledEpisodeAssets(assets) > 0 {
			if s.logger != nil {
				s.logger.Info("project asset extraction settled before storyboard split",
					zap.Uint64("project_id", projectID),
					zap.Int("asset_count", countSettledEpisodeAssets(assets)),
					zap.Bool("expect_dispatch", expectDispatch),
				)
			}
			return nil
		}
		if time.Since(started) < dispatchGrace {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
			continue
		}
		if s.logger != nil {
			s.logger.Warn("project asset extraction sentinel did not appear within grace window; continuing storyboard split",
				zap.Uint64("project_id", projectID),
				zap.Duration("grace", dispatchGrace),
			)
		}
		return nil
	}
}

// waitForEpisodeAssetExtraction blocks until async episode extraction finishes or the context expires.
// When expectDispatch is true, tolerate a short window where character-service has accepted the
// extract request (202) but not yet created the __extracting__ sentinel.
func (s *EpisodeService) waitForEpisodeAssetExtraction(ctx context.Context, projectID, episodeID uint64, expectDispatch bool) error {
	if !s.autoAssetPipelineReady() {
		return nil
	}
	const dispatchGrace = 30 * time.Second
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	started := time.Now()

	for {
		assets, err := s.listEpisodeAssetSnapshots(ctx, projectID, episodeID)
		if err != nil {
			return err
		}
		if episodeAssetExtractionInFlight(assets) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
			continue
		}
		if !expectDispatch || countSettledEpisodeAssets(assets) > 0 {
			if s.logger != nil {
				s.logger.Info("episode asset extraction settled before storyboard split",
					zap.Uint64("project_id", projectID),
					zap.Uint64("episode_id", episodeID),
					zap.Int("asset_count", len(assets)),
					zap.Bool("expect_dispatch", expectDispatch),
				)
			}
			return nil
		}
		if time.Since(started) < dispatchGrace {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
			continue
		}
		if s.logger != nil {
			s.logger.Warn("episode asset extraction sentinel did not appear within grace window; continuing storyboard split",
				zap.Uint64("project_id", projectID),
				zap.Uint64("episode_id", episodeID),
				zap.Duration("grace", dispatchGrace),
			)
		}
		return nil
	}
}

func (s *EpisodeService) fetchAssetReferences(ctx context.Context, projectID uint64, episodeID *uint64) []assetReference {
	if !s.autoAssetPipelineReady() {
		return nil
	}
	token, err := s.buildServiceToken(projectID)
	if err != nil {
		return nil
	}
	url := fmt.Sprintf("%s/api/v1/projects/%d/assets?status=completed&page=1&page_size=500", s.characterBaseURL, projectID)
	if episodeID != nil {
		url += fmt.Sprintf("&episode_id=%d", *episodeID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)

	type assetItem struct {
		ID          int64   `json:"id"`
		Type        string  `json:"type"`
		Name        string  `json:"name"`
		Description string  `json:"description"`
		ImageURL    string  `json:"image_url"`
		EpisodeIDs  []int64 `json:"episode_ids"`
	}

	var paged struct {
		Data struct {
			Items []assetItem `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &paged); err == nil && len(paged.Data.Items) > 0 {
		out := make([]assetReference, 0, len(paged.Data.Items))
		for _, item := range paged.Data.Items {
			out = append(out, assetReference{ID: item.ID, Type: item.Type, Name: item.Name, Description: item.Description, ImageURL: item.ImageURL, EpisodeIDs: item.EpisodeIDs})
		}
		return out
	}

	var legacy struct {
		Data []assetItem `json:"data"`
	}
	if err := json.Unmarshal(body, &legacy); err == nil && len(legacy.Data) > 0 {
		out := make([]assetReference, 0, len(legacy.Data))
		for _, item := range legacy.Data {
			out = append(out, assetReference{ID: item.ID, Type: item.Type, Name: item.Name, Description: item.Description, ImageURL: item.ImageURL, EpisodeIDs: item.EpisodeIDs})
		}
		return out
	}
	return nil
}

func filterAssetReferencesByEpisode(assets []assetReference, episodeID uint64) []assetReference {
	if len(assets) == 0 {
		return nil
	}
	out := make([]assetReference, 0, len(assets))
	for _, asset := range assets {
		if len(asset.EpisodeIDs) == 0 {
			out = append(out, asset)
			continue
		}
		for _, eid := range asset.EpisodeIDs {
			if uint64(eid) == episodeID {
				out = append(out, asset)
				break
			}
		}
	}
	return out
}

func matchAssetsToScene(scene llmScene, assets []assetReference) []assetReference {
	if len(assets) == 0 {
		return nil
	}
	lookupText := strings.ToLower(strings.Join([]string{scene.Description, scene.Location, scene.Dialogue, strings.Join(scene.Characters, " "), strings.Join(scene.Items, " ")}, " "))
	seen := map[int64]struct{}{}
	var matched []assetReference
	for _, asset := range assets {
		name := strings.ToLower(strings.TrimSpace(asset.Name))
		desc := strings.ToLower(strings.TrimSpace(asset.Description))
		if name == "" {
			continue
		}
		score := 0
		if strings.Contains(lookupText, name) {
			score += 3
		}
		if desc != "" && strings.Contains(lookupText, desc) {
			score++
		}
		switch asset.Type {
		case "character":
			for _, ch := range scene.Characters {
				if strings.EqualFold(strings.TrimSpace(ch), strings.TrimSpace(asset.Name)) {
					score += 5
					break
				}
			}
		case "scene", "location":
			if scene.Location != "" && (strings.Contains(strings.ToLower(scene.Location), name) || strings.Contains(name, strings.ToLower(scene.Location))) {
				score += 4
			}
		case "prop", "item":
			for _, item := range scene.Items {
				if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(asset.Name)) {
					score += 4
					break
				}
			}
		}
		if score >= 4 {
			if _, ok := seen[asset.ID]; ok {
				continue
			}
			seen[asset.ID] = struct{}{}
			matched = append(matched, asset)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].ID < matched[j].ID })
	return matched
}

func buildAssetReferenceNote(assets []assetReference) string {
	if len(assets) == 0 {
		return ""
	}
	parts := make([]string, 0, len(assets))
	for _, asset := range assets {
		piece := strings.TrimSpace(asset.Name)
		if asset.Description != "" {
			piece += "（" + strings.TrimSpace(asset.Description) + "）"
		}
		if piece != "" {
			parts = append(parts, piece)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "关联资源参考：" + strings.Join(parts, "；") + "。"
}

func assetReferenceIDs(assets []assetReference) []int64 {
	if len(assets) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(assets))
	for _, asset := range assets {
		ids = append(ids, asset.ID)
	}
	return ids
}

func (s *EpisodeService) extractAssetsAfterSplit(ctx context.Context, projectID uint64, episodes []model.Episode) {
	if s.characterBaseURL == "" || s.jwtSecret == "" || len(episodes) == 0 {
		return
	}

	sorted := append([]model.Episode(nil), episodes...)
	sort.Slice(sorted, func(i, j int) bool {
		left, right := sorted[i].EpisodeNumber, sorted[j].EpisodeNumber
		if left <= 0 {
			left = int(sorted[i].ID)
		}
		if right <= 0 {
			right = int(sorted[j].ID)
		}
		if left == right {
			return sorted[i].ID < sorted[j].ID
		}
		return left < right
	})

	if err := s.deleteExistingAssets(ctx, projectID); err != nil {
		if s.logger != nil {
			s.logger.Error("delete existing assets before episode extraction failed; aborting re-extraction to avoid duplicates",
				zap.Uint64("project_id", projectID),
				zap.Error(err),
			)
		}
		return // don't proceed — we'd create duplicate assets
	}

	dispatched := 0
	for _, ep := range sorted {
		if err := s.extractAssetsForEpisode(ctx, projectID, ep.ID); err != nil {
			if s.logger != nil {
				s.logger.Warn("episode asset extraction dispatch failed",
					zap.Uint64("project_id", projectID),
					zap.Uint64("episode_id", ep.ID),
					zap.Int("episode_number", ep.EpisodeNumber),
					zap.Error(err),
				)
			}
			continue
		}
		dispatched++
	}
	if dispatched == 0 {
		if s.logger != nil {
			s.logger.Warn("no episode asset extraction requests were dispatched",
				zap.Uint64("project_id", projectID),
				zap.Int("episode_count", len(sorted)),
			)
		}
		return
	}

	if err := s.waitForProjectAssetExtraction(ctx, projectID, true); err != nil && s.logger != nil {
		s.logger.Warn("project asset extraction did not fully settle before storyboard split; continuing",
			zap.Uint64("project_id", projectID),
			zap.Int("dispatched_episodes", dispatched),
			zap.Error(err),
		)
	}
	if s.logger != nil {
		s.logger.Info("episode asset extraction completed for all dispatched episodes",
			zap.Uint64("project_id", projectID),
			zap.Int("episode_count", dispatched),
		)
	}
}

// ─── Progress tracking ──────────────────────────────────────────────────────

// StageProgress tracks a single pipeline stage.
type StageProgress struct {
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Current   int    `json:"current,omitempty"`
	Status    string `json:"status"` // pending | running | done | failed
}

type storyboardRuntimeConfig struct {
	Duration                   int    `json:"duration"`
	VideoModel                 string `json:"video_model"`
	StylePreset                string `json:"style_preset"`
	MotionMode                 string `json:"motion_mode"`
	AspectRatio                string `json:"aspect_ratio"`
	Resolution                 string `json:"resolution"`
	SpeechPace                 string `json:"speech_pace"`
	AutoSplitAfterOptimization bool   `json:"auto_split_after_optimization"`
}

// ProgressInfo is the JSON structure persisted in project.progress.
type AutoSplitMeta struct {
	Enabled               bool   `json:"enabled,omitempty"`
	Duration              int    `json:"duration,omitempty"`
	VideoModel            string `json:"video_model,omitempty"`
	StylePreset           string `json:"style_preset,omitempty"`
	ScriptLength          int    `json:"script_length,omitempty"`
	EstimatedEpisodes     int    `json:"estimated_episodes,omitempty"`
	TargetCharsPerEpisode int    `json:"target_chars_per_episode,omitempty"`
	OriginalScript        string `json:"original_script,omitempty"`
	OptimizedScript       string `json:"optimized_script,omitempty"`
	ConsistencyPremise    string `json:"consistency_premise,omitempty"`
	OptimizationPrompt    string `json:"optimization_prompt,omitempty"`
}

type ProgressInfo struct {
	Stage          string         `json:"stage"` // episode_splitting | scene_splitting | script_prepping | idle
	EpisodeSplit   *StageProgress `json:"episode_split,omitempty"`
	SceneSplit     *StageProgress `json:"scene_split,omitempty"`
	Message        string         `json:"message,omitempty"`
	PhaseLabel     string         `json:"phase_label,omitempty"`
	NextStep       string         `json:"next_step,omitempty"`
	CurrentEpisode int            `json:"current_episode,omitempty"`
	TotalEpisodes  int            `json:"total_episodes,omitempty"`
	AutoSplit      *AutoSplitMeta `json:"auto_split,omitempty"`
	StartedAt      string         `json:"started_at,omitempty"`
	UpdatedAt      string         `json:"updated_at,omitempty"`
}

const (
	keywordExtractionTimeout = 15 * time.Second
	profileEnrichmentTimeout = 120 * time.Second
	episodeSplitTimeout      = 30 * time.Second
	episodeEnrichTimeout     = 20 * time.Second
)

var ErrScreenplayNotReady = errors.New("screenplay not ready")

func parseStoryboardRuntimeConfig(project *model.Project) storyboardRuntimeConfig {
	cfg := storyboardRuntimeConfig{}
	if project == nil || len(project.StoryboardConfig) == 0 {
		return cfg
	}
	_ = json.Unmarshal(project.StoryboardConfig, &cfg)
	cfg.VideoModel = strings.TrimSpace(cfg.VideoModel)
	cfg.StylePreset = stylepreset.Canonical(strings.TrimSpace(cfg.StylePreset))
	cfg.MotionMode = strings.TrimSpace(cfg.MotionMode)
	cfg.AspectRatio = strings.TrimSpace(cfg.AspectRatio)
	cfg.Resolution = strings.TrimSpace(cfg.Resolution)
	cfg.SpeechPace = canonicalSpeechPace(cfg.SpeechPace)
	if cfg.SpeechPace == "" {
		cfg.SpeechPace = productionmode.DefaultSpeechPace(cfg.StylePreset)
	}
	if cfg.SpeechPace == "" {
		cfg.SpeechPace = "normal"
	}
	return cfg
}

func toProductionRuntimeConfig(runtimeCfg storyboardRuntimeConfig) productionmode.RuntimeConfig {
	return productionmode.RuntimeConfig{
		Duration:                   runtimeCfg.Duration,
		VideoModel:                 runtimeCfg.VideoModel,
		StylePreset:                runtimeCfg.StylePreset,
		AutoSplitAfterOptimization: runtimeCfg.AutoSplitAfterOptimization,
	}
}

func fromProductionAutoSplitMeta(meta productionmode.AutoSplitMeta) AutoSplitMeta {
	return AutoSplitMeta{
		Enabled:               meta.Enabled,
		Duration:              meta.Duration,
		VideoModel:            meta.VideoModel,
		StylePreset:           meta.StylePreset,
		ScriptLength:          meta.ScriptLength,
		TargetCharsPerEpisode: meta.TargetCharsPerEpisode,
		EstimatedEpisodes:     meta.EstimatedEpisodes,
		OriginalScript:        meta.OriginalScript,
		OptimizedScript:       meta.OptimizedScript,
		ConsistencyPremise:    meta.ConsistencyPremise,
	}
}

func buildAutoSplitMeta(scriptText string, runtimeCfg storyboardRuntimeConfig, profile productionmode.Profile) AutoSplitMeta {
	return fromProductionAutoSplitMeta(productionmode.BuildAutoSplitMeta(scriptText, toProductionRuntimeConfig(runtimeCfg), profile))
}

func estimateAutoSplitTargetEpisodes(scriptText string, runtimeCfg storyboardRuntimeConfig, profile productionmode.Profile) int {
	return buildAutoSplitMeta(scriptText, runtimeCfg, profile).EstimatedEpisodes
}

type optimizedAdScriptResult struct {
	OptimizedScript    string `json:"optimized_script"`
	ConsistencyPremise string `json:"consistency_premise"`
}

func (s *EpisodeService) buildDefaultAdCopyUserPrompt(project *model.Project, trimmed string) string {
	runtimeCfg := parseStoryboardRuntimeConfig(project)
	duration := runtimeCfg.Duration
	if duration <= 0 {
		duration = 10
	}
	stylePreset := stylepreset.Canonical(runtimeCfg.StylePreset)
	if stylePreset == "" {
		stylePreset = stylepreset.Default
	}
	styleLabel := stylePreset
	videoModel := strings.TrimSpace(runtimeCfg.VideoModel)
	if videoModel == "" {
		videoModel = "default"
	}
	aspectRatio := strings.TrimSpace(runtimeCfg.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = "未指定"
	}
	resolution := strings.TrimSpace(runtimeCfg.Resolution)
	if resolution == "" {
		resolution = "未指定"
	}
	return fmt.Sprintf("项目标题：%s\n目标风格：%s（canonical=%s）\n目标视频模型：%s\n目标画面比例：%s\n目标分辨率：%s\n目标单分镜时长：%d 秒\n\n请先优化下面这篇广告文案，使其更适合后续按“台词 / 口播承载量 + 上述视频约束”自动切分，并单独输出后续必须继承的一致性前提。\n\n其中：\n- 画面比例会直接影响主体排布、左右留白、前景/中景/后景层次与镜头构图，请不要忽略。\n- 分辨率会影响单镜可承载的细节密度；分辨率较低时避免在一个镜头里塞入过多主体、过多小字、过多细碎动作。\n- 单分镜时长优先决定单镜可承载的台词 / 口播长度；不要把明显超出该时长的台词硬塞进同一镜头，也不要把本可在该时长内说完的一句口播拆成多个空镜。\n\n请围绕这 14 个方向补全并写实：\n1. 世界观 / 故事发生的视觉宇宙\n2. 空间 / 在哪里\n3. 时间 / 几点、白天还是夜晚\n4. 人物 / 谁在说、谁在动\n5. 服装 / 穿什么\n6. 动作 / 做什么\n7. 核心物件 / 镜头重点\n8. 光线 / 怎么打光\n9. 色彩 / 什么色调\n10. 材质 / 表面质感是什么\n11. 镜头运动 / 怎么拍\n12. 情绪 / 传达什么感觉\n13. 转场 / 怎么切\n14. 字幕 / 屏幕文字、配音 / 口播内容、以及最终给 AI 的 Prompt 描述\n\n一致性前提必须至少覆盖：\n1. 人物身份/外观/服装/年龄感/口播身份\n2. 世界观、核心场景与空间锚点（室内外、前后左右、景别、机位）\n3. 关键道具、品牌信息与镜头重点\n4. 光线/色调/材质的稳定风格\n5. 时间线、动作链与转场约束\n6. 台词/旁白/字幕/CTA 收束方式\n7. 最终给 AI 生成时必须保留的 Prompt 关键元素\n\n原始广告文案如下：\n\n%s",
		strings.TrimSpace(project.Title), styleLabel, stylePreset, videoModel, aspectRatio, resolution, duration, trimmed)
}

func isAdWorkbenchProject(project *model.Project) bool {
	return productionmode.IsAd(project)
}

func adWorkbenchPromptDirective() string {
	return productionmode.AdWorkbenchDirective()
}


func (s *EpisodeService) optimizeProjectScriptForAutoSplit(ctx context.Context, project *model.Project, scriptText string, customPrompt string) (*optimizedAdScriptResult, error) {
	llmCfg := s.resolveProjectLLMConfig(ctx, project)
	trimmed := strings.TrimSpace(scriptText)
	if trimmed == "" {
		return nil, errors.New("empty script text")
	}

	systemPrompt := normalizeAdCopyPrompt(customPrompt) + "\n\n返回严格 JSON（不要 markdown 代码块）：\n{\n  \"optimized_script\": \"优化后的完整广告文案\",\n  \"consistency_premise\": \"后续分镜/图片/视频都必须继承的一致性前提，使用条目化自然语言\"\n}"
	if isAdWorkbenchProject(project) {
		systemPrompt += "\n\n广告工作台补充规则：\n" + adWorkbenchPromptDirective()
	}

	userPrompt := s.buildDefaultAdCopyUserPrompt(project, trimmed)

	reqBody := map[string]interface{}{
		"model": llmCfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature":     0.5,
		"max_tokens":      8192,
		"response_format": map[string]string{"type": "json_object"},
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, llmCfg.BaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+llmCfg.APIKey)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM responded %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}
	content := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	var result optimizedAdScriptResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse optimized script json: %w", err)
	}
	result.OptimizedScript = strings.TrimSpace(result.OptimizedScript)
	result.ConsistencyPremise = strings.TrimSpace(result.ConsistencyPremise)
	if result.OptimizedScript == "" {
		return nil, errors.New("optimizer returned empty optimized_script")
	}
	return &result, nil
}

// updateProgress —— 将进度信息序列化并持久化到项目的 progress 字段
func (s *EpisodeService) updateProgress(projectID uint64, info ProgressInfo) {
	var previous ProgressInfo
	if project, err := s.projectRepo.FindByIDNoAuth(projectID); err == nil && len(project.Progress) > 0 {
		_ = json.Unmarshal(project.Progress, &previous)
	}
	if info.Stage == "" {
		info.Stage = previous.Stage
	}
	if info.Message == "" {
		info.Message = previous.Message
	}
	if info.PhaseLabel == "" {
		info.PhaseLabel = previous.PhaseLabel
	}
	if info.NextStep == "" {
		info.NextStep = previous.NextStep
	}
	if info.CurrentEpisode == 0 {
		info.CurrentEpisode = previous.CurrentEpisode
	}
	if info.TotalEpisodes == 0 {
		info.TotalEpisodes = previous.TotalEpisodes
	}
	if info.EpisodeSplit == nil {
		info.EpisodeSplit = previous.EpisodeSplit
	}
	if info.SceneSplit == nil {
		info.SceneSplit = previous.SceneSplit
	}
	if info.AutoSplit == nil {
		info.AutoSplit = previous.AutoSplit
	}
	if info.StartedAt == "" {
		if previous.Stage == info.Stage {
			info.StartedAt = previous.StartedAt
		}
	}
	if info.StartedAt == "" {
		info.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if info.UpdatedAt == "" {
		info.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.Marshal(info)
	if err != nil {
		return
	}
	_ = s.projectRepo.UpdateProgress(projectID, data)
}

// MarkGenerationFailed forcefully marks a project generation pipeline as failed.
func (s *EpisodeService) MarkGenerationFailed(projectID uint64, message string) {
	project, err := s.projectRepo.FindByIDNoAuth(projectID)
	if err != nil {
		return
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "后台生成任务失败"
	}
	s.updateProgress(projectID, ProgressInfo{
		Stage:        "idle",
		Message:      msg,
		EpisodeSplit: &StageProgress{Status: "failed"},
	})
	_ = s.projectRepo.UpdateStatus(projectID, project.UserID, "failed")
	if s.logger != nil {
		s.logger.Error("generation marked failed after panic or forced abort",
			zap.Uint64("project_id", projectID),
			zap.String("message", msg),
		)
	}
}

// Create —— 手动创建单条剧集记录
func (s *EpisodeService) Create(projectID uint64, req CreateEpisodeReq) (*model.Episode, error) {
	episode := &model.Episode{
		ProjectID:         projectID,
		EpisodeNumber:     req.EpisodeNumber,
		Title:             req.Title,
		Summary:           req.Summary,
		ScriptExcerpt:     req.ScriptExcerpt,
		WordCount:         utf8.RuneCountInString(req.ScriptExcerpt),
		EstimatedDuration: utf8.RuneCountInString(req.ScriptExcerpt) / 5,
		Status:            "draft",
	}
	if err := s.episodeRepo.Create(episode); err != nil {
		return nil, err
	}
	// Advance project from 'draft' to 'script_ready' when the first manual episode is created
	// so downstream tabs (storyboard, video) treat the project as ready to proceed.
	if project, err := s.projectRepo.FindByIDNoAuth(projectID); err == nil && project.Status == "draft" {
		_ = s.projectRepo.UpdateStatus(projectID, project.UserID, "script_ready")
	}
	return episode, nil
}

// ListByProject —— 查询指定项目下的所有剧集列表
func (s *EpisodeService) ListByProject(projectID uint64) ([]model.Episode, error) {
	return s.episodeRepo.FindByProjectID(projectID)
}

// ExtractStoryboards —— 为项目或单集执行真正的分镜拆分，并创建 storyboard 记录
func (s *EpisodeService) ExtractStoryboards(ctx context.Context, projectID uint64, episodeID *uint64) (int, error) {
	if s.storyboardSvc == nil || s.storyboardSvc.repo == nil {
		return 0, errors.New("storyboard service not configured")
	}
	if err := s.acquireStoryboardRun(projectID, episodeID); err != nil {
		if s.logger != nil {
			s.logger.Warn("skip storyboard extraction because another run is active",
				zap.Uint64("project_id", projectID),
				zap.String("requested_scope", storyboardRunScope(episodeID)),
				zap.Error(err),
			)
		}
		return 0, err
	}
	defer s.releaseStoryboardRun(projectID)

	project, err := s.projectRepo.FindByIDNoAuth(projectID)
	if err != nil {
		return 0, fmt.Errorf("project not found: %w", err)
	}
	if s.logger != nil {
		s.logger.Info("starting manual storyboard extraction",
			zap.Uint64("project_id", projectID),
			zap.Bool("single_episode", episodeID != nil),
		)
	}

	// Ensure the project still has usable script text so manual extraction follows
	// the same input basis as the original episode/storyboard generation pipeline.
	scriptText := strings.TrimSpace(project.ScriptText)
	if scriptText == "" && strings.TrimSpace(project.ScriptFileURL) != "" {
		body, fetchErr := s.fetchScriptContent(ctx, project.ScriptFileURL)
		if fetchErr != nil {
			return 0, fmt.Errorf("fetch script: %w", fetchErr)
		}
		scriptText = strings.TrimSpace(body)
		project.ScriptText = body
		_ = s.projectRepo.Update(project)
	}

	var episodes []model.Episode
	startSequence := 0
	if episodeID != nil {
		ep, err := s.episodeRepo.FindByID(*episodeID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, errors.New("episode not found")
			}
			return 0, err
		}
		if ep.ProjectID != projectID {
			return 0, errors.New("episode not found")
		}
		episodes = []model.Episode{*ep}
		if err := s.storyboardSvc.repo.DeleteByEpisodeID(projectID, *episodeID); err != nil {
			return 0, fmt.Errorf("delete existing episode storyboards: %w", err)
		}
		startSequence, err = s.storyboardSvc.repo.MaxSequenceNumber(projectID)
		if err != nil {
			return 0, fmt.Errorf("query storyboard sequence: %w", err)
		}
	} else {
		episodes, err = s.episodeRepo.FindByProjectID(projectID)
		if err != nil {
			return 0, err
		}
		if err := s.storyboardSvc.DeleteByProjectID(projectID); err != nil {
			return 0, fmt.Errorf("delete existing project storyboards: %w", err)
		}
	}

	if len(episodes) == 0 {
		return 0, nil
	}
	var notReady []string
	for _, ep := range episodes {
		// Accept optimized text, raw script excerpt, or summary — any non-empty content is sufficient.
		hasContent := strings.TrimSpace(ep.OptimizedText) != "" ||
			strings.TrimSpace(ep.ScriptExcerpt) != "" ||
			strings.TrimSpace(ep.Summary) != ""
		if hasContent {
			continue
		}
		notReady = append(notReady, fmt.Sprintf("第%d集", ep.EpisodeNumber))
	}
	if len(notReady) > 0 {
		if len(notReady) > 5 {
			notReady = append(notReady[:5], "...")
		}
		return 0, fmt.Errorf("%w: %s 暂无可用剧本内容，请先完成剧本录入或优化", ErrScreenplayNotReady, strings.Join(notReady, "、"))
	}
	if s.logger != nil {
		episodeNumbers := make([]int, 0, len(episodes))
		for _, ep := range episodes {
			episodeNumbers = append(episodeNumbers, ep.EpisodeNumber)
		}
		s.logger.Info("manual storyboard extraction episodes resolved",
			zap.Uint64("project_id", projectID),
			zap.Int("episode_count", len(episodes)),
			zap.Ints("episode_numbers", episodeNumbers),
			zap.Int("start_sequence", startSequence),
		)
	}

	var progressSnapshot ProgressInfo
	if len(project.Progress) > 0 {
		_ = json.Unmarshal(project.Progress, &progressSnapshot)
	}
	s.updateProgress(projectID, ProgressInfo{
		Stage: "scene_splitting",
		EpisodeSplit: &StageProgress{
			Total: len(episodes), Completed: len(episodes), Status: "done",
		},
		SceneSplit: &StageProgress{
			Total: len(episodes), Completed: 0, Status: "running",
		},
		Message:        "正在准备分镜拆分…",
		PhaseLabel:     "自动拆分分镜中",
		NextStep:       "分镜拆分完成后即可继续出图或进入后续制作",
		CurrentEpisode: 0,
		TotalEpisodes:  len(episodes),
		AutoSplit:      progressSnapshot.AutoSplit,
	})
	_ = s.projectRepo.UpdateStatus(projectID, project.UserID, "storyboard_generating")

	clipDuration := 5
	videoModel := ""
	if len(project.StoryboardConfig) > 0 {
		var cfg struct {
			Duration   int    `json:"duration"`
			VideoModel string `json:"video_model"`
		}
		if err := json.Unmarshal(project.StoryboardConfig, &cfg); err == nil {
			if cfg.Duration > 0 {
				clipDuration = cfg.Duration
			}
			videoModel = strings.TrimSpace(cfg.VideoModel)
		}
	}

	// Rebuild / enrich keyword library when needed so manual extraction matches the
	// earlier pipeline quality instead of using a stale or incomplete glossary.
	var kwLib KeywordLibrary
	if len(project.KeywordLibrary) > 0 {
		_ = json.Unmarshal(project.KeywordLibrary, &kwLib)
	}
	if len(kwLib.Characters) == 0 && len(kwLib.Locations) == 0 && len(kwLib.Events) == 0 && len(kwLib.Props) == 0 && scriptText != "" {
		kwLib = s.extractKeywordLibrary(ctx, scriptText)
	}
	if (len(kwLib.CharacterProfiles) == 0 && len(kwLib.LocationProfiles) == 0 && len(kwLib.PropProfiles) == 0) && scriptText != "" {
		profileCtx, cancelProfile := context.WithTimeout(ctx, profileEnrichmentTimeout)
		scriptSample := scriptText
		const profileSampleLimit = 15000
		if utf8.RuneCountInString(scriptSample) > profileSampleLimit {
			scriptSample = string([]rune(scriptSample)[:profileSampleLimit])
		}
		s.enrichKeywordLibraryWithProfiles(profileCtx, &kwLib, scriptSample)
		cancelProfile()
	}
	if kwJSON, err := json.Marshal(kwLib); err == nil {
		project.KeywordLibrary = kwJSON
		_ = s.projectRepo.Update(project)
	}
	if s.logger != nil {
		s.logger.Info("manual storyboard extraction context prepared",
			zap.Uint64("project_id", projectID),
			zap.Int("keyword_characters", len(kwLib.Characters)),
			zap.Int("keyword_locations", len(kwLib.Locations)),
			zap.Int("keyword_events", len(kwLib.Events)),
			zap.Int("keyword_props", len(kwLib.Props)),
			zap.Int("character_profiles", len(kwLib.CharacterProfiles)),
			zap.Int("location_profiles", len(kwLib.LocationProfiles)),
			zap.Int("prop_profiles", len(kwLib.PropProfiles)),
			zap.Int("clip_duration", clipDuration),
			zap.String("video_model", videoModel),
		)
	}
	skipEpisodeAssetRefresh := episodeID != nil && shouldSkipEpisodeAssetRefresh(ctx)
	if s.characterBaseURL != "" {
		if episodeID != nil {
			if skipEpisodeAssetRefresh {
				if s.logger != nil {
					s.logger.Info("manual storyboard extraction skipped asset pre-refresh",
						zap.Uint64("project_id", projectID),
						zap.Uint64("episode_id", *episodeID),
					)
				}
			} else if err := s.extractAssetsForEpisode(WithSkipEpisodeStoryboardTrigger(ctx), projectID, *episodeID); err != nil && s.logger != nil {
				s.logger.Warn("manual storyboard extraction asset pre-refresh failed",
					zap.Uint64("project_id", projectID),
					zap.Uint64("episode_id", *episodeID),
					zap.Error(err),
				)
			}
		} else {
			s.extractAssetsAfterSplit(WithSkipEpisodeStoryboardTrigger(ctx), projectID, episodes)
		}
	}
	if episodeID != nil && s.characterBaseURL != "" {
		// Single-episode path either just dispatched extraction above or relies on a prior dispatch.
		if err := s.waitForEpisodeAssetExtraction(ctx, projectID, *episodeID, true); err != nil {
			if s.logger != nil {
				s.logger.Warn("episode asset extraction did not settle before storyboard split; continuing",
					zap.Uint64("project_id", projectID),
					zap.Uint64("episode_id", *episodeID),
					zap.Error(err),
				)
			}
		}
	}

	runtimeCfg := parseStoryboardRuntimeConfig(project)
	created := s.generateStoryboardsParallelWithOffset(ctx, projectID, project.UserID, episodes, &kwLib, clipDuration, videoModel, runtimeCfg.AspectRatio, runtimeCfg.Resolution, runtimeCfg.SpeechPace, project.ProjectType, startSequence)
	patchedContinuity := 0
	if created > 0 && s.storyboardSvc != nil && s.storyboardSvc.continuityAuditor != nil {
		auditCtx, cancelAudit := context.WithTimeout(ctx, 90*time.Second)
		result, auditErr := s.storyboardSvc.AuditSceneContinuity(auditCtx, projectID, episodeID)
		cancelAudit()
		if auditErr != nil {
			if s.logger != nil {
				s.logger.Warn("automatic storyboard continuity audit failed",
					zap.Uint64("project_id", projectID),
					zap.Bool("single_episode", episodeID != nil),
					zap.Error(auditErr),
				)
			}
		} else if result != nil {
			patchedContinuity = result.TotalPatched
			if s.logger != nil {
				s.logger.Info("automatic storyboard continuity audit completed",
					zap.Uint64("project_id", projectID),
					zap.Bool("single_episode", episodeID != nil),
					zap.Int("patched_storyboards", patchedContinuity),
					zap.Int("groups", result.TotalGroups),
				)
			}
		}
	}
	if episodeID != nil {
		if err := s.storyboardSvc.repo.ReindexSequenceNumbers(projectID); err != nil {
			return 0, fmt.Errorf("reindex storyboard sequence: %w", err)
		}
		if s.logger != nil {
			s.logger.Info("manual storyboard extraction sequence reindexed",
				zap.Uint64("project_id", projectID),
				zap.Uint64("episode_id", *episodeID),
			)
		}
	}
	s.updateProgress(projectID, ProgressInfo{
		Stage: "idle",
		EpisodeSplit: &StageProgress{
			Total: len(episodes), Completed: len(episodes), Status: "done",
		},
		SceneSplit: &StageProgress{
			Total: len(episodes), Completed: len(episodes), Status: "done",
		},
		Message: func() string {
			if patchedContinuity > 0 {
				return fmt.Sprintf("分镜拆分完成，共生成 %d 条分镜，并自动修正 %d 处连贯性问题", created, patchedContinuity)
			}
			return fmt.Sprintf("分镜拆分完成，共生成 %d 条分镜", created)
		}(),
		PhaseLabel: "分镜已就绪",
		NextStep: func() string {
			if patchedContinuity > 0 {
				return "分镜已完成自动连贯性校正，可以继续生成分镜图片、配音或视频"
			}
			return "可以继续生成分镜图片、配音或视频"
		}(),
		CurrentEpisode: len(episodes),
		TotalEpisodes:  len(episodes),
	})
	_ = s.projectRepo.UpdateStatus(projectID, project.UserID, "storyboard_ready")
	if s.logger != nil {
		s.logger.Info("manual storyboard extraction completed",
			zap.Uint64("project_id", projectID),
			zap.Bool("single_episode", episodeID != nil),
			zap.Int("created_storyboards", created),
		)
	}
	return created, nil
}

// Update —— 按字段映射局部更新剧集信息
// Update applies a partial map of fields to an episode.
func (s *EpisodeService) Update(id, projectID uint64, req map[string]interface{}) (*model.Episode, error) {
	episode, err := s.episodeRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("episode not found")
		}
		return nil, err
	}

	// Verify the episode belongs to the specified project.
	if episode.ProjectID != projectID {
		return nil, errors.New("episode not found")
	}

	if v, ok := req["episode_number"]; ok {
		if num, ok := toInt(v); ok {
			episode.EpisodeNumber = num
		}
	}
	if v, ok := req["title"]; ok {
		if s, ok := v.(string); ok {
			episode.Title = s
		}
	}
	if v, ok := req["summary"]; ok {
		if s, ok := v.(string); ok {
			episode.Summary = s
		}
	}
	if v, ok := req["script_excerpt"]; ok {
		if s, ok := v.(string); ok {
			episode.ScriptExcerpt = s
			episode.WordCount = utf8.RuneCountInString(s)
			if episode.WordCount > 0 {
				episode.EstimatedDuration = episode.WordCount / 5
			}
		}
	}
	if v, ok := req["status"]; ok {
		if s, ok := v.(string); ok {
			episode.Status = s
		}
	}
	episode.UpdatedAt = time.Now()

	if err := s.episodeRepo.Update(episode); err != nil {
		return nil, err
	}
	return episode, nil
}

// Delete —— 按 ID 和项目 ID 删除剧集，并级联清理 video-service 中关联的任务数据
func (s *EpisodeService) Delete(id, projectID uint64) error {
	if err := s.episodeRepo.Delete(id, projectID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("episode not found")
		}
		return err
	}
	// 级联清理 video-service 中该剧集的 VideoTask/DubbingTask
	if s.videoBaseURL != "" {
		url := fmt.Sprintf("%s/api/v1/projects/%d/episodes/%d/videos/runtime-data", s.videoBaseURL, projectID, id)
		req, err := http.NewRequest(http.MethodDelete, url, nil)
		if err == nil {
			resp, err := s.httpClient.Do(req)
			if err != nil {
				if s.logger != nil {
					s.logger.Warn("episode video cleanup failed", zap.Uint64("episode_id", id), zap.Error(err))
				}
			} else {
				resp.Body.Close()
				// 404 means no tasks existed — treat as success
			}
		}
	}
	return nil
}

// PolishEpisode calls the LLM (guided by active writing skills) to rewrite
// the episode's title, summary and script_excerpt in-place.
func (s *EpisodeService) PolishEpisode(ctx context.Context, id, projectID uint64) (*model.Episode, error) {
	episode, err := s.episodeRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("episode not found")
		}
		return nil, err
	}
	if episode.ProjectID != projectID {
		return nil, errors.New("episode not found")
	}

	// Fetch writing-skill hints from character-service.
	writingHints := s.fetchWritingSkillHints(ctx, projectID)
	productionHints := s.fetchProductionSkillHints(ctx, projectID)

	// Load project keyword library for consistency bible injection.
	var projectRef *model.Project
	var kwLib *KeywordLibrary
	if project, pErr := s.projectRepo.FindByIDNoAuth(projectID); pErr == nil {
		projectRef = project
		var lib KeywordLibrary
		if len(project.KeywordLibrary) > 0 {
			if jsonErr := json.Unmarshal(project.KeywordLibrary, &lib); jsonErr == nil {
				kwLib = &lib
			}
		}
	}

	polished, err := s.callLLMPolish(ctx, projectRef, episode, writingHints, productionHints, kwLib)
	if err != nil {
		return nil, fmt.Errorf("LLM polish failed: %w", err)
	}

	// Apply polished fields.
	if polished.Title != "" {
		episode.Title = polished.Title
	}
	if polished.Summary != "" {
		episode.Summary = polished.Summary
	}
	if polished.ScriptExcerpt != "" {
		episode.ScriptExcerpt = polished.ScriptExcerpt
		episode.WordCount = utf8.RuneCountInString(polished.ScriptExcerpt)
		if episode.WordCount > 0 {
			episode.EstimatedDuration = episode.WordCount / 5
		}
	}

	if err := s.episodeRepo.Update(episode); err != nil {
		return nil, fmt.Errorf("save polished episode: %w", err)
	}
	return episode, nil
}

// fetchSkillHintsByUseCase calls character-service to get active skills for a project by use_case.
// Shared implementation for writing, storyboard and storyboard_prep hint fetchers.
func (s *EpisodeService) fetchSkillHintsByUseCase(ctx context.Context, projectID uint64, useCase string) string {
	if s.characterBaseURL == "" || s.jwtSecret == "" {
		return ""
	}
	token, err := s.buildServiceToken(projectID)
	if err != nil {
		return ""
	}
	url := fmt.Sprintf("%s/api/v1/skills?project_id=%d&use_case=%s&is_active=true&page_size=50", s.characterBaseURL, projectID, useCase)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var envelope struct {
		Data struct {
			Items []struct {
				PromptText string `json:"prompt_text"`
				Label      string `json:"label"`
				Name       string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return ""
	}
	parts := make([]string, 0, len(envelope.Data.Items))
	for _, item := range envelope.Data.Items {
		content := strings.TrimSpace(item.PromptText)
		if content == "" {
			content = strings.TrimSpace(item.Label)
		}
		if content == "" {
			content = strings.TrimSpace(item.Name)
		}
		if content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n")
}

func (s *EpisodeService) fetchWritingSkillHints(ctx context.Context, projectID uint64) string {
	return s.fetchSkillHintsByUseCase(ctx, projectID, "writing")
}

func (s *EpisodeService) fetchStoryboardSkillHints(ctx context.Context, projectID uint64) string {
	return s.fetchSkillHintsByUseCase(ctx, projectID, "storyboard")
}

func (s *EpisodeService) fetchScriptPrepSkillHints(ctx context.Context, projectID uint64) string {
	return s.fetchSkillHintsByUseCase(ctx, projectID, "storyboard_prep")
}

func (s *EpisodeService) prepareScriptForStoryboard(ctx context.Context, project *model.Project, content string, episodeNum int, kwLib *KeywordLibrary, profile productionmode.Profile, prepSkillHints string) string {
	llmCfg := s.resolveProjectLLMConfig(ctx, project)
	if strings.TrimSpace(content) == "" {
		return content
	}
	if profile.ShouldSkipScriptPrep() {
		return content
	}

	const maxRunes = 40000
	runes := []rune(content)
	if len(runes) > maxRunes {
		content = string(runes[:maxRunes])
	}

	systemPrompt := productionmode.ScriptPrepSystemPrompt(profile.Mode)
	if systemPrompt == "" {
		return content
	}

	if prepSkillHints != "" {
		systemPrompt += "\n\n本项目专属分镜预处理指引：\n" + prepSkillHints
	}
	runtimeCfg := parseStoryboardRuntimeConfig(project)
	systemPrompt += "\n\n" + productionmode.ScriptPrepRuntimeContext(
		runtimeCfg.StylePreset,
		runtimeCfg.MotionMode,
		runtimeCfg.Duration,
		runtimeCfg.SpeechPace,
	)
	if bible := buildConsistencyBibleBlock(kwLib); bible != "" {
		systemPrompt += "\n\n" + bible + "\n所有标注中的人物姓名和场景描述必须与以上一致性词库保持一致。"
	}
	systemPrompt += "\n\n" + dialoguePreservationPromptBlock(content)

	reqBody := map[string]interface{}{
		"model": llmCfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": productionmode.ScriptPrepUserContent(profile.Mode, episodeNum, content)},
		},
		"temperature": 0.4,
		"max_tokens":  8192,
	}
	data, _ := json.Marshal(reqBody)

	prepCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(prepCtx, http.MethodPost, llmCfg.BaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return content
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+llmCfg.APIKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("script prep LLM request failed, using original", zap.Int("episode", episodeNum), zap.Error(err))
		}
		return content
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if s.logger != nil {
			s.logger.Warn("script prep LLM non-200", zap.Int("episode", episodeNum), zap.Int("status", resp.StatusCode))
		}
		return content
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return content
	}
	optimized := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	if len(optimized) < 50 {
		return content
	}
	return s.preserveDialogueFromSource(content, optimized)
}

func (s *EpisodeService) fetchProductionSkillHints(ctx context.Context, projectID uint64) string {
	return s.fetchSkillHintsByUseCase(ctx, projectID, "production")
}

func (s *EpisodeService) fetchStoryboardPromptTemplate(ctx context.Context, styleKey string) string {
	if s.scriptBaseURL == "" || strings.TrimSpace(styleKey) == "" {
		return ""
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, key := range storyboardTemplateLookupKeys(styleKey) {
		url := fmt.Sprintf("%s/api/v1/prompt-templates?style_key=%s&resource_type=storyboard&active_only=true", s.scriptBaseURL, key)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("fetch storyboard prompt template failed", zap.String("style_key", key), zap.Error(err))
			}
			continue
		}
		var result struct {
			Data []struct {
				Content  string `json:"content"`
				IsActive bool   `json:"is_active"`
			} `json:"data"`
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}
		for _, tpl := range result.Data {
			if tpl.IsActive && strings.TrimSpace(tpl.Content) != "" {
				return tpl.Content
			}
		}
	}
	return ""
}

func storyboardTemplateLookupKeys(styleKey string) []string {
	trimmed := strings.TrimSpace(styleKey)
	if trimmed == "" {
		return nil
	}
	keys := []string{trimmed}
	if trimmed == "storyboard_anime2d" {
		keys = append(keys, "animation_v43")
	}
	return keys
}

func applyPromptTemplate(template, scene, characters, action, mood string) string {
	result := template
	result = strings.ReplaceAll(result, "{scene}", strings.TrimSpace(scene))
	result = strings.ReplaceAll(result, "{characters}", strings.TrimSpace(characters))
	result = strings.ReplaceAll(result, "{action}", strings.TrimSpace(action))
	result = strings.ReplaceAll(result, "{mood}", strings.TrimSpace(mood))
	return strings.TrimSpace(result)
}

func storyboardStyleKey(stylePreset string) string {
	switch stylepreset.Canonical(stylePreset) {
	case stylepreset.Anime2D:
		return "storyboard_anime2d"
	case stylepreset.Anime3D:
		return "storyboard_anime3d"
	case stylepreset.LiveActionFilm, stylepreset.LiveActionShort:
		return "storyboard_cinematic"
	default:
		return "storyboard_cinematic"
	}
}

type polishedEpisode struct {
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	ScriptExcerpt string `json:"script_excerpt"`
}

// callLLMPolish sends the episode to the LLM for professional rewriting.
func (s *EpisodeService) callLLMPolish(ctx context.Context, project *model.Project, ep *model.Episode, writingHints string, productionHints string, kwLib *KeywordLibrary) (*polishedEpisode, error) {
	mode := productionmode.ModeScriptDrama
	if project != nil {
		mode = productionmode.Resolve(project)
	}
	systemPrompt := productionmode.EpisodePolishSystemPrompt(mode)
	if writingHints != "" {
		systemPrompt += "\n\n**本项目专属优化指引（请务必遵守）：**\n" + writingHints
	}
	// Inject production department annotations if any are active.
	if productionHints != "" {
		systemPrompt += "\n\n**影视部门标注要求（请在 script_excerpt 中内联标注）：**\n"
		systemPrompt += "在 script_excerpt 的相应位置嵌入以下格式的内联标注：[标签:内容]，例如 [字幕:你好吗][摄影:特写镜头][音效:雨声]。\n"
		systemPrompt += "标注紧跟在相关文字之后，不要单独成行，不要影响正文流畅度。\n\n"
		systemPrompt += productionHints
	}
	// Inject consistency bible so polish preserves character appearances and locations consistently.
	if bible := buildConsistencyBibleBlock(kwLib); bible != "" {
		systemPrompt += bible
	}
	systemPrompt += "\n\n" + dialoguePreservationPromptBlock(ep.ScriptExcerpt)

	userContent := fmt.Sprintf("第%d集《%s》\n\n【当前简介】\n%s\n\n【当前内容】\n%s",
		ep.EpisodeNumber,
		ep.Title,
		ep.Summary,
		ep.ScriptExcerpt,
	)

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": "请对以下分集进行专业优化：\n\n" + userContent},
		},
		"temperature":     0.7,
		"max_tokens":      8192,
		"response_format": map[string]string{"type": "json_object"},
	}
	data, _ := json.Marshal(reqBody)

	polishCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(polishCtx, http.MethodPost, s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("LLM request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM responded %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}
	content := strings.TrimSpace(llmResp.Choices[0].Message.Content)

	var result polishedEpisode
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse polished JSON: %w", err)
	}
	if result.ScriptExcerpt != "" {
		result.ScriptExcerpt = s.preserveDialogueFromSource(ep.ScriptExcerpt, result.ScriptExcerpt)
	}
	return &result, nil
}

// toInt —— 将 JSON 数字类型转换为 int
// toInt converts json number types to int.
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	case int64:
		return int(n), true
	}
	return 0, false
}

// GenerateFromScript —— 编排完整的剧集生成流水线：关键词提取、分集、分镜拆分
// GenerateFromScript orchestrates the full pipeline: keyword extraction → episode splitting → scene splitting.
// Each phase reports progress to project.progress so the frontend can track status.
func (s *EpisodeService) GenerateFromScript(ctx context.Context, projectID uint64, userKeywords *KeywordLibrary) ([]model.Episode, error) {
	return s.GenerateFromScriptWithOptions(ctx, projectID, userKeywords, false, false)
}

func (s *EpisodeService) GenerateFromScriptWithOptions(ctx context.Context, projectID uint64, userKeywords *KeywordLibrary, force bool, autoStoryboard bool) ([]model.Episode, error) {
	project, err := s.projectRepo.FindByIDNoAuth(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}
	var previousProgress ProgressInfo
	if len(project.Progress) > 0 {
		_ = json.Unmarshal(project.Progress, &previousProgress)
	}

	// ── Layer 2: Status guard ──
	// If already processing, refuse to start another generation.
	if project.Status == "script_processing" && !force {
		return nil, fmt.Errorf("project is already processing, cannot start again")
	}

	// Save user-provided keywords to project if given
	if userKeywords != nil {
		if kwJSON, err := json.Marshal(userKeywords); err == nil {
			project.KeywordLibrary = kwJSON
			_ = s.projectRepo.UpdateKeywordLibrary(projectID, kwJSON)
		}
	}

	// Immediately mark as script_processing + initialize progress
	_ = s.projectRepo.UpdateStatus(projectID, project.UserID, "script_processing")
	progressMessage := "准备中…"
	phaseLabel := ""
	nextStep := ""
	if force && (previousProgress.Stage == "episode_splitting" || project.Status == "script_processing") {
		progressMessage = "服务恢复后已重新开始分集生成…"
		phaseLabel = "已自动恢复分集生成"
		nextStep = "系统会重新分析剧本并恢复分集结构，完成后自动进入后续剧本准备流程"
	} else if force {
		progressMessage = "已按当前最新文本重新开始分集拆分…"
		phaseLabel = "按最新文本重建中"
		nextStep = "系统会清空旧分集结果，并基于当前原文 / 优化稿重新生成新的分集结构"
	}
	s.updateProgress(projectID, ProgressInfo{
		Stage:        "episode_splitting",
		Message:      progressMessage,
		PhaseLabel:   phaseLabel,
		NextStep:     nextStep,
		EpisodeSplit: &StageProgress{Status: "running"},
	})

	episodes, err := s.doGenerateFromScript(ctx, project, autoStoryboard, force)
	if err != nil {
		s.updateProgress(projectID, ProgressInfo{
			Stage:        "idle",
			Message:      fmt.Sprintf("失败: %s", err.Error()),
			EpisodeSplit: &StageProgress{Status: "failed"},
		})
		_ = s.projectRepo.UpdateStatus(projectID, project.UserID, "failed")
		if s.logger != nil {
			s.logger.Error("episode generation failed",
				zap.Uint64("project_id", projectID),
				zap.Error(err),
			)
		}
		return nil, err
	}
	return episodes, nil
}

func (s *EpisodeService) IsGenerationStalled(projectID uint64, threshold time.Duration) (bool, error) {
	project, err := s.projectRepo.FindByIDNoAuth(projectID)
	if err != nil {
		return false, err
	}

	var progress ProgressInfo
	if len(project.Progress) > 0 {
		if err := json.Unmarshal(project.Progress, &progress); err != nil {
			return false, err
		}
	}

	if project.Status != "script_processing" && progress.Stage != "episode_splitting" && progress.Stage != "scene_splitting" && progress.Stage != "script_prepping" {
		return false, nil
	}
	if progress.UpdatedAt == "" {
		return false, nil
	}

	updatedAt, err := time.Parse(time.RFC3339, progress.UpdatedAt)
	if err != nil {
		return false, err
	}

	return time.Since(updatedAt) >= threshold, nil
}

func (s *EpisodeService) HasActiveGeneration(projectID uint64) (bool, error) {
	project, err := s.projectRepo.FindByIDNoAuth(projectID)
	if err != nil {
		return false, err
	}

	var progress ProgressInfo
	if len(project.Progress) > 0 {
		if err := json.Unmarshal(project.Progress, &progress); err != nil {
			return false, err
		}
	}

	return project.Status == "script_processing" ||
		progress.Stage == "episode_splitting" ||
		progress.Stage == "scene_splitting" ||
		progress.Stage == "script_prepping", nil
}

func (s *EpisodeService) episodeAutoPrepared(ep model.Episode) bool {
	optimizedText := strings.TrimSpace(ep.OptimizedText)
	if optimizedText == "" {
		return false
	}
	if ep.OptimizeStatus != "done" || ep.ReviewStatus != "done" {
		return false
	}
	return strings.TrimSpace(ep.ScriptExcerpt) == optimizedText
}

func (s *EpisodeService) countPreparedEpisodes(eps []model.Episode) int {
	count := 0
	for _, ep := range eps {
		if s.episodeAutoPrepared(ep) {
			count++
		}
	}
	return count
}

func firstEpisodeByNumber(eps []model.Episode) model.Episode {
	if len(eps) == 0 {
		return model.Episode{}
	}
	first := eps[0]
	for _, ep := range eps[1:] {
		if ep.EpisodeNumber > 0 && (first.EpisodeNumber <= 0 || ep.EpisodeNumber < first.EpisodeNumber) {
			first = ep
		}
	}
	return first
}

func (s *EpisodeService) finishAutoPreparation(projectID, userID uint64, totalEpisodes, processedEpisodes int, timedOut, resumed, textOnly bool) {
	finalStatus := "script_ready"
	sceneStatus := "pending"
	if processedEpisodes > 0 && !textOnly {
		sceneStatus = "done"
	}
	message := "第 1 集自动处理已完成"
	phaseLabel := "示范集自动处理完成"
	nextStep := "请在左侧单集列表为其他分集点击「自动处理」"
	completed := processedEpisodes
	if textOnly && processedEpisodes > 0 && !timedOut {
		if totalEpisodes <= 1 {
			message = "分集与第 1 集剧本润色优化已完成"
			phaseLabel = "剧本文本处理完成"
			nextStep = "可在单集列表点击「自动处理」衔接资源提取与分镜拆分"
		} else {
			message = fmt.Sprintf("分集完成，第 1 集剧本润色优化已完成（项目共 %d 集）", totalEpisodes)
			phaseLabel = "示范集文本处理完成"
			nextStep = "请为各分集点击「自动处理」以衔接资源提取与分镜拆分"
		}
	} else if totalEpisodes <= 1 {
		message = "自动剧本处理已完成"
		phaseLabel = "剧本自动处理完成"
		nextStep = "可以继续资源提取、分镜拆分或进入后续制作"
		if !textOnly {
			sceneStatus = "done"
		}
		completed = totalEpisodes
	} else if timedOut {
		finalStatus = "failed"
		sceneStatus = "failed"
		message = fmt.Sprintf("自动处理提前结束（%d/%d 集），可手动继续资源与分镜", processedEpisodes, totalEpisodes)
		phaseLabel = "自动处理已中断"
		nextStep = "建议在单集工作区为未完成的分集点击「自动处理」"
	} else if !textOnly && s.autoAssetPipelineReady() {
		finalStatus = "asset_generating"
		message = fmt.Sprintf("第 1 集已自动处理（项目共 %d 集），资源与分镜正在后台继续", totalEpisodes)
		phaseLabel = "示范集资源与分镜处理中"
		nextStep = "其余分集请在左侧单集列表点击「自动处理」"
	}
	if resumed {
		switch {
		case timedOut:
			message = fmt.Sprintf("服务重启后已尝试恢复，但自动处理仍提前结束（%d/%d 集）", processedEpisodes, totalEpisodes)
			phaseLabel = "恢复后处理已中断"
		case finalStatus == "asset_generating":
			message = fmt.Sprintf("服务重启后已自动恢复，第 1 集资源与分镜继续处理中（项目共 %d 集）", totalEpisodes)
			phaseLabel = "已自动恢复示范集流程"
		case finalStatus == "script_ready":
			if totalEpisodes > 1 {
				message = fmt.Sprintf("服务重启后已自动恢复，第 1 集处理完成（项目共 %d 集）", totalEpisodes)
			} else {
				message = fmt.Sprintf("服务重启后已自动恢复并完成剧本处理（%d/%d 集）", totalEpisodes, totalEpisodes)
			}
			phaseLabel = "已自动恢复剧本处理"
		}
	}
	s.updateProgress(projectID, ProgressInfo{
		Stage: "idle",
		EpisodeSplit: &StageProgress{
			Total: totalEpisodes, Completed: totalEpisodes, Status: "done",
		},
		SceneSplit: &StageProgress{
			Total: totalEpisodes, Completed: completed, Status: sceneStatus,
		},
		Message:        message,
		PhaseLabel:     phaseLabel,
		NextStep:       nextStep,
		CurrentEpisode: completed,
		TotalEpisodes:  totalEpisodes,
	})
	_ = s.projectRepo.UpdateStatus(projectID, userID, finalStatus)
}

func (s *EpisodeService) runEpisodeAutoPipeline(ctx context.Context, project model.Project, ep model.Episode, totalEpisodes int, resumed, isFirstAutoEpisode, triggerAssets bool) bool {
	if s.episodeAutoPrepared(ep) {
		if !triggerAssets {
			return true
		}
		// Text already prepared; continue with asset extraction and let defer trigger storyboards.
		if _, applyErr := s.ApplyOptimizedText(WithSkipEpisodeStoryboardTrigger(ctx), ep.ID, project.ID); applyErr != nil && s.logger != nil {
			s.logger.Warn("auto apply prepared episode text failed",
				zap.Uint64("episode_id", ep.ID),
				zap.Error(applyErr),
			)
			return false
		}
		return true
	}

	autoWritingHints := s.fetchWritingSkillHints(ctx, project.ID)
	autoProductionHints := s.fetchProductionSkillHints(ctx, project.ID)
	var autoKwLib *KeywordLibrary
	if proj, pErr := s.projectRepo.FindByIDNoAuth(project.ID); pErr == nil {
		var lib KeywordLibrary
		if len(proj.KeywordLibrary) > 0 {
			if jsonErr := json.Unmarshal(proj.KeywordLibrary, &lib); jsonErr == nil {
				autoKwLib = &lib
			}
		}
	}

	message := fmt.Sprintf("正在润色并优化第 %d 集剧本…", ep.EpisodeNumber)
	phaseLabel := "单集剧本处理中"
	nextStep := "文本处理完成后可点击「自动处理」衔接资源与分镜"
	if triggerAssets {
		message = fmt.Sprintf("正在润色并准备第 %d 集，随后自动提取资源并拆分分镜…", ep.EpisodeNumber)
		phaseLabel = "单集自动处理中"
		nextStep = "当前集处理完成后可进入工作台继续出图或成片"
	}
	if resumed {
		message = fmt.Sprintf("服务重启后已自动恢复，正在准备第 %d 集…", ep.EpisodeNumber)
		phaseLabel = "已自动恢复单集处理"
	}
	if totalEpisodes > 1 && isFirstAutoEpisode {
		if triggerAssets {
			message = fmt.Sprintf("正在自动处理第 %d 集（示范集），其余 %d 集请在单集列表手动启动", ep.EpisodeNumber, max(totalEpisodes-1, 0))
			nextStep = "示范集完成后，请为其他分集点击「自动处理」"
		} else {
			message = fmt.Sprintf("正在润色优化第 %d 集示范剧本（项目共 %d 集）…", ep.EpisodeNumber, totalEpisodes)
			nextStep = "示范集文本完成后，请为各分集点击「自动处理」衔接资源与分镜"
		}
	}
	sceneSplitStatus := "pending"
	if triggerAssets {
		sceneSplitStatus = "running"
	}
	s.updateProgress(project.ID, ProgressInfo{
		Stage: "script_prepping",
		EpisodeSplit: &StageProgress{
			Total: totalEpisodes, Completed: totalEpisodes, Status: "done",
		},
		SceneSplit: &StageProgress{
			Total: totalEpisodes, Completed: 0, Status: sceneSplitStatus,
		},
		Message:        message,
		PhaseLabel:     phaseLabel,
		NextStep:       nextStep,
		CurrentEpisode: ep.EpisodeNumber,
		TotalEpisodes:  totalEpisodes,
	})

	if _, err := s.polishEpisodeInternal(ctx, ep.ID, project.ID, autoWritingHints, autoProductionHints, autoKwLib); err != nil && s.logger != nil {
		s.logger.Warn("auto-polish episode failed", zap.Uint64("episode_id", ep.ID), zap.Error(err))
	}
	updated, err := s.autoOptimizeReviewInternal(ctx, ep.ID, project.ID, autoWritingHints, autoKwLib)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("auto optimize-review episode failed", zap.Uint64("episode_id", ep.ID), zap.Error(err))
		}
		if triggerAssets && s.characterBaseURL != "" {
			if extractErr := s.extractAssetsForEpisode(WithSkipEpisodeStoryboardTrigger(ctx), project.ID, ep.ID); extractErr != nil && s.logger != nil {
				s.logger.Warn("fallback asset extraction after optimize-review failure", zap.Uint64("episode_id", ep.ID), zap.Error(extractErr))
			}
		}
		return false
	}

	if updated.OptimizedText == "" {
		return false
	}
	applyCtx := WithSkipEpisodeStoryboardTrigger(ctx)
	if !triggerAssets {
		applyCtx = WithSkipEpisodeAssetExtraction(applyCtx)
	}
	if _, applyErr := s.ApplyOptimizedText(applyCtx, ep.ID, project.ID); applyErr != nil {
		if s.logger != nil {
			s.logger.Warn("auto apply optimized text failed", zap.Uint64("episode_id", ep.ID), zap.Error(applyErr))
		}
		return false
	}
	return true
}

type episodeAutoPipelineJobOptions struct {
	TriggerStoryboards bool
	TriggerAssets      bool
	TextOnly           bool
	Resumed            bool
	IsFirstAutoEpisode bool
	OnComplete         func()
	Timeout            time.Duration
}

func (s *EpisodeService) launchEpisodeAutoPipelineJob(project model.Project, episode model.Episode, totalEpisodes int, opts episodeAutoPipelineJobOptions) {
	go func(project model.Project, episode model.Episode, totalEpisodes int, opts episodeAutoPipelineJobOptions) {
		timeout := opts.Timeout
		if timeout <= 0 {
			timeout = 45 * time.Minute
		}
		autoCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if opts.OnComplete != nil {
			defer opts.OnComplete()
		}

		processedEpisodes := 0
		textAlreadyPrepared := s.episodeAutoPrepared(episode)
		if textAlreadyPrepared && !opts.TriggerAssets {
			processedEpisodes = 1
		}
		timedOut := false
		storyboardsTriggered := false
		defer func() {
			if opts.TriggerStoryboards && !timedOut && processedEpisodes >= 1 && s.storyboardSvc != nil {
				epID := episode.ID
				if opts.TriggerAssets {
					if err := s.waitForEpisodeAssetExtraction(autoCtx, project.ID, epID, true); err != nil && s.logger != nil {
						s.logger.Warn("auto pipeline asset extraction did not settle before generation",
							zap.Uint64("project_id", project.ID),
							zap.Uint64("episode_id", epID),
							zap.Error(err),
						)
					}
					if err := s.triggerEpisodeAssetGeneration(autoCtx, project.ID, epID); err != nil && s.logger != nil {
						s.logger.Warn("auto pipeline episode asset generation failed",
							zap.Uint64("project_id", project.ID),
							zap.Uint64("episode_id", epID),
							zap.Error(err),
						)
					}
				}
				if s.logger != nil {
					s.logger.Info("auto pipeline finished; triggering storyboard extraction",
						zap.Uint64("project_id", project.ID),
						zap.Uint64("episode_id", epID),
						zap.Int("episode_number", episode.EpisodeNumber),
						zap.Int("episode_count", totalEpisodes),
						zap.Bool("resumed", opts.Resumed),
						zap.Bool("first_auto_episode", opts.IsFirstAutoEpisode),
					)
				}
				storyboardCtx := WithSkipEpisodeAssetRefresh(autoCtx)
				if _, err := s.ExtractStoryboards(storyboardCtx, project.ID, &epID); err != nil {
					if s.logger != nil {
						s.logger.Warn("auto pipeline storyboard extraction failed",
							zap.Uint64("project_id", project.ID),
							zap.Uint64("episode_id", epID),
							zap.Bool("resumed", opts.Resumed),
							zap.Error(err),
						)
					}
				} else {
					storyboardsTriggered = true
				}
			}
			if !storyboardsTriggered {
				s.finishAutoPreparation(project.ID, project.UserID, totalEpisodes, processedEpisodes, timedOut, opts.Resumed, opts.TextOnly)
			}
		}()

		if processedEpisodes >= 1 && !opts.TriggerAssets {
			return
		}

		select {
		case <-autoCtx.Done():
			timedOut = true
			if s.logger != nil {
				s.logger.Warn("episode auto pipeline stopped before completion",
					zap.Uint64("project_id", project.ID),
					zap.Uint64("episode_id", episode.ID),
					zap.Int("episode_number", episode.EpisodeNumber),
					zap.Int("total_episodes", totalEpisodes),
					zap.Bool("resumed", opts.Resumed),
					zap.Error(autoCtx.Err()),
				)
			}
			return
		default:
		}

		if s.runEpisodeAutoPipeline(autoCtx, project, episode, totalEpisodes, opts.Resumed, opts.IsFirstAutoEpisode, opts.TriggerAssets) {
			processedEpisodes = 1
		}
	}(project, episode, totalEpisodes, opts)
}

// StartEpisodeAutoPipeline runs polish → optimize → assets → storyboards for one episode.
func (s *EpisodeService) StartEpisodeAutoPipeline(projectID, episodeID uint64, onComplete func()) error {
	project, err := s.projectRepo.FindByIDNoAuth(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}
	ep, err := s.episodeRepo.FindByID(episodeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("episode not found")
		}
		return err
	}
	if ep.ProjectID != projectID {
		return errors.New("episode not found")
	}

	allEpisodes, err := s.episodeRepo.FindByProjectID(projectID)
	if err != nil {
		return err
	}

	s.launchEpisodeAutoPipelineJob(*project, *ep, len(allEpisodes), episodeAutoPipelineJobOptions{
		TriggerStoryboards: true,
		TriggerAssets:      true,
		OnComplete:         onComplete,
		Timeout:            45 * time.Minute,
	})
	return nil
}

type autoPrepMode int

const (
	autoPrepOff autoPrepMode = iota
	autoPrepTextOnly
	autoPrepFullFirstEpisode
)

func (s *EpisodeService) startAutoPreparationPipeline(project *model.Project, eps []model.Episode, resumed bool, mode autoPrepMode) {
	if len(eps) == 0 || mode == autoPrepOff {
		return
	}
	firstEp := firstEpisodeByNumber(eps)
	opts := episodeAutoPipelineJobOptions{
		Resumed:            resumed,
		IsFirstAutoEpisode: true,
		Timeout:            90 * time.Minute,
	}
	if mode == autoPrepFullFirstEpisode {
		opts.TriggerStoryboards = true
		opts.TriggerAssets = true
		opts.TextOnly = false
	} else {
		opts.TriggerStoryboards = false
		opts.TriggerAssets = false
		opts.TextOnly = true
	}
	s.launchEpisodeAutoPipelineJob(*project, firstEp, len(eps), opts)
}

// ResumeInterruptedAutoPreparation restarts projects that were left in
// script_prepping after an unclean restart.
func (s *EpisodeService) ResumeInterruptedAutoPreparation(limit int) (int, error) {
	projects, err := s.projectRepo.FindAutoPreparationCandidates(limit)
	if err != nil {
		return 0, err
	}
	resumed := 0
	for i := range projects {
		episodes, epErr := s.episodeRepo.FindByProjectID(projects[i].ID)
		if epErr != nil {
			if s.logger != nil {
				s.logger.Warn("resume auto preparation skipped: list episodes failed",
					zap.Uint64("project_id", projects[i].ID),
					zap.Error(epErr),
				)
			}
			continue
		}
		if len(episodes) == 0 {
			continue
		}
		if s.logger != nil {
			s.logger.Info("resuming interrupted auto preparation",
				zap.Uint64("project_id", projects[i].ID),
				zap.Int("episode_count", len(episodes)),
				zap.Int("already_prepared", s.countPreparedEpisodes(episodes)),
			)
		}
		s.startAutoPreparationPipeline(&projects[i], episodes, true, autoPrepTextOnly)
		resumed++
	}
	return resumed, nil
}

// ResumeInterruptedEpisodeGeneration restarts projects that were still in the
// episode split phase when the service was restarted.
func (s *EpisodeService) ResumeInterruptedEpisodeGeneration(limit int) (int, error) {
	projects, err := s.projectRepo.FindEpisodeGenerationCandidates(limit)
	if err != nil {
		return 0, err
	}
	resumed := 0
	for _, project := range projects {
		resumed++
		if s.logger != nil {
			s.logger.Info("resuming interrupted episode generation",
				zap.Uint64("project_id", project.ID),
			)
		}
		go func(projectID uint64) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
			defer cancel()
			if _, resumeErr := s.GenerateFromScriptWithOptions(ctx, projectID, nil, true, false); resumeErr != nil && s.logger != nil {
				s.logger.Warn("resume interrupted episode generation failed",
					zap.Uint64("project_id", projectID),
					zap.Error(resumeErr),
				)
			}
		}(project.ID)
	}
	return resumed, nil
}

// doGenerateFromScript —— 执行剧集生成的核心逻辑，包含数据清理、分集和分镜创建
func (s *EpisodeService) doGenerateFromScript(ctx context.Context, project *model.Project, autoStoryboard bool, isResplit bool) ([]model.Episode, error) {
	projectID := uint64(project.ID)

	// Get script content
	scriptText := project.ScriptText
	if scriptText == "" && project.ScriptFileURL != "" {
		body, err := s.fetchScriptContent(ctx, project.ScriptFileURL)
		if err != nil {
			return nil, fmt.Errorf("fetch script: %w", err)
		}
		scriptText = body
		project.ScriptText = scriptText
		_ = s.projectRepo.UpdateScriptText(projectID, scriptText)
	}
	if strings.TrimSpace(scriptText) == "" {
		return nil, errors.New("project has no script content, please upload a script first")
	}

	// ══════════════════════════════════════════════════════════════════════════
	// Phase 0: Cleanup old data
	// ══════════════════════════════════════════════════════════════════════════
	s.updateProgress(projectID, ProgressInfo{
		Stage:        "episode_splitting",
		Message:      "清除旧数据…",
		EpisodeSplit: &StageProgress{Status: "running"},
	})
	// Delete old storyboards first (depends on episodes).
	// Must succeed before replacing episodes — if storyboards survive with old
	// episode_id references and episodes are then replaced (new auto-increment IDs),
	// the storyboard rows become orphaned FK references causing stuck progress.
	if s.storyboardSvc != nil {
		if err := s.storyboardSvc.DeleteByProjectID(projectID); err != nil {
			return nil, fmt.Errorf("delete old storyboards before scene split: %w", err)
		}
	}
	// Clear stale project-wide assets/sentinels so a re-split does not resume
	// interrupted full-project extraction against the new episode set.
	if err := s.deleteExistingAssets(ctx, projectID); err != nil {
		if s.logger != nil {
			s.logger.Warn("delete old assets before episode re-split failed; continuing",
				zap.Uint64("project_id", projectID),
				zap.Error(err),
			)
		}
	}
	// Episodes will be atomically replaced later via ReplaceAllForProject

	// ══════════════════════════════════════════════════════════════════════════
	// Phase 1: Keyword extraction
	// ══════════════════════════════════════════════════════════════════════════
	s.updateProgress(projectID, ProgressInfo{
		Stage:        "episode_splitting",
		Message:      "正在提取关键词库…",
		EpisodeSplit: &StageProgress{Status: "running"},
	})

	keywordCtx, cancelKeyword := context.WithTimeout(ctx, keywordExtractionTimeout)
	kwLib := s.extractKeywordLibrary(keywordCtx, scriptText)
	cancelKeyword()

	// Merge with user-provided keywords (user keywords take priority, prepended)
	var existingKW KeywordLibrary
	if len(project.KeywordLibrary) > 0 {
		_ = json.Unmarshal(project.KeywordLibrary, &existingKW)
	}
	kwLib = mergeKeywordLibraries(existingKW, kwLib)

	if s.logger != nil {
		s.logger.Info("keyword library merged",
			zap.Uint64("project_id", projectID),
			zap.Int("characters", len(kwLib.Characters)),
			zap.Int("locations", len(kwLib.Locations)),
			zap.Int("events", len(kwLib.Events)),
		)
	}
	if kwJSON, err := json.Marshal(kwLib); err == nil {
		project.KeywordLibrary = kwJSON
		_ = s.projectRepo.UpdateKeywordLibrary(projectID, kwJSON)
	}

	// ══════════════════════════════════════════════════════════════════════════
	// Phase 1b: Enrich keyword library with visual profiles for consistency
	// Generates character appearance, location, and prop descriptions used to
	// keep visuals consistent across all scenes, storyboards, and dubbing.
	// ══════════════════════════════════════════════════════════════════════════
	s.updateProgress(projectID, ProgressInfo{
		Stage:        "episode_splitting",
		Message:      "正在生成视觉一致性档案（人物/场景/道具描述）…",
		EpisodeSplit: &StageProgress{Status: "running"},
	})
	profileCtx, cancelProfile := context.WithTimeout(ctx, profileEnrichmentTimeout)
	// Use a representative script sample for the enrichment prompt
	scriptSample := scriptText
	const profileSampleLimit = 15000
	if utf8.RuneCountInString(scriptSample) > profileSampleLimit {
		scriptSample = string([]rune(scriptSample)[:profileSampleLimit])
	}
	s.enrichKeywordLibraryWithProfiles(profileCtx, &kwLib, scriptSample)
	cancelProfile()

	// Persist enriched library (with profiles) back to DB
	if kwJSON, err := json.Marshal(kwLib); err == nil {
		project.KeywordLibrary = kwJSON
		_ = s.projectRepo.UpdateKeywordLibrary(projectID, kwJSON)
	}

	// T3C: Auto-create Skills in character-service from detected character capability hints
	if len(kwLib.CharacterProfiles) > 0 && s.characterBaseURL != "" {
		skillCtx, cancelSkills := context.WithTimeout(ctx, 15*time.Second)
		s.autoCreateCharacterSkills(skillCtx, projectID, kwLib.CharacterProfiles)
		cancelSkills()
	}
	runtimeCfg := parseStoryboardRuntimeConfig(project)
	productionProfile := productionmode.ResolveProfile(project)
	autoSplitAfterOptimization := productionProfile.ShouldOptimizeScriptBeforeSplit(runtimeCfg.AutoSplitAfterOptimization)
	optimizedScriptText := strings.TrimSpace(scriptText)
	autoSplitProgress := AutoSplitMeta{
		Enabled:        autoSplitAfterOptimization,
		Duration:       runtimeCfg.Duration,
		VideoModel:     runtimeCfg.VideoModel,
		StylePreset:    stylepreset.Canonical(runtimeCfg.StylePreset),
		OriginalScript: strings.TrimSpace(scriptText),
	}
	if autoSplitAfterOptimization {
		s.updateProgress(projectID, ProgressInfo{
			Stage:        "episode_splitting",
			Message:      productionmode.OptimizeBeforeSplitMessage(productionProfile.Mode),
			EpisodeSplit: &StageProgress{Status: "running"},
			AutoSplit:    &autoSplitProgress,
		})
		if improved, err := s.optimizeProjectScriptForAutoSplit(ctx, project, scriptText, s.currentAdCopyOptimizationPrompt(project)); err != nil {
			if s.logger != nil {
				s.logger.Warn("project-level script optimization before auto split failed; fallback to original script",
					zap.Uint64("project_id", projectID),
					zap.Error(err),
				)
			}
		} else if improved != nil && strings.TrimSpace(improved.OptimizedScript) != "" {
			optimizedScriptText = strings.TrimSpace(improved.OptimizedScript)
			autoSplitProgress.OptimizedScript = optimizedScriptText
			autoSplitProgress.ConsistencyPremise = strings.TrimSpace(improved.ConsistencyPremise)
			autoSplitProgress.ScriptLength = utf8.RuneCountInString(optimizedScriptText)
			s.updateProgress(projectID, ProgressInfo{
				Stage:        "episode_splitting",
				Message:      productionmode.OptimizeAfterSplitMessage(productionProfile.Mode),
				EpisodeSplit: &StageProgress{Status: "running"},
				AutoSplit:    &autoSplitProgress,
			})
		}
	}

	// Priority: chapter markers → user keywords → LLM estimate → simple fallback
	// Normalize front matter (简介/营销块) before structural split.
	originalScriptText := strings.TrimSpace(scriptText)
	normOptimized := scriptsplit.NormalizeForEpisodeSplit(optimizedScriptText)
	optimizedScriptText = normOptimized.Text
	normOriginal := scriptsplit.NormalizeForEpisodeSplit(originalScriptText)
	normalizedOriginalScript := normOriginal.Text
	if synopsis := strings.TrimSpace(firstNonEmpty(normOptimized.StrippedSynopsis, normOriginal.StrippedSynopsis)); synopsis != "" {
		if strings.TrimSpace(project.Description) == "" {
			_ = s.projectRepo.UpdateDescription(projectID, synopsis)
			project.Description = synopsis
		}
		if s.logger != nil && (normOptimized.Changed || normOriginal.Changed) {
			s.logger.Info("script front matter normalized before episode split",
				zap.Uint64("project_id", projectID),
				zap.Strings("removed_blocks", normOptimized.RemovedBlocks),
				zap.Int("synopsis_chars", utf8.RuneCountInString(synopsis)),
			)
		}
	}

	// ══════════════════════════════════════════════════════════════════════════
	s.updateProgress(projectID, ProgressInfo{
		Stage:        "episode_splitting",
		Message:      "正在按原文章节优先拆分剧本为集数…",
		EpisodeSplit: &StageProgress{Status: "running"},
		AutoSplit:    &autoSplitProgress,
	})

	episodes, splitMethod := resolveStructuralEpisodeSplit(optimizedScriptText, normalizedOriginalScript, kwLib.SplitKeywords)

	chapterSplit := len(episodes) > 0
	llmSplitUsed := false

	if s.logger != nil {
		s.logger.Info("episode split result",
			zap.Uint64("project_id", projectID),
			zap.String("method", splitMethod),
			zap.Bool("success", chapterSplit),
			zap.Int("episodes_found", len(episodes)),
			zap.Int("script_length", utf8.RuneCountInString(optimizedScriptText)),
			zap.Int("original_script_length", utf8.RuneCountInString(normalizedOriginalScript)),
			zap.Bool("original_has_chapter_markers", scriptsplit.HasChapterMarkers(normalizedOriginalScript)),
			zap.Int("requested_target_episodes", project.TargetEpisodes),
			zap.Int("runtime_duration", runtimeCfg.Duration),
			zap.String("runtime_video_model", runtimeCfg.VideoModel),
			zap.String("runtime_style_preset", runtimeCfg.StylePreset),
		)
	}

	autoSplitMeta := buildAutoSplitMeta(optimizedScriptText, runtimeCfg, productionProfile)
	autoSplitMeta.OriginalScript = strings.TrimSpace(scriptText)
	autoSplitMeta.OptimizedScript = strings.TrimSpace(optimizedScriptText)
	if !chapterSplit {
		targetEpisodes := autoSplitMeta.EstimatedEpisodes
		if project.TargetEpisodes > 0 {
			targetEpisodes = project.TargetEpisodes
			autoSplitMeta.EstimatedEpisodes = project.TargetEpisodes
		}
		if targetEpisodes <= 1 {
			episodes = s.simpleSplit(optimizedScriptText, targetEpisodes, productionProfile)
		} else {
			splitCtx, cancelSplit := context.WithTimeout(ctx, episodeSplitTimeout)
			var err error
			writingHints := s.fetchWritingSkillHints(splitCtx, projectID)
			episodes, err = s.callLLMSplit(splitCtx, project, optimizedScriptText, targetEpisodes, &kwLib, writingHints, productionProfile)
			cancelSplit()
			llmSplitUsed = err == nil && len(episodes) > 0
			if err != nil {
				episodes = s.simpleSplit(optimizedScriptText, targetEpisodes, productionProfile)
			}
		}
	}

	// Enrich chapter-split episodes with LLM summary + keywords.
	// Fetch writing skills once here; they are also used in the LLM-split path (line ~844).
	chapterWritingHints := s.fetchWritingSkillHints(ctx, projectID)
	if chapterSplit && len(episodes) > 0 {
		s.updateProgress(projectID, ProgressInfo{
			Stage:        "episode_splitting",
			Message:      fmt.Sprintf("正在为 %d 集生成摘要和关键词…", len(episodes)),
			EpisodeSplit: &StageProgress{Total: len(episodes), Status: "running"},
		})
		enrichCtx, cancelEnrich := context.WithTimeout(ctx, episodeEnrichTimeout)
		s.enrichEpisodesParallel(enrichCtx, project, episodes, &kwLib, chapterWritingHints)
		cancelEnrich()
	}

	episodes = s.repairEpisodeSplitStructure(ctx, episodes, splitMethod, llmSplitUsed, productionProfile)

	if s.logger != nil {
		s.logger.Info("episodes split complete",
			zap.Uint64("project_id", projectID),
			zap.Int("count", len(episodes)),
		)
	}

	// ── Save episodes with status "pending" (awaiting scene split) ───────────
	var dbEpisodes []model.Episode
	for i, ep := range episodes {
		kws := make([]string, 0, len(ep.Keywords))
		kws = append(kws, ep.Keywords...)
		dbEpisodes = append(dbEpisodes, model.Episode{
			ProjectID:         projectID,
			EpisodeNumber:     i + 1,
			Title:             ep.Title,
			Summary:           ep.Summary,
			ScriptExcerpt:     ep.Excerpt,
			WordCount:         utf8.RuneCountInString(ep.Excerpt),
			EstimatedDuration: utf8.RuneCountInString(ep.Excerpt) / 5,
			Status:            "pending", // Awaiting scene split
			Version:           1,
			Keywords:          kws,
		})
	}

	// ── Layer 3: Atomic delete+create in a single transaction ──
	if err := s.episodeRepo.ReplaceAllForProject(projectID, dbEpisodes); err != nil {
		return nil, fmt.Errorf("replace episodes: %w", err)
	}

	// Report episode split done
	autoSplitMeta.EstimatedEpisodes = len(dbEpisodes)
	if autoStoryboard {
		prepMode := autoPrepFullFirstEpisode
		sceneSplitStatus := "running"
		var prepMessage string
		var prepNextStep string
		if isResplit {
			prepMode = autoPrepTextOnly
			sceneSplitStatus = "pending"
			if len(dbEpisodes) <= 1 {
				prepMessage = fmt.Sprintf("分集完成（%d 集），开始润色优化剧本（仅文本处理）…", len(dbEpisodes))
			} else {
				prepMessage = fmt.Sprintf("分集完成（%d 集），将自动润色优化第 1 集示范剧本（仅文本），资源与分镜请手动启动", len(dbEpisodes))
			}
			prepNextStep = "示范集文本处理完成后，请在单集列表点击「自动处理」衔接资源与分镜"
		} else if len(dbEpisodes) <= 1 {
			prepMessage = fmt.Sprintf("分集完成（%d 集），开始自动处理第 1 集（润色 → 资源 → 分镜）…", len(dbEpisodes))
			prepNextStep = "完成后可在第 1 集工作台继续出图与成片"
		} else {
			prepMessage = fmt.Sprintf("分集完成（%d 集），将自动处理第 1 集示范流程（润色 → 资源 → 分镜）", len(dbEpisodes))
			prepNextStep = "示范集完成后，请为其余分集点击「自动处理」"
		}
		s.updateProgress(projectID, ProgressInfo{
			Stage: "script_prepping",
			EpisodeSplit: &StageProgress{
				Total: len(dbEpisodes), Completed: len(dbEpisodes), Status: "done",
			},
			SceneSplit: &StageProgress{
				Total: len(dbEpisodes), Completed: 0, Status: sceneSplitStatus,
			},
			Message:    prepMessage,
			PhaseLabel: "分集已完成",
			NextStep:   prepNextStep,
			AutoSplit:  &autoSplitMeta,
		})
		s.startAutoPreparationPipeline(project, dbEpisodes, false, prepMode)
	} else {
		s.updateProgress(projectID, ProgressInfo{
			Stage: "idle",
			EpisodeSplit: &StageProgress{
				Total: len(dbEpisodes), Completed: len(dbEpisodes), Status: "done",
			},
			SceneSplit: &StageProgress{
				Total: len(dbEpisodes), Completed: 0, Status: "pending",
			},
			Message: func() string {
				if len(dbEpisodes) <= 1 {
					return fmt.Sprintf("分集完成（%d 集），请手动推进润色、资源提取或分镜流程", len(dbEpisodes))
				}
				return fmt.Sprintf("分集完成（%d 集），请逐集手动优化或使用「自动处理」", len(dbEpisodes))
			}(),
			PhaseLabel: "分集已完成",
			NextStep:   "可在单集列表中逐集优化剧本，或点击「自动处理」衔接资源与分镜",
			AutoSplit:  &autoSplitMeta,
		})
		_ = s.projectRepo.UpdateStatus(projectID, project.UserID, "script_ready")
		if s.logger != nil {
			s.logger.Info("episode split finished without auto storyboard pipeline",
				zap.Uint64("project_id", projectID),
				zap.Int("episode_count", len(dbEpisodes)),
			)
		}
	}

	return dbEpisodes, nil
}

type llmEpisode struct {
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	Excerpt   string   `json:"excerpt"`
	StartText string   `json:"start_text"`
	EndText   string   `json:"end_text"`
	Keywords  []string `json:"keywords"`
}

// CharacterProfile holds the canonical visual and voice description for a character.
// Generated by LLM from the script and stored in project.KeywordLibrary to ensure
// consistent character appearance across all scenes and storyboards.
type CharacterProfile struct {
	Name         string   `json:"name"`
	Appearance   string   `json:"appearance"`              // visual description in Chinese for scene descriptions
	AppearanceEN string   `json:"appearance_en,omitempty"` // English visual description for AI image generation prompts
	VoiceHint    string   `json:"voice_hint"`              // male/female/child/narrator — aids auto-voice assignment
	SkillHints   []string `json:"skill_hints,omitempty"`   // detected capability tags: combat|exploration|social|special
}

// LocationProfile holds the canonical visual description for a scene location.
type LocationProfile struct {
	Name          string `json:"name"`
	Description   string `json:"description"`              // visual environment description in Chinese
	DescriptionEN string `json:"description_en,omitempty"` // English environment description for AI image generation prompts
}

// PropProfile holds the canonical visual description for an important prop or item.
type PropProfile struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// KeywordLibrary represents extracted project-level keyword glossary.
type KeywordLibrary struct {
	Characters    []string `json:"characters"`               // 人物
	Locations     []string `json:"locations"`                // 地点
	Events        []string `json:"events"`                   // 事件/概念
	Props         []string `json:"props"`                    // 道具
	SplitKeywords []string `json:"split_keywords,omitempty"` // 用户自定义分集边界关键字

	// Visual consistency profiles — generated by enrichKeywordLibraryWithProfiles.
	// Injected into every scene-split and storyboard-image prompt.
	CharacterProfiles []CharacterProfile `json:"character_profiles,omitempty"`
	LocationProfiles  []LocationProfile  `json:"location_profiles,omitempty"`
	PropProfiles      []PropProfile      `json:"prop_profiles,omitempty"`
}

// mergeKeywordLibraries —— 合并用户提供和 LLM 提取的关键词库，去重后返回
// mergeKeywordLibraries merges user-provided keywords (a) with LLM-extracted keywords (b).
// User keywords appear first and duplicates are removed.
func mergeKeywordLibraries(user, llm KeywordLibrary) KeywordLibrary {
	dedup := func(a, b []string) []string {
		seen := make(map[string]bool, len(a)+len(b))
		var out []string
		for _, lists := range [2][]string{a, b} {
			for _, s := range lists {
				s = strings.TrimSpace(s)
				if s != "" && !seen[s] {
					seen[s] = true
					out = append(out, s)
				}
			}
		}
		return out
	}
	merged := KeywordLibrary{
		Characters: dedup(user.Characters, llm.Characters),
		Locations:  dedup(user.Locations, llm.Locations),
		Events:     dedup(user.Events, llm.Events),
		Props:      dedup(user.Props, llm.Props),
	}
	// Preserve existing visual profiles from a prior enrichment run or manual edits.
	if len(user.CharacterProfiles) > 0 {
		merged.CharacterProfiles = user.CharacterProfiles
	}
	if len(user.LocationProfiles) > 0 {
		merged.LocationProfiles = user.LocationProfiles
	}
	if len(user.PropProfiles) > 0 {
		merged.PropProfiles = user.PropProfiles
	}
	return merged
}

type llmCharacterState struct {
	Name    string `json:"name"`
	Action  string `json:"action"`  // what character is doing
	Emotion string `json:"emotion"` // emotional state
}

type llmScene struct {
	Description     string              `json:"description"`
	ShotType        string              `json:"shot_type"` // close-up | medium | full | wide | overhead | low-angle
	Characters      []string            `json:"characters"`
	CharacterStates []llmCharacterState `json:"character_states,omitempty"` // T3B: per-character action/emotion
	Items           []string            `json:"items,omitempty"`            // visible props/objects in the scene
	Location        string              `json:"location"`
	Duration        int                 `json:"duration"`
	Dialogue        string              `json:"dialogue"`
	Mood            string              `json:"mood,omitempty"` // T3B: emotional tone of the scene
}

// generateStoryboardsParallel —— 使用工作池并行为所有剧集创建分镜，返回总分镜数
// generateStoryboardsParallel creates storyboards for all episodes using a worker pool
// for LLM scene breakdown. Reports per-episode progress and updates episode status.
func (s *EpisodeService) generateStoryboardsParallel(ctx context.Context, projectID, userID uint64, dbEpisodes []model.Episode, kwLib *KeywordLibrary, clipDuration int, videoModel string, aspectRatio string, resolution string, speechPace string, projectType string) int {
	return s.generateStoryboardsParallelWithOffset(ctx, projectID, userID, dbEpisodes, kwLib, clipDuration, videoModel, aspectRatio, resolution, speechPace, projectType, 0)
}

func (s *EpisodeService) generateStoryboardsParallelWithOffset(ctx context.Context, projectID, userID uint64, dbEpisodes []model.Episode, kwLib *KeywordLibrary, clipDuration int, videoModel string, aspectRatio string, resolution string, speechPace string, projectType string, startSequence int) int {
	projectRef, _ := s.projectRepo.FindByIDNoAuth(projectID)
	type episodeScenes struct {
		idx    int
		scenes []llmScene
	}

	const maxWorkers = 5
	sem := make(chan struct{}, maxWorkers)
	var mu sync.Mutex
	var allResults []episodeScenes
	var completedCount int32 // atomic counter for progress
	cancelled := false
	createdCount := 0

	var wg sync.WaitGroup

	// Fetch storyboard skill hints and script-prep skill hints once before the parallel loop.
	storyboardHints := s.fetchStoryboardSkillHints(ctx, projectID)
	prepSkillHints := s.fetchScriptPrepSkillHints(ctx, projectID)

	// Fetch the project's storyboard style preset and matching PromptTemplate once.
	// The template is used to produce PromptUsed for each storyboard at creation time.
	var storyboardPromptTemplate string
	projectVisualEra := ""
	projectStylePreset := stylepreset.Default
	projectMotionMode := ""
	productionProfile := productionmode.Profile{Mode: productionmode.ModeScriptDrama}
	serialSceneEnabled := strings.TrimSpace(projectType) == "video_serial"
	assetRefs := s.fetchAssetReferences(ctx, projectID, nil)
	if project, err := s.projectRepo.FindByIDNoAuth(projectID); err == nil {
		productionProfile = productionmode.ResolveProfile(project)
		runtimeCfg := parseStoryboardRuntimeConfig(project)
		projectStylePreset = runtimeCfg.StylePreset
		if projectStylePreset == "" {
			projectStylePreset = stylepreset.Default
		}
		projectMotionMode = runtimeCfg.MotionMode
		serialSceneEnabled = shouldEnableSceneSerial(projectType)
		sk := storyboardStyleKey(projectStylePreset)
		storyboardPromptTemplate = s.fetchStoryboardPromptTemplate(ctx, sk)
		if productionProfile.IsAd() {
			adDirective := productionmode.AdWorkbenchDirective()
			if strings.TrimSpace(storyboardPromptTemplate) != "" {
				storyboardPromptTemplate = strings.TrimSpace(storyboardPromptTemplate) + "\n\n广告工作台追加约束：\n" + adDirective
			}
		}
		projectVisualEra = inferVisualEra(strings.TrimSpace(project.ScriptText))
		if s.logger != nil {
			if storyboardPromptTemplate != "" {
				s.logger.Info("fetched storyboard prompt template",
					zap.Uint64("project_id", projectID),
					zap.String("style_key", sk),
				)
			}
			s.logger.Info("scene serial mode resolved",
				zap.Uint64("project_id", projectID),
				zap.String("project_type", projectType),
				zap.Bool("serial_scene_enabled", serialSceneEnabled),
			)
		}
	}

	for i, ep := range dbEpisodes {
		// Check context before launching new workers
		select {
		case <-ctx.Done():
			if s.logger != nil {
				s.logger.Warn("storyboard generation cancelled",
					zap.Uint64("project_id", projectID),
					zap.Int("queued_episodes", i),
				)
			}
			cancelled = true
		default:
		}
		if cancelled {
			break
		}

		// Mark episode as scene_splitting
		_ = s.episodeRepo.UpdateStatus(uint64(ep.ID), "scene_splitting")

		// Prefer screenplay-format text (produced by OptimizeEpisode/AutoOptimizeReview)
		// over raw novel text so the scene splitter gets properly structured input.
		content := ep.OptimizedText
		if content == "" {
			content = ep.ScriptExcerpt
		}
		if content == "" {
			content = ep.Summary
		}

		wg.Add(1)
		sem <- struct{}{}

		optimizeStatus := ep.OptimizeStatus
		reviewStatus := ep.ReviewStatus
		contentSource := "summary"
		switch {
		case strings.TrimSpace(ep.OptimizedText) != "":
			contentSource = "optimized_text"
		case strings.TrimSpace(ep.ScriptExcerpt) != "":
			contentSource = "script_excerpt"
		}
		go func(idx int, epID uint64, epNum int, epContent, optimizeStatus, reviewStatus, contentSource string) {
			defer wg.Done()
			defer func() { <-sem }()

			if s.logger != nil {
				s.logger.Info("breaking episode into scenes",
					zap.Int("episode", epNum),
					zap.String("content_source", contentSource),
					zap.Int("content_len", utf8.RuneCountInString(epContent)),
				)
			}

			optimized := epContent
			if productionmode.ShouldSkipScriptPrepAfterAutoOptimize(optimizeStatus, reviewStatus, epContent, productionProfile.Mode) {
				if s.logger != nil {
					s.logger.Info("skip script prep because auto-optimize output already annotated",
						zap.Int("episode", epNum),
						zap.Uint64("episode_id", epID),
					)
				}
			} else {
				// Pre-optimization: run a professional storyboard-prep pass before scene splitting
				// to add explicit visual markers, camera suggestions and pacing cues.
				_ = s.episodeRepo.UpdateStatus(epID, "script_prepping")
				optimized = s.prepareScriptForStoryboard(ctx, projectRef, epContent, epNum, kwLib, productionProfile, prepSkillHints)
				if s.logger != nil && optimized != epContent {
					s.logger.Info("script prep optimization applied",
						zap.Int("episode", epNum),
						zap.Int("original_len", utf8.RuneCountInString(epContent)),
						zap.Int("optimized_len", utf8.RuneCountInString(optimized)),
					)
				}
			}

			customStoryboardSplitPrompt := s.currentStoryboardSplitPrompt(projectRef)
			scenes := s.breakEpisodeIntoScenes(ctx, optimized, epNum, storyboardHints, kwLib, clipDuration, videoModel, aspectRatio, resolution, speechPace, productionProfile, customStoryboardSplitPrompt, projectStylePreset, projectMotionMode)
			if s.logger != nil {
				s.logger.Info("episode scene split completed",
					zap.Uint64("project_id", projectID),
					zap.Uint64("episode_id", epID),
					zap.Int("episode", epNum),
					zap.Int("scene_count", len(scenes)),
				)
			}

			// Update episode status
			if len(scenes) > 0 {
				_ = s.episodeRepo.UpdateStatus(epID, "scene_ready")
			} else {
				_ = s.episodeRepo.UpdateStatus(epID, "scene_ready") // fallback created at write time
			}

			mu.Lock()
			allResults = append(allResults, episodeScenes{idx: idx, scenes: scenes})
			completedCount++
			completed := int(completedCount)
			mu.Unlock()

			// Report progress
			var progressSnapshot ProgressInfo
			if refreshedProject, err := s.projectRepo.FindByIDNoAuth(projectID); err == nil && len(refreshedProject.Progress) > 0 {
				_ = json.Unmarshal(refreshedProject.Progress, &progressSnapshot)
			}
			s.updateProgress(projectID, ProgressInfo{
				Stage: "scene_splitting",
				EpisodeSplit: &StageProgress{
					Total: len(dbEpisodes), Completed: len(dbEpisodes), Status: "done",
				},
				SceneSplit: &StageProgress{
					Total: len(dbEpisodes), Completed: completed, Current: epNum, Status: "running",
				},
				Message:   fmt.Sprintf("正在拆分分镜 %d/%d（第%d集）", completed, len(dbEpisodes), epNum),
				AutoSplit: progressSnapshot.AutoSplit,
			})
		}(i, uint64(ep.ID), ep.EpisodeNumber, content, optimizeStatus, reviewStatus, contentSource)
	}
	wg.Wait()

	// Write storyboards in episode order
	sortedScenes := make([][]llmScene, len(dbEpisodes))
	for _, r := range allResults {
		sortedScenes[r.idx] = r.scenes
	}

	globalSeq := startSequence
	// crossEpisodePrevPrompt carries the last generated prompt from episode N
	// to episode N+1, so the LLM can maintain visual continuity across episode boundaries.
	var crossEpisodePrevPrompt string
	var prevSceneForContinuity *llmScene
	for i, scenes := range sortedScenes {
		ep := dbEpisodes[i]
		epID := uint64(ep.ID)

		if len(scenes) == 0 {
			globalSeq++
			// Prefer screenplay-format text for the fallback storyboard, same priority as scene splitter.
			sceneDesc := ep.Summary
			if strings.TrimSpace(ep.ScriptExcerpt) != "" {
				sceneDesc = ep.ScriptExcerpt
			}
			if strings.TrimSpace(ep.OptimizedText) != "" {
				sceneDesc = ep.OptimizedText
			}
			promptUsed := sceneDesc
			if storyboardPromptTemplate != "" {
				promptUsed = applyPromptTemplate(storyboardPromptTemplate, sceneDesc, "", "", "")
			}
			_, err := s.storyboardSvc.Create(projectID, CreateStoryboardReq{
				EpisodeID:        &epID,
				SequenceNumber:   globalSeq,
				SceneDescription: sceneDesc,
				Duration:         max(ep.EstimatedDuration, clipDuration),
				PromptUsed:       promptUsed,
			})
			if err != nil && s.logger != nil {
				s.logger.Warn("auto-create storyboard failed", zap.Uint64("episode_id", epID), zap.Error(err))
			} else if err == nil {
				createdCount++
			}
			continue
		}

		// LLM refinement pass: produce cohesive, skill-injected image prompts for all scenes in this episode.
		// kwLib provides character/location appearance profiles for visual consistency across prompts.
		// crossEpisodePrevPrompt seeds the first batch with the last scene from the previous episode.
		episodeAssets := filterAssetReferencesByEpisode(assetRefs, epID)
		refinedPrompts := s.refineScenePrompts(ctx, scenes, storyboardHints, storyboardPromptTemplate, kwLib, ep.EpisodeNumber, projectType, crossEpisodePrevPrompt, projectStylePreset)
		if s.logger != nil {
			s.logger.Info("episode prompt refinement completed",
				zap.Uint64("project_id", projectID),
				zap.Uint64("episode_id", epID),
				zap.Int("episode", ep.EpisodeNumber),
				zap.Int("scene_count", len(scenes)),
				zap.Int("refined_prompt_count", len(refinedPrompts)),
			)
		}
		// Update cross-episode context with the last refined prompt from this episode.
		if len(refinedPrompts) > 0 {
			if last := strings.TrimSpace(refinedPrompts[len(refinedPrompts)-1]); last != "" {
				crossEpisodePrevPrompt = last
			}
		}

		// 串行模式：记录每集中每个场景组首次出现，首次为 IsSceneFirstClip=true，其余跳过图片生成。
		// 普通 video 项目不应写入 scene_group_key / is_scene_first_clip，否则会被后续出图逻辑误判为串行链路。
		seenSceneGroupsInEpisode := make(map[string]bool)
		for j, scene := range scenes {
			globalSeq++
			chars := make([]string, len(scene.Characters))
			copy(chars, scene.Characters)
			matchedAssets := matchAssetsToScene(scene, episodeAssets)
			charAnchors, propAnchors, sceneAnchors := extractAssetVisualAnchors(matchedAssets)
			// Enrich scene description with era, mood atmosphere, character appearance and continuity notes.
			desc := enrichSceneDescription(scene, prevSceneForContinuity, kwLib, projectVisualEra)
			sceneGroupKey := ""
			isSceneFirstClip := false
			if serialSceneEnabled {
				sceneGroupKey = normalizeSceneKey(scene.Location)
				if sceneGroupKey == "" {
					sceneGroupKey = normalizeSceneKey(scene.Description)
				}
			}
			if sceneGroupKey != "" {
				if !seenSceneGroupsInEpisode[sceneGroupKey] {
					isSceneFirstClip = true
					seenSceneGroupsInEpisode[sceneGroupKey] = true
				}
			}

			// Build PromptUsed: prefer LLM-refined prompt, then template substitution, then raw description.
			var promptUsed string
			if j < len(refinedPrompts) && strings.TrimSpace(refinedPrompts[j]) != "" {
				promptUsed = refinedPrompts[j]
			} else if storyboardPromptTemplate != "" {
				action := ""
				if len(scene.CharacterStates) > 0 {
					var actionParts []string
					for _, cs := range scene.CharacterStates {
						if cs.Action != "" {
							actionParts = append(actionParts, cs.Action)
						}
					}
					action = strings.Join(actionParts, ", ")
				}
				promptUsed = applyPromptTemplate(storyboardPromptTemplate, desc, strings.Join(chars, ", "), action, scene.Mood)
			} else {
				constraintHints := deriveStoryboardConstraintHints(StoryboardGenerateRequest{
					SceneDescription: desc,
					Characters:       chars,
					Location:         scene.Location,
					CameraMovement:   shotTypeToCameraMovement(scene.ShotType),
					Mood:             scene.Mood,
				}, chars, nil, nil)
				promptUsed = composeStoryboardPrompt(StoryboardPromptParts{
					Subject:            desc,
					CharacterAnchors:   charAnchors,
					PropAnchors:        propAnchors,
					SceneAnchors:       sceneAnchors,
					CameraGrammar:      shotTypeToCameraMovement(scene.ShotType),
					PoseConstraint:     constraintHints.PoseConstraint,
					ActionConstraint:   constraintHints.ActionConstraint,
					SpatialConstraint:  constraintHints.SpatialConstraint,
					WardrobeConstraint: constraintHints.WardrobeConstraint,
				})
			}

			storyboardDuration := clampDuration(scene.Duration, 2, 12)
			storyboardDialogue := scene.Dialogue
			if quotes := speechtext.ExtractCharacterQuotesFromScene(desc, chars); len(quotes) > 0 {
				var extras []string
				for _, q := range quotes {
					extras = append(extras, q.Speaker+"："+q.Quote)
				}
				storyboardDialogue = strings.TrimSpace(storyboardDialogue + "\n" + strings.Join(extras, "\n"))
			}
			storyboardDialogue = speechtext.FitStoryboardDialogue(
				storyboardDialogue,
				storyboardDuration,
				speechPace,
				productionProfile.IsCommentaryComic(),
			)

			_, err := s.storyboardSvc.Create(projectID, CreateStoryboardReq{
				EpisodeID:        &epID,
				SequenceNumber:   globalSeq,
				SceneDescription: desc,
				Characters:       chars,
				Location:         scene.Location,
				Duration:         storyboardDuration,
				Dialogue:         storyboardDialogue,
				CameraMovement:   shotTypeToCameraMovement(scene.ShotType),
				Mood:             scene.Mood,
				PromptUsed:       promptUsed,
				AssetIDs:         assetReferenceIDs(matchedAssets),
				SceneGroupKey:    sceneGroupKey,
				IsSceneFirstClip: isSceneFirstClip,
			})
			if err != nil && s.logger != nil {
				s.logger.Warn("auto-create storyboard failed", zap.Uint64("episode_id", epID), zap.Int("seq", globalSeq), zap.Error(err))
			} else if err == nil {
				createdCount++
				captured := scene
				captured.Description = desc
				prevSceneForContinuity = &captured
			}
		}
		// Mark as done after storyboards are written
		_ = s.episodeRepo.UpdateStatus(epID, "done")
		if s.logger != nil {
			s.logger.Info("episode storyboard persistence completed",
				zap.Uint64("project_id", projectID),
				zap.Uint64("episode_id", epID),
				zap.Int("episode", ep.EpisodeNumber),
				zap.Int("created_so_far", createdCount),
			)
		}
	}
	return createdCount
}

// breakEpisodeIntoScenes —— 将单集内容拆分为视觉场景，带重试和降级策略
// breakEpisodeIntoScenes calls LLM to split an episode into visual scenes for storyboarding.
// It retries up to 2 times on failure, and falls back to paragraph-based splitting if LLM fails entirely.
func (s *EpisodeService) breakEpisodeIntoScenes(ctx context.Context, episodeContent string, episodeNum int, skillHints string, kwLib *KeywordLibrary, clipDuration int, videoModel string, aspectRatio string, resolution string, speechPace string, profile productionmode.Profile, customStoryboardSplitPrompt string, stylePreset string, motionMode string) []llmScene {
	if strings.TrimSpace(episodeContent) == "" {
		return nil
	}

	fallback := func() []llmScene {
		return s.postProcessAndAlignCommentaryScenes(
			episodeContent,
			s.fallbackSceneSplit(episodeContent, episodeNum, clipDuration, profile),
			clipDuration,
			speechPace,
			profile,
		)
	}

	// Try LLM-based scene splitting with retries
	const maxRetries = 2
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Brief pause between retries to avoid rate limiting
			select {
			case <-ctx.Done():
				if s.logger != nil {
					s.logger.Warn("scene split cancelled", zap.Int("episode", episodeNum))
				}
				return fallback()
			case <-time.After(2 * time.Second):
			}
		}

		scenes := s.callLLMSceneSplit(ctx, episodeContent, episodeNum, skillHints, kwLib, clipDuration, videoModel, aspectRatio, resolution, speechPace, profile, customStoryboardSplitPrompt, stylePreset, motionMode)
		if len(scenes) > 0 {
			return s.postProcessAndAlignCommentaryScenes(episodeContent, scenes, clipDuration, speechPace, profile)
		}
	}

	if s.logger != nil {
		s.logger.Warn("LLM scene split failed after retries, using paragraph fallback",
			zap.Int("episode", episodeNum))
	}
	return fallback()
}

// callLLMSceneSplit —— 调用 LLM 将剧集内容拆分为原子场景，支持长文本分块
// callLLMSceneSplit splits episode content into atomic scenes via LLM.
// Supports up to 100k chars; automatically chunks long texts at paragraph boundaries.
func (s *EpisodeService) callLLMSceneSplit(ctx context.Context, episodeContent string, episodeNum int, skillHints string, kwLib *KeywordLibrary, clipDuration int, videoModel string, aspectRatio string, resolution string, speechPace string, profile productionmode.Profile, customStoryboardSplitPrompt string, stylePreset string, motionMode string) []llmScene {
	const maxChars = 100000
	if runeLen := utf8.RuneCountInString(episodeContent); runeLen > maxChars {
		episodeContent = string([]rune(episodeContent)[:maxChars])
	}

	// For long texts (>30k chars), split into chunks at paragraph boundaries
	const chunkLimit = 30000
	if utf8.RuneCountInString(episodeContent) > chunkLimit {
		return s.sceneSplitChunked(ctx, episodeContent, episodeNum, chunkLimit, skillHints, kwLib, clipDuration, videoModel, aspectRatio, resolution, speechPace, profile, customStoryboardSplitPrompt, stylePreset, motionMode)
	}

	return s.sceneSplitSingle(ctx, episodeContent, episodeNum, skillHints, kwLib, clipDuration, videoModel, aspectRatio, resolution, speechPace, profile, customStoryboardSplitPrompt, stylePreset, motionMode)
}

// sceneSplitChunked —— 将长文本按段落边界分块后逐块调用 LLM 拆分场景
// sceneSplitChunked splits long content into paragraph-aligned chunks and processes each via LLM.
func (s *EpisodeService) sceneSplitChunked(ctx context.Context, content string, episodeNum int, chunkLimit int, skillHints string, kwLib *KeywordLibrary, clipDuration int, videoModel string, aspectRatio string, resolution string, speechPace string, profile productionmode.Profile, customStoryboardSplitPrompt string, stylePreset string, motionMode string) []llmScene {
	paragraphs := splitIntoParagraphs(content)

	var chunks []string
	var buf strings.Builder
	bufLen := 0

	for _, p := range paragraphs {
		pLen := utf8.RuneCountInString(p)
		if bufLen+pLen > chunkLimit && bufLen > 0 {
			chunks = append(chunks, buf.String())
			buf.Reset()
			bufLen = 0
		}
		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		buf.WriteString(p)
		bufLen += pLen
	}
	if buf.Len() > 0 {
		chunks = append(chunks, buf.String())
	}

	if s.logger != nil {
		s.logger.Info("scene split chunked",
			zap.Int("episode", episodeNum),
			zap.Int("chunks", len(chunks)),
			zap.Int("total_chars", utf8.RuneCountInString(content)),
		)
	}

	var allScenes []llmScene
	for i, chunk := range chunks {
		select {
		case <-ctx.Done():
			return allScenes
		default:
		}
		scenes := s.sceneSplitSingle(ctx, chunk, episodeNum, skillHints, kwLib, clipDuration, videoModel, aspectRatio, resolution, speechPace, profile, customStoryboardSplitPrompt, stylePreset, motionMode)
		if s.logger != nil {
			s.logger.Info("chunk scene split done",
				zap.Int("episode", episodeNum),
				zap.Int("chunk", i+1),
				zap.Int("total_chunks", len(chunks)),
				zap.Int("scenes", len(scenes)),
			)
		}
		allScenes = append(allScenes, scenes...)
	}
	return allScenes
}

// sceneSplitSingle —— 单次 LLM 调用将内容拆分为原子视觉场景
// sceneSplitSingle makes a single LLM call to split content into atomic scenes.
func (s *EpisodeService) sceneSplitSingle(ctx context.Context, content string, episodeNum int, skillHints string, kwLib *KeywordLibrary, clipDuration int, videoModel string, aspectRatio string, resolution string, speechPace string, profile productionmode.Profile, customStoryboardSplitPrompt string, stylePreset string, motionMode string) []llmScene {
	// clipDuration is the user-selected target clip length for ad storyboards.
	// Scene splitting should follow that fixed per-clip duration instead of
	// letting each storyboard land on a different saved duration.
	refDuration := clipDuration
	if refDuration < 3 {
		refDuration = 3
	}
	// Build model-specific duration constraint hint for the LLM.
	modelDurationHint := videoModelDurationHint(videoModel, refDuration)
	visualHint := visualConstraintHint(aspectRatio, resolution, refDuration)
	speechHint := speechPaceHint(speechPace, refDuration)

	splitParams := productionmode.SceneSplitParams{
		EpisodeNum:    episodeNum,
		Content:       content,
		RefDuration:   refDuration,
		ModelDuration: modelDurationHint,
		VisualHint:    visualHint,
		SpeechHint:    speechHint,
		StylePreset:   stylepreset.Canonical(stylePreset),
		StyleHint:     productionmode.StyleSplitVisualHint(stylePreset, motionMode),
	}
	prompt := productionmode.SceneSplitUserPrompt(profile.Mode, splitParams)
	sceneSystemPrompt := productionmode.SceneSplitSystemPrompt(profile.Mode)
	if styleBlock := productionmode.RefinePromptStyleBlock(stylePreset); styleBlock != "" {
		sceneSystemPrompt += "\n\n" + styleBlock
	}
	if styleHint := videoModelStyleHint(videoModel); styleHint != "" {
		sceneSystemPrompt += "\n\n" + styleHint
	}
	if skillHints != "" {
		sceneSystemPrompt += "\n\n**本项目专属分镜指引（请务必遵守）：**\n" + skillHints
	}
	if strings.TrimSpace(customStoryboardSplitPrompt) != "" {
		sceneSystemPrompt += "\n\n**项目级分镜拆分补充规则（优先于通用经验，但不得违反系统硬约束）：**\n" + strings.TrimSpace(customStoryboardSplitPrompt)
	}

	// Inject consistency bible so the LLM writes visually grounded, consistent scene descriptions.
	if bible := buildConsistencyBibleBlock(kwLib); bible != "" {
		sceneSystemPrompt += bible
	}
	if eraHint := inferVisualEra(content); eraHint != "" {
		sceneSystemPrompt += "\n\n**时代与造型约束（必须保持一致）：**\n" + eraHint + "\n所有场景中的服装、发型、建筑、道具、色彩和光线都必须符合这一时代背景，不得漂移到其他年代。"
	}

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": sceneSystemPrompt},
			{"role": "user", "content": prompt},
		},
		"temperature":     0.3,
		"max_tokens":      16384,
		"response_format": map[string]string{"type": "json_object"},
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("create scene split request failed", zap.Int("episode", episodeNum), zap.Error(err))
		}
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("scene split LLM call failed", zap.Int("episode", episodeNum), zap.Error(err))
		}
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		if s.logger != nil {
			s.logger.Warn("scene split LLM returned error",
				zap.Int("episode", episodeNum),
				zap.Int("status", resp.StatusCode),
				zap.String("body_preview", string(body[:min(300, len(body))])),
			)
		}
		return nil
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		if s.logger != nil {
			s.logger.Warn("parse LLM response failed",
				zap.Int("episode", episodeNum),
				zap.Error(err),
				zap.String("body_preview", string(body[:min(300, len(body))])),
			)
		}
		return nil
	}

	if llmResp.Choices[0].FinishReason == "length" && s.logger != nil {
		s.logger.Warn("scene split response was truncated (finish_reason=length)",
			zap.Int("episode", episodeNum))
	}

	respContent := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	if respContent == "" {
		return nil
	}

	var wrapper struct {
		Scenes []llmScene `json:"scenes"`
	}
	if err := json.Unmarshal([]byte(respContent), &wrapper); err == nil && len(wrapper.Scenes) > 0 {
		if s.logger != nil {
			s.logger.Info("scene split succeeded",
				zap.Int("episode", episodeNum),
				zap.Int("scene_count", len(wrapper.Scenes)),
			)
		}
		return wrapper.Scenes
	}

	var scenes []llmScene
	if err := json.Unmarshal([]byte(respContent), &scenes); err != nil {
		if s.logger != nil {
			s.logger.Warn("parse scene split json failed",
				zap.Int("episode", episodeNum),
				zap.String("content_preview", respContent[:min(300, len(respContent))]),
			)
		}
		return nil
	}
	return scenes
}

// fallbackSceneSplit —— 降级方案：按段落将剧集内容拆分为场景
// fallbackSceneSplit creates scenes from episode text using paragraph-based splitting.
// Each paragraph becomes its own scene for maximum granularity.
func (s *EpisodeService) fallbackSceneSplit(episodeContent string, episodeNum int, clipDuration int, profile productionmode.Profile) []llmScene {
	paragraphs := splitIntoParagraphs(episodeContent)

	if len(paragraphs) <= 1 {
		return nil // Caller will use the single-storyboard fallback
	}

	dur := clipDuration
	if dur <= 0 {
		dur = 5
	}

	var scenes []llmScene
	for i, p := range paragraphs {
		runes := []rune(p)
		descLen := 150
		if len(runes) < descLen {
			descLen = len(runes)
		}
		desc := string(runes[:descLen])
		if len(runes) > descLen {
			desc += "..."
		}

		dialogue := sanitizeStoryboardDialogue(p)
		if profile.IsCommentaryComic() {
			if narr := speechtext.ExtractParagraphNarration(p); narr != "" {
				dialogue = narr
			} else {
				dialogue = ""
			}
		}
		scenes = append(scenes, llmScene{
			Description: fmt.Sprintf("第%d集·场景%d：%s", episodeNum, i+1, desc),
			Characters:  nil,
			Location:    "",
			Duration:    dur,
			Dialogue:    dialogue,
		})
	}

	if s.logger != nil {
		s.logger.Info("fallback scene split",
			zap.Int("episode", episodeNum),
			zap.Int("scene_count", len(scenes)),
		)
	}
	return scenes
}

func normalizeAdSceneDuration(duration int, clipDuration int) int {
	if clipDuration > 0 {
		return clipDuration
	}
	if duration > 0 {
		return duration
	}
	return 5
}

func (s *EpisodeService) postProcessAdScenes(scenes []llmScene, clipDuration int, speechPace string) []llmScene {
	if len(scenes) == 0 {
		return scenes
	}
	const minDialogueRunes = 8
	const minLeadDialogueRunes = 14
	const longDialogueRunes = 36
	processed := make([]llmScene, 0, len(scenes))
	mergeIntoPrev := func(prev *llmScene, scene llmScene) {
		if scene.Dialogue != "" {
			if prev.Dialogue == "" {
				prev.Dialogue = scene.Dialogue
			} else {
				prev.Dialogue = strings.TrimSpace(prev.Dialogue + "\n" + scene.Dialogue)
			}
		}
		if scene.Description != "" {
			if prev.Description == "" {
				prev.Description = scene.Description
			} else {
				prev.Description = strings.TrimSpace(prev.Description + "；" + scene.Description)
			}
		}
		if prev.Location == "" {
			prev.Location = scene.Location
		}
		if prev.Mood == "" {
			prev.Mood = scene.Mood
		}
		prev.Duration = normalizeAdSceneDuration(prev.Duration, clipDuration)
		if len(prev.Characters) == 0 && len(scene.Characters) > 0 {
			prev.Characters = append(prev.Characters, scene.Characters...)
		}
		if len(prev.Items) == 0 && len(scene.Items) > 0 {
			prev.Items = append(prev.Items, scene.Items...)
		}
		if len(prev.CharacterStates) == 0 && len(scene.CharacterStates) > 0 {
			prev.CharacterStates = append(prev.CharacterStates, scene.CharacterStates...)
		}
	}
	hasStructuralShift := func(prev, current llmScene) bool {
		prevLocation := strings.TrimSpace(prev.Location)
		currentLocation := strings.TrimSpace(current.Location)
		if prevLocation != "" && currentLocation != "" && !strings.EqualFold(prevLocation, currentLocation) {
			return true
		}
		prevChars := strings.Join(prev.Characters, "|")
		currentChars := strings.Join(current.Characters, "|")
		if prevChars != "" && currentChars != "" && !strings.EqualFold(prevChars, currentChars) {
			return true
		}
		currentDesc := strings.ToLower(strings.TrimSpace(current.Description))
		for _, marker := range []string{"转场", "切到", "镜头切", "来到", "进入", "切换到", "场景切换"} {
			if strings.Contains(currentDesc, marker) {
				return true
			}
		}
		return false
	}
	for idx, scene := range scenes {
		scene.Dialogue = sanitizeStoryboardDialogue(scene.Dialogue)
		scene.Description = strings.TrimSpace(scene.Description)
		if scene.Duration <= 0 {
			scene.Duration = normalizeAdSceneDuration(scene.Duration, clipDuration)
		}
		if len(processed) == 0 {
			processed = append(processed, scene)
			continue
		}
		prev := &processed[len(processed)-1]
		dialogueLen := utf8.RuneCountInString(scene.Dialogue)
		isEmptyDialogue := scene.Dialogue == ""
		isShortDialogue := dialogueLen > 0 && dialogueLen < minDialogueRunes
		mustMerge := isEmptyDialogue
		isLastOriginalScene := idx == len(scenes)-1
		shouldMergeShort := isShortDialogue && !isLastOriginalScene && !hasStructuralShift(*prev, scene)
		if mustMerge || shouldMergeShort {
			mergeIntoPrev(prev, scene)
			continue
		}
		processed = append(processed, scene)
	}
	if len(processed) > 1 {
		firstDialogue := strings.TrimSpace(processed[0].Dialogue)
		secondDialogue := strings.TrimSpace(processed[1].Dialogue)
		if firstDialogue != "" && utf8.RuneCountInString(firstDialogue) < minLeadDialogueRunes && secondDialogue != "" && !hasStructuralShift(processed[0], processed[1]) {
			mergeIntoPrev(&processed[1], processed[0])
			processed = processed[1:]
		}
	}
	if len(processed) > 2 {
		collapsed := make([]llmScene, 0, len(processed))
		for i := 0; i < len(processed); i++ {
			current := processed[i]
			currentDialogue := strings.TrimSpace(current.Dialogue)
			if i > 0 && i < len(processed)-1 && currentDialogue != "" && utf8.RuneCountInString(currentDialogue) < minDialogueRunes {
				next := processed[i+1]
				if !hasStructuralShift(current, next) {
					mergeIntoPrev(&next, current)
					processed[i+1] = next
					continue
				}
			}
			collapsed = append(collapsed, current)
		}
		processed = collapsed
	}
	if len(processed) > 1 {
		balanced := make([]llmScene, 0, len(processed))
		for i := 0; i < len(processed); i++ {
			current := processed[i]
			currentDialogue := strings.TrimSpace(current.Dialogue)
			if i > 0 && i < len(processed)-1 && currentDialogue != "" && utf8.RuneCountInString(currentDialogue) < minLeadDialogueRunes {
				next := processed[i+1]
				nextDialogue := strings.TrimSpace(next.Dialogue)
				if nextDialogue != "" && utf8.RuneCountInString(nextDialogue) >= longDialogueRunes && !hasStructuralShift(current, next) {
					mergeIntoPrev(&next, current)
					processed[i+1] = next
					continue
				}
			}
			balanced = append(balanced, current)
		}
		processed = balanced
	}
	if len(processed) > 1 {
		last := processed[len(processed)-1]
		lastDialogue := strings.TrimSpace(last.Dialogue)
		prev := &processed[len(processed)-2]
		if lastDialogue == "" {
			mergeIntoPrev(prev, last)
			processed = processed[:len(processed)-1]
		} else if utf8.RuneCountInString(lastDialogue) < minDialogueRunes && !hasStructuralShift(*prev, last) {
			mergeIntoPrev(prev, last)
			processed = processed[:len(processed)-1]
		}
	}
	for i := range processed {
		processed[i].Duration = normalizeAdSceneDuration(processed[i].Duration, clipDuration)
		refitSceneDialogue(&processed[i], clipDuration, speechPace, false)
	}
	return processed
}

// refineScenePrompts calls LLM to produce cohesive, skill-injected image generation prompts
// for all scenes of a single episode. Scenes are processed in batches; the last prompt of
// each batch is passed as context to the next batch for visual continuity.
// kwLib provides character/location visual profiles for cross-scene consistency.
// After generation, all prompts are audited: sensitive words replaced, near-duplicates
// diversified, and flagged prompts rewritten by an LLM reviewer.
// Returns a slice of the same length as scenes; empty strings mean caller should fallback.
func (s *EpisodeService) refineScenePrompts(ctx context.Context, scenes []llmScene, skillHints string, promptTemplate string, kwLib *KeywordLibrary, episodeNum int, projectType string, prevEpisodeContext string, stylePreset string) []string {
	if len(scenes) == 0 {
		return nil
	}
	const maxBatch = 25
	results := make([]string, len(scenes))
	// Seed with the last prompt from the previous episode for cross-episode visual continuity.
	prevPrompt := prevEpisodeContext

	for start := 0; start < len(scenes); start += maxBatch {
		end := start + maxBatch
		if end > len(scenes) {
			end = len(scenes)
		}
		batch := scenes[start:end]
		prompts := s.refineScenePromptsBatch(ctx, batch, skillHints, promptTemplate, kwLib, episodeNum, start, prevPrompt, projectType, stylePreset)
		if len(prompts) == len(batch) {
			copy(results[start:end], prompts)
			prevPrompt = prompts[len(prompts)-1]
		}
		// If batch refinement failed, results[start:end] remain empty → caller falls back
	}

	// Audit all generated prompts: sensitive word scan → dedup → LLM reviewer.
	if s.auditor != nil {
		audited := s.auditor.AuditBatch(ctx, results, "image")
		for _, a := range audited {
			if a.Final != "" {
				results[a.Index] = a.Final
			}
		}
	}
	return results
}

// refineScenePromptsBatch makes a single LLM call to produce optimized image prompts
// for one batch of scenes, ensuring visual continuity from the previous batch.
// kwLib injects character appearance descriptions and location profiles as a consistency bible.
func (s *EpisodeService) refineScenePromptsBatch(ctx context.Context, scenes []llmScene, skillHints string, promptTemplate string, kwLib *KeywordLibrary, episodeNum int, offset int, prevContext string, projectType string, stylePreset string) []string {
	// Build per-character appearance lookup (prefer English for image generation).
	charAppearance := map[string]string{}
	// Build per-location visual profile lookup (prefer English).
	locDescription := map[string]string{}
	// Build per-prop visual profile lookup.
	propDescMap := map[string]string{}
	if kwLib != nil {
		for _, cp := range kwLib.CharacterProfiles {
			if cp.Name == "" {
				continue
			}
			if cp.AppearanceEN != "" {
				charAppearance[cp.Name] = cp.AppearanceEN
			} else if cp.Appearance != "" {
				charAppearance[cp.Name] = cp.Appearance // LLM will translate if needed
			}
		}
		for _, lp := range kwLib.LocationProfiles {
			if lp.Name == "" {
				continue
			}
			if lp.DescriptionEN != "" {
				locDescription[lp.Name] = lp.DescriptionEN
			} else if lp.Description != "" {
				locDescription[lp.Name] = lp.Description
			}
		}
		for _, pp := range kwLib.PropProfiles {
			if pp.Name != "" && pp.Description != "" {
				propDescMap[pp.Name] = pp.Description
			}
		}
	}

	type sceneInput struct {
		Index               int    `json:"index"`
		Description         string `json:"description"`
		Mood                string `json:"mood"`
		Location            string `json:"location"`
		LocationDescription string `json:"location_description,omitempty"` // visual profile of the scene location from kwLib
		ShotType            string `json:"shot_type"`
		Characters          string `json:"characters"`
		CharacterAppearance string `json:"character_appearance,omitempty"` // injected from kwLib (prefer English)
		CharacterEmotions   string `json:"character_emotions,omitempty"`   // emotion states per character
		Action              string `json:"action"`
		Items               string `json:"items,omitempty"`             // visible props/objects in scene
		PropVisual          string `json:"prop_visual,omitempty"`       // visual descriptions of key props from kwLib
		LightingNote        string `json:"lighting_note,omitempty"`     // from [灯光:] annotations
		ArtNote             string `json:"art_note,omitempty"`          // from [美术:] annotations
		PropNote            string `json:"prop_note,omitempty"`         // from [道具:] annotations
		SpatialAnchor       string `json:"spatial_anchor,omitempty"`    // extracted spatial anchor hints from scene description
		SubjectPositions    string `json:"subject_positions,omitempty"` // extracted left/center/right or facing hints
		TransitionNote      string `json:"transition_note,omitempty"`   // extracted visible transition / movement hints
	}

	var inputs []sceneInput
	for i, sc := range scenes {
		chars := strings.Join(sc.Characters, ", ")
		var actions []string
		var emotions []string
		for _, cs := range sc.CharacterStates {
			if cs.Name != "" && cs.Action != "" {
				actions = append(actions, cs.Name+": "+cs.Action)
			}
			if cs.Name != "" && cs.Emotion != "" {
				emotions = append(emotions, cs.Name+": "+cs.Emotion)
			}
		}
		// Inject character appearance (fuzzy name matching so "李明总裁" matches profile "李明").
		var appearances []string
		for _, name := range sc.Characters {
			if app := lookupByFuzzyName(name, charAppearance); app != "" {
				appearances = append(appearances, name+": "+app)
			}
		}
		// Inject location visual profile for this scene (fuzzy match on location field).
		locDesc := lookupByFuzzyName(strings.TrimSpace(sc.Location), locDescription)

		// Extract production annotation hints from scene description.
		lightingNotes := extractAnnotationsFromText(sc.Description, "灯光")
		artNotes := extractAnnotationsFromText(sc.Description, "美术")
		propNotes := extractAnnotationsFromText(sc.Description, "道具")
		spatialAnchor := extractSpatialAnchorHint(sc.Description)
		subjectPositions := extractSubjectPositionHint(sc.Description)
		transitionNote := extractTransitionHint(sc.Description)
		// Merge items from llmScene.Items + [道具:] annotations into a single props string.
		allItems := append([]string{}, sc.Items...)
		allItems = append(allItems, propNotes...)
		// Deduplicate inline.
		seenItems := map[string]struct{}{}
		var dedupItems []string
		for _, it := range allItems {
			if it = strings.TrimSpace(it); it != "" {
				if _, seen := seenItems[it]; !seen {
					seenItems[it] = struct{}{}
					dedupItems = append(dedupItems, it)
				}
			}
		}
		// Inject prop visual profiles for items appearing in this scene (fuzzy match).
		var propVisuals []string
		for _, item := range dedupItems {
			if desc := lookupByFuzzyName(item, propDescMap); desc != "" {
				propVisuals = append(propVisuals, item+": "+desc)
			}
		}

		inputs = append(inputs, sceneInput{
			Index:               offset + i + 1,
			Description:         sc.Description,
			Mood:                sc.Mood,
			Location:            sc.Location,
			LocationDescription: locDesc,
			ShotType:            sc.ShotType,
			Characters:          chars,
			CharacterAppearance: strings.Join(appearances, " | "),
			CharacterEmotions:   strings.Join(emotions, "; "),
			Action:              strings.Join(actions, "; "),
			Items:               strings.Join(dedupItems, ", "),
			PropVisual:          strings.Join(propVisuals, " | "),
			LightingNote:        strings.Join(lightingNotes, "; "),
			ArtNote:             strings.Join(artNotes, "; "),
			PropNote:            strings.Join(propNotes, "; "),
			SpatialAnchor:       spatialAnchor,
			SubjectPositions:    subjectPositions,
			TransitionNote:      transitionNote,
		})
	}

	inputJSON, _ := json.Marshal(inputs)

	continuityNote := ""
	if prevContext != "" {
		continuityNote = fmt.Sprintf("\n\n**VISUAL BRIDGE — last scene's prompt (your first scene MUST visually continue from this):**\n%s", prevContext)
	}
	templateNote := ""
	if promptTemplate != "" {
		templateNote = fmt.Sprintf("\n\nStyle template (use as visual style reference, not as fill-in template):\n%s", promptTemplate)
	}

	var systemPrompt string
	if projectType == "comics" {
		systemPrompt = `You are a professional manga/comics image generation prompt engineer specializing in comic panel art direction.
Your task: produce polished, optimized image generation prompts for a sequence of comic panels.

━━━━━━━━━━━━━━━━━━━━━━━━
PROMPT STRUCTURE — every panel prompt must follow this 5-layer order:
━━━━━━━━━━━━━━━━━━━━━━━━
① Subject anchor: character(s) name/role + exact position in panel (left/center/right) + posture
② Facial expression: specific muscle-level descriptor (furrowed brow, jaw clenched, wide eyes, etc.)
③ Action beat: what the character is physically doing (verb + result)
④ Environment layer: foreground prop + midground set + background atmosphere (3 depth planes)
⑤ Style & lighting: lighting direction/quality + ink style keywords

Rules:
1. Each prompt must be 60-200 words, entirely in English, for AI image generators (Stable Diffusion / Flux / DALL-E).
2. Use manga/comics art direction language: panel composition, ink line art, bold outlines, screen tone shading, dynamic action lines, chibi/realistic/stylized as context demands.
3. When a scene has "character_appearance" data, embed those EXACT visual descriptors (hair, clothing, face) — do NOT invent different appearances.
4. When a scene has "character_emotions" data, reflect those emotional states in specific facial muscle descriptors and body language (e.g., "lips pressed thin, eyes narrowed" not "angry expression").
5. When a scene has "location_description" data, use that EXACT environment description as the panel background — do NOT invent different scenery.
6. When a scene has "items" or "prop_visual" data, ensure those specific props/objects are clearly visible in the foreground or midground with their exact described appearance.
7. Maintain VISUAL CONTINUITY: consistent character design, matching environment, smooth mood transitions between adjacent panels.
8. Do NOT reference dialogue or story plot — only describe what is VISIBLE in the static panel image.
9. Use static composition language only: NO camera motion, NO panning, NO dolly.
10. The "shot_type" hints at panel framing: face-closeup → tight on face filling 80% panel; bust → waist-up; full-body → full figure with environment; wide → environment-dominant; establishing → location reveal; insert → detail close-up.
11. If "lighting_note" is present, translate it to static panel lighting (e.g., "rim light" → "strong rim highlight on left side, hair backlit, face in shadow").
12. If "art_note" is present, use it for background and set details.
13. Append manga style keywords: "manga style, ink line art, high contrast black and white, screen tone, comic panel border, expressive character design".
14. PANEL CONTINUITY: if adjacent panels share the same location, preserve costume, hairstyle, prop placement, lighting direction, and left/right subject placement unless the input explicitly changes them.
15. ONE DECISIVE PANEL BEAT: each prompt must focus on one dominant visual action or emotion, not two competing panel ideas.
16. Return ONLY a JSON object: {"prompts": ["prompt for panel 1", "prompt for panel 2", ...]}`
	} else {
		systemPrompt = `You are a professional image generation prompt engineer specializing in AI-driven video storyboards.
Your task: produce polished, optimized image generation prompts for a sequence of storyboard scenes that will be used BOTH as reference images AND as video generation seeds.

━━━━━━━━━━━━━━━━━━━━━━━━
PROMPT STRUCTURE — every scene prompt should follow this order (comma-separated):
━━━━━━━━━━━━━━━━━━━━━━━━
① Subject anchor: character(s) + posture/stance + wardrobe silhouette
② Face & expression: specific muscle-level descriptor (e.g., "jaw slightly dropped, eyebrows raised" — NOT "surprised face")
③ Action beat: one clear physical action with visible result
④ Framing: shot type + simple lens/framing cue from shot_type (do NOT invent left/right screen direction unless provided in input)
⑤ Environment: setting, atmosphere, key props, readable background
⑥ Light & style: key light direction + color temperature + grade keywords

━━━━━━━━━━━━━━━━━━━━━━━━
Rules:
━━━━━━━━━━━━━━━━━━━━━━━━
1. Each prompt must be 60-160 words, entirely in English, for AI image generators (Stable Diffusion / Flux / DALL-E).
2. Prioritize one decisive visual beat per frame. Favor readable action and emotion over blocking jargon.
3. When a scene has "character_appearance" data, embed those EXACT visual descriptors — NEVER invent different appearances.
4. When a scene has "character_emotions" data, translate to micro-expression descriptors, not emotion adjectives alone.
5. When a scene has "location_description" data, use that EXACT environment description as the scene background.
6. When a scene has "items" or "prop_visual" data, make those props clearly visible.
7. Use "spatial_anchor", "subject_positions", or "transition_note" ONLY when the input already contains them. Do NOT invent screen-left/right placement, axis language, or furniture geography.
8. If the source description is under-specified, infer a usable pose and hand behavior, but avoid camera-operator jargon.
9. VISUAL CONTINUITY (lightweight):
   - Adjacent scenes in the same location should keep wardrobe, hairstyle, lighting color, and major set dressing stable.
   - If VISUAL BRIDGE is provided, the first scene should extend its color grading and general staging, not copy blocking coordinates verbatim.
   - Preserve eyeline and held props only when the input already established them.
10. Do NOT reference dialogue, narration, or story plot — only describe what is VISIBLE.
11. The "shot_type" field dictates framing: close-up → face dominant; medium → waist-up; wide → figure + environment; establishing → location dominant.
12. Preserve explicit era / period / costume cues.
13. VIDEO-FRIENDLY COMPOSITION: clean subject-background separation, uncluttered frame, one hero action.
14. ACTION CONTINUITY: adjacent scenes in one action chain should progress pose naturally, without teleporting between unrelated poses.
15. ONE DECISIVE VISUAL IDEA PER FRAME.
16. Return ONLY a JSON object: {"prompts": ["prompt for scene 1", "prompt for scene 2", ...]}`
	}

	if styleBlock := productionmode.RefinePromptStyleBlock(stylePreset); styleBlock != "" {
		systemPrompt += "\n\n" + styleBlock
	}
	if skillHints != "" {
		systemPrompt += "\n\n**Project art style and visual skill guidelines (MUST follow):**\n" + skillHints
	}
	// Inject global location profiles as a grounding reference (also injected per-scene via location_description).
	if kwLib != nil && len(kwLib.LocationProfiles) > 0 {
		systemPrompt += "\n\n**Location visual references — these MUST match the location_description field in each scene:**"
		for _, lp := range kwLib.LocationProfiles {
			if lp.Name == "" {
				continue
			}
			desc := lp.DescriptionEN
			if desc == "" {
				desc = lp.Description
			}
			if desc != "" {
				systemPrompt += fmt.Sprintf("\n- %s: %s", lp.Name, desc)
			}
		}
	}
	if eraHint := inferVisualEra(scenesToTextForEraInference(scenes)); eraHint != "" {
		systemPrompt += "\n\n**Project era and styling anchor (must preserve):**\n" + eraHint
	}

	userContent := fmt.Sprintf(
		"Generate optimized image generation prompts for %d storyboard scenes from episode %d.%s%s\n\nIMPORTANT: The scenes are ordered. Keep wardrobe, lighting, and major set dressing consistent across adjacent scenes in the same location. Bridge action and emotion naturally, but do not add left/right blocking or camera-axis jargon unless the scene JSON already contains it.\n\nScenes (JSON):\n%s\n\nReturn JSON: {\"prompts\": [\"prompt1\", \"prompt2\", ...]}",
		len(scenes), episodeNum, continuityNote, templateNote, string(inputJSON),
	)

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userContent},
		},
		"temperature":     0.4,
		"max_tokens":      8192,
		"response_format": map[string]string{"type": "json_object"},
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("refine prompts: create request failed", zap.Int("episode", episodeNum), zap.Error(err))
		}
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("refine prompts: LLM call failed", zap.Int("episode", episodeNum), zap.Error(err))
		}
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		if s.logger != nil {
			s.logger.Warn("refine prompts: LLM error",
				zap.Int("episode", episodeNum),
				zap.Int("status", resp.StatusCode),
				zap.String("body_preview", string(body[:min(300, len(body))])),
			)
		}
		return nil
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return nil
	}

	respContent := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	var result struct {
		Prompts []string `json:"prompts"`
	}
	if err := json.Unmarshal([]byte(respContent), &result); err != nil || len(result.Prompts) != len(scenes) {
		if s.logger != nil {
			s.logger.Warn("refine prompts: parse failed or count mismatch",
				zap.Int("episode", episodeNum),
				zap.Int("expected", len(scenes)),
				zap.Int("got", len(result.Prompts)),
				zap.String("preview", func() string {
					if len(respContent) > 200 {
						return respContent[:200]
					}
					return respContent
				}()),
			)
		}
		return nil
	}

	if s.logger != nil {
		s.logger.Info("scene prompts refined",
			zap.Int("episode", episodeNum),
			zap.Int("count", len(scenes)),
			zap.Int("offset", offset),
		)
	}
	return result.Prompts
}

// videoModelFamilyFromName returns the model family (kling/wan/vidu/doubao/suanneng) from a full model name.
func videoModelFamilyFromName(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "kling"):
		return "kling"
	case strings.Contains(lower, "wan") || strings.Contains(lower, "wanx"):
		return "wan"
	case strings.Contains(lower, "vidu"):
		return "vidu"
	case strings.Contains(lower, "doubao") || strings.Contains(lower, "seedance") || strings.Contains(lower, "seeedance"):
		return "doubao"
	case strings.Contains(lower, "suanneng") || strings.Contains(lower, "suan"):
		return "suanneng"
	default:
		return ""
	}
}

// videoModelStyleHint returns a concise line to inject in the storyboard LLM prompt,
// telling the writer which target video model will render these scenes so descriptions
// are tuned to the model's strengths.
func videoModelStyleHint(videoModel string) string {
	family := videoModelFamilyFromName(videoModel)
	switch family {
	case "kling":
		return "目标视频模型：Kling — 擅长流畅运动弧线与角色表演，分镜 description 可强调肢体动作、表情变化与镜头运动轨迹。"
	case "wan":
		return "目标视频模型：Wan — 擅长环境纵深与大气透视，分镜 description 可强调景深层次、远近景关系与环境氛围。"
	case "vidu":
		return "目标视频模型：Vidu — 擅长真实物理运动与节奏切换，分镜 description 可强调速度感、惯性与清晰方向。"
	case "doubao":
		return "目标视频模型：豆包/Seedance — 擅长面部表情与口型同步，分镜 description 可强调角色情绪与台词表演。"
	case "suanneng":
		return "目标视频模型：算能 — 画面锐利色彩饱和，分镜 description 可强调动作幅度与情节张力。"
	default:
		return ""
	}
}

// videoModelDurationHint returns the LLM prompt text describing valid duration values for the given video model.
// refDuration is the user-configured default and is used as fallback guidance when no specific model is set.
func visualConstraintHint(aspectRatio, resolution string, refDuration int) string {
	ar := strings.TrimSpace(aspectRatio)
	if ar == "" {
		ar = "未指定"
	}
	res := strings.TrimSpace(resolution)
	if res == "" {
		res = "未指定"
	}
	return fmt.Sprintf("  目标画面比例：%s。构图、主体数量、左右留白、前景/中景/后景层次必须适配该比例，不要沿用默认构图。\n  目标分辨率：%s。若分辨率较低，避免单镜塞入过多主体、小字或复杂微动作，优先保证主体清晰、卖点突出。\n  目标单分镜时长参考：%d 秒。单镜承载的信息量、口播长度、动作阶段数都必须服务于这个时长。", ar, res, refDuration)
}

func canonicalSpeechPace(raw string) string {
	pace := strings.TrimSpace(strings.ToLower(raw))
	switch pace {
	case "", "normal":
		return "normal"
	case "slightly_fast":
		return "slightly_fast"
	case "with_pauses":
		return "with_pauses"
	case "very_fast":
		return "very_fast"
	case "medium_fast":
		return "medium_fast"
	case "medium_steady":
		return "medium_steady"
	default:
		return "normal"
	}
}

func speechPaceHint(raw string, refDuration int) string {
	pace := canonicalSpeechPace(raw)
	duration := refDuration
	if duration <= 0 {
		duration = 10
	}
	type paceProfile struct {
		label         string
		charsPer10Sec int
		directive     string
	}
	profile := paceProfile{
		label:         "正常",
		charsPer10Sec: 48,
		directive:     "按标准商业口播节奏拆分，句子完整优先，避免为了凑镜头数生硬切断。",
	}
	switch pace {
	case "slightly_fast":
		profile = paceProfile{label: "稍快", charsPer10Sec: 56, directive: "允许信息密度略高，但仍要保留自然换气点；只有超过该承载量或卖点明显切换时再拆镜。"}
	case "with_pauses":
		profile = paceProfile{label: "有停顿", charsPer10Sec: 38, directive: "把停顿、强调、留白也算进时长；同样 10 秒内可承载的台词量更少，应更积极拆镜，避免一句话塞太满。"}
	case "very_fast":
		profile = paceProfile{label: "很快", charsPer10Sec: 66, directive: "允许高密度口播，但仍不能牺牲语义完整性；优先按卖点组块而不是机械按字数平均切。"}
	case "medium_fast":
		profile = paceProfile{label: "中速偏快", charsPer10Sec: 52, directive: "比正常稍紧凑，适合信息流广告；在保证顺口的前提下，可比正常档少拆一镜。"}
	case "medium_steady":
		profile = paceProfile{label: "中速稳重", charsPer10Sec: 42, directive: "节奏稳、句间更讲究停连；需要给强调和停顿留空间，宁可多一镜，也不要单镜负担过重。"}
	}
	targetChars := profile.charsPer10Sec * duration / 10
	if targetChars < 20 {
		targetChars = 20
	}
	return fmt.Sprintf("  当前语速档位：%s。按 %d 秒口播参考，这一档约可自然承载 %d 个中文字符（按 10 秒≈%d 字折算）。%s", profile.label, duration, targetChars, profile.charsPer10Sec, profile.directive)
}

func videoModelDurationHint(videoModel string, refDuration int) string {
	family := videoModelFamilyFromName(videoModel)
	switch family {
	case "kling":
		return "  只能填写 5 或 10（Kling 模型只支持这两个时长）\n  • 短动作/特写/快切 → 5秒；长对话/建立镜头/复杂场面 → 10秒"
	case "wan":
		return "  固定填写 5（Wan 模型只支持 5 秒时长）"
	case "vidu":
		return "  只能填写 4 或 8（Vidu 模型只支持这两个时长）\n  • 快切/动作/特写 → 4秒；标准对话/建立镜头/情感高潮 → 8秒"
	case "doubao", "suanneng":
		return "  只能填写 5、8 或 10（该模型只支持这三个时长）\n  • 快切/动作 → 5秒；标准场景/对话 → 8秒；长对话/建立镜头/复杂场面 → 10秒"
	default:
		return fmt.Sprintf("  根据场景复杂度智能估算（参考配置时长 %d 秒）：\n  • 快速反应/特写切换/纯动作：2-4秒\n  • 标准对话/过渡镜头：3-6秒\n  • 建立性场景/风景展示：4-7秒\n  • 长对话/情感高潮/复杂场面：6-12秒\n  • 不要强制统一时长，根据实际叙事节奏判断", refDuration)
	}
}

// splitIntoParagraphs splits text by double newlines or paragraph-like breaks.
func splitIntoParagraphs(text string) []string {
	// Split on double newlines first
	rawParts := regexp.MustCompile(`\n\s*\n`).Split(text, -1)

	var paragraphs []string
	for _, p := range rawParts {
		p = strings.TrimSpace(p)
		if utf8.RuneCountInString(p) >= 20 { // Skip very short fragments
			paragraphs = append(paragraphs, p)
		}
	}

	// If only 1 paragraph, try splitting on single newlines
	if len(paragraphs) <= 1 {
		rawParts = strings.Split(text, "\n")
		paragraphs = nil
		for _, p := range rawParts {
			p = strings.TrimSpace(p)
			if utf8.RuneCountInString(p) >= 20 {
				paragraphs = append(paragraphs, p)
			}
		}
	}

	return paragraphs
}

// mergeIntoGroups —— 将段落列表均匀分为 n 组
// mergeIntoGroups divides paragraphs into n roughly equal groups.
func mergeIntoGroups(paragraphs []string, n int) [][]string {
	if n <= 0 {
		n = 1
	}
	if n > len(paragraphs) {
		n = len(paragraphs)
	}

	groups := make([][]string, n)
	chunkSize := len(paragraphs) / n
	remainder := len(paragraphs) % n

	idx := 0
	for i := 0; i < n; i++ {
		size := chunkSize
		if i < remainder {
			size++
		}
		groups[i] = paragraphs[idx : idx+size]
		idx += size
	}
	return groups
}

// callLLMSplit —— 调用 LLM 将剧本拆分为指定集数的剧集
func (s *EpisodeService) callLLMSplit(ctx context.Context, project *model.Project, scriptText string, targetEpisodes int, kwLib *KeywordLibrary, writingHints string, profile productionmode.Profile) ([]llmEpisode, error) {
	// For very large episode counts, split via multiple LLM calls in batches
	if targetEpisodes > 30 {
		return s.callLLMSplitBatched(ctx, project, scriptText, targetEpisodes, kwLib, writingHints, profile)
	}

	// Truncate very long scripts for the prompt
	maxChars := 50000
	truncated := scriptText
	if utf8.RuneCountInString(truncated) > maxChars {
		runes := []rune(truncated)
		truncated = string(runes[:maxChars]) + "\n...(truncated)"
	}

	// Build keyword context block
	kwContext := buildKeywordContextBlock(kwLib)

	prompt := fmt.Sprintf(`你是一位专业的影视剧本分析师，擅长将长篇剧本/小说精准拆分为集数。

请将以下剧本内容拆分为 %d 集（episodes）。

%s
**拆分规则：**
- 按照剧情的起承转合进行分集，每集应有完整的叙事弧（开端-发展-高潮/悬念）
- 如果原文有明显的章节/幕/段落分割标记，优先参考这些自然分界线
- 确保每集字数大致均匀，控制在总字数的 1/%d 左右浮动
- 分集时注意关键词库中的人物/地点，确保情节连贯
- 禁止把【简介】/全书概括/结局剧透单独拆成第 1 集；第 1 集必须从具体叙事场景或第一章正文开始
- 有现场感的【导语】应并入第 1 集正文，不要单独成集

**输出要求：**
对每一集，请提供：
1. title: 集的标题（简短精炼，5-10字，概括本集核心事件）
2. summary: 该集的详细剧情摘要（150-300字，涵盖主要角色、行动、情感变化和情节转折）
3. keywords: 本集出现的关键词列表（从关键词库中选取，包括人物名、地点名、重要事件，最多15个）
4. start_text: 该集在原文中**起始位置**的前20个字（必须是原文的精确文字）
5. end_text: 该集在原文中**结束位置**的最后20个字（必须是原文的精确文字）

请严格按以下 JSON 格式返回：
{"episodes": [
  {"title": "标题", "summary": "详细摘要", "keywords": ["关键词1","关键词2"], "start_text": "起始20字", "end_text": "结束20字"}
]}

剧本内容：
%s`, targetEpisodes, kwContext, targetEpisodes, truncated)

	systemPrompt := "你是剧本分析助手，只输出JSON，不要输出其他内容。"
	if directive := productionmode.EpisodeSplitDirective(profile.Mode); directive != "" {
		systemPrompt += "\n\n**分集模式规则（强制遵守）：**\n" + directive
	}
	if writingHints != "" {
		systemPrompt += "\n\n**本项目专属写作指引（分集时请遵守）：**\n" + writingHints
	}
	// Inject consistency bible so LLM-generated episode summaries reference characters consistently.
	if bible := buildConsistencyBibleBlock(kwLib); bible != "" {
		systemPrompt += bible
	}

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": prompt},
		},
		"temperature":     0.3,
		"max_tokens":      8192,
		"response_format": map[string]string{"type": "json_object"},
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("llm status %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	// Parse OpenAI-compatible response
	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil {
		return nil, fmt.Errorf("parse llm response: %w", err)
	}
	if len(llmResp.Choices) == 0 {
		return nil, errors.New("llm returned no choices")
	}

	content := strings.TrimSpace(llmResp.Choices[0].Message.Content)

	// Parse as object with "episodes" key (matches prompt format)
	var wrapper struct {
		Episodes []llmEpisode `json:"episodes"`
	}
	if err := json.Unmarshal([]byte(content), &wrapper); err != nil {
		// Fallback: try as bare array
		var episodes []llmEpisode
		if err2 := json.Unmarshal([]byte(content), &episodes); err2 != nil {
			return nil, fmt.Errorf("parse episodes json: %w (content: %s)", err, content[:min(len(content), 300)])
		}
		wrapper.Episodes = episodes
	}

	if len(wrapper.Episodes) == 0 {
		return nil, errors.New("llm returned empty episodes")
	}

	// Fill in Excerpt by locating start_text/end_text in the original script
	s.fillExcerptsFromBoundaries(scriptText, wrapper.Episodes)

	return wrapper.Episodes, nil
}

// callLLMSplitBatched —— 大集数场景下先按比例切分文本再并行调用 LLM 丰富摘要
// callLLMSplitBatched handles large episode counts (>30) by first using simpleSplit
// to create text segments, then enriching each segment with an LLM-generated
// title, summary and keywords in parallel batches.
func (s *EpisodeService) callLLMSplitBatched(ctx context.Context, project *model.Project, scriptText string, targetEpisodes int, kwLib *KeywordLibrary, writingHints string, profile productionmode.Profile) ([]llmEpisode, error) {
	if s.logger != nil {
		s.logger.Info("using batched split for large episode count",
			zap.Int("target", targetEpisodes))
	}

	// Start with a simple proportional split to get text segments
	segments := s.simpleSplit(scriptText, targetEpisodes, profile)

	// Enrich each segment with LLM-generated title+summary+keywords in batches
	const batchSize = 10
	const workers = 3

	type enrichResult struct {
		idx      int
		title    string
		summary  string
		keywords []string
	}

	for batchStart := 0; batchStart < len(segments); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(segments) {
			batchEnd = len(segments)
		}
		batch := segments[batchStart:batchEnd]

		results := make(chan enrichResult, len(batch))
		sem := make(chan struct{}, workers)

		for i, seg := range batch {
			select {
			case <-ctx.Done():
				return segments, nil // Return what we have
			case sem <- struct{}{}:
			}

			go func(idx int, ep llmEpisode) {
				defer func() { <-sem }()
				title, summary, kws := s.enrichEpisodeWithLLM(ctx, project, ep.Title, ep.Excerpt, idx+1, kwLib, writingHints)
				results <- enrichResult{idx: idx, title: title, summary: summary, keywords: kws}
			}(batchStart+i, seg)
		}

		// Collect results
		for range batch {
			r := <-results
			if r.title != "" {
				segments[r.idx].Title = r.title
			}
			if r.summary != "" {
				segments[r.idx].Summary = r.summary
			}
			if len(r.keywords) > 0 {
				segments[r.idx].Keywords = r.keywords
			}
		}
	}

	return segments, nil
}

// enrichEpisodesParallel —— 并行调用 LLM 为章节拆分的剧集生成标题、摘要和关键词
// enrichEpisodesParallel enriches a slice of chapter-split episodes with LLM-generated
// summaries and keyword tags, using up to 5 parallel workers.
// writingHints is the concatenated text of active writing skills (may be empty).
func (s *EpisodeService) enrichEpisodesParallel(ctx context.Context, project *model.Project, episodes []llmEpisode, kwLib *KeywordLibrary, writingHints string) {
	const workers = 5
	const batchSize = 10

	type enrichResult struct {
		idx      int
		title    string
		summary  string
		keywords []string
	}

	for batchStart := 0; batchStart < len(episodes); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(episodes) {
			batchEnd = len(episodes)
		}
		batch := episodes[batchStart:batchEnd]

		results := make(chan enrichResult, len(batch))
		sem := make(chan struct{}, workers)

		for i, seg := range batch {
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			go func(idx int, ep llmEpisode) {
				defer func() { <-sem }()
				title, summary, kws := s.enrichEpisodeWithLLM(ctx, project, ep.Title, ep.Excerpt, idx+1, kwLib, writingHints)
				results <- enrichResult{idx: idx, title: title, summary: summary, keywords: kws}
			}(batchStart+i, seg)
		}

		for range batch {
			r := <-results
			if r.title != "" {
				episodes[r.idx].Title = r.title
			}
			if r.summary != "" {
				episodes[r.idx].Summary = r.summary
			}
			if len(r.keywords) > 0 {
				episodes[r.idx].Keywords = r.keywords
			}
		}
	}
}

// enrichEpisodeWithLLM —— 调用 LLM 为单集生成标题、摘要和关键词标签
// enrichEpisodeWithLLM calls LLM to generate title, summary, and keywords for one episode.
// chapterTitle is the original chapter heading (may be empty for non-chapter splits).
// writingHints is the concatenated text of active writing skills for this project (may be empty).
func (s *EpisodeService) enrichEpisodeWithLLM(ctx context.Context, project *model.Project, chapterTitle, excerpt string, episodeNum int, kwLib *KeywordLibrary, writingHints string) (string, string, []string) {
	maxChars := 5000
	truncated := excerpt
	if utf8.RuneCountInString(truncated) > maxChars {
		runes := []rune(truncated)
		truncated = string(runes[:maxChars])
	}

	kwContext := buildKeywordContextBlock(kwLib)

	titleHint := ""
	if chapterTitle != "" {
		titleHint = fmt.Sprintf("（原章节标题：%s）", chapterTitle)
	}

	prompt := fmt.Sprintf(`请分析以下第 %d 集的内容，生成标题、摘要和关键词。%s

%s
要求：
- title: 5-15字的标题，概括本集核心事件（可参考原章节标题）
- summary: 200-400字的详细剧情摘要，涵盖主要人物行动、冲突、情感变化和情节转折
- keywords: 本集出现的关键词，从上方关键词库中选取相关词汇，最多15个

请严格按 JSON 格式返回：
{"title": "标题", "summary": "摘要", "keywords": ["词1","词2"]}

内容：
%s`, episodeNum, titleHint, kwContext, truncated)

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": func() string {
				sys := "你是影视剧本分析专家，只输出JSON，不要输出其他内容。"
				if directive := productionmode.EpisodeEnrichDirective(productionmode.Resolve(project)); directive != "" {
					sys += "\n\n**分集摘要模式规则（强制遵守）：**\n" + directive
				}
				if writingHints != "" {
					sys += "\n\n**本项目专属写作指引（生成摘要时请遵守）：**\n" + writingHints
				}
				if bible := buildConsistencyBibleBlock(kwLib); bible != "" {
					sys += bible
				}
				return sys
			}()},
			{"role": "user", "content": prompt},
		},
		"temperature":     0.3,
		"max_tokens":      2048,
		"response_format": map[string]string{"type": "json_object"},
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", "", nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", nil
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return "", "", nil
	}

	var result struct {
		Title    string   `json:"title"`
		Summary  string   `json:"summary"`
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(llmResp.Choices[0].Message.Content), &result); err != nil {
		return "", "", nil
	}
	return result.Title, result.Summary, result.Keywords
}

// extractKeywordLibrary —— 调用 LLM 从剧本中提取人物、地点、事件、道具关键词库
// extractKeywordLibrary uses LLM to build a keyword glossary from the script.
// It reads the first ~30000 chars to extract characters, locations, events, and props.
func (s *EpisodeService) extractKeywordLibrary(ctx context.Context, scriptText string) KeywordLibrary {
	maxChars := 30000
	sample := scriptText
	if utf8.RuneCountInString(sample) > maxChars {
		runes := []rune(sample)
		sample = string(runes[:maxChars])
	}

	prompt := fmt.Sprintf(`请从以下剧本/小说内容中提取关键词库，分为四类：人物、地点、重要事件/概念、重要道具。

要求：
- characters（人物）：所有出现的人名、角色名，包括别称（如"孙悟空"、"美猴王"、"齐天大圣"视为同一人，保留最常用名）
- locations（地点）：所有地名、场所名
- events（事件/概念）：故事中的重要事件名、特殊称谓、专有名词（如"取经"、"蟠桃会"）
- props（道具）：重要的物品、法宝、武器等

请严格按 JSON 格式返回（每类最多50个）：
{"characters":["..."],"locations":["..."],"events":["..."],"props":["..."]}

剧本内容（节选）：
%s`, sample)

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": "你是剧本分析专家，只输出JSON，不要输出其他内容。"},
			{"role": "user", "content": prompt},
		},
		"temperature":     0.2,
		"max_tokens":      4096,
		"response_format": map[string]string{"type": "json_object"},
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return KeywordLibrary{}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return KeywordLibrary{}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		if s.logger != nil {
			s.logger.Warn("keyword extraction failed", zap.Int("status", resp.StatusCode))
		}
		return KeywordLibrary{}
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return KeywordLibrary{}
	}

	var lib KeywordLibrary
	if err := json.Unmarshal([]byte(llmResp.Choices[0].Message.Content), &lib); err != nil {
		return KeywordLibrary{}
	}
	return lib
}

func buildKeywordContextBlock(kwLib *KeywordLibrary) string {
	if kwLib == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("**关键词库（用于分析内容关联性）：**\n")
	if len(kwLib.Characters) > 0 {
		sb.WriteString("- 人物：" + strings.Join(kwLib.Characters, "、") + "\n")
	}
	if len(kwLib.Locations) > 0 {
		sb.WriteString("- 地点：" + strings.Join(kwLib.Locations, "、") + "\n")
	}
	if len(kwLib.Events) > 0 {
		sb.WriteString("- 事件/概念：" + strings.Join(kwLib.Events, "、") + "\n")
	}
	if len(kwLib.Props) > 0 {
		sb.WriteString("- 道具：" + strings.Join(kwLib.Props, "、") + "\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

// enrichKeywordLibraryWithProfiles —— 使用 LLM 为关键词库中的人物/地点/道具生成视觉描述
// It enriches lib in-place with CharacterProfiles, LocationProfiles, PropProfiles.
// scriptSample should be the first ~10k characters of the script for context.
func (s *EpisodeService) enrichKeywordLibraryWithProfiles(ctx context.Context, lib *KeywordLibrary, scriptSample string) {
	chars := lib.Characters
	if len(chars) > 20 {
		chars = chars[:20]
	}
	locs := lib.Locations
	if len(locs) > 20 {
		locs = locs[:20]
	}
	props := lib.Props
	if len(props) > 15 {
		props = props[:15]
	}
	if len(chars) == 0 && len(locs) == 0 && len(props) == 0 {
		return
	}

	// Limit script sample for prompt efficiency.
	// 15000 chars covers more of a long script so late-appearing characters/locations are captured.
	const maxSample = 15000
	if utf8.RuneCountInString(scriptSample) > maxSample {
		scriptSample = string([]rune(scriptSample)[:maxSample])
	}

	prompt := fmt.Sprintf(`根据以下剧本内容，为每个实体生成详细的视觉描述，用于 AI 图像生成时保持跨集、跨场景的视觉一致性。

**要求：**
- character_profiles（人物外貌）：
  - appearance（中文）：性别、年龄段、发型发色、服装颜色与款式、体型、肤色、面部特征，50-120字
  - appearance_en（英文）：同等内容的英文描述，50-120 words，用于 AI 图像生成（Stable Diffusion / Flux / DALL-E）。请详细描述 hair color, hair style, face features, skin tone, clothing color and style, body build。
  - voice_hint 填写：male（男性成人）/ female（女性成人）/ child（儿童）/ narrator（旁白/内心独白）
  - skill_hints：根据剧本中该角色的行为特征，从以下选项中选择适用的能力标签（可多选，不适用时填空数组）：
    "combat"（战斗/打斗/武功）、"exploration"（探索/侦探/冒险）、"social"（外交/情感/领导）、"special"（魔法/超能力/特殊技能）
- location_profiles（场景环境）：
  - description（中文）：建筑风格、光线色温、主色调、时代背景、标志性元素，30-80字
  - description_en（英文）：同等内容的英文描述，30-80 words，用于 AI 图像生成，包含 architectural style, lighting, color palette, era, distinctive visual elements。
- prop_profiles（重要道具）：形状、颜色、材质、独特特征，20-60字，语言：中文

人物列表：%s
地点列表：%s
道具列表：%s

请严格按以下 JSON 格式返回（只输出 JSON，不输出任何其他内容）：
{
  "character_profiles": [{"name":"人物名","appearance":"中文外貌描述","appearance_en":"English appearance description","voice_hint":"male/female/child/narrator","skill_hints":["combat","social"]}],
  "location_profiles": [{"name":"地点名","description":"中文环境描述","description_en":"English environment description"}],
  "prop_profiles": [{"name":"道具名","description":"外观描述"}]
}

剧本内容（节选，用于参考）：
%s`,
		strings.Join(chars, "、"),
		strings.Join(locs, "、"),
		strings.Join(props, "、"),
		scriptSample,
	)

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": "你是视觉一致性分析专家，只输出JSON，不要输出任何其他内容。"},
			{"role": "user", "content": prompt},
		},
		"temperature":     0.2,
		"max_tokens":      4096,
		"response_format": map[string]string{"type": "json_object"},
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("enrich keyword profiles: LLM call failed", zap.Error(err))
		}
		return
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		if s.logger != nil {
			s.logger.Warn("enrich keyword profiles: LLM returned error",
				zap.Int("status", resp.StatusCode),
				zap.String("body_preview", string(body[:min(200, len(body))])),
			)
		}
		return
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return
	}

	var profiles struct {
		CharacterProfiles []CharacterProfile `json:"character_profiles"`
		LocationProfiles  []LocationProfile  `json:"location_profiles"`
		PropProfiles      []PropProfile      `json:"prop_profiles"`
	}
	if err := json.Unmarshal([]byte(llmResp.Choices[0].Message.Content), &profiles); err != nil {
		if s.logger != nil {
			s.logger.Warn("parse keyword profiles failed", zap.Error(err))
		}
		return
	}

	if len(profiles.CharacterProfiles) > 0 {
		lib.CharacterProfiles = profiles.CharacterProfiles
	}
	if len(profiles.LocationProfiles) > 0 {
		lib.LocationProfiles = profiles.LocationProfiles
	}
	if len(profiles.PropProfiles) > 0 {
		lib.PropProfiles = profiles.PropProfiles
	}

	if s.logger != nil {
		s.logger.Info("keyword profiles enriched",
			zap.Int("characters", len(lib.CharacterProfiles)),
			zap.Int("locations", len(lib.LocationProfiles)),
			zap.Int("props", len(lib.PropProfiles)),
		)
	}
}

// autoCreateCharacterSkills —— T3C: 根据 LLM 提取的 SkillHints 在 character-service 中自动创建 Skill 记录
// Idempotent: fetches existing skill names first and skips duplicates.
func (s *EpisodeService) autoCreateCharacterSkills(ctx context.Context, projectID uint64, profiles []CharacterProfile) {
	if s.characterBaseURL == "" || s.jwtSecret == "" {
		return
	}
	token, err := s.buildServiceToken(projectID)
	if err != nil {
		return
	}

	// Fetch existing skill names once to avoid duplicates
	existingNames := make(map[string]struct{})
	existingURL := fmt.Sprintf("%s/api/v1/skills?project_id=%d&page_size=200", s.characterBaseURL, projectID)
	if listReq, err := http.NewRequestWithContext(ctx, http.MethodGet, existingURL, nil); err == nil {
		listReq.Header.Set("Authorization", "Bearer "+token)
		if listResp, err := s.httpClient.Do(listReq); err == nil {
			var listResult struct {
				Data struct {
					Items []struct {
						Name string `json:"name"`
					} `json:"items"`
				} `json:"data"`
			}
			if body, err := io.ReadAll(listResp.Body); err == nil {
				_ = json.Unmarshal(body, &listResult)
				for _, sk := range listResult.Data.Items {
					existingNames[sk.Name] = struct{}{}
				}
			}
			listResp.Body.Close()
		}
	}

	skillTypeLabels := map[string]string{
		"combat":      "战斗能力",
		"exploration": "探索能力",
		"social":      "社交能力",
		"special":     "特殊能力",
	}
	created := 0
	for _, p := range profiles {
		for _, hint := range p.SkillHints {
			label, ok := skillTypeLabels[hint]
			if !ok {
				continue
			}
			skillName := fmt.Sprintf("%s - %s", p.Name, label)
			if _, exists := existingNames[skillName]; exists {
				continue // skip duplicate
			}
			skill := map[string]interface{}{
				"project_id":  projectID,
				"name":        skillName,
				"skill_type":  hint,
				"use_case":    "storyboard",
				"description": fmt.Sprintf("自动提取自剧本：%s 具有 %s，在相关场景分镜中突出体现该角色的这一特质。", p.Name, label),
				"is_active":   true,
			}
			body, _ := json.Marshal(skill)
			createReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
				fmt.Sprintf("%s/api/v1/skills", s.characterBaseURL),
				bytes.NewReader(body))
			if err != nil {
				continue
			}
			createReq.Header.Set("Content-Type", "application/json")
			createReq.Header.Set("Authorization", "Bearer "+token)
			createResp, err := s.httpClient.Do(createReq)
			if err != nil {
				continue
			}
			io.Copy(io.Discard, createResp.Body) //nolint:errcheck
			createResp.Body.Close()
			existingNames[skillName] = struct{}{} // mark as created
			created++
		}
	}
	if s.logger != nil && created > 0 {
		s.logger.Info("auto-created character skills from profiles", zap.Int("count", created), zap.Uint64("project_id", projectID))
	}
}

func extractAnnotationsFromText(text, tag string) []string {
	re := regexp.MustCompile(`\[` + regexp.QuoteMeta(tag) + `:([^\]]+)\]`)
	matches := re.FindAllStringSubmatch(text, -1)
	var results []string
	for _, m := range matches {
		if len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			results = append(results, strings.TrimSpace(m[1]))
		}
	}
	return results
}

func extractSpatialAnchorHint(text string) string {
	keywords := []string{"门", "窗", "桌", "椅", "沙发", "床", "楼梯", "柜台", "车", "路口", "走廊", "墙边", "窗边", "门口", "前景", "后景", "远景", "背景"}
	return collectSceneHintByKeywords(text, keywords)
}

func extractSubjectPositionHint(text string) string {
	keywords := []string{"左侧", "右侧", "居中", "中央", "左前方", "右前方", "左后方", "右后方", "面朝", "背对", "对视", "侧身", "站在", "靠近"}
	return collectSceneHintByKeywords(text, keywords)
}

func extractTransitionHint(text string) string {
	keywords := []string{"走近", "后退", "转身", "绕过", "停下", "起身", "坐下", "推门", "回头", "移步", "迈步", "靠近", "离开", "穿过"}
	return collectSceneHintByKeywords(text, keywords)
}

func collectSceneHintByKeywords(text string, keywords []string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	segments := regexp.MustCompile(`[，。；！？」\n]`).Split(trimmed, -1)
	seen := map[string]struct{}{}
	var picks []string
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		for _, kw := range keywords {
			if strings.Contains(seg, kw) {
				if _, ok := seen[seg]; !ok {
					seen[seg] = struct{}{}
					picks = append(picks, seg)
				}
				break
			}
		}
		if len(picks) >= 3 {
			break
		}
	}
	return strings.Join(picks, " | ")
}

// lookupByFuzzyName looks up a value from a name→value map using fuzzy matching.
// Strategy: (1) exact match, (2) profile key is substring of query name, (3) query name is substring of key.
// This handles cases like "李明总裁" matching profile key "李明", or "皇宫大殿" matching key "皇宫".
func lookupByFuzzyName(name string, lookup map[string]string) string {
	if name == "" || len(lookup) == 0 {
		return ""
	}
	// 1. Exact match
	if v, ok := lookup[name]; ok {
		return v
	}
	// 2. Substring match
	for key, val := range lookup {
		if key == "" {
			continue
		}
		if strings.Contains(name, key) || strings.Contains(key, name) {
			return val
		}
	}
	return ""
}

func buildConsistencyBibleBlock(kwLib *KeywordLibrary) string {
	if kwLib == nil {
		return ""
	}
	hasChars := len(kwLib.CharacterProfiles) > 0
	hasLocs := len(kwLib.LocationProfiles) > 0
	hasProps := len(kwLib.PropProfiles) > 0
	if !hasChars && !hasLocs && !hasProps {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n**【一致性圣经 — 视觉参考】**\n")
	sb.WriteString("*在每个分镜的 description 中，请严格沿用以下人物外貌、场景环境和道具的固定视觉特征，不得自行发明不同描述：*\n")

	if hasChars {
		sb.WriteString("\n**人物外貌（必须保持一致）：**\n")
		for _, p := range kwLib.CharacterProfiles {
			if p.Name != "" && p.Appearance != "" {
				sb.WriteString(fmt.Sprintf("- %s：%s\n", p.Name, p.Appearance))
			}
		}
	}
	if hasLocs {
		sb.WriteString("\n**场景环境（必须保持一致）：**\n")
		for _, p := range kwLib.LocationProfiles {
			if p.Name != "" && p.Description != "" {
				sb.WriteString(fmt.Sprintf("- %s：%s\n", p.Name, p.Description))
			}
		}
	}
	if hasProps {
		sb.WriteString("\n**重要道具（必须保持一致）：**\n")
		for _, p := range kwLib.PropProfiles {
			if p.Name != "" && p.Description != "" {
				sb.WriteString(fmt.Sprintf("- %s：%s\n", p.Name, p.Description))
			}
		}
	}
	return sb.String()
}

func inferVisualEra(text string) string {
	text = strings.TrimSpace(strings.ToLower(text))
	if text == "" {
		return ""
	}
	checks := []struct {
		keywords []string
		hint     string
	}{
		{[]string{"皇上", "娘娘", "王爷", "丞相", "宫殿", "宗门", "飞剑", "仙门", "江湖", "长袍", "发簪"}, "时代背景：古代东方/仙侠语境，建筑、服装、发型、器物和光线都应保持古风或仙侠质感。人物造型以长袍、束发、发冠、簪饰、古制兵器和宫廷/门派环境为主。"},
		{[]string{"民国", "旗袍", "军阀", "黄包车", "长衫", "报馆", "留声机", "老上海"}, "时代背景：民国近代语境，服装、发型、街景和室内陈设应保持民国风格。人物造型优先采用旗袍、长衫、礼帽、西式复古套装、旧式街灯与报馆洋楼环境。"},
		{[]string{"公司", "总裁", "办公室", "手机", "地铁", "咖啡厅", "西装", "直播", "短视频", "公寓"}, "时代背景：现代都市语境，服装、建筑和道具必须保持当代现实风格。人物造型优先采用现代发型、西装、职业装、休闲便服、手机电脑、写字楼和城市夜景。"},
		{[]string{"机甲", "星舰", "飞船", "星际", "实验舱", "人工智能", "赛博", "全息", "义体"}, "时代背景：科幻未来语境，场景、服装、道具和灯光应保持未来科技视觉。人物造型优先采用功能性战术服、科技材质、冷色霓虹、全息界面和未来城市/舰舱环境。"},
	}
	for _, check := range checks {
		for _, kw := range check.keywords {
			if strings.Contains(text, strings.ToLower(kw)) {
				return check.hint
			}
		}
	}
	return ""
}

func scenesToTextForEraInference(scenes []llmScene) string {
	var parts []string
	for _, sc := range scenes {
		if sc.Description != "" {
			parts = append(parts, sc.Description)
		}
		if sc.Location != "" {
			parts = append(parts, sc.Location)
		}
		if len(parts) >= 8 {
			break
		}
	}
	return strings.Join(parts, " ")
}

func buildCharacterAppearanceMap(kwLib *KeywordLibrary) map[string]string {
	out := make(map[string]string)
	if kwLib == nil {
		return out
	}
	for _, p := range kwLib.CharacterProfiles {
		if p.Name != "" && p.Appearance != "" {
			out[p.Name] = p.Appearance
		}
	}
	return out
}

func buildSharedItemsNote(prevScene *llmScene, scene llmScene) string {
	if prevScene == nil || len(prevScene.Items) == 0 || len(scene.Items) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(prevScene.Items))
	for _, item := range prevScene.Items {
		if item = strings.TrimSpace(item); item != "" {
			seen[item] = struct{}{}
		}
	}
	var shared []string
	for _, item := range scene.Items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			shared = append(shared, item)
		}
	}
	if len(shared) == 0 {
		return ""
	}
	return "关键道具延续：" + strings.Join(shared, "、") + "保持同一外观与位置逻辑。"
}

// enrichSceneDescription normalizes the user-facing scene description.
// Spatial continuity and mood live in PromptUsed / dedicated fields, not in Chinese description.
func enrichSceneDescription(scene llmScene, prevScene *llmScene, kwLib *KeywordLibrary, eraHint string) string {
	_ = prevScene
	_ = kwLib
	_ = eraHint
	desc := sanitizeUserSceneDescription(scene.Description)
	if desc != "" {
		return desc
	}
	if len(scene.CharacterStates) == 0 {
		return desc
	}
	var stateParts []string
	for _, cs := range scene.CharacterStates {
		if cs.Name == "" {
			continue
		}
		parts := []string{}
		if cs.Action != "" {
			parts = append(parts, cs.Action)
		}
		if cs.Emotion != "" {
			parts = append(parts, cs.Emotion)
		}
		if len(parts) > 0 {
			stateParts = append(stateParts, cs.Name+"："+strings.Join(parts, "，"))
		}
	}
	if len(stateParts) == 0 {
		return desc
	}
	fallback := strings.Join(stateParts, "；")
	if strings.TrimSpace(scene.Location) != "" {
		fallback = scene.Location + "，" + fallback
	}
	return sanitizeUserSceneDescription(fallback)
}

// using the start_text/end_text boundary markers returned by the LLM.
func (s *EpisodeService) fillExcerptsFromBoundaries(scriptText string, episodes []llmEpisode) {
	runes := []rune(scriptText)
	totalLen := len(runes)
	lastEnd := 0

	for i := range episodes {
		ep := &episodes[i]

		startIdx := lastEnd // default: start where previous episode ended
		endIdx := totalLen  // default: rest of text

		// Try to find start_text in the script (search from lastEnd forward)
		if ep.StartText != "" {
			searchFrom := string(runes[lastEnd:])
			pos := strings.Index(searchFrom, ep.StartText)
			if pos >= 0 {
				startIdx = lastEnd + utf8.RuneCountInString(searchFrom[:pos])
			}
		}

		// Try to find end_text in the script (search from startIdx forward)
		if ep.EndText != "" {
			searchFrom := string(runes[startIdx:])
			pos := strings.LastIndex(searchFrom, ep.EndText)
			if pos >= 0 {
				endIdx = startIdx + utf8.RuneCountInString(searchFrom[:pos]) + utf8.RuneCountInString(ep.EndText)
			}
		} else if i < len(episodes)-1 {
			// If no end_text, try to use next episode's start_text
			next := episodes[i+1]
			if next.StartText != "" {
				searchFrom := string(runes[startIdx:])
				pos := strings.Index(searchFrom, next.StartText)
				if pos > 0 {
					endIdx = startIdx + utf8.RuneCountInString(searchFrom[:pos])
				}
			}
		}

		if endIdx > totalLen {
			endIdx = totalLen
		}
		if startIdx >= endIdx {
			// Fallback: proportional split
			chunkSize := totalLen / len(episodes)
			startIdx = i * chunkSize
			endIdx = startIdx + chunkSize
			if i == len(episodes)-1 {
				endIdx = totalLen
			}
		}

		ep.Excerpt = string(runes[startIdx:endIdx])
		lastEnd = endIdx
	}
}

// resolveStructuralEpisodeSplit prefers chapter headings from the original manuscript,
// then optimized script, then user-provided split keywords.
func resolveStructuralEpisodeSplit(optimizedScript, originalScript string, splitKeywords []string) ([]llmEpisode, string) {
	originalScript = strings.TrimSpace(originalScript)
	optimizedScript = strings.TrimSpace(optimizedScript)

	// Prefer chapter markers from the normalized original; LLM polish may strip 01/02 headings.
	if originalScript != "" {
		if episodes := draftEpisodesToLLM(scriptsplit.SplitByChapters(originalScript)); len(episodes) > 0 {
			method := "chapters"
			if optimizedScript != "" && optimizedScript != originalScript {
				method = "chapters_original"
			}
			return episodes, method
		}
	}
	if optimizedScript != "" && optimizedScript != originalScript {
		if episodes := draftEpisodesToLLM(scriptsplit.SplitByChapters(optimizedScript)); len(episodes) > 0 {
			return episodes, "chapters_optimized"
		}
	}
	if episodes := splitByUserKeywords(optimizedScript, splitKeywords); len(episodes) > 0 {
		return episodes, "user_keywords"
	}
	return nil, ""
}

func draftEpisodesToLLM(drafts []scriptsplit.DraftEpisode) []llmEpisode {
	if len(drafts) == 0 {
		return nil
	}
	out := make([]llmEpisode, 0, len(drafts))
	for _, d := range drafts {
		out = append(out, llmEpisode{
			Title:   d.Title,
			Summary: d.Summary,
			Excerpt: d.Excerpt,
		})
	}
	return out
}

func llmEpisodesToDrafts(episodes []llmEpisode) []scriptsplit.DraftEpisode {
	out := make([]scriptsplit.DraftEpisode, 0, len(episodes))
	for _, ep := range episodes {
		out = append(out, scriptsplit.DraftEpisode{
			Title:   ep.Title,
			Summary: ep.Summary,
			Excerpt: ep.Excerpt,
		})
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func dialoguePreservationPromptBlock(source string) string {
	block := scriptpreserve.DialoguePreservationDirective()
	if locked := scriptpreserve.FormatLockedDialoguePromptBlock(scriptpreserve.ExtractLockedDialogues(source)); locked != "" {
		block += "\n\n" + locked
	}
	return block
}

func (s *EpisodeService) preserveDialogueFromSource(source, rewritten string) string {
	source = strings.TrimSpace(source)
	rewritten = strings.TrimSpace(rewritten)
	if source == "" || rewritten == "" || source == rewritten {
		return rewritten
	}
	enforced, restored := scriptpreserve.EnforceLockedDialogues(source, rewritten)
	if restored > 0 && s.logger != nil {
		s.logger.Info("restored locked dialogues after script rewrite",
			zap.Int("restored_count", restored),
		)
	}
	return enforced
}

// splitByUserKeywords —— 按用户提供的分集关键词在文本中定位并拆分为剧集
// splitByUserKeywords splits text at user-provided keyword positions.
// Each keyword marks the start of a new episode. Keywords are found in order of
// their first appearance in the text. Text before the first keyword is included
// as a prologue episode if it is substantial (>100 chars).
func splitByUserKeywords(text string, keywords []string) []llmEpisode {
	if len(keywords) == 0 {
		return nil
	}

	type marker struct {
		keyword string
		pos     int
	}

	var markers []marker
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		pos := strings.Index(text, kw)
		if pos >= 0 {
			markers = append(markers, marker{keyword: kw, pos: pos})
		}
	}

	if len(markers) == 0 {
		return nil
	}

	sort.Slice(markers, func(i, j int) bool { return markers[i].pos < markers[j].pos })

	var episodes []llmEpisode

	// Include text before first keyword only when it looks like narrative lead-in.
	if markers[0].pos > 100 {
		preText := strings.TrimSpace(text[:markers[0].pos])
		if preText != "" && shouldKeepKeywordPrologue(preText) {
			summary := preText
			if utf8.RuneCountInString(summary) > 200 {
				summary = string([]rune(summary)[:200]) + "..."
			}
			episodes = append(episodes, llmEpisode{
				Title:   "序",
				Summary: summary,
				Excerpt: preText,
			})
		}
	}

	for i, m := range markers {
		start := m.pos
		end := len(text)
		if i+1 < len(markers) {
			end = markers[i+1].pos
		}

		excerpt := strings.TrimSpace(text[start:end])
		if excerpt == "" {
			continue
		}

		// Use keyword as title; first 200 chars of body as summary
		title := m.keyword
		if utf8.RuneCountInString(title) > 50 {
			title = string([]rune(title)[:50])
		}
		body := excerpt
		kwEnd := strings.Index(excerpt, "\n")
		if kwEnd > 0 && kwEnd < len(excerpt) {
			body = strings.TrimSpace(excerpt[kwEnd:])
		}
		summary := body
		if utf8.RuneCountInString(summary) > 200 {
			summary = string([]rune(summary)[:200]) + "..."
		}

		episodes = append(episodes, llmEpisode{
			Title:   title,
			Summary: summary,
			Excerpt: excerpt,
		})
	}

	return episodes
}

func shouldKeepKeywordPrologue(preText string) bool {
	preText = strings.TrimSpace(preText)
	if preText == "" || utf8.RuneCountInString(preText) <= 100 {
		return false
	}
	if strings.Contains(preText, "【简介】") || strings.HasPrefix(preText, "简介") {
		return false
	}
	return !scriptsplit.LooksLikeSynopsisOnly(preText)
}

// splitByChapters is kept for tests that call it directly.
func splitByChapters(text string) []llmEpisode {
	return draftEpisodesToLLM(scriptsplit.SplitByChapters(text))
}

type adSemanticUnit struct {
	Text  string
	Title string
}

// simpleSplit —— 降级方案：优先按广告语义段切分，再按目标集数合并。
// 避免在广告文案里生硬按字数均分，尽量贴合卖点段、转场句、CTA、口播句群。
func (s *EpisodeService) simpleSplit(scriptText string, n int, profile productionmode.Profile) []llmEpisode {
	if !profile.UseAdSimpleSplit() {
		return simpleSplitByLength(scriptText, n)
	}
	return s.simpleSplitAd(scriptText, n)
}

func (s *EpisodeService) simpleSplitAd(scriptText string, n int) []llmEpisode {
	trimmed := strings.TrimSpace(scriptText)
	if trimmed == "" {
		return nil
	}
	if n <= 0 {
		n = 1
	}

	units := splitAdSemanticUnits(trimmed)
	if len(units) == 0 {
		units = []adSemanticUnit{{Text: trimmed}}
	}
	if len(units) < n {
		units = splitOversizedSemanticUnits(units, n)
	}
	if len(units) == 0 {
		units = []adSemanticUnit{{Text: trimmed}}
	}
	if len(units) < n {
		n = len(units)
	}
	if n <= 0 {
		n = 1
	}

	totalChars := 0
	for _, unit := range units {
		totalChars += utf8.RuneCountInString(unit.Text)
	}
	if totalChars <= 0 {
		totalChars = utf8.RuneCountInString(trimmed)
	}
	targetChars := totalChars / n
	if targetChars < 80 {
		targetChars = 80
	}

	groups := make([][]adSemanticUnit, 0, n)
	current := make([]adSemanticUnit, 0)
	currentChars := 0
	for idx, unit := range units {
		unitChars := utf8.RuneCountInString(unit.Text)
		remainingUnitsAfter := len(units) - idx
		remainingGroupsAfterCurrent := n - len(groups) - 1
		shouldFlush := false
		if len(current) > 0 && len(groups) < n-1 {
			if currentChars+unitChars > targetChars && remainingUnitsAfter > remainingGroupsAfterCurrent {
				shouldFlush = true
			}
			if remainingUnitsAfter == remainingGroupsAfterCurrent {
				shouldFlush = true
			}
		}
		if shouldFlush {
			groups = append(groups, current)
			current = nil
			currentChars = 0
		}
		current = append(current, unit)
		currentChars += unitChars
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	for len(groups) > n {
		last := groups[len(groups)-1]
		groups = groups[:len(groups)-1]
		groups[len(groups)-1] = append(groups[len(groups)-1], last...)
	}

	var episodes []llmEpisode
	for i, group := range groups {
		excerpt := joinSemanticUnits(group)
		if strings.TrimSpace(excerpt) == "" {
			continue
		}
		title := deriveSemanticTitle(group, i+1)
		summary := excerpt
		if utf8.RuneCountInString(summary) > 100 {
			summary = string([]rune(summary)[:100]) + "..."
		}
		episodes = append(episodes, llmEpisode{
			Title:   title,
			Summary: summary,
			Excerpt: excerpt,
		})
	}
	if len(episodes) == 0 {
		return []llmEpisode{{
			Title:   "第1集",
			Summary: trimmed,
			Excerpt: trimmed,
		}}
	}
	return episodes
}

func splitAdSemanticUnits(text string) []adSemanticUnit {
	text = normalizeSemanticSplitText(text)
	if text == "" {
		return nil
	}

	paragraphs := splitSemanticParagraphs(text)
	var units []adSemanticUnit
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		if looksLikeStrongAdBoundary(paragraph) && utf8.RuneCountInString(paragraph) <= 140 {
			units = append(units, adSemanticUnit{Text: paragraph, Title: extractSemanticHeading(paragraph)})
			continue
		}
		for _, chunk := range splitParagraphIntoSemanticChunks(paragraph) {
			chunk = strings.TrimSpace(chunk)
			if chunk == "" {
				continue
			}
			units = append(units, adSemanticUnit{Text: chunk, Title: extractSemanticHeading(chunk)})
		}
	}
	return units
}

func normalizeSemanticSplitText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "。\n", "。\n\n")
	text = strings.ReplaceAll(text, "！\n", "！\n\n")
	text = strings.ReplaceAll(text, "？\n", "？\n\n")
	return strings.TrimSpace(text)
}

func splitSemanticParagraphs(text string) []string {
	re := regexp.MustCompile(`\n\s*\n+`)
	parts := re.Split(text, -1)
	if len(parts) == 0 {
		return []string{text}
	}
	return parts
}

func looksLikeStrongAdBoundary(paragraph string) bool {
	markers := []string{
		"【", "卖点", "痛点", "转场", "镜头", "场景", "口播", "旁白", "CTA", "行动号召", "立即", "马上", "下单", "购买", "点击", "关注", "现在就",
	}
	for _, marker := range markers {
		if strings.Contains(paragraph, marker) {
			return true
		}
	}
	return false
}

func splitParagraphIntoSemanticChunks(paragraph string) []string {
	paragraph = strings.TrimSpace(paragraph)
	if paragraph == "" {
		return nil
	}
	if utf8.RuneCountInString(paragraph) <= 140 {
		return []string{paragraph}
	}

	var segments []string
	var currentSegment strings.Builder
	flushSegment := func() {
		segment := strings.TrimSpace(currentSegment.String())
		if segment != "" {
			segments = append(segments, segment)
		}
		currentSegment.Reset()
	}
	for _, r := range paragraph {
		if r == '\n' {
			flushSegment()
			continue
		}
		currentSegment.WriteRune(r)
		switch r {
		case '。', '！', '？', '!', '?', '；', ';':
			flushSegment()
		}
	}
	flushSegment()
	var chunks []string
	var current []string
	currentLen := 0
	flush := func() {
		if len(current) == 0 {
			return
		}
		chunks = append(chunks, strings.TrimSpace(strings.Join(current, "")))
		current = nil
		currentLen = 0
	}
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		segLen := utf8.RuneCountInString(segment)
		if len(current) > 0 && (currentLen+segLen > 140 || looksLikeStrongAdBoundary(segment)) {
			flush()
		}
		current = append(current, segment)
		currentLen += segLen
		if currentLen >= 110 || looksLikeStrongAdBoundary(segment) {
			flush()
		}
	}
	flush()
	if len(chunks) == 0 {
		return []string{paragraph}
	}
	return chunks
}

func splitOversizedSemanticUnits(units []adSemanticUnit, targetCount int) []adSemanticUnit {
	if len(units) >= targetCount || targetCount <= 0 {
		return units
	}
	var expanded []adSemanticUnit
	for _, unit := range units {
		text := strings.TrimSpace(unit.Text)
		if text == "" {
			continue
		}
		if utf8.RuneCountInString(text) > 220 {
			parts := splitParagraphIntoSemanticChunks(text)
			if len(parts) > 1 {
				for _, part := range parts {
					expanded = append(expanded, adSemanticUnit{Text: part, Title: extractSemanticHeading(part)})
				}
				continue
			}
		}
		expanded = append(expanded, unit)
	}
	return expanded
}

func joinSemanticUnits(units []adSemanticUnit) string {
	parts := make([]string, 0, len(units))
	for _, unit := range units {
		if text := strings.TrimSpace(unit.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func deriveSemanticTitle(units []adSemanticUnit, index int) string {
	for _, unit := range units {
		if title := strings.TrimSpace(unit.Title); title != "" {
			return title
		}
		lines := strings.Split(strings.TrimSpace(unit.Text), "\n")
		if len(lines) == 0 {
			continue
		}
		line := strings.TrimSpace(lines[0])
		if line != "" && utf8.RuneCountInString(line) <= 20 {
			return line
		}
	}
	return fmt.Sprintf("第%d集", index)
}

func extractSemanticHeading(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	first := strings.TrimSpace(lines[0])
	if first == "" {
		return ""
	}
	first = strings.Trim(first, "【】[]#：:*- ")
	if utf8.RuneCountInString(first) > 20 {
		return ""
	}
	return first
}

// fetchScriptContent —— 通过 storage-service 获取剧本文件内容
// fetchScriptContent retrieves script content via storage-service presigned URL.
// MinIO direct URLs return 403, so we extract the bucket + object key from the URL,
// request a presigned URL from storage-service, then fetch the content.
func (s *EpisodeService) fetchScriptContent(ctx context.Context, fileURL string) (string, error) {
	// Extract bucket and object key from MinIO URL:
	// http://localhost:9000/scripts/0/20260325/uuid.txt
	// parts: ["http:", "", "localhost:9000", "scripts", "0/20260325/uuid.txt"]
	parts := strings.SplitN(fileURL, "/", 5)
	if len(parts) < 5 {
		return "", fmt.Errorf("cannot parse MinIO URL: %s", fileURL)
	}
	objectKey := parts[4] // e.g. "0/20260325/uuid.txt" (without bucket)

	// Get presigned URL from storage-service via query params
	presignURL := fmt.Sprintf("%s/api/v1/storage/url?key=%s&expiry=300",
		s.storageBaseURL, objectKey)
	req, err := http.NewRequestWithContext(ctx, "GET", presignURL, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("storage-service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("storage-service returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode presign response: %w", err)
	}
	if result.Data.URL == "" {
		return "", fmt.Errorf("storage-service returned empty presigned URL")
	}

	// Fetch actual content via presigned URL
	return fetchURL(ctx, result.Data.URL)
}

// fetchURL —— 发起 HTTP GET 请求获取指定 URL 的文本内容
func fetchURL(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("fetch returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// min —— 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// shotTypeToCameraMovement maps an LLM-returned shot_type to the storyboard camera_movement enum.
func shotTypeToCameraMovement(shotType string) string {
	switch strings.TrimSpace(strings.ToLower(shotType)) {
	case "close-up", "extreme-close-up", "特写", "大特写":
		return "static"
	case "medium", "medium-shot", "近景", "中景":
		return "static"
	case "full", "full-shot", "全景":
		return "static"
	case "wide", "wide-shot", "wide-angle", "大全景", "远景":
		return "pull-out"
	case "overhead", "bird-eye", "俯拍", "俯视":
		return "static"
	case "low-angle", "仰拍", "仰视":
		return "static"
	case "tracking", "跟拍":
		return "tracking"
	case "handheld", "手持":
		return "handheld"
	default:
		return ""
	}
}

// clampDuration clamps a storyboard clip duration to [minSec, maxSec].
// This replaces the old hard floor of 4s — the LLM now chooses freely within
// a wider range based on scene complexity, and we only guard against extremes.
func clampDuration(d, minSec, maxSec int) int {
	if d < minSec {
		return minSec
	}
	if d > maxSec {
		return maxSec
	}
	return d
}

// normalizeSceneKey 将场景地点字符串标准化为用于串行分组的 key：
// 小写、去除前后空白、将内部多余空格合并为下划线，截断到 180 字节。
func normalizeSceneKey(location string) string {
	if location == "" {
		return ""
	}
	key := strings.ToLower(strings.TrimSpace(location))
	// collapse internal whitespace
	parts := strings.Fields(key)
	key = strings.Join(parts, "_")
	// truncate
	if len(key) > 180 {
		key = key[:180]
	}
	return key
}

// ─────────────────────────────────────────────────────────────────────────────
// ScriptOptimize — 将分集小说文本转化为标准剧本格式，保存优化后结果
// ─────────────────────────────────────────────────────────────────────────────

type OptimizedEpisode struct {
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	OptimizedText string `json:"optimized_text"`
}

// OptimizeEpisode converts the episode's script_excerpt to screenplay format
// using the keyword library for character/location consistency.
func (s *EpisodeService) OptimizeEpisode(ctx context.Context, id, projectID uint64) (*model.Episode, error) {
	episode, err := s.episodeRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("episode not found")
		}
		return nil, err
	}
	if episode.ProjectID != projectID {
		return nil, errors.New("episode not found")
	}

	sourceText := episode.ScriptExcerpt
	if sourceText == "" {
		return nil, errors.New("episode has no script content to optimize")
	}

	// Mark as optimizing
	episode.OptimizeStatus = "optimizing"
	_ = s.episodeRepo.Update(episode)

	// Load keyword library for consistency
	var kwLib *KeywordLibrary
	if project, pErr := s.projectRepo.FindByIDNoAuth(projectID); pErr == nil {
		var lib KeywordLibrary
		if len(project.KeywordLibrary) > 0 {
			if jsonErr := json.Unmarshal(project.KeywordLibrary, &lib); jsonErr == nil {
				kwLib = &lib
			}
		}
	}

	writingHints := s.fetchWritingSkillHints(ctx, projectID)
	productionMode := s.resolveProductionMode(projectID)
	result, err := s.callLLMOptimize(ctx, episode, writingHints, kwLib, productionMode)
	if err != nil {
		episode.OptimizeStatus = "failed"
		_ = s.episodeRepo.Update(episode)
		return nil, fmt.Errorf("LLM optimize failed: %w", err)
	}

	// Save original excerpt before overwriting
	if episode.OriginalExcerpt == "" {
		episode.OriginalExcerpt = episode.ScriptExcerpt
	}
	episode.OptimizedText = result.OptimizedText
	episode.OptimizeStatus = "done"
	if result.Title != "" {
		episode.Title = result.Title
	}
	if result.Summary != "" {
		episode.Summary = result.Summary
	}

	s.ensureCommentaryScriptFormat(ctx, episode, projectID, writingHints, kwLib)

	if err := s.episodeRepo.Update(episode); err != nil {
		return nil, fmt.Errorf("save optimized episode: %w", err)
	}
	return episode, nil
}

// ApplyOptimizedText copies optimized_text → script_excerpt (user confirmed).
func (s *EpisodeService) ApplyOptimizedText(ctx context.Context, id, projectID uint64) (*model.Episode, error) {
	episode, err := s.episodeRepo.FindByID(id)
	if err != nil || episode.ProjectID != projectID {
		return nil, errors.New("episode not found")
	}
	if episode.OptimizedText == "" {
		return nil, errors.New("no optimized text to apply")
	}
	episode.ScriptExcerpt = episode.OptimizedText
	episode.WordCount = utf8.RuneCountInString(episode.OptimizedText)
	if episode.WordCount > 0 {
		episode.EstimatedDuration = episode.WordCount / 5
	}
	if err := s.episodeRepo.Update(episode); err != nil {
		return nil, fmt.Errorf("apply optimized text: %w", err)
	}
	if !shouldSkipEpisodeAssetExtraction(ctx) {
		if err := s.extractAssetsForEpisode(WithSkipEpisodeStoryboardTrigger(ctx), projectID, id); err != nil {
			return nil, fmt.Errorf("apply optimized text trigger assets: %w", err)
		}
		if s.logger != nil {
			s.logger.Info("applied optimized text and triggered asset extraction",
				zap.Uint64("project_id", projectID),
				zap.Uint64("episode_id", id),
			)
		}
	} else if s.logger != nil {
		s.logger.Info("applied optimized text without asset extraction",
			zap.Uint64("project_id", projectID),
			zap.Uint64("episode_id", id),
		)
	}
	return episode, nil
}

func (s *EpisodeService) resolveProductionMode(projectID uint64) productionmode.Mode {
	project, err := s.projectRepo.FindByIDNoAuth(projectID)
	if err != nil || project == nil {
		return productionmode.ModeScriptDrama
	}
	return productionmode.Resolve(project)
}

func (s *EpisodeService) callLLMOptimize(ctx context.Context, ep *model.Episode, writingHints string, kwLib *KeywordLibrary, mode productionmode.Mode) (*OptimizedEpisode, error) {
	systemPrompt := productionmode.EpisodeOptimizeSystemPrompt(mode)

	if writingHints != "" {
		systemPrompt += "\n\n**本项目专属指引（务必遵守）：**\n" + writingHints
	}

	if bible := buildConsistencyBibleBlock(kwLib); bible != "" {
		systemPrompt += bible
	}
	systemPrompt += "\n\n" + dialoguePreservationPromptBlock(ep.ScriptExcerpt)

	userContent := fmt.Sprintf("第%d集《%s》\n\n【当前简介】\n%s\n\n【原始文本】\n%s",
		ep.EpisodeNumber, ep.Title, ep.Summary, ep.ScriptExcerpt)

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": productionmode.EpisodeOptimizeUserAction(mode) + "\n\n" + userContent},
		},
		"temperature":     0.65,
		"max_tokens":      8192,
		"response_format": map[string]string{"type": "json_object"},
	}
	data, _ := json.Marshal(reqBody)

	optCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(optCtx, http.MethodPost, s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM responded %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}
	content := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	// strip optional markdown fences
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result OptimizedEpisode
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse optimized JSON: %w", err)
	}
	if result.OptimizedText != "" {
		result.OptimizedText = s.preserveDialogueFromSource(ep.ScriptExcerpt, result.OptimizedText)
	}
	return &result, nil
}

func (s *EpisodeService) repairEpisodeSplitStructure(ctx context.Context, episodes []llmEpisode, splitMethod string, llmSplitUsed bool, profile productionmode.Profile) []llmEpisode {
	if len(episodes) == 0 {
		return episodes
	}

	drafts := llmEpisodesToDrafts(episodes)
	repaired, ruleActions := scriptsplit.RepairSplit(drafts)
	needsReview := llmSplitUsed || splitMethod == "" || splitMethod == "user_keywords" || scriptsplit.NeedsStructuralReview(repaired)

	if needsReview && s.llmBaseURL != "" && s.llmAPIKey != "" {
		if review, err := s.callLLMSplitStructureReview(ctx, repaired, profile); err != nil {
			if s.logger != nil {
				s.logger.Warn("episode split structure review failed; using rule-based repair only", zap.Error(err))
			}
		} else if review != nil && !review.Passed && len(review.Actions) > 0 {
			repaired = scriptsplit.ApplySplitReviewActions(repaired, review.Actions)
			if s.logger != nil {
				s.logger.Info("episode split structure repaired by LLM review",
					zap.Int("actions", len(review.Actions)),
					zap.Strings("rule_actions", ruleActions),
				)
			}
		}
	} else if len(ruleActions) > 0 && s.logger != nil {
		s.logger.Info("episode split structure repaired by rules",
			zap.Strings("actions", ruleActions),
			zap.String("split_method", splitMethod),
		)
	}

	if len(repaired) == 0 {
		return episodes
	}
	return draftEpisodesToLLM(repaired)
}

func (s *EpisodeService) callLLMSplitStructureReview(ctx context.Context, episodes []scriptsplit.DraftEpisode, profile productionmode.Profile) (*scriptsplit.SplitReviewResult, error) {
	if len(episodes) == 0 {
		return &scriptsplit.SplitReviewResult{Passed: true}, nil
	}

	var b strings.Builder
	for i, ep := range episodes {
		excerpt := strings.TrimSpace(ep.Excerpt)
		start := excerpt
		end := excerpt
		runes := []rune(excerpt)
		if len(runes) > 80 {
			start = string(runes[:80])
			end = string(runes[len(runes)-80:])
		}
		fmt.Fprintf(&b, "第 %d 集 | title=%q | chars=%d | start=%q | end=%q\n",
			i+1, ep.Title, utf8.RuneCountInString(excerpt), start, end)
	}

	systemPrompt := productionmode.EpisodeSplitReviewSystemPrompt(profile.Mode)
	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": "请审查以下分集边界是否合理：\n\n" + b.String()},
		},
		"temperature":     0.2,
		"max_tokens":      2048,
		"response_format": map[string]string{"type": "json_object"},
	}
	data, _ := json.Marshal(reqBody)

	reviewCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reviewCtx, http.MethodPost, s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM split review request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM split review responded %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil {
		return nil, fmt.Errorf("parse split review response: %w", err)
	}
	if len(llmResp.Choices) == 0 {
		return nil, errors.New("llm split review returned no choices")
	}

	var result scriptsplit.SplitReviewResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(llmResp.Choices[0].Message.Content)), &result); err != nil {
		return nil, fmt.Errorf("parse split review json: %w", err)
	}
	return &result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ScriptReview — AI 审查：人物/场景/道具一致性、衔接性、台词质量
// ─────────────────────────────────────────────────────────────────────────────

type ReviewScore struct {
	Completeness  int `json:"completeness"`   // 完整度 0-100
	Integrity     int `json:"integrity"`      // 完善度 0-100
	Consistency   int `json:"consistency"`    // 一致性 0-100
	Transitions   int `json:"transitions"`    // 衔接性 0-100
	DialogQuality int `json:"dialog_quality"` // 台词质量 0-100
}

type ReviewIssue struct {
	Severity    string `json:"severity"` // critical | warning | info
	Type        string `json:"type"`     // character_inconsistency | prop_inconsistency | scene_transition | dialog | plot_gap
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

type ReviewResult struct {
	Score     ReviewScore   `json:"score"`
	Issues    []ReviewIssue `json:"issues"`
	Overall   string        `json:"overall"`   // 总体评价（1-2句）
	Strengths string        `json:"strengths"` // 亮点
}

// ReviewEpisode runs AI consistency & quality review on an episode's script.
func (s *EpisodeService) ReviewEpisode(ctx context.Context, id, projectID uint64) (*model.Episode, error) {
	episode, err := s.episodeRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("episode not found")
		}
		return nil, err
	}
	if episode.ProjectID != projectID {
		return nil, errors.New("episode not found")
	}

	// Use optimized text if available, fallback to script_excerpt
	textToReview := episode.OptimizedText
	if textToReview == "" {
		textToReview = episode.ScriptExcerpt
	}
	if textToReview == "" {
		return nil, errors.New("episode has no script content to review")
	}

	episode.ReviewStatus = "reviewing"
	_ = s.episodeRepo.Update(episode)

	var kwLib *KeywordLibrary
	if project, pErr := s.projectRepo.FindByIDNoAuth(projectID); pErr == nil {
		var lib KeywordLibrary
		if len(project.KeywordLibrary) > 0 {
			if jsonErr := json.Unmarshal(project.KeywordLibrary, &lib); jsonErr == nil {
				kwLib = &lib
			}
		}
	}

	productionMode := s.resolveProductionMode(projectID)
	result, err := s.callLLMReview(ctx, episode, textToReview, kwLib, productionMode)
	if err != nil {
		episode.ReviewStatus = "failed"
		_ = s.episodeRepo.Update(episode)
		return nil, fmt.Errorf("LLM review failed: %w", err)
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal review result: %w", err)
	}
	episode.ReviewStatus = "done"
	episode.ReviewResult = resultJSON
	if err := s.episodeRepo.Update(episode); err != nil {
		return nil, fmt.Errorf("save review result: %w", err)
	}
	return episode, nil
}

func (s *EpisodeService) callLLMReview(ctx context.Context, ep *model.Episode, text string, kwLib *KeywordLibrary, mode productionmode.Mode) (*ReviewResult, error) {
	systemPrompt := productionmode.EpisodeReviewSystemPrompt(mode)

	if bible := buildConsistencyBibleBlock(kwLib); bible != "" {
		systemPrompt += "\n\n以下是项目词库，请重点检查剧本是否与词库定义一致：" + bible
	}

	userContent := fmt.Sprintf("第%d集《%s》\n\n【简介】\n%s\n\n【剧本内容】\n%s",
		ep.EpisodeNumber, ep.Title, ep.Summary, text)

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": "请对以下剧本进行全面审查：\n\n" + userContent},
		},
		"temperature":     0.3,
		"max_tokens":      4096,
		"response_format": map[string]string{"type": "json_object"},
	}
	data, _ := json.Marshal(reqBody)

	reviewCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(reviewCtx, http.MethodPost, s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM responded %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}
	content := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result ReviewResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse review JSON: %w", err)
	}
	// Clamp scores to 0-100
	clampScore := func(v int) int {
		if v < 0 {
			return 0
		}
		if v > 100 {
			return 100
		}
		return v
	}
	result.Score.Completeness = clampScore(result.Score.Completeness)
	result.Score.Integrity = clampScore(result.Score.Integrity)
	result.Score.Consistency = clampScore(result.Score.Consistency)
	result.Score.Transitions = clampScore(result.Score.Transitions)
	result.Score.DialogQuality = clampScore(result.Score.DialogQuality)
	return &result, nil
}

// AutoOptimizeReview runs optimize → review → repair (if issues found) in one shot.
// The repair pass asks the LLM to fix critical/warning issues found by the review.
func (s *EpisodeService) AutoOptimizeReview(ctx context.Context, id, projectID uint64) (*model.Episode, error) {
	// Step 1: optimize
	episode, err := s.OptimizeEpisode(ctx, id, projectID)
	if err != nil {
		return nil, fmt.Errorf("auto-optimize: %w", err)
	}

	// Step 2: load keyword library (shared between review and repair)
	var kwLib *KeywordLibrary
	if project, pErr := s.projectRepo.FindByIDNoAuth(projectID); pErr == nil {
		var lib KeywordLibrary
		if len(project.KeywordLibrary) > 0 {
			if jsonErr := json.Unmarshal(project.KeywordLibrary, &lib); jsonErr == nil {
				kwLib = &lib
			}
		}
	}

	// Step 3: review the freshly optimized text
	textToReview := episode.OptimizedText
	if textToReview == "" {
		textToReview = episode.ScriptExcerpt
	}
	episode.ReviewStatus = "reviewing"
	_ = s.episodeRepo.Update(episode)

	productionMode := s.resolveProductionMode(projectID)
	reviewResult, reviewErr := s.callLLMReview(ctx, episode, textToReview, kwLib, productionMode)
	if reviewErr != nil {
		episode.ReviewStatus = "failed"
		_ = s.episodeRepo.Update(episode)
		// Optimize already succeeded — return it without review
		return episode, nil
	}
	resultJSON, _ := json.Marshal(reviewResult)
	episode.ReviewStatus = "done"
	episode.ReviewResult = resultJSON
	_ = s.episodeRepo.Update(episode)

	// Step 4: repair if needed (critical issues or average score < 75)
	criticalCount := 0
	for _, issue := range reviewResult.Issues {
		if issue.Severity == "critical" {
			criticalCount++
		}
	}
	avgScore := (reviewResult.Score.Completeness + reviewResult.Score.Integrity +
		reviewResult.Score.Consistency + reviewResult.Score.Transitions +
		reviewResult.Score.DialogQuality) / 5

	if criticalCount > 0 || avgScore < 75 {
		writingHints := s.fetchWritingSkillHints(ctx, projectID)
		repaired, repairErr := s.callLLMRepair(ctx, episode, reviewResult, writingHints, kwLib, productionMode)
		if repairErr == nil && repaired.OptimizedText != "" {
			episode.OptimizedText = repaired.OptimizedText
			if repaired.Title != "" {
				episode.Title = repaired.Title
			}
			if repaired.Summary != "" {
				episode.Summary = repaired.Summary
			}
			_ = s.episodeRepo.Update(episode)
		}
	}
	writingHints := s.fetchWritingSkillHints(ctx, projectID)
	s.ensureCommentaryScriptFormat(ctx, episode, projectID, writingHints, kwLib)

	if err := s.episodeRepo.Update(episode); err != nil {
		return nil, fmt.Errorf("save auto-optimize-review: %w", err)
	}
	return episode, nil
}

// polishEpisodeInternal is the inner body of PolishEpisode with pre-fetched hints and kwLib.
// Used by runAutoPolishPipeline to avoid redundant per-episode HTTP calls.
func (s *EpisodeService) polishEpisodeInternal(ctx context.Context, id, projectID uint64, writingHints, productionHints string, kwLib *KeywordLibrary) (*model.Episode, error) {
	episode, err := s.episodeRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("episode not found")
		}
		return nil, err
	}
	if episode.ProjectID != projectID {
		return nil, errors.New("episode not found")
	}
	projectRef, _ := s.projectRepo.FindByIDNoAuth(projectID)
	polished, err := s.callLLMPolish(ctx, projectRef, episode, writingHints, productionHints, kwLib)
	if err != nil {
		return nil, fmt.Errorf("LLM polish failed: %w", err)
	}
	if polished.Title != "" {
		episode.Title = polished.Title
	}
	if polished.Summary != "" {
		episode.Summary = polished.Summary
	}
	if polished.ScriptExcerpt != "" {
		episode.ScriptExcerpt = polished.ScriptExcerpt
		episode.WordCount = utf8.RuneCountInString(polished.ScriptExcerpt)
		if episode.WordCount > 0 {
			episode.EstimatedDuration = episode.WordCount / 5
		}
	}
	if err := s.episodeRepo.Update(episode); err != nil {
		return nil, fmt.Errorf("save polished episode: %w", err)
	}
	return episode, nil
}

// optimizeEpisodeInternal is the inner body of OptimizeEpisode with pre-fetched hints and kwLib.
func (s *EpisodeService) optimizeEpisodeInternal(ctx context.Context, id, projectID uint64, writingHints string, kwLib *KeywordLibrary) (*model.Episode, error) {
	episode, err := s.episodeRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("episode not found")
		}
		return nil, err
	}
	if episode.ProjectID != projectID {
		return nil, errors.New("episode not found")
	}
	if episode.ScriptExcerpt == "" {
		return nil, errors.New("episode has no script content to optimize")
	}
	episode.OptimizeStatus = "optimizing"
	_ = s.episodeRepo.Update(episode)
	productionMode := s.resolveProductionMode(projectID)
	result, err := s.callLLMOptimize(ctx, episode, writingHints, kwLib, productionMode)
	if err != nil {
		episode.OptimizeStatus = "failed"
		_ = s.episodeRepo.Update(episode)
		return nil, fmt.Errorf("LLM optimize failed: %w", err)
	}
	if episode.OriginalExcerpt == "" {
		episode.OriginalExcerpt = episode.ScriptExcerpt
	}
	episode.OptimizedText = result.OptimizedText
	episode.OptimizeStatus = "done"
	if result.Title != "" {
		episode.Title = result.Title
	}
	if result.Summary != "" {
		episode.Summary = result.Summary
	}
	s.ensureCommentaryScriptFormat(ctx, episode, projectID, writingHints, kwLib)
	if err := s.episodeRepo.Update(episode); err != nil {
		return nil, fmt.Errorf("save optimized episode: %w", err)
	}
	return episode, nil
}

// autoOptimizeReviewInternal is AutoOptimizeReview with pre-fetched writingHints and kwLib.
// Used by runAutoPolishPipeline to eliminate per-episode HTTP calls.
func (s *EpisodeService) autoOptimizeReviewInternal(ctx context.Context, id, projectID uint64, writingHints string, kwLib *KeywordLibrary) (*model.Episode, error) {
	// Step 1: optimize (uses pre-fetched hints)
	episode, err := s.optimizeEpisodeInternal(ctx, id, projectID, writingHints, kwLib)
	if err != nil {
		return nil, fmt.Errorf("auto-optimize: %w", err)
	}

	// Step 2: review
	textToReview := episode.OptimizedText
	if textToReview == "" {
		textToReview = episode.ScriptExcerpt
	}
	episode.ReviewStatus = "reviewing"
	_ = s.episodeRepo.Update(episode)

	productionMode := s.resolveProductionMode(projectID)
	reviewResult, reviewErr := s.callLLMReview(ctx, episode, textToReview, kwLib, productionMode)
	if reviewErr != nil {
		episode.ReviewStatus = "failed"
		_ = s.episodeRepo.Update(episode)
		return episode, nil
	}
	resultJSON, _ := json.Marshal(reviewResult)
	episode.ReviewStatus = "done"
	episode.ReviewResult = resultJSON
	_ = s.episodeRepo.Update(episode)

	// Step 3: repair if needed
	criticalCount := 0
	for _, issue := range reviewResult.Issues {
		if issue.Severity == "critical" {
			criticalCount++
		}
	}
	avgScore := (reviewResult.Score.Completeness + reviewResult.Score.Integrity +
		reviewResult.Score.Consistency + reviewResult.Score.Transitions +
		reviewResult.Score.DialogQuality) / 5

	if criticalCount > 0 || avgScore < 75 {
		repaired, repairErr := s.callLLMRepair(ctx, episode, reviewResult, writingHints, kwLib, productionMode)
		if repairErr == nil && repaired.OptimizedText != "" {
			episode.OptimizedText = repaired.OptimizedText
			if repaired.Title != "" {
				episode.Title = repaired.Title
			}
			if repaired.Summary != "" {
				episode.Summary = repaired.Summary
			}
			_ = s.episodeRepo.Update(episode)
		}
	}
	s.ensureCommentaryScriptFormat(ctx, episode, projectID, writingHints, kwLib)

	if err := s.episodeRepo.Update(episode); err != nil {
		return nil, fmt.Errorf("save auto-optimize-review: %w", err)
	}
	return episode, nil
}

func buildCommentaryFormatReviewResult(text string) *ReviewResult {
	issues := productionmode.CommentaryFormatIssues(text)
	if len(issues) == 0 {
		return nil
	}
	result := &ReviewResult{
		Score: ReviewScore{
			Completeness:  70,
			Integrity:     45,
			Consistency:   75,
			Transitions:   65,
			DialogQuality: 35,
		},
		Overall: "文稿格式不符合解说漫旁白驱动要求",
	}
	for _, issue := range issues {
		result.Issues = append(result.Issues, ReviewIssue{
			Severity:    "critical",
			Type:        issue.Type,
			Description: issue.Description,
			Suggestion:  issue.Suggestion,
		})
	}
	return result
}

// ensureCommentaryScriptFormat repairs optimize output that was miswritten as script drama.
func (s *EpisodeService) ensureCommentaryScriptFormat(ctx context.Context, episode *model.Episode, projectID uint64, writingHints string, kwLib *KeywordLibrary) {
	mode := s.resolveProductionMode(projectID)
	if mode != productionmode.ModeCommentaryComic {
		return
	}
	if !productionmode.NeedsCommentaryFormatRepair(episode.OptimizedText) {
		return
	}
	if s.logger != nil {
		s.logger.Warn("commentary script format repair triggered",
			zap.Uint64("project_id", projectID),
			zap.Uint64("episode_id", episode.ID),
			zap.Int("episode", episode.EpisodeNumber),
		)
	}
	s.repairCommentaryScriptFormat(ctx, episode, projectID, writingHints, kwLib, false)
	if productionmode.NeedsCommentaryFormatRepair(episode.OptimizedText) && strings.TrimSpace(episode.OriginalExcerpt) != "" {
		s.repairCommentaryScriptFormat(ctx, episode, projectID, writingHints, kwLib, true)
	}
}

func (s *EpisodeService) repairCommentaryScriptFormat(ctx context.Context, episode *model.Episode, projectID uint64, writingHints string, kwLib *KeywordLibrary, useOriginalReference bool) {
	review := buildCommentaryFormatReviewResult(episode.OptimizedText)
	if review == nil {
		return
	}
	mode := s.resolveProductionMode(projectID)
	hints := writingHints
	if useOriginalReference {
		ref := strings.TrimSpace(episode.OriginalExcerpt)
		if ref == "" {
			ref = strings.TrimSpace(episode.ScriptExcerpt)
		}
		if ref != "" {
			hints += "\n\n【原始旁白参考（必须恢复讲解口径与信息点，不要改写成短剧场景剧本）】\n" + ref
		}
	}
	repaired, err := s.callLLMRepair(ctx, episode, review, hints, kwLib, mode)
	if err != nil || repaired == nil || strings.TrimSpace(repaired.OptimizedText) == "" {
		if s.logger != nil && err != nil {
			s.logger.Warn("commentary script format repair failed",
				zap.Uint64("episode_id", episode.ID),
				zap.Error(err),
			)
		}
		return
	}
	episode.OptimizedText = repaired.OptimizedText
	if repaired.Title != "" {
		episode.Title = repaired.Title
	}
	if repaired.Summary != "" {
		episode.Summary = repaired.Summary
	}
}

// callLLMRepair takes the optimized text and review issues and produces a repaired version.
func (s *EpisodeService) callLLMRepair(ctx context.Context, ep *model.Episode, review *ReviewResult, writingHints string, kwLib *KeywordLibrary, mode productionmode.Mode) (*OptimizedEpisode, error) {
	// Build a focused issue list for the prompt
	var issueLines []string
	for _, issue := range review.Issues {
		if issue.Severity == "critical" || issue.Severity == "warning" {
			issueLines = append(issueLines, fmt.Sprintf("[%s/%s] %s → 建议：%s",
				issue.Severity, issue.Type, issue.Description, issue.Suggestion))
		}
	}
	issueBlock := strings.Join(issueLines, "\n")

	systemPrompt := productionmode.EpisodeRepairSystemPrompt(mode)

	if writingHints != "" {
		systemPrompt += "\n\n**本项目专属写作指引：**\n" + writingHints
	}
	if bible := buildConsistencyBibleBlock(kwLib); bible != "" {
		systemPrompt += bible
	}
	dialogueSource := firstNonEmpty(ep.OriginalExcerpt, ep.ScriptExcerpt)
	systemPrompt += "\n\n" + dialoguePreservationPromptBlock(dialogueSource)

	userContent := fmt.Sprintf("第%d集《%s》\n\n【审查发现的问题（需修复）】\n%s\n\n【当前综合评分】完整度%d 完善度%d 一致性%d 衔接%d 台词%d\n\n【需修改的剧本正文】\n%s",
		ep.EpisodeNumber, ep.Title,
		issueBlock,
		review.Score.Completeness, review.Score.Integrity, review.Score.Consistency,
		review.Score.Transitions, review.Score.DialogQuality,
		ep.OptimizedText)

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": "请根据审查意见修改以下剧本，弥补不足：\n\n" + userContent},
		},
		"temperature":     0.5,
		"max_tokens":      8192,
		"response_format": map[string]string{"type": "json_object"},
	}
	data, _ := json.Marshal(reqBody)

	repairCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(repairCtx, http.MethodPost, s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM repair request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM repair responded %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("parse LLM repair response: %w", err)
	}
	content := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result OptimizedEpisode
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse repair JSON: %w", err)
	}
	if result.OptimizedText != "" {
		result.OptimizedText = s.preserveDialogueFromSource(dialogueSource, result.OptimizedText)
	}
	return &result, nil
}

// BatchOptimizeEpisodes optimizes all episodes of a project concurrently (max 3 parallel).
func (s *EpisodeService) BatchOptimizeEpisodes(ctx context.Context, projectID uint64) (int, error) {
	episodes, err := s.episodeRepo.FindByProjectID(projectID)
	if err != nil {
		return 0, err
	}
	sem := make(chan struct{}, 3)
	var mu sync.Mutex
	var count int
	var wg sync.WaitGroup
	for i := range episodes {
		ep := episodes[i]
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if _, err := s.OptimizeEpisode(ctx, ep.ID, projectID); err == nil {
				mu.Lock()
				count++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return count, nil
}

// BatchReviewEpisodes reviews all episodes of a project concurrently (max 3 parallel).
func (s *EpisodeService) BatchReviewEpisodes(ctx context.Context, projectID uint64) (int, error) {
	episodes, err := s.episodeRepo.FindByProjectID(projectID)
	if err != nil {
		return 0, err
	}
	sem := make(chan struct{}, 3)
	var mu sync.Mutex
	var count int
	var wg sync.WaitGroup
	for i := range episodes {
		ep := episodes[i]
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if _, err := s.ReviewEpisode(ctx, ep.ID, projectID); err == nil {
				mu.Lock()
				count++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return count, nil
}
