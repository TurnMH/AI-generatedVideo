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
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/autovideo/character-service/internal/model"
	"github.com/autovideo/character-service/internal/repository"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ExtractService extracts assets from script text using LLM.
type ExtractService struct {
	assetSvc          *AssetService
	skillRepo         repository.SkillRepository
	llmBaseURL        string
	llmAPIKey         string
	llmModel          string
	llmTimeout        time.Duration
	projectServiceURL string
	jwtSecret         string
	logger            *zap.Logger
}

// NewExtractService —— 创建资源提取服务实例，返回 *ExtractService
func NewExtractService(
	assetSvc *AssetService,
	skillRepo repository.SkillRepository,
	llmBaseURL, llmAPIKey, llmModel string,
	llmTimeout time.Duration,
	projectServiceURL, jwtSecret string,
	logger *zap.Logger,
) *ExtractService {
	return &ExtractService{
		assetSvc:          assetSvc,
		skillRepo:         skillRepo,
		llmBaseURL:        strings.TrimRight(llmBaseURL, "/"),
		llmAPIKey:         llmAPIKey,
		llmModel:          llmModel,
		llmTimeout:        llmTimeout,
		projectServiceURL: strings.TrimRight(projectServiceURL, "/"),
		jwtSecret:         jwtSecret,
		logger:            logger,
	}
}

func (s *ExtractService) buildServiceToken(projectID uint64) (string, error) {
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

// ResumeStaleExtractions restarts project/episode extraction flows that were
// interrupted before the sentinel asset could be cleared.
func (s *ExtractService) ResumeStaleExtractions(ctx context.Context, limit int) (int, error) {
	sentinels, err := s.assetSvc.repo.FindActiveExtractionSentinels(limit)
	if err != nil {
		return 0, err
	}
	resumed := 0
	for _, sentinel := range sentinels {
		jwtToken, tokenErr := s.buildServiceToken(sentinel.ProjectID)
		if tokenErr != nil {
			if s.logger != nil {
				s.logger.Warn("resume stale extraction skipped: build token failed",
					zap.Uint64("project_id", sentinel.ProjectID),
					zap.Error(tokenErr),
				)
			}
			continue
		}
		resumed++
		if len(sentinel.EpisodeIDs) == 1 {
			episodeID := uint64(sentinel.EpisodeIDs[0])
			go func(projectID, episodeID uint64, token string) {
				resumeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
				defer cancel()
				if s.logger != nil {
					s.logger.Info("resuming interrupted episode asset extraction",
						zap.Uint64("project_id", projectID),
						zap.Uint64("episode_id", episodeID),
					)
				}
				if _, resumeErr := s.ExtractFromEpisode(resumeCtx, projectID, episodeID, token); resumeErr != nil && s.logger != nil {
					s.logger.Warn("resume interrupted episode extraction failed",
						zap.Uint64("project_id", projectID),
						zap.Uint64("episode_id", episodeID),
						zap.Error(resumeErr),
					)
				}
			}(sentinel.ProjectID, episodeID, jwtToken)
			continue
		}
		go func(projectID uint64, token string) {
			resumeCtx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
			defer cancel()
			if s.logger != nil {
				s.logger.Info("resuming interrupted project asset extraction", zap.Uint64("project_id", projectID))
			}
			if err := s.assetSvc.repo.DeleteByProjectID(projectID); err != nil {
				if s.logger != nil {
					s.logger.Warn("resume interrupted project extraction skipped: clear assets failed",
						zap.Uint64("project_id", projectID),
						zap.Error(err),
					)
				}
				return
			}
			if _, resumeErr := s.ExtractFromProject(resumeCtx, projectID, token); resumeErr != nil && s.logger != nil {
				s.logger.Warn("resume interrupted project extraction failed",
					zap.Uint64("project_id", projectID),
					zap.Error(resumeErr),
				)
			}
		}(sentinel.ProjectID, jwtToken)
	}
	return resumed, nil
}

type extractedAsset struct {
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	EpisodeIDs  []int64 `json:"-"` // populated during extraction, not from LLM
}

type llmExtractResult struct {
	Assets []extractedAsset `json:"assets"`
}

func appendUniqueEpisodeIDs(existing []int64, extra []int64) []int64 {
	seen := make(map[int64]struct{}, len(existing)+len(extra))
	merged := make([]int64, 0, len(existing)+len(extra))
	for _, id := range existing {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		merged = append(merged, id)
	}
	for _, id := range extra {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		merged = append(merged, id)
	}
	return merged
}

func mergeExtractedDescription(existing, extracted string) string {
	existing = strings.TrimSpace(existing)
	extracted = strings.TrimSpace(extracted)

	// 不再需要分离视觉基调，因为视觉基调不再存储在 description 中
	// 直接比较和合并描述
	switch {
	case existing == "":
		return extracted
	case extracted == "":
		return existing
	case strings.Contains(existing, extracted):
		return existing
	case strings.Contains(extracted, existing):
		return extracted
	default:
		// Keep the more detailed description (longer wins).
		if utf8.RuneCountInString(extracted) > utf8.RuneCountInString(existing) {
			return extracted
		}
		return existing
	}
}

func (s *ExtractService) upsertExtractedAsset(projectID uint64, assetType, name, description string, episodeIDs []int64) (*model.Asset, error) {
	// 注意：不再将视觉基调存储到 description 中
	// 视觉基调在生成图片时动态从项目配置获取
	// 这样可以避免每个资产都存储大量重复的时代/风格描述
	existing, err := s.assetSvc.repo.FindByProjectTypeAndName(projectID, assetType, name)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err == nil && existing != nil {
		existing.Description = mergeExtractedDescription(existing.Description, description)
		existing.EpisodeIDs = appendUniqueEpisodeIDs(existing.EpisodeIDs, episodeIDs)
		existing.UpdatedAt = time.Now()
		if updateErr := s.assetSvc.repo.Update(existing); updateErr != nil {
			return nil, updateErr
		}
		return existing, nil
	}

	asset := &model.Asset{
		ProjectID:   projectID,
		Type:        assetType,
		Name:        name,
		Description: description,
		Status:      "pending",
		EpisodeIDs:  episodeIDs,
	}
	if createErr := s.assetSvc.Create(asset); createErr != nil {
		return nil, createErr
	}
	return asset, nil
}

func (s *ExtractService) triggerStoryboardExtraction(ctx context.Context, projectID uint64, episodeID *uint64, jwtToken string) error {
	if strings.TrimSpace(s.projectServiceURL) == "" {
		return nil
	}

	path := fmt.Sprintf("/api/v1/projects/%d/episodes/extract-storyboards", projectID)
	fields := []zap.Field{zap.Uint64("project_id", projectID), zap.Bool("single_episode", episodeID != nil)}
	if episodeID != nil {
		path = fmt.Sprintf("/api/v1/projects/%d/episodes/%d/extract-storyboards", projectID, *episodeID)
		fields = append(fields, zap.Uint64("episode_id", *episodeID))
	}

	url := s.projectServiceURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	if strings.TrimSpace(jwtToken) != "" {
		req.Header.Set("Authorization", "Bearer "+jwtToken)
	}
	req.Header.Set("Content-Type", "application/json")
	if episodeID != nil {
		req.Header.Set("X-Autovideo-Skip-Asset-Refresh", "true")
	}

	client := &http.Client{Timeout: 20 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("trigger storyboard extraction failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if s.logger != nil {
		s.logger.Info("triggered storyboard extraction after asset extraction", fields...)
	}
	return nil
}

// ExtractFromProject —— 从项目剧本和分集内容中通过 LLM 提取资源（角色/场景/道具），支持分块并行提取和去重
// ExtractFromProject fetches the script text and episode data from project-service and extracts assets via LLM.
// For long scripts, it splits into chunks, extracts from each chunk separately, then deduplicates.
// A sentinel asset with status "extracting" is created at the start so the frontend can track progress.
func (s *ExtractService) ExtractFromProject(ctx context.Context, projectID uint64, jwtToken string) ([]model.Asset, error) {
	// 0. Clear any stale sentinel from a previous interrupted extraction, then create a fresh one.
	s.assetSvc.DeleteSentinel(projectID)
	sentinel := &model.Asset{
		ProjectID:   projectID,
		Type:        "character",
		Name:        "__extracting__",
		Description: "资源提取进行中…",
		Status:      "extracting",
	}
	if err := s.assetSvc.Create(sentinel); err != nil {
		s.logger.Warn("failed to create sentinel asset", zap.Error(err))
	}
	sentinelID := sentinel.ID

	// Helper to clean up sentinel
	removeSentinel := func() {
		if sentinelID > 0 {
			_ = s.assetSvc.Delete(sentinelID)
		}
	}

	// 1. Fetch project profile to get script text and project-level visual guidance
	projectProfile, err := fetchProjectVisualProfile(ctx, s.projectServiceURL, projectID, jwtToken)
	if err != nil {
		removeSentinel()
		return nil, fmt.Errorf("fetch project profile: %w", err)
	}
	scriptText := projectProfile.ScriptText
	projectVisualHint := buildProjectVisualHint(projectProfile)

	// 2. Fetch episodes — prefer episode-based chunks over raw script splitting
	episodeSummaries := s.fetchEpisodeSummaries(ctx, projectID, jwtToken)

	if strings.TrimSpace(scriptText) == "" && len(episodeSummaries) == 0 {
		removeSentinel()
		return nil, fmt.Errorf("project %d has no script text", projectID)
	}

	// 3. Build text chunks for extraction (each chunk tracks which episodes it contains)
	type extractionChunk struct {
		Text       string
		EpisodeIDs []int64
	}
	var extractChunks []extractionChunk

	if len(episodeSummaries) > 0 {
		// Episode-based chunking with episode ID tracking
		type chunkBuilder struct {
			text       strings.Builder
			length     int
			episodeIDs []int64
		}
		var current chunkBuilder
		var result []extractionChunk

		flushCurrent := func() {
			if current.text.Len() > 0 {
				result = append(result, extractionChunk{Text: current.text.String(), EpisodeIDs: current.episodeIDs})
				current = chunkBuilder{}
			}
		}

		for _, ep := range episodeSummaries {
			content := ep.Excerpt
			if strings.TrimSpace(content) == "" {
				content = ep.Summary
			}
			if strings.TrimSpace(content) == "" {
				continue
			}
			epText := fmt.Sprintf("【第%d集: %s】\n%s\n\n", ep.Number, ep.Title, content)
			epRunes := utf8.RuneCountInString(epText)

			if epRunes > chunkMaxChars {
				flushCurrent()
				result = append(result, extractionChunk{Text: epText, EpisodeIDs: []int64{int64(ep.ID)}})
				continue
			}
			if current.length+epRunes > chunkMaxChars {
				flushCurrent()
			}
			current.text.WriteString(epText)
			current.length += epRunes
			current.episodeIDs = append(current.episodeIDs, int64(ep.ID))
		}
		flushCurrent()
		extractChunks = result
	}

	if len(extractChunks) == 0 {
		// Fallback: raw script chunking (no episode association)
		for _, c := range splitTextIntoChunks(scriptText, chunkMaxChars, 500) {
			extractChunks = append(extractChunks, extractionChunk{Text: c})
		}
	}

	s.logger.Info("starting chunked asset extraction",
		zap.Uint64("project_id", projectID),
		zap.Int("chunks", len(extractChunks)),
	)

	// Build extraction skill hints once (fetched from DB, injected into each LLM call)
	skillHints := s.buildExtractionSkillHints(projectID)

	// 4. Extract from each chunk in parallel (up to 3 concurrent LLM calls)
	type chunkResult struct {
		idx        int
		assets     []extractedAsset
		episodeIDs []int64
		err        error
	}

	const extractWorkers = 3
	results := make(chan chunkResult, len(extractChunks))
	sem := make(chan struct{}, extractWorkers)
	var wg sync.WaitGroup

	for i, ec := range extractChunks {
		wg.Add(1)
		sem <- struct{}{} // Acquire worker slot
		go func(idx int, chunkText string, epIDs []int64) {
			defer wg.Done()
			defer func() { <-sem }()
			extracted, err := s.callLLMExtract(ctx, chunkText, projectVisualHint, skillHints)
			if err != nil {
				results <- chunkResult{idx: idx, err: err}
				return
			}
			results <- chunkResult{idx: idx, assets: extracted.Assets, episodeIDs: epIDs}
		}(i, ec.Text, ec.EpisodeIDs)
	}

	// Close results channel when all workers finish
	go func() {
		wg.Wait()
		close(results)
	}()

	var allExtracted []extractedAsset
	for r := range results {
		if r.err != nil {
			s.logger.Warn("chunk extraction failed, skipping",
				zap.Int("chunk", r.idx+1),
				zap.Int("total_chunks", len(extractChunks)),
				zap.Error(r.err),
			)
			continue
		}
		// Tag each extracted asset with the episode IDs from its chunk
		for i := range r.assets {
			r.assets[i].EpisodeIDs = r.episodeIDs
		}
		allExtracted = append(allExtracted, r.assets...)
	}

	// 5. Deduplicate by normalized (type + name)
	deduped := deduplicateAssets(allExtracted)

	s.logger.Info("extraction complete after dedup",
		zap.Int("raw_count", len(allExtracted)),
		zap.Int("deduped_count", len(deduped)),
	)

	// 6. Create Asset records
	var created []model.Asset
	for _, ea := range deduped {
		assetType := normalizeAssetType(ea.Type)
		if assetType == "" {
			continue
		}
		cleanedDesc := ea.Description
		if assetType == "scene" || assetType == "prop" {
			cleanedDesc = stripCharacterMentions(assetType, cleanedDesc)
		}
		asset := &model.Asset{
			ProjectID:   projectID,
			Type:        assetType,
			Name:        ea.Name,
			// 不再存储视觉基调到 description
			Description: cleanedDesc,
			Status:      "pending",
			EpisodeIDs:  ea.EpisodeIDs,
		}
		if err := s.assetSvc.Create(asset); err != nil {
			s.logger.Warn("create asset failed", zap.String("name", ea.Name), zap.Error(err))
			continue
		}
		created = append(created, *asset)
	}

	// 7. Remove sentinel — extraction complete
	removeSentinel()

	s.logger.Info("assets created",
		zap.Uint64("project_id", projectID),
		zap.Int("count", len(created)),
	)

	// Auto-match voices for all project assets after extraction
	if _, err := s.assetSvc.AutoMatchVoices(projectID); err != nil {
		s.logger.Warn("auto-match voices after project extraction", zap.Error(err))
	}
	if err := s.triggerStoryboardExtraction(ctx, projectID, nil, jwtToken); err != nil {
		s.logger.Warn("auto trigger project storyboard extraction after asset extraction failed",
			zap.Uint64("project_id", projectID),
			zap.Error(err),
		)
	}

	return created, nil
}

// ExtractFromEpisode —— 从单集剧本内容中通过 LLM 提取资源
// ExtractFromEpisode extracts assets from a single episode's content via LLM.
func (s *ExtractService) ExtractFromEpisode(ctx context.Context, projectID, episodeID uint64, jwtToken string) ([]model.Asset, error) {
	if err := s.assetSvc.repo.ClearEpisodeAssets(projectID, episodeID); err != nil {
		return nil, fmt.Errorf("clear previous episode assets: %w", err)
	}

	// Create sentinel asset
	sentinel := &model.Asset{
		ProjectID:   projectID,
		Type:        "character",
		Name:        "__extracting__",
		Description: fmt.Sprintf("正在提取第%d集资源…", episodeID),
		Status:      "extracting",
		EpisodeIDs:  pq.Int64Array{int64(episodeID)},
	}
	if err := s.assetSvc.Create(sentinel); err != nil {
		s.logger.Warn("failed to create sentinel asset", zap.Error(err))
	}
	sentinelID := sentinel.ID
	removeSentinel := func() {
		if sentinelID > 0 {
			_ = s.assetSvc.Delete(sentinelID)
		}
	}

	episodes := s.fetchEpisodeSummaries(ctx, projectID, jwtToken)

	var target *episodeSummary
	for i := range episodes {
		if episodes[i].ID == episodeID {
			target = &episodes[i]
			break
		}
	}
	if target == nil {
		removeSentinel()
		return nil, fmt.Errorf("episode %d not found in project %d", episodeID, projectID)
	}

	content := target.Excerpt
	if strings.TrimSpace(content) == "" {
		content = target.Summary
	}
	if strings.TrimSpace(content) == "" {
		removeSentinel()
		return nil, fmt.Errorf("episode %d has no content for extraction", episodeID)
	}

	s.logger.Info("starting single episode extraction",
		zap.Uint64("project_id", projectID),
		zap.Uint64("episode_id", episodeID),
		zap.Int("episode_number", target.Number),
		zap.Int("content_len", utf8.RuneCountInString(content)),
	)

	projectProfile, err := fetchProjectVisualProfile(ctx, s.projectServiceURL, projectID, jwtToken)
	if err != nil {
		removeSentinel()
		return nil, fmt.Errorf("fetch project profile: %w", err)
	}
	projectVisualHint := buildProjectVisualHint(projectProfile)
	skillHints := s.buildExtractionSkillHints(projectID)

	// Parse production annotation hints from the annotated script_excerpt.
	annotationHints := buildProductionAnnotationHints(content)
	combinedHints := skillHints + annotationHints

	extracted, err := s.callLLMExtract(ctx, fmt.Sprintf("【第%d集: %s】\n%s", target.Number, target.Title, content), projectVisualHint, combinedHints)
	if err != nil {
		removeSentinel()
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	deduped := deduplicateAssets(extracted.Assets)

	var created []model.Asset
	for _, ea := range deduped {
		assetType := normalizeAssetType(ea.Type)
		if assetType == "" {
			continue
		}
		cleanedDesc := ea.Description
		if assetType == "scene" || assetType == "prop" {
			cleanedDesc = stripCharacterMentions(assetType, cleanedDesc)
		}
		asset, err := s.upsertExtractedAsset(projectID, assetType, ea.Name, cleanedDesc, pq.Int64Array{int64(episodeID)})
		if err != nil {
			s.logger.Warn("create asset failed", zap.String("name", ea.Name), zap.Error(err))
			continue
		}
		created = append(created, *asset)
	}

	removeSentinel()

	s.logger.Info("episode assets created",
		zap.Uint64("project_id", projectID),
		zap.Uint64("episode_id", episodeID),
		zap.Int("count", len(created)),
	)

	// Auto-match voices after episode extraction
	if _, err := s.assetSvc.AutoMatchVoices(projectID); err != nil {
		s.logger.Warn("auto-match voices after episode extraction", zap.Error(err))
	}
	if err := s.triggerStoryboardExtraction(ctx, projectID, &episodeID, jwtToken); err != nil {
		s.logger.Warn("auto trigger episode storyboard extraction after asset extraction failed",
			zap.Uint64("project_id", projectID),
			zap.Uint64("episode_id", episodeID),
			zap.Error(err),
		)
	}

	return created, nil
}

const chunkMaxChars = 30000 // max chars per LLM extraction chunk (supports full chapter text)

// buildExtractionChunks —— 将文本按分集或固定长度拆分为提取用的文本块
// buildExtractionChunks creates text segments for extraction.
// If episodes exist, each episode (or group of short episodes) becomes a chunk.
// Otherwise, the raw script is split by character count with overlap.
func (s *ExtractService) buildExtractionChunks(scriptText string, episodes []episodeSummary) []string {
	// Strategy A: episode-based chunking
	if len(episodes) > 0 {
		return s.buildEpisodeChunks(episodes, scriptText)
	}
	// Strategy B: fixed-size chunking on raw text
	return splitTextIntoChunks(scriptText, chunkMaxChars, 500)
}

// buildEpisodeChunks —— 将多集内容按 LLM 窗口大小分组合并为文本块
// buildEpisodeChunks groups episodes into chunks that fit within the LLM token window.
// Prefers full chapter text (Excerpt) over short summary for richer extraction.
func (s *ExtractService) buildEpisodeChunks(episodes []episodeSummary, scriptText string) []string {
	var chunks []string
	var currentChunk strings.Builder
	currentLen := 0

	flushChunk := func() {
		if currentChunk.Len() > 0 {
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
			currentLen = 0
		}
	}

	for _, ep := range episodes {
		// Prefer full chapter text over summary
		content := ep.Excerpt
		if strings.TrimSpace(content) == "" {
			content = ep.Summary
		}
		if strings.TrimSpace(content) == "" {
			continue
		}

		epText := fmt.Sprintf("【第%d集: %s】\n%s\n\n", ep.Number, ep.Title, content)
		epRunes := utf8.RuneCountInString(epText)

		// If single episode exceeds limit, add it as its own chunk
		if epRunes > chunkMaxChars {
			flushChunk()
			chunks = append(chunks, epText)
			continue
		}

		// If adding this episode exceeds limit, flush and start new chunk
		if currentLen+epRunes > chunkMaxChars {
			flushChunk()
		}
		currentChunk.WriteString(epText)
		currentLen += epRunes
	}
	flushChunk()

	// If no chunks were created from episodes, fall back to raw script
	if len(chunks) == 0 && strings.TrimSpace(scriptText) != "" {
		return splitTextIntoChunks(scriptText, chunkMaxChars, 500)
	}
	return chunks
}

// splitTextIntoChunks —— 将文本按固定字符数切分为带重叠的文本块列表
// splitTextIntoChunks splits text into fixed-size chunks with overlap to avoid losing context at boundaries.
func splitTextIntoChunks(text string, maxChars, overlap int) []string {
	runes := []rune(text)
	total := len(runes)
	if total <= maxChars {
		return []string{text}
	}

	var chunks []string
	step := maxChars - overlap
	if step <= 0 {
		step = maxChars
	}
	for start := 0; start < total; start += step {
		end := start + maxChars
		if end > total {
			end = total
		}
		chunks = append(chunks, string(runes[start:end]))
		if end == total {
			break
		}
	}
	return chunks
}

// deduplicateAssets —— 按类型+名称去重资产列表，保留最详细的描述并合并剧集 ID
// deduplicateAssets merges assets by type+name, keeping the longest description.
func deduplicateAssets(assets []extractedAsset) []extractedAsset {
	type key struct {
		typ  string
		name string
	}
	seen := make(map[key]extractedAsset)
	var order []key // preserve insertion order

	for _, a := range assets {
		normalName := strings.TrimSpace(a.Name)
		normalType := strings.ToLower(strings.TrimSpace(a.Type))
		if normalName == "" {
			continue
		}
		k := key{typ: normalType, name: normalName}
		if existing, ok := seen[k]; ok {
			// Keep the longer/more detailed description
			if utf8.RuneCountInString(a.Description) > utf8.RuneCountInString(existing.Description) {
				epIDs := mergeInt64s(existing.EpisodeIDs, a.EpisodeIDs)
				a.EpisodeIDs = epIDs
				seen[k] = a
			} else {
				existing.EpisodeIDs = mergeInt64s(existing.EpisodeIDs, a.EpisodeIDs)
				seen[k] = existing
			}
		} else {
			seen[k] = a
			order = append(order, k)
		}
	}

	result := make([]extractedAsset, 0, len(order))
	for _, k := range order {
		result = append(result, seen[k])
	}
	return result
}

// mergeInt64s —— 合并两个 int64 切片并去重
// mergeInt64s merges two int64 slices, deduplicating values.
func mergeInt64s(a, b []int64) []int64 {
	set := make(map[int64]struct{}, len(a)+len(b))
	for _, v := range a {
		set[v] = struct{}{}
	}
	for _, v := range b {
		set[v] = struct{}{}
	}
	result := make([]int64, 0, len(set))
	for v := range set {
		result = append(result, v)
	}
	return result
}

type episodeSummary struct {
	ID      uint64
	Number  int
	Title   string
	Summary string
	Excerpt string // full chapter text (ScriptExcerpt)
}

// fetchEpisodeSummaries —— 从 project-service 获取项目的剧集摘要列表
// fetchEpisodeSummaries retrieves episode data from project-service for context enrichment.
func (s *ExtractService) fetchEpisodeSummaries(ctx context.Context, projectID uint64, jwtToken string) []episodeSummary {
	url := fmt.Sprintf("%s/api/v1/projects/%d/episodes", s.projectServiceURL, projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	if jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+jwtToken)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var result struct {
		Data []struct {
			ID            uint64 `json:"id"`
			EpisodeNumber int    `json:"episode_number"`
			Title         string `json:"title"`
			Summary       string `json:"summary"`
			ScriptExcerpt string `json:"script_excerpt"`
		} `json:"data"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return nil
	}

	var summaries []episodeSummary
	for _, ep := range result.Data {
		summaries = append(summaries, episodeSummary{
			ID:      ep.ID,
			Number:  ep.EpisodeNumber,
			Title:   ep.Title,
			Summary: ep.Summary,
			Excerpt: ep.ScriptExcerpt,
		})
	}
	return summaries
}

// fetchScriptText —— 从 project-service 获取项目的剧本文本

// buildExtractionSkillHints 从技能库构建用于注入 extraction 提示词的指引字符串
func (s *ExtractService) buildExtractionSkillHints(projectID uint64) string {
	if s.skillRepo == nil {
		return ""
	}
	skills, err := s.skillRepo.ListActiveByUseCase(int64(projectID), "extraction")
	if err != nil || len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n**提取增强指引（来自项目技能库）：**\n")
	for _, sk := range skills {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", sk.Name, sk.Description))
	}
	return sb.String()
}

// callLLMExtract —— 调用 LLM 接口从文本中提取资源，返回结构化提取结果
// extractAnnotationsByTag extracts values from [tag:value] inline annotations embedded
// in script_excerpt by the production skills polish step.
func extractAnnotationsByTag(text, tag string) []string {
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

// buildProductionAnnotationHints parses [道具:] and [服化:] tags from the annotated
// script_excerpt and returns a hint string for the extraction LLM.
func buildProductionAnnotationHints(content string) string {
	props := extractAnnotationsByTag(content, "道具")
	costumes := extractAnnotationsByTag(content, "服化")
	if len(props) == 0 && len(costumes) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n**剧本中已标注的资源（直接提取，勿遗漏）：**\n")
	if len(props) > 0 {
		sb.WriteString("道具标注：\n")
		for _, p := range props {
			sb.WriteString("  - 道具: " + p + "\n")
		}
	}
	if len(costumes) > 0 {
		sb.WriteString("服装/化妆标注：\n")
		for _, c := range costumes {
			sb.WriteString("  - 服化: " + c + "\n")
		}
	}
	return sb.String()
}

func (s *ExtractService) callLLMExtract(ctx context.Context, scriptText, projectVisualHint, skillHints string) (*llmExtractResult, error) {
	// Safety: truncate if a single chunk is still oversized
	if runes := []rune(scriptText); len(runes) > chunkMaxChars+5000 {
		scriptText = string(runes[:chunkMaxChars+5000])
	}

	systemPrompt := `你是一个剧本分析助手。请从给定的剧本/小说文本中提取**所有**重要的资源（角色、场景、道具）。

**提取规则：**
- character（角色/人物）：提取所有有名字的角色，包括主角、配角、甚至只出现一次的有名字角色。description 要包含外貌特征、服饰、年龄、性格等可视化信息。
- scene（场景/地点）：提取所有出现过的场景地点，包括室内外环境、不同时间段的同一地点也算不同场景。description 要包含环境特征、光线、氛围、时间段等可视化信息。
- prop（道具/物品）：提取所有重要的道具和物品，包括武器、食物、信件、交通工具等。description 要包含外观、材质、颜色、大小等可视化信息。

**内联标注识别（优先级最高）：**
文本中可能包含 [道具:xxx]、[服化:xxx]、[摄影:xxx]、[灯光:xxx]、[美术:xxx] 等格式的影视标注，这些标注由专业编辑预先添加，请务必将其中的道具和服化信息直接提取为资源，确保不遗漏：
- [道具:xxx] → 提取为 prop 类型资源
- [服化:角色名+描述] → 提取为对应 character 资源，并更新其 description 中的服装/妆造部分

**重要：**
- 请仔细遍历所有分集内容，确保不遗漏任何角色、场景或道具
- **不要限制数量**，有多少提取多少，宁可多提取也不要遗漏
- 每种类型至少提取 1 个
- 同名角色/场景不要重复
- description 要足够详细（至少30字），能够直接用作 AI 图像生成的提示词

请严格按照以下 JSON 格式输出，不要输出任何其他内容：
{
  "assets": [
    {"type": "character", "name": "角色名", "description": "角色外貌和性格的详细描述"},
    {"type": "scene", "name": "场景名", "description": "场景环境的详细描述"},
    {"type": "prop", "name": "道具名", "description": "道具外观和用途的描述"}
  ]
}

type 只能是 character、scene、prop 三种。`
	if strings.TrimSpace(projectVisualHint) != "" {
		systemPrompt += "\n\n**项目视觉要求：**\n- 本项目后续会按以下方向生成资源图和分镜图：" + projectVisualHint + "\n- 如果要求偏真人/写实/真实场景，请在 description 中明确体现真实人物、真实空间、自然光影、真实材质等特征\n- 不要把明确的真人/写实项目描述成动漫、Q版、二次元或卡通设定\n- **服饰描述须与角色性别严格对应**：男性角色只描述男性服饰（如圆领袍衫、幞头、布靴、儒衫、道袍），绝不出现女性服饰词汇（如齐胸裙、帔帛、步摇、高髻）；女性角色同理；服饰性别须以剧本中角色明确性别为准"
	}
	if strings.TrimSpace(skillHints) != "" {
		systemPrompt += skillHints
	}

	reqBody := map[string]interface{}{
		"model": s.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": "请从以下文本中提取资源：\n\n" + scriptText},
		},
		"temperature":     0.3,
		"max_tokens":      16384,
		"response_format": map[string]string{"type": "json_object"},
	}
	data, _ := json.Marshal(reqBody)

	llmCtx, cancel := context.WithTimeout(ctx, s.llmTimeout)
	defer cancel()

	url := s.llmBaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(llmCtx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.llmAPIKey)

	client := &http.Client{Timeout: s.llmTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm error %d: %s", resp.StatusCode, string(body))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil {
		preview := string(body)
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return nil, fmt.Errorf("parse llm response: %w — body: %s", err, preview)
	}
	if llmResp.Error != nil {
		return nil, fmt.Errorf("llm api error: %s", llmResp.Error.Message)
	}
	if len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("llm returned no choices")
	}

	content := llmResp.Choices[0].Message.Content
	// Strip markdown fences if present
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) > 2 {
			lines = lines[1 : len(lines)-1]
		}
		content = strings.Join(lines, "\n")
	}

	var result llmExtractResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse llm json: %w (content: %s)", err, content[:min(200, len(content))])
	}
	return &result, nil
}

// normalizeAssetType —— 将中英文资产类型标准化为 character/scene/prop
func normalizeAssetType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "character", "人物", "角色":
		return "character"
	case "scene", "场景", "地点":
		return "scene"
	case "prop", "道具", "物品":
		return "prop"
	default:
		return ""
	}
}

// BackfillEpisodeIDs —— 回填已有资产的剧集关联 ID，通过名称匹配剧集内容
// BackfillEpisodeIDs updates existing assets that have empty episode_ids by matching
// asset names against episode content. This is a one-time fix for assets extracted
// via project-wide extraction before episode tracking was added.
func (s *ExtractService) BackfillEpisodeIDs(ctx context.Context, projectID uint64, jwtToken string) (int, error) {
	episodes := s.fetchEpisodeSummaries(ctx, projectID, jwtToken)
	if len(episodes) == 0 {
		return 0, fmt.Errorf("no episodes found for project %d", projectID)
	}

	assets, err := s.assetSvc.List(projectID, "", "", nil)
	if err != nil {
		return 0, fmt.Errorf("list assets: %w", err)
	}

	updated := 0
	for _, asset := range assets {
		if len(asset.EpisodeIDs) > 0 {
			continue // already has episode associations
		}
		var matchedIDs []int64
		for _, ep := range episodes {
			content := ep.Excerpt
			if content == "" {
				content = ep.Summary
			}
			if content == "" {
				continue
			}
			if strings.Contains(content, asset.Name) {
				matchedIDs = append(matchedIDs, int64(ep.ID))
			}
		}
		if len(matchedIDs) > 0 {
			asset.EpisodeIDs = matchedIDs
			if err := s.assetSvc.repo.Update(&asset); err != nil {
				s.logger.Warn("backfill episode_ids failed", zap.Uint64("asset_id", asset.ID), zap.Error(err))
				continue
			}
			updated++
		}
	}

	s.logger.Info("episode_ids backfill complete",
		zap.Uint64("project_id", projectID),
		zap.Int("updated", updated),
		zap.Int("total_assets", len(assets)),
	)
	return updated, nil
}
