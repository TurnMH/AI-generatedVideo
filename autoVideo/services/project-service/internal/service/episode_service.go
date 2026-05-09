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
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/autovideo/project-service/internal/model"
	"github.com/autovideo/project-service/internal/repository"
	"github.com/autovideo/project-service/internal/stylepreset"
)

type CreateEpisodeReq struct {
	EpisodeNumber int    `json:"episode_number" binding:"required,min=1"`
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	ScriptExcerpt string `json:"script_excerpt"` // optional scene content for storyboard generation
}

type EpisodeService struct {
	episodeRepo      *repository.EpisodeRepo
	projectRepo      *repository.ProjectRepo
	storyboardSvc    *StoryboardService
	characterBaseURL string
	scriptBaseURL    string // for fetching PromptTemplates
	videoBaseURL     string // for cascade-deleting video tasks on episode delete
	jwtSecret        string
	httpClient       *http.Client
	llmBaseURL       string
	llmAPIKey        string
	llmModel         string
	storageBaseURL   string
	auditor          *PromptAuditorService // prompt dedup + sensitive word + LLM review
	logger           *zap.Logger
}

type episodeContextKey string

const skipEpisodeAssetRefreshContextKey episodeContextKey = "skipEpisodeAssetRefresh"

func WithSkipEpisodeAssetRefresh(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipEpisodeAssetRefreshContextKey, true)
}

func shouldSkipEpisodeAssetRefresh(ctx context.Context) bool {
	skip, _ := ctx.Value(skipEpisodeAssetRefreshContextKey).(bool)
	return skip
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
	if s.characterBaseURL == "" || s.jwtSecret == "" {
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
	if s.characterBaseURL == "" || s.jwtSecret == "" {
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
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("extract episode assets failed: %s", strings.TrimSpace(string(body)))
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

func (s *EpisodeService) fetchAssetReferences(ctx context.Context, projectID uint64, episodeID *uint64) []assetReference {
	if s.characterBaseURL == "" || s.jwtSecret == "" {
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

	if err := s.deleteExistingAssets(ctx, projectID); err != nil {
		if s.logger != nil {
			s.logger.Error("delete existing assets before episode extraction failed; aborting re-extraction to avoid duplicates",
				zap.Uint64("project_id", projectID),
				zap.Error(err),
			)
		}
		return // don't proceed — we'd create duplicate assets
	}

	const workers = 2
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for i := range episodes {
		episode := episodes[i]
		wg.Add(1)
		sem <- struct{}{}
		go func(ep model.Episode) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := s.extractAssetsForEpisode(ctx, projectID, uint64(ep.ID)); err != nil && s.logger != nil {
				s.logger.Warn("episode asset extraction failed",
					zap.Uint64("project_id", projectID),
					zap.Uint64("episode_id", uint64(ep.ID)),
					zap.Int("episode_number", ep.EpisodeNumber),
					zap.Error(err),
				)
				return
			}
			if s.logger != nil {
				s.logger.Info("episode asset extraction completed",
					zap.Uint64("project_id", projectID),
					zap.Uint64("episode_id", uint64(ep.ID)),
					zap.Int("episode_number", ep.EpisodeNumber),
				)
			}
		}(episode)
	}
	wg.Wait()
}

// ─── Progress tracking ──────────────────────────────────────────────────────

// StageProgress tracks a single pipeline stage.
type StageProgress struct {
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Current   int    `json:"current,omitempty"`
	Status    string `json:"status"` // pending | running | done | failed
}

// ProgressInfo is the JSON structure persisted in project.progress.
type ProgressInfo struct {
	Stage        string         `json:"stage"` // episode_splitting | scene_splitting | script_prepping | idle
	EpisodeSplit *StageProgress `json:"episode_split,omitempty"`
	SceneSplit   *StageProgress `json:"scene_split,omitempty"`
	Message      string         `json:"message,omitempty"`
	StartedAt    string         `json:"started_at,omitempty"`
	UpdatedAt    string         `json:"updated_at,omitempty"`
}

const (
	keywordExtractionTimeout = 15 * time.Second
	profileEnrichmentTimeout = 120 * time.Second
	episodeSplitTimeout      = 30 * time.Second
	episodeEnrichTimeout     = 20 * time.Second
)

var ErrScreenplayNotReady = errors.New("screenplay not ready")

// updateProgress —— 将进度信息序列化并持久化到项目的 progress 字段
func (s *EpisodeService) updateProgress(projectID uint64, info ProgressInfo) {
	if info.StartedAt == "" {
		if project, err := s.projectRepo.FindByIDNoAuth(projectID); err == nil && len(project.Progress) > 0 {
			var previous ProgressInfo
			if err := json.Unmarshal(project.Progress, &previous); err == nil && previous.Stage == info.Stage {
				info.StartedAt = previous.StartedAt
			}
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

	s.updateProgress(projectID, ProgressInfo{
		Stage: "scene_splitting",
		EpisodeSplit: &StageProgress{
			Total: len(episodes), Completed: len(episodes), Status: "done",
		},
		SceneSplit: &StageProgress{
			Total: len(episodes), Completed: 0, Status: "running",
		},
		Message: "正在准备分镜拆分…",
	})

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
			} else if err := s.extractAssetsForEpisode(ctx, projectID, *episodeID); err != nil && s.logger != nil {
				s.logger.Warn("manual storyboard extraction asset pre-refresh failed",
					zap.Uint64("project_id", projectID),
					zap.Uint64("episode_id", *episodeID),
					zap.Error(err),
				)
			}
		} else {
			s.extractAssetsAfterSplit(ctx, projectID, episodes)
		}
	}

	created := s.generateStoryboardsParallelWithOffset(ctx, projectID, project.UserID, episodes, &kwLib, clipDuration, videoModel, project.ProjectType, startSequence)
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
	} else if s.characterBaseURL != "" {
		s.extractAssetsAfterSplit(ctx, projectID, episodes)
	}
	s.updateProgress(projectID, ProgressInfo{
		Stage: "idle",
		EpisodeSplit: &StageProgress{
			Total: len(episodes), Completed: len(episodes), Status: "done",
		},
		SceneSplit: &StageProgress{
			Total: len(episodes), Completed: len(episodes), Status: "done",
		},
		Message: fmt.Sprintf("分镜拆分完成，共生成 %d 条分镜", created),
	})
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
	var kwLib *KeywordLibrary
	if project, pErr := s.projectRepo.FindByIDNoAuth(projectID); pErr == nil {
		var lib KeywordLibrary
		if len(project.KeywordLibrary) > 0 {
			if jsonErr := json.Unmarshal(project.KeywordLibrary, &lib); jsonErr == nil {
				kwLib = &lib
			}
		}
	}

	polished, err := s.callLLMPolish(ctx, episode, writingHints, productionHints, kwLib)
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
	var result struct {
		Data struct {
			Items []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				IsActive    bool   `json:"is_active"`
			} `json:"items"`
		} `json:"data"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}
	var lines []string
	for _, sk := range result.Data.Items {
		if sk.IsActive && strings.TrimSpace(sk.Description) != "" {
			lines = append(lines, fmt.Sprintf("- [%s] %s", sk.Name, sk.Description))
		}
	}
	return strings.Join(lines, "\n")
}

// fetchWritingSkillHints returns active writing skills for a project.
func (s *EpisodeService) fetchWritingSkillHints(ctx context.Context, projectID uint64) string {
	return s.fetchSkillHintsByUseCase(ctx, projectID, "writing")
}

// fetchStoryboardSkillHints returns active storyboard skills for a project.
func (s *EpisodeService) fetchStoryboardSkillHints(ctx context.Context, projectID uint64) string {
	return s.fetchSkillHintsByUseCase(ctx, projectID, "storyboard")
}

// fetchScriptPrepSkillHints returns active storyboard_prep skills for a project.
func (s *EpisodeService) fetchScriptPrepSkillHints(ctx context.Context, projectID uint64) string {
	return s.fetchSkillHintsByUseCase(ctx, projectID, "storyboard_prep")
}

// prepareScriptForStoryboard runs the episode script through a professional LLM pre-optimization pass
// to make it more suitable for scene splitting and video storyboarding.
// It adds explicit visual markers, camera suggestions, character positions and pacing cues.
// The returned string is a fully annotated script ready for breakEpisodeIntoScenes.
// On any failure the original content is returned unchanged so the pipeline is never blocked.
func (s *EpisodeService) prepareScriptForStoryboard(ctx context.Context, content string, episodeNum int, kwLib *KeywordLibrary, projectType string, prepSkillHints string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}
	// Skip prep for comics — the panel prompt already handles raw text well.
	if projectType == "comics" {
		return content
	}

	// Truncate very long inputs to avoid overwhelming the LLM.
	const maxRunes = 40000
	runes := []rune(content)
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
		content = string(runes)
	}

	systemPrompt := `你是一位专业的分镜统筹导演，同时兼任影视文学编辑，拥有丰富的短剧视频制作经验。

你的任务是对给定的分集剧本进行**分镜预处理优化**，将其转化为结构清晰、视觉可执行的分镜脚本，为后续 AI 自动拆分分镜做铺垫。

**优化目标：**
1. 保持原有故事情节、对白和人物关系完整不变
2. 将隐含的视觉信息显式化，加入影视专业标注
3. 优化节奏结构，突出视觉高潮和情感转折点
4. 使每个场景的视觉元素清晰可读

**必须添加的内联标注（紧跟相关文字，不单独成行）：**
- [场景:地点描述] — 每次场景切换时标注新场景的地点与氛围（如 [场景:现代都市高楼天台,夜晚,城市灯光]）
- [人物:姓名/动作/情绪/语气/表情] — 每次角色出现、说话或状态改变时标注（如 [人物:李明/推开门/愤怒/压低声音质问/眉头紧皱牙关咬紧]）；说话场景必须标注语气和表情细节
- [摄影:景别/角度/运镜] — 关键场景的镜头建议（如 [摄影:中景/平视/缓推] [摄影:特写/仰拍/固定]）
- [构图:方式] — 构图建议（如 [构图:三分法] [构图:对称] [构图:斜线引导]）
- [氛围:描述] — 当前段落的视觉氛围基调（如 [氛围:压抑紧张] [氛围:温暖明亮]）
- [道具:物品名称] — 剧情中重要道具（如 [道具:信封] [道具:手枪]）
- [情绪:氛围词] — 段落的整体情绪基调（如 [情绪:紧张对峙] [情绪:温情暖心]）
- [节奏:快切/慢镜/停顿] — 关键情节的节奏标注（如 [节奏:快切] [节奏:慢镜回放]）
- [长镜头:秒数/动作说明] — 需要用单个连续镜头处理的场景（如 [长镜头:6s/人物走近后开口说话，摄影机缓跟，全程保持中景]）；特别用于对话场景，说明角色动作与台词如何在单个连续镜头内自然衔接，避免后续视频生成产生跳切
- [字幕:对白原文] — 【最重要】将原文中所有对话台词、角色独白用此标注紧贴在对白文字之前（如 [字幕:你为什么要离开？] [字幕:因为我已经累了。]）。凡原文出现冒号引用句、引号内容、角色发言，均需添加此标注，绝不遗漏

**结构优化：**
- 在场景切换处添加明确的转场标记
- 将长段独白或叙述拆分为行动+对白的组合
- 对关键情感节点（冲突高潮、转折、结尾悬念）加强视觉描写
- 确保每个段落都有明确的视觉主体
- 对话场景中，用[人物:姓名/动作/情绪/语气/表情]完整标注说话者的肢体状态（如微微前倾/握拳/轻柔哄劝/眼含泪光）
- 对需要人物连续说话或复杂动作的段落，用[长镜头:时长/说明]明确标注，指出应在单个连续镜头内完成，避免视频生成时出现跳切或场景衔接断裂
- 相邻分镜之间，确保动作有交代（起身→走近→开口），避免无缘由的位置跳变

**输出要求：**
- 返回纯文本格式（不要JSON），直接输出优化后的分集脚本内容
- 保持原有文字风格，只在视觉关键节点加入标注
- 字数与原文相近（不超过原文字数的1.3倍）`

	if prepSkillHints != "" {
		systemPrompt += "\n\n**本项目专属分镜预处理指引（请务必遵守）：**\n" + prepSkillHints
	}
	if bible := buildConsistencyBibleBlock(kwLib); bible != "" {
		systemPrompt += "\n\n" + bible + "\n所有标注中的人物姓名和场景描述必须与以上一致性词库保持一致。"
	}

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": fmt.Sprintf("请对第 %d 集剧本进行分镜预处理优化，添加视觉标注后返回优化后的脚本：\n\n%s", episodeNum, content)},
		},
		"temperature": 0.4,
		"max_tokens":  8192,
	}
	data, _ := json.Marshal(reqBody)

	prepCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(prepCtx, http.MethodPost, s.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return content
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

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
	return optimized
}

// fetchProductionSkillHints calls character-service to get active production department skills for a project.
// Returns a formatted string with each skill's label_tag and system_prompt for LLM injection.
func (s *EpisodeService) fetchProductionSkillHints(ctx context.Context, projectID uint64) string {
	if s.characterBaseURL == "" || s.jwtSecret == "" {
		return ""
	}
	token, err := s.buildServiceToken(projectID)
	if err != nil {
		return ""
	}
	url := fmt.Sprintf("%s/api/v1/projects/%d/production-skills", s.characterBaseURL, projectID)
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
	var result struct {
		Data struct {
			Items []struct {
				LabelTag     string `json:"label_tag"`
				Name         string `json:"name"`
				SystemPrompt string `json:"system_prompt"`
				IsActive     bool   `json:"is_active"`
			} `json:"items"`
		} `json:"data"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}
	var lines []string
	for _, sk := range result.Data.Items {
		if sk.IsActive && strings.TrimSpace(sk.SystemPrompt) != "" {
			lines = append(lines, fmt.Sprintf("【%s — 标签[%s:]】%s", sk.Name, sk.LabelTag, sk.SystemPrompt))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// fetchStoryboardPromptTemplate fetches a PromptTemplate from script-service by style_key.
func (s *EpisodeService) fetchStoryboardPromptTemplate(ctx context.Context, styleKey string) string {
	if s.scriptBaseURL == "" || strings.TrimSpace(styleKey) == "" {
		return ""
	}
	url := fmt.Sprintf("%s/api/v1/prompt-templates?style_key=%s&resource_type=storyboard&active_only=true", s.scriptBaseURL, styleKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("fetch storyboard prompt template failed", zap.String("style_key", styleKey), zap.Error(err))
		}
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var result struct {
		Data []struct {
			Content  string `json:"content"`
			IsActive bool   `json:"is_active"`
		} `json:"data"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}
	for _, t := range result.Data {
		if t.IsActive && strings.TrimSpace(t.Content) != "" {
			return t.Content
		}
	}
	return ""
}

// applyPromptTemplate replaces {scene}, {characters}, {action}, {mood} placeholders
// in a PromptTemplate content string with the given values.
func applyPromptTemplate(template, scene, characters, action, mood string) string {
	result := template
	result = strings.ReplaceAll(result, "{scene}", strings.TrimSpace(scene))
	result = strings.ReplaceAll(result, "{characters}", strings.TrimSpace(characters))
	result = strings.ReplaceAll(result, "{action}", strings.TrimSpace(action))
	result = strings.ReplaceAll(result, "{mood}", strings.TrimSpace(mood))
	return strings.TrimSpace(result)
}

// storyboardStyleKey maps a canonical stylePreset to its PromptTemplate style_key.
func storyboardStyleKey(stylePreset string) string {
	switch stylePreset {
	case stylepreset.Anime2D:
		return "storyboard_anime2d"
	case stylepreset.Anime3D:
		return "storyboard_anime3d"
	case stylepreset.LiveActionFilm:
		return "storyboard_cinematic"
	case stylepreset.LiveActionShort:
		return "storyboard_live_action"
	default:
		if stylePreset != "" {
			// Convert "live-action-film" → "storyboard_live_action_film"
			safe := strings.ToLower(strings.ReplaceAll(stylePreset, "-", "_"))
			return "storyboard_" + safe
		}
		return "storyboard_anime2d" // safe default
	}
}

type polishedEpisode struct {
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	ScriptExcerpt string `json:"script_excerpt"`
}

// callLLMPolish sends the episode to the LLM for professional rewriting.
func (s *EpisodeService) callLLMPolish(ctx context.Context, ep *model.Episode, writingHints string, productionHints string, kwLib *KeywordLibrary) (*polishedEpisode, error) {
	systemPrompt := `你是专业的短剧编剧顾问。请对给定的分集内容进行专业优化润色，返回严格JSON格式（不要markdown代码块），字段如下：
{
  "title": "优化后的集标题（简洁有力，20字以内）",
  "summary": "优化后的分集简介（100-200字，突出核心冲突和看点）",
  "script_excerpt": "优化后的分集内容（保留原有故事情节，提升可读性和戏剧张力）"
}

**优化原则：**
- 保留原有故事情节和人物关系，不要改变核心情节
- 提升语言表现力，增强戏剧张力
- 每集结构清晰：开头钩子 → 情节发展 → 结尾悬念/情感落点
- title 简洁有吸引力，可以是疑问句或关键词组合
- summary 像平台简介文案，吸引观众点击
- script_excerpt 保持原长度，重点提升场景描写和对话质量`

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

	// ── Layer 2: Status guard ──
	// If already processing, refuse to start another generation.
	if project.Status == "script_processing" && !force {
		return nil, fmt.Errorf("project is already processing, cannot start again")
	}

	// Save user-provided keywords to project if given
	if userKeywords != nil {
		if kwJSON, err := json.Marshal(userKeywords); err == nil {
			project.KeywordLibrary = kwJSON
			_ = s.projectRepo.Update(project)
		}
	}

	// Immediately mark as script_processing + initialize progress
	_ = s.projectRepo.UpdateStatus(projectID, project.UserID, "script_processing")
	s.updateProgress(projectID, ProgressInfo{
		Stage:        "episode_splitting",
		Message:      "准备中…",
		EpisodeSplit: &StageProgress{Status: "running"},
	})

	episodes, err := s.doGenerateFromScript(ctx, project, autoStoryboard)
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

	if project.Status != "script_processing" && progress.Stage != "episode_splitting" && progress.Stage != "scene_splitting" {
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
		progress.Stage == "scene_splitting", nil
}

// doGenerateFromScript —— 执行剧集生成的核心逻辑，包含数据清理、分集和分镜创建
func (s *EpisodeService) doGenerateFromScript(ctx context.Context, project *model.Project, autoStoryboard bool) ([]model.Episode, error) {
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
		_ = s.projectRepo.Update(project)
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
		_ = s.projectRepo.Update(project)
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
		_ = s.projectRepo.Update(project)
	}

	// T3C: Auto-create Skills in character-service from detected character capability hints
	if len(kwLib.CharacterProfiles) > 0 && s.characterBaseURL != "" {
		skillCtx, cancelSkills := context.WithTimeout(ctx, 15*time.Second)
		s.autoCreateCharacterSkills(skillCtx, projectID, kwLib.CharacterProfiles)
		cancelSkills()
	}
	// Priority: user keywords → chapter markers → LLM → simple fallback
	// ══════════════════════════════════════════════════════════════════════════
	s.updateProgress(projectID, ProgressInfo{
		Stage:        "episode_splitting",
		Message:      "正在拆分剧本为集数…",
		EpisodeSplit: &StageProgress{Status: "running"},
	})

	// Try user-provided split keywords first
	episodes := splitByUserKeywords(scriptText, kwLib.SplitKeywords)
	splitMethod := "user_keywords"

	// Fallback to chapter markers
	if len(episodes) == 0 {
		episodes = splitByChapters(scriptText)
		splitMethod = "chapters"
	}

	chapterSplit := len(episodes) > 0

	if s.logger != nil {
		s.logger.Info("episode split result",
			zap.Uint64("project_id", projectID),
			zap.String("method", splitMethod),
			zap.Bool("success", chapterSplit),
			zap.Int("episodes_found", len(episodes)),
			zap.Int("script_length", utf8.RuneCountInString(scriptText)),
		)
	}

	if !chapterSplit {
		targetEpisodes := project.TargetEpisodes
		if targetEpisodes <= 0 {
			wordCount := utf8.RuneCountInString(scriptText)
			targetEpisodes = wordCount / 2000
			if targetEpisodes < 1 {
				targetEpisodes = 1
			}
			if targetEpisodes > 200 {
				targetEpisodes = 200
			}
		}
		if targetEpisodes <= 1 {
			episodes = s.simpleSplit(scriptText, targetEpisodes)
		} else {
			splitCtx, cancelSplit := context.WithTimeout(ctx, episodeSplitTimeout)
			var err error
			writingHints := s.fetchWritingSkillHints(splitCtx, projectID)
			episodes, err = s.callLLMSplit(splitCtx, scriptText, targetEpisodes, &kwLib, writingHints)
			cancelSplit()
			if err != nil {
				episodes = s.simpleSplit(scriptText, targetEpisodes)
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
		s.enrichEpisodesParallel(enrichCtx, episodes, &kwLib, chapterWritingHints)
		cancelEnrich()
	}

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
	s.updateProgress(projectID, ProgressInfo{
		Stage: "script_prepping",
		EpisodeSplit: &StageProgress{
			Total: len(dbEpisodes), Completed: len(dbEpisodes), Status: "done",
		},
		SceneSplit: &StageProgress{
			Total: len(dbEpisodes), Completed: 0, Status: "pending",
		},
		Message: fmt.Sprintf("分集完成（%d 集），开始润色、格式化并串联资源与分镜…", len(dbEpisodes)),
	})

	// Post-split auto pipeline (background, sequential per episode):
	//   1. PolishEpisode    — AI润色：提升标题/摘要/内容可读性（依赖完整关键词库）
	//   2. AutoOptimizeReview — 转剧本格式 → AI审查 → 自动修复不足
	//   3. ApplyOptimizedText — 将剧本格式结果写入 script_excerpt，并串联资源提取 → 分镜提取
	go func(eps []model.Episode, pid uint64) {
		autoCtx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
		defer cancel()

		// Pre-fetch project-level data once for all episodes to avoid N×HTTP redundancy.
		autoWritingHints := s.fetchWritingSkillHints(autoCtx, pid)
		autoProductionHints := s.fetchProductionSkillHints(autoCtx, pid)
		var autoKwLib *KeywordLibrary
		if proj, pErr := s.projectRepo.FindByIDNoAuth(pid); pErr == nil {
			var lib KeywordLibrary
			if len(proj.KeywordLibrary) > 0 {
				if jsonErr := json.Unmarshal(proj.KeywordLibrary, &lib); jsonErr == nil {
					autoKwLib = &lib
				}
			}
		}

		for _, ep := range eps {
			select {
			case <-autoCtx.Done():
				return
			default:
			}
			// Step 1: polish novel text using the project keyword library.
			if _, err := s.polishEpisodeInternal(autoCtx, ep.ID, pid, autoWritingHints, autoProductionHints, autoKwLib); err != nil && s.logger != nil {
				s.logger.Warn("auto-polish episode failed", zap.Uint64("episode_id", ep.ID), zap.Error(err))
			}
			// Step 2: convert to screenplay format + AI review + repair
			updated, err := s.autoOptimizeReviewInternal(autoCtx, ep.ID, pid, autoWritingHints, autoKwLib)
			if err != nil {
				if s.logger != nil {
					s.logger.Warn("auto optimize-review episode failed", zap.Uint64("episode_id", ep.ID), zap.Error(err))
				}
				// Even if optimize-review fails, still trigger asset extraction from the
				// existing script_excerpt so downstream steps are not fully blocked.
				if s.characterBaseURL != "" {
					if extractErr := s.extractAssetsForEpisode(autoCtx, pid, ep.ID); extractErr != nil && s.logger != nil {
						s.logger.Warn("fallback asset extraction after optimize-review failure", zap.Uint64("episode_id", ep.ID), zap.Error(extractErr))
					}
				}
				continue
			}
			// Step 3: auto-apply the screenplay text as the canonical script_excerpt
			if updated.OptimizedText != "" {
				if _, applyErr := s.ApplyOptimizedText(autoCtx, ep.ID, pid); applyErr != nil && s.logger != nil {
					s.logger.Warn("auto apply optimized text failed", zap.Uint64("episode_id", ep.ID), zap.Error(applyErr))
				}
			}
		}
	}(dbEpisodes, projectID)

	// ══════════════════════════════════════════════════════════════════════════
	// Phase 3: Scene splitting per episode (storyboard generation)
	// ══════════════════════════════════════════════════════════════════════════
	/* [重构] 停止项目级自动分镜拆分
	if s.storyboardSvc != nil {
		if s.logger != nil {
			s.logger.Info("starting storyboard scene breakdown",
				zap.Uint64("project_id", projectID),
				zap.Int("episode_count", len(dbEpisodes)),
			)
		}

		totalSb := func() int {
			// Extract clip duration and video model from project's storyboard config.
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
					videoModel = cfg.VideoModel
				}
			}
			return s.generateStoryboardsParallel(ctx, projectID, project.UserID, dbEpisodes, &kwLib, clipDuration, videoModel, project.ProjectType)
		}()

		if s.logger != nil {
			s.logger.Info("storyboard generation complete",
				zap.Uint64("project_id", projectID),
				zap.Int("total_storyboards", totalSb),
			)
		}

		if autoStoryboard && totalSb > 0 && s.logger != nil {
			s.logger.Info("skip immediate storyboard image generation until assets are complete",
				zap.Uint64("project_id", projectID),
				zap.Int("storyboards_pending", totalSb),
			)
		}
	}
	*/

	// ── All done ─────────────────────────────────────────────────────────────
	s.updateProgress(projectID, ProgressInfo{
		Stage: "idle",
		EpisodeSplit: &StageProgress{
			Total: len(dbEpisodes), Completed: len(dbEpisodes), Status: "done",
		},
		SceneSplit: &StageProgress{
			Total: len(dbEpisodes), Completed: len(dbEpisodes), Status: "done",
		},
		Message: "全部完成",
	})
	_ = s.projectRepo.UpdateStatus(projectID, project.UserID, "script_ready")

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
	Appearance   string   `json:"appearance"`               // visual description in Chinese for scene descriptions
	AppearanceEN string   `json:"appearance_en,omitempty"`  // English visual description for AI image generation prompts
	VoiceHint    string   `json:"voice_hint"`               // male/female/child/narrator — aids auto-voice assignment
	SkillHints   []string `json:"skill_hints,omitempty"`    // detected capability tags: combat|exploration|social|special
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
func (s *EpisodeService) generateStoryboardsParallel(ctx context.Context, projectID, userID uint64, dbEpisodes []model.Episode, kwLib *KeywordLibrary, clipDuration int, videoModel string, projectType string) int {
	return s.generateStoryboardsParallelWithOffset(ctx, projectID, userID, dbEpisodes, kwLib, clipDuration, videoModel, projectType, 0)
}

func (s *EpisodeService) generateStoryboardsParallelWithOffset(ctx context.Context, projectID, userID uint64, dbEpisodes []model.Episode, kwLib *KeywordLibrary, clipDuration int, videoModel string, projectType string, startSequence int) int {
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
	assetRefs := s.fetchAssetReferences(ctx, projectID, nil)
	if project, err := s.projectRepo.FindByIDNoAuth(projectID); err == nil {
		sk := storyboardStyleKey(storyboardStylePreset(project))
		storyboardPromptTemplate = s.fetchStoryboardPromptTemplate(ctx, sk)
		projectVisualEra = inferVisualEra(strings.TrimSpace(project.ScriptText))
		if s.logger != nil && storyboardPromptTemplate != "" {
			s.logger.Info("fetched storyboard prompt template",
				zap.Uint64("project_id", projectID),
				zap.String("style_key", sk),
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

		go func(idx int, epID uint64, epNum int, epContent string) {
			defer wg.Done()
			defer func() { <-sem }()

			contentSource := "summary"
			switch {
			case strings.TrimSpace(ep.OptimizedText) != "":
				contentSource = "optimized_text"
			case strings.TrimSpace(ep.ScriptExcerpt) != "":
				contentSource = "script_excerpt"
			}

			if s.logger != nil {
				s.logger.Info("breaking episode into scenes",
					zap.Int("episode", epNum),
					zap.String("content_source", contentSource),
					zap.Int("content_len", utf8.RuneCountInString(epContent)),
				)
			}

			// Pre-optimization: run a professional storyboard-prep pass before scene splitting
			// to add explicit visual markers, camera suggestions and pacing cues.
			_ = s.episodeRepo.UpdateStatus(epID, "script_prepping")
			optimized := s.prepareScriptForStoryboard(ctx, epContent, epNum, kwLib, projectType, prepSkillHints)
			if s.logger != nil && optimized != epContent {
				s.logger.Info("script prep optimization applied",
					zap.Int("episode", epNum),
					zap.Int("original_len", utf8.RuneCountInString(epContent)),
					zap.Int("optimized_len", utf8.RuneCountInString(optimized)),
				)
			}

			scenes := s.breakEpisodeIntoScenes(ctx, optimized, epNum, storyboardHints, kwLib, clipDuration, videoModel, projectType)
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
			s.updateProgress(projectID, ProgressInfo{
				Stage: "scene_splitting",
				EpisodeSplit: &StageProgress{
					Total: len(dbEpisodes), Completed: len(dbEpisodes), Status: "done",
				},
				SceneSplit: &StageProgress{
					Total: len(dbEpisodes), Completed: completed, Current: epNum, Status: "running",
				},
				Message: fmt.Sprintf("正在拆分分镜 %d/%d（第%d集）", completed, len(dbEpisodes), epNum),
			})
		}(i, uint64(ep.ID), ep.EpisodeNumber, content)
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
		refinedPrompts := s.refineScenePrompts(ctx, scenes, storyboardHints, storyboardPromptTemplate, kwLib, ep.EpisodeNumber, projectType, crossEpisodePrevPrompt)
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
			if projectType == "video_serial" {
				sceneGroupKey = normalizeSceneKey(scene.Location)
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
				promptUsed = composeStoryboardPrompt(StoryboardPromptParts{
				Subject:          desc,
				CharacterAnchors: charAnchors,
				PropAnchors:      propAnchors,
				SceneAnchors:     sceneAnchors,
				CameraGrammar:    shotTypeToCameraMovement(scene.ShotType),
			})
			}

			_, err := s.storyboardSvc.Create(projectID, CreateStoryboardReq{
				EpisodeID:        &epID,
				SequenceNumber:   globalSeq,
				SceneDescription: desc,
				Characters:       chars,
				Location:         scene.Location,
				Duration:         clampDuration(scene.Duration, 2, 12),
				Dialogue:         scene.Dialogue,
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
func (s *EpisodeService) breakEpisodeIntoScenes(ctx context.Context, episodeContent string, episodeNum int, skillHints string, kwLib *KeywordLibrary, clipDuration int, videoModel string, projectType string) []llmScene {
	if strings.TrimSpace(episodeContent) == "" {
		return nil
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
				return s.fallbackSceneSplit(episodeContent, episodeNum, clipDuration)
			case <-time.After(2 * time.Second):
			}
		}

		scenes := s.callLLMSceneSplit(ctx, episodeContent, episodeNum, skillHints, kwLib, clipDuration, videoModel, projectType)
		if len(scenes) > 0 {
			return scenes
		}
	}

	if s.logger != nil {
		s.logger.Warn("LLM scene split failed after retries, using paragraph fallback",
			zap.Int("episode", episodeNum))
	}
	return s.fallbackSceneSplit(episodeContent, episodeNum, clipDuration)
}

// callLLMSceneSplit —— 调用 LLM 将剧集内容拆分为原子场景，支持长文本分块
// callLLMSceneSplit splits episode content into atomic scenes via LLM.
// Supports up to 100k chars; automatically chunks long texts at paragraph boundaries.
func (s *EpisodeService) callLLMSceneSplit(ctx context.Context, episodeContent string, episodeNum int, skillHints string, kwLib *KeywordLibrary, clipDuration int, videoModel string, projectType string) []llmScene {
	const maxChars = 100000
	if runeLen := utf8.RuneCountInString(episodeContent); runeLen > maxChars {
		episodeContent = string([]rune(episodeContent)[:maxChars])
	}

	// For long texts (>30k chars), split into chunks at paragraph boundaries
	const chunkLimit = 30000
	if utf8.RuneCountInString(episodeContent) > chunkLimit {
		return s.sceneSplitChunked(ctx, episodeContent, episodeNum, chunkLimit, skillHints, kwLib, clipDuration, videoModel, projectType)
	}

	return s.sceneSplitSingle(ctx, episodeContent, episodeNum, skillHints, kwLib, clipDuration, videoModel, projectType)
}

// sceneSplitChunked —— 将长文本按段落边界分块后逐块调用 LLM 拆分场景
// sceneSplitChunked splits long content into paragraph-aligned chunks and processes each via LLM.
func (s *EpisodeService) sceneSplitChunked(ctx context.Context, content string, episodeNum int, chunkLimit int, skillHints string, kwLib *KeywordLibrary, clipDuration int, videoModel string, projectType string) []llmScene {
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
		scenes := s.sceneSplitSingle(ctx, chunk, episodeNum, skillHints, kwLib, clipDuration, videoModel, projectType)
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
func (s *EpisodeService) sceneSplitSingle(ctx context.Context, content string, episodeNum int, skillHints string, kwLib *KeywordLibrary, clipDuration int, videoModel string, projectType string) []llmScene {
	// clipDuration is the user-configured default; we still expose it as a soft reference
	// but allow the LLM to deviate based on actual scene complexity.
	refDuration := clipDuration
	if refDuration < 3 {
		refDuration = 3
	}
	// Build model-specific duration constraint hint for the LLM.
	modelDurationHint := videoModelDurationHint(videoModel, refDuration)

	var prompt string
	if projectType == "comics" {
		// Comics panel split: static composition, no camera motion, no duration.
		prompt = fmt.Sprintf(`你是一位专业的漫画分镜师（漫画分格助手）。请将以下第 %d 集的内容拆分为最细粒度的漫画格（panel）。

**核心原则：最小化漫画格**
每一格应当是一个不可再拆分的叙事单元——即一个关键动作、一句对白、一个情绪节点或一个场景切换。

**拆分规则：**
- 每次人物动作变化、场景切换、对白转换、情绪转折都应独立为一格
- 不限制格数，根据内容自然拆分，宁多勿少
- description 用中文描述画面内容（50-150字），包含：
  ① 画面主体：人物姿态、表情、手势、位置
  ② 构图类型：如特写、半身、全身、广角、俯瞰
  ③ 背景与环境：场景细节、光线氛围、时间（日/夜）
  ④ 道具与服装细节
- shot_type 使用漫画构图类型：face-closeup / bust / full-body / wide / establishing / insert / reaction
- characters 列出该格中出现的角色名
- character_states 每个角色的姿态和情绪（name/action/emotion）
- mood：tense / romantic / comedic / sad / epic / mysterious / action / calm / dramatic
- location：场景地点（2-20字）
- duration 固定为 0（漫画格无时长）
- dialogue：该格中的对白或心理独白（保持原文；如无则留空）。原文中引号内容、冒号引用句、[字幕:]标注均必须提取到此字段，禁止遗漏

**内联标注识别（优先级最高）：**
内容中可能包含影视标注：
- [摄影:xxx] → 映射到构图类型（shot_type），融入 description 的视角描述
- [美术:xxx] → 融入 description 的背景/环境细节
- [道具:xxx] → 在 description 中明确提及该道具
- [服化:xxx] → 在人物描述中体现服装细节
- [字幕:对白内容] → 对白内容【必须】直接填入 dialogue 字段，这是 TTS 配音的唯一数据来源

请严格按以下 JSON 格式返回：
{"scenes": [
  {"description": "中文画面描述：角色1站在窗边，表情凝重，手持宝剑。中景，侧逆光，夜晚室内。", "shot_type": "bust", "characters": ["角色1"], "character_states": [{"name": "角色1", "action": "grips sword", "emotion": "determined"}], "mood": "tense", "location": "地点", "duration": 0, "dialogue": "对白"}
]}

第 %d 集内容：
%s`, episodeNum, episodeNum, content)
	} else {
		prompt = fmt.Sprintf(`你是一位专业的分镜师和摄影指导。请将以下第 %d 集的内容拆分为最细粒度的视觉场景（分镜）。

**核心原则：最小化分镜**
每个分镜应当是一个不可再拆分的最小视觉单元——即一个动作、一个表情变化、一个画面切换。

**拆分规则：**
- 每次人物动作变化、场景切换、对话转换、情绪转折都应独立为一个分镜
- 不限制分镜数量，根据内容自然拆分，宁多勿少
- description 用中文描述画面内容（50-150字），包含以下内容：
  ① 画面主体：人物位置、动作、表情、肢体语言
  ② 景别与构图：如特写、中景、全景，镜头角度
  ③ 光线与氛围：光线方向、色温、情绪基调
  ④ 环境细节：背景、道具、天气、时间（日/夜）等视觉元素
- shot_type 推荐景别：close-up / medium / full / wide / overhead / low-angle / tracking / handheld
- characters 列出该场景中出现的角色名（保持原文名称）
- character_states 列出每个角色的行为状态，每项包含 name/action/emotion（简短中英文均可）
- items 列出该场景中可见的关键道具或物品（如 ["书桌","蜡烛","宝剑"]；无则留空数组）
- mood 该场景的情绪基调，从以下选取：tense / romantic / comedic / sad / epic / mysterious / action / calm / dramatic
- location 描述场景发生的地点环境（2-20字）
- duration 该分镜的视频时长（秒数，整数）：
%s
- dialogue 该场景中的对白（保持原文语言；如无则留空字符串）

**内联标注识别（优先级最高）：**
内容中可能包含影视标注，请按以下规则映射到对应字段：
- [摄影:xxx] → 直接决定 shot_type，并将摄影指令融入 description 的构图描述中
- [灯光:xxx] → 融入 description 的光线与氛围部分（如 "warm yellow 3200K backlight"）
- [美术:xxx] → 融入 description 的环境细节部分（如 "classical study with wooden shelves"）
- [字幕:对白内容] → 对白内容【必须】直接填入 dialogue 字段，这是 TTS 配音的唯一数据来源，绝对不能遗漏或放入 description
- [道具:xxx] → 在 description 和 items 中明确提及该道具
- [服化:xxx] → 在 description 的人物描述中体现对应服装细节
- [场记:xxx] → 在 character_states 的 action 中体现连贯性要求
- [剪辑:xxx] → 影响 shot_type 或 mood（如"情绪高潮切"对应 dramatic）

**对白提取强制规则（TTS 配音关键）：**
原文中以下形式的内容必须提取到 dialogue 字段，禁止遗漏：
① [字幕:…] 标注内的全部文字 ② 引号内容（"…" 「…」'…'）③ 冒号引用句（角色名：内容）④ 角色的心理独白
若一个分镜含多句对白，全部用 \n 拼接放入 dialogue，不截断不省略

请严格按以下 JSON 格式返回：
{"scenes": [
  {"description": "中文画面描述：角色1快步走进昏暗走廊，表情紧张，四周灯光昏黄。中景，正面机位。", "shot_type": "medium", "characters": ["角色1"], "character_states": [{"name": "角色1", "action": "walking fast", "emotion": "nervous"}], "items": ["道具1"], "mood": "tense", "location": "地点", "duration": %d, "dialogue": "对白"}
]}

第 %d 集内容：
%s`, episodeNum, modelDurationHint, refDuration, episodeNum, content)
	}

	sceneSystemPrompt := "你是分镜场景拆分助手，只输出JSON，不要输出其他内容。"
	if styleHint := videoModelStyleHint(videoModel); styleHint != "" {
		sceneSystemPrompt += "\n\n" + styleHint
	}
	if skillHints != "" {
		sceneSystemPrompt += "\n\n**本项目专属分镜指引（请务必遵守）：**\n" + skillHints
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
func (s *EpisodeService) fallbackSceneSplit(episodeContent string, episodeNum int, clipDuration int) []llmScene {
	paragraphs := splitIntoParagraphs(episodeContent)

	if len(paragraphs) <= 1 {
		return nil // Caller will use the single-storyboard fallback
	}

	dur := clipDuration
	if dur <= 0 {
		dur = 5
	}

	// Each paragraph = one scene (maximum granularity, no merging)
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

		scenes = append(scenes, llmScene{
			Description: fmt.Sprintf("第%d集·场景%d：%s", episodeNum, i+1, desc),
			Characters:  nil,
			Location:    "",
			Duration:    dur,
			Dialogue:    "",
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

// refineScenePrompts calls LLM to produce cohesive, skill-injected image generation prompts
// for all scenes of a single episode. Scenes are processed in batches; the last prompt of
// each batch is passed as context to the next batch for visual continuity.
// kwLib provides character/location visual profiles for cross-scene consistency.
// After generation, all prompts are audited: sensitive words replaced, near-duplicates
// diversified, and flagged prompts rewritten by an LLM reviewer.
// Returns a slice of the same length as scenes; empty strings mean caller should fallback.
func (s *EpisodeService) refineScenePrompts(ctx context.Context, scenes []llmScene, skillHints string, promptTemplate string, kwLib *KeywordLibrary, episodeNum int, projectType string, prevEpisodeContext string) []string {
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
		prompts := s.refineScenePromptsBatch(ctx, batch, skillHints, promptTemplate, kwLib, episodeNum, start, prevPrompt, projectType)
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
func (s *EpisodeService) refineScenePromptsBatch(ctx context.Context, scenes []llmScene, skillHints string, promptTemplate string, kwLib *KeywordLibrary, episodeNum int, offset int, prevContext string, projectType string) []string {
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
		Items               string `json:"items,omitempty"`         // visible props/objects in scene
		PropVisual          string `json:"prop_visual,omitempty"`   // visual descriptions of key props from kwLib
		LightingNote        string `json:"lighting_note,omitempty"` // from [灯光:] annotations
		ArtNote             string `json:"art_note,omitempty"`      // from [美术:] annotations
		PropNote            string `json:"prop_note,omitempty"`     // from [道具:] annotations
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
14. Return ONLY a JSON object: {"prompts": ["prompt for panel 1", "prompt for panel 2", ...]}`
	} else {
		systemPrompt = `You are a professional image generation prompt engineer specializing in AI-driven video storyboards.
Your task: produce polished, optimized image generation prompts for a sequence of storyboard scenes that will be used BOTH as reference images AND as video generation seeds.

━━━━━━━━━━━━━━━━━━━━━━━━
PROMPT STRUCTURE — every scene prompt must follow this 6-layer order (comma-separated):
━━━━━━━━━━━━━━━━━━━━━━━━
① Subject anchor: character(s) + exact frame position (left/center/right of frame) + posture/stance
② Face & expression: specific muscle-level descriptor (e.g., "jaw slightly dropped, eyebrows raised, pupils dilated" — NOT "surprised face")
③ Action beat: what the character is physically doing right now (verb + visible result)
④ Camera & depth: shot type + lens (e.g., "medium close-up, 85mm portrait lens, shallow DOF, subject sharp, background softly blurred")
⑤ Environment layers: foreground element + midground set detail + background atmosphere (all 3 planes present)
⑥ Light & style: key light direction + color temperature + fill/rim ratio + grade keywords

━━━━━━━━━━━━━━━━━━━━━━━━
Rules:
━━━━━━━━━━━━━━━━━━━━━━━━
1. Each prompt must be 80-220 words, entirely in English, for AI image generators (Stable Diffusion / Flux / DALL-E).
2. All 6 structural layers must appear in every prompt — never omit face/expression or environment layers.
3. When a scene has "character_appearance" data, embed those EXACT visual descriptors (hair color, clothing color/texture, face features) — NEVER invent different appearances.
4. When a scene has "character_emotions" data, translate to specific micro-expression descriptors: muscles, gaze direction, lip shape, brow tension — NOT emotion adjectives alone.
5. When a scene has "location_description" data, use that EXACT environment description as the scene background — NEVER invent different architectural details, color temperature, or set dressing.
6. When a scene has "items" or "prop_visual" data, place those specific props/objects at a specific depth plane (foreground/midground) with their exact described appearance.
7. VISUAL CONTINUITY RULES (critical for video generation):
   - Adjacent scenes sharing the same "location" MUST describe identical background architecture, furniture placement, and ambient lighting color temperature.
   - Characters appearing across multiple scenes MUST wear exactly the same clothing and hairstyle unless explicitly changed.
   - If the VISUAL BRIDGE prompt is provided, your FIRST scene must visually extend from it: same color grading, same character frame position, matching depth planes.
   - Frame position consistency: if a character is on the left in scene N, keep them left in scene N+1 unless a motivated move is described.
8. Do NOT reference dialogue, narration, or story plot — only describe what is VISIBLE.
9. Incorporate provided art style and skill guidelines naturally into every prompt.
10. The "shot_type" field dictates framing: close-up → face fills 60%+ of frame; medium → waist-up, environment visible; wide → full figure + environment; establishing → location dominant, figure small.
11. If "lighting_note" is present, incorporate the EXACT color temperature, direction, and quality (hard/soft, spot/fill ratio).
12. If "art_note" is present, use it to describe scene environment and set dressing accurately.
13. Preserve explicit era / period / costume cues. Dynasty/scifi/modern/retro setting details must be kept and strengthened.
14. VIDEO-FRIENDLY COMPOSITION: avoid cluttered mid-ground; keep subject-background separation clean for AI video motion to work well.
15. Return ONLY a JSON object: {"prompts": ["prompt for scene 1", "prompt for scene 2", ...]}`
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
		"Generate optimized image generation prompts for %d storyboard scenes from episode %d.%s%s\n\nIMPORTANT: The scenes are ordered. For each scene after the first, visually bridge its prompt from the preceding scene to maintain seamless flow.\n\nScenes (JSON):\n%s\n\nReturn JSON: {\"prompts\": [\"prompt1\", \"prompt2\", ...]}",
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
func (s *EpisodeService) callLLMSplit(ctx context.Context, scriptText string, targetEpisodes int, kwLib *KeywordLibrary, writingHints string) ([]llmEpisode, error) {
	// For very large episode counts, split via multiple LLM calls in batches
	if targetEpisodes > 30 {
		return s.callLLMSplitBatched(ctx, scriptText, targetEpisodes, kwLib, writingHints)
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
func (s *EpisodeService) callLLMSplitBatched(ctx context.Context, scriptText string, targetEpisodes int, kwLib *KeywordLibrary, writingHints string) ([]llmEpisode, error) {
	if s.logger != nil {
		s.logger.Info("using batched split for large episode count",
			zap.Int("target", targetEpisodes))
	}

	// Start with a simple proportional split to get text segments
	segments := s.simpleSplit(scriptText, targetEpisodes)

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
				title, summary, kws := s.enrichEpisodeWithLLM(ctx, ep.Title, ep.Excerpt, idx+1, kwLib, writingHints)
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
func (s *EpisodeService) enrichEpisodesParallel(ctx context.Context, episodes []llmEpisode, kwLib *KeywordLibrary, writingHints string) {
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
				title, summary, kws := s.enrichEpisodeWithLLM(ctx, ep.Title, ep.Excerpt, idx+1, kwLib, writingHints)
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
func (s *EpisodeService) enrichEpisodeWithLLM(ctx context.Context, chapterTitle, excerpt string, episodeNum int, kwLib *KeywordLibrary, writingHints string) (string, string, []string) {
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

// enrichSceneDescription enhances the raw LLM scene description with era/mood atmosphere,
// character appearance anchors and continuity notes so adjacent storyboard descriptions stay coherent.
func enrichSceneDescription(scene llmScene, prevScene *llmScene, kwLib *KeywordLibrary, eraHint string) string {
	desc := strings.TrimSpace(scene.Description)
	if desc == "" {
		return desc
	}

	var extras []string
	if eraHint != "" {
		extras = append(extras, eraHint)
	}

	appearanceMap := buildCharacterAppearanceMap(kwLib)
	if len(scene.Characters) > 0 {
		var appearanceNotes []string
		for _, name := range scene.Characters {
			if app := strings.TrimSpace(appearanceMap[name]); app != "" {
				appearanceNotes = append(appearanceNotes, fmt.Sprintf("%s保持%s", name, app))
			}
		}
		if len(appearanceNotes) > 0 {
			extras = append(extras, "人物造型："+strings.Join(appearanceNotes, "；")+"。")
		}
	}

	if prevScene != nil {
		sharedLocation := strings.TrimSpace(prevScene.Location) != "" && strings.TrimSpace(prevScene.Location) == strings.TrimSpace(scene.Location)
		sharedCharacters := 0
		prevChars := map[string]struct{}{}
		for _, name := range prevScene.Characters {
			prevChars[strings.TrimSpace(name)] = struct{}{}
		}
		for _, name := range scene.Characters {
			if _, ok := prevChars[strings.TrimSpace(name)]; ok {
				sharedCharacters++
			}
		}
		if sharedLocation || sharedCharacters > 0 {
			bridge := "镜头衔接：承接上一镜头"
			if sharedLocation {
				bridge += "，保持同一场景空间方位、布景陈设与光线色温"
			}
			if sharedCharacters > 0 {
				bridge += "，人物朝向、站位、服装发型与动作延续需连贯"
			}
			bridge += "。"
			extras = append(extras, bridge)
		}
		if itemsNote := buildSharedItemsNote(prevScene, scene); itemsNote != "" {
			extras = append(extras, itemsNote)
		}
	}

	var moodExtras []string
	if mood := strings.TrimSpace(scene.Mood); mood != "" {
		moodCue := map[string]string{
			"tense":      "氛围：紧张压迫，强烈对比光影。",
			"romantic":   "氛围：温馨浪漫，暖金色柔光。",
			"comedic":    "氛围：明快轻松，均匀柔和光线。",
			"sad":        "氛围：低沉悲伤，冷色去饱和。",
			"epic":       "氛围：史诗宏大，强烈侧光，壮阔场景。",
			"mysterious": "氛围：神秘诡谲，低调光影，暗部丰富。",
			"action":     "氛围：动感迅猛，动态模糊，高能量构图。",
			"calm":       "氛围：平和宁静，自然柔光，舒缓节奏。",
			"dramatic":   "氛围：戏剧张力，高反差，情绪强烈。",
		}[mood]
		if moodCue != "" {
			moodExtras = append(moodExtras, moodCue)
		}
	}

	if len(scene.CharacterStates) > 0 {
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
				stateParts = append(stateParts, cs.Name+": "+strings.Join(parts, "，"))
			}
		}
		if len(stateParts) > 0 {
			moodExtras = append(moodExtras, "角色状态——"+strings.Join(stateParts, "；")+"。")
		}
	}

	extras = append(extras, moodExtras...)
	if len(extras) == 0 {
		return desc
	}
	return desc + " " + strings.Join(extras, " ")
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

	// Include text before first keyword if substantial
	if markers[0].pos > 100 {
		preText := strings.TrimSpace(text[:markers[0].pos])
		if preText != "" {
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

// splitByChapters detects chapter markers in the text and splits by chapter boundaries.
// Supports common Chinese formats: 第X回, 第X章, 第X节, 第X集, 第X卷, 第X幕
// as well as: Chapter X, CHAPTER X, 序章, 楔子, 尾声, etc.
func splitByChapters(text string) []llmEpisode {
	// Regex matches common chapter heading patterns at the start of a line
	chapterRe := regexp.MustCompile(
		`(?m)^[　 \t]*(` +
			`第[零一二三四五六七八九十百千万\d]+[回章节集卷幕]` + // 第X回/章/节/集/卷/幕
			`|Chapter\s+\d+` + // Chapter 1
			`|CHAPTER\s+\d+` + // CHAPTER 1
			`|序[章言幕]` + // 序章/序言/序幕
			`|楔\s*子` + // 楔子
			`|引\s*子` + // 引子
			`|尾\s*声` + // 尾声
			`|终\s*章` + // 终章
			`|番\s*外` + // 番外
			`)[　 \t]*(.*)$`,
	)

	matches := chapterRe.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		// No chapter markers found
		return nil
	}

	var episodes []llmEpisode
	for i, loc := range matches {
		start := loc[0]
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}

		chapterText := strings.TrimSpace(text[start:end])
		if chapterText == "" {
			continue
		}

		// Extract title from the first line (the chapter heading)
		firstNewline := strings.IndexAny(chapterText, "\n\r")
		title := chapterText
		if firstNewline > 0 {
			title = strings.TrimSpace(chapterText[:firstNewline])
		}
		// Limit title length
		if utf8.RuneCountInString(title) > 50 {
			title = string([]rune(title)[:50])
		}

		// Generate summary from first ~200 chars of body (after title)
		body := chapterText
		if firstNewline > 0 && firstNewline < len(chapterText) {
			body = strings.TrimSpace(chapterText[firstNewline:])
		}
		summary := body
		if utf8.RuneCountInString(summary) > 200 {
			summary = string([]rune(summary)[:200]) + "..."
		}

		episodes = append(episodes, llmEpisode{
			Title:   title,
			Summary: summary,
			Excerpt: chapterText,
		})
	}

	return episodes
}

// simpleSplit —— 降级方案：将剧本按字数均匀切分为 N 集
// simpleSplit divides the script into N roughly equal parts as a fallback.
func (s *EpisodeService) simpleSplit(scriptText string, n int) []llmEpisode {
	runes := []rune(scriptText)
	total := len(runes)
	chunkSize := total / n
	if chunkSize < 100 {
		chunkSize = total
		n = 1
	}

	var episodes []llmEpisode
	for i := 0; i < n; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if i == n-1 || end > total {
			end = total
		}
		excerpt := string(runes[start:end])
		// Find a reasonable title from the first line
		title := fmt.Sprintf("第%d集", i+1)
		lines := strings.SplitN(excerpt, "\n", 3)
		if len(lines) > 0 && utf8.RuneCountInString(lines[0]) <= 20 && lines[0] != "" {
			title = strings.TrimSpace(lines[0])
		}
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
	return episodes
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
	result, err := s.callLLMOptimize(ctx, episode, writingHints, kwLib)
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
	if err := s.extractAssetsForEpisode(ctx, projectID, id); err != nil {
		return nil, fmt.Errorf("apply optimized text trigger assets: %w", err)
	}
	if s.logger != nil {
		s.logger.Info("applied optimized text and triggered asset extraction",
			zap.Uint64("project_id", projectID),
			zap.Uint64("episode_id", id),
		)
	}
	return episode, nil
}

func (s *EpisodeService) callLLMOptimize(ctx context.Context, ep *model.Episode, writingHints string, kwLib *KeywordLibrary) (*OptimizedEpisode, error) {
	systemPrompt := `你是专业的短剧剧本改编专家。请将给定的小说/故事文本改编为标准剧本格式，返回严格 JSON（不要 markdown 代码块）：
{
  "title": "集标题（简洁有力，20字以内）",
  "summary": "分集简介（100-200字，突出核心冲突和看点）",
  "optimized_text": "标准剧本格式正文"
}

**剧本格式规范：**
场景用【场景标题】开头，格式：【内景/外景 · 地点 · 时间段】
动作描述：简洁描述人物动作与环境，不超过3行
台词格式：
角色名（表情/情绪/状态）
　　台词内容

**改编要求：**
- 保留原有故事情节和人物关系，不得改变核心情节
- 每个场景清晰标注内外景、地点、时间
- 台词自然流畅，符合角色性格
- 场景间衔接顺畅，有明确的镜头感
- 每集结构：开头钩子 → 情节发展 → 结尾悬念/情感落点
- 人物名称、外貌、性格前后严格一致`

	if writingHints != "" {
		systemPrompt += "\n\n**本项目专属指引（务必遵守）：**\n" + writingHints
	}
	if bible := buildConsistencyBibleBlock(kwLib); bible != "" {
		systemPrompt += bible
	}

	userContent := fmt.Sprintf("第%d集《%s》\n\n【当前简介】\n%s\n\n【原始文本】\n%s",
		ep.EpisodeNumber, ep.Title, ep.Summary, ep.ScriptExcerpt)

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": "请将以下分集改编为标准剧本格式：\n\n" + userContent},
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
			Message struct{ Content string `json:"content"` } `json:"message"`
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
	return &result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ScriptReview — AI 审查：人物/场景/道具一致性、衔接性、台词质量
// ─────────────────────────────────────────────────────────────────────────────

type ReviewScore struct {
	Completeness  int `json:"completeness"`  // 完整度 0-100
	Integrity     int `json:"integrity"`     // 完善度 0-100
	Consistency   int `json:"consistency"`   // 一致性 0-100
	Transitions   int `json:"transitions"`   // 衔接性 0-100
	DialogQuality int `json:"dialog_quality"` // 台词质量 0-100
}

type ReviewIssue struct {
	Severity    string `json:"severity"`    // critical | warning | info
	Type        string `json:"type"`        // character_inconsistency | prop_inconsistency | scene_transition | dialog | plot_gap
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

	result, err := s.callLLMReview(ctx, episode, textToReview, kwLib)
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

func (s *EpisodeService) callLLMReview(ctx context.Context, ep *model.Episode, text string, kwLib *KeywordLibrary) (*ReviewResult, error) {
	systemPrompt := `你是专业的短剧剧本审稿专家。请对给定的剧本内容进行全面AI审查，返回严格 JSON（不要 markdown 代码块）：
{
  "score": {
    "completeness": 85,
    "integrity": 90,
    "consistency": 72,
    "transitions": 80,
    "dialog_quality": 78
  },
  "issues": [
    {
      "severity": "critical",
      "type": "character_inconsistency",
      "description": "具体问题描述",
      "suggestion": "修改建议"
    }
  ],
  "overall": "总体评价（1-2句）",
  "strengths": "剧本亮点"
}

**审查维度说明：**
- completeness（完整度）：剧情是否完整，有无缺失情节
- integrity（完善度）：人物塑造是否立体，细节是否充分
- consistency（一致性）：人物外貌/性格/称谓、道具前后是否一致，场景设定是否自洽
- transitions（衔接性）：场景间切换是否自然，时间线是否清晰
- dialog_quality（台词质量）：台词是否自然、符合角色性格、避免说明文式对白

**issue 类型枚举：**
character_inconsistency | prop_inconsistency | scene_transition | dialog | plot_gap | timeline | other

**severity 枚举：** critical（严重，需修改）| warning（建议修改）| info（小建议）

**请着重检查：**
1. 同一角色的外貌描述、性格、称谓在不同场景是否前后一致
2. 重要道具/物品的出现逻辑是否合理
3. 场景切换是否有明确过渡，时间跳跃是否交代清楚
4. 台词是否符合人物身份和当前情绪
5. 情节有无明显逻辑漏洞`

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
			Message struct{ Content string `json:"content"` } `json:"message"`
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
		if v < 0 { return 0 }
		if v > 100 { return 100 }
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

	reviewResult, reviewErr := s.callLLMReview(ctx, episode, textToReview, kwLib)
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
		repaired, repairErr := s.callLLMRepair(ctx, episode, reviewResult, writingHints, kwLib)
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
	polished, err := s.callLLMPolish(ctx, episode, writingHints, productionHints, kwLib)
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
	result, err := s.callLLMOptimize(ctx, episode, writingHints, kwLib)
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

	reviewResult, reviewErr := s.callLLMReview(ctx, episode, textToReview, kwLib)
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
		repaired, repairErr := s.callLLMRepair(ctx, episode, reviewResult, writingHints, kwLib)
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

	if err := s.episodeRepo.Update(episode); err != nil {
		return nil, fmt.Errorf("save auto-optimize-review: %w", err)
	}
	return episode, nil
}

// callLLMRepair takes the optimized text and review issues and produces a repaired version.
func (s *EpisodeService) callLLMRepair(ctx context.Context, ep *model.Episode, review *ReviewResult, writingHints string, kwLib *KeywordLibrary) (*OptimizedEpisode, error) {
	// Build a focused issue list for the prompt
	var issueLines []string
	for _, issue := range review.Issues {
		if issue.Severity == "critical" || issue.Severity == "warning" {
			issueLines = append(issueLines, fmt.Sprintf("[%s/%s] %s → 建议：%s",
				issue.Severity, issue.Type, issue.Description, issue.Suggestion))
		}
	}
	issueBlock := strings.Join(issueLines, "\n")

	systemPrompt := `你是专业的短剧剧本修改专家。请根据审查意见对剧本进行针对性修改，弥补不足、保留优点，返回严格 JSON（不要 markdown 代码块）：
{
  "title": "集标题（可保持不变或优化）",
  "summary": "分集简介（可保持不变或优化）",
  "optimized_text": "修改后的完整剧本格式正文"
}

**修改要求：**
- 严格按照审查意见修复 critical 和 warning 级别问题
- 保持场景标题格式：【内景/外景 · 地点 · 时间段】
- 台词格式：角色名（表情）\n　　台词内容
- 不得改变核心情节，只修改有问题的部分
- 保留原有亮点和已写好的场景`

	if writingHints != "" {
		systemPrompt += "\n\n**本项目专属写作指引：**\n" + writingHints
	}
	if bible := buildConsistencyBibleBlock(kwLib); bible != "" {
		systemPrompt += bible
	}

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
			Message struct{ Content string `json:"content"` } `json:"message"`
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
