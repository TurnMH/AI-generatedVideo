package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/autovideo/project-service/internal/repository"
	"github.com/autovideo/project-service/internal/stylepreset"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// StoryboardGenerateRequest is published to Kafka to trigger image generation.
type StoryboardGenerateRequest struct {
	StoryboardID     uint64   `json:"storyboard_id"`
	VersionID        uint64   `json:"version_id"`
	ProjectID        uint64   `json:"project_id"`
	SceneDescription string   `json:"scene_description"`
	Characters       []string `json:"characters"`
	Location         string   `json:"location"`
	CameraMovement   string   `json:"camera_movement"`
	Mood             string   `json:"mood,omitempty"`
	AspectRatio      string   `json:"aspect_ratio"`
	PromptUsed       string   `json:"prompt_used"`
	ModelName        string   `json:"model_name,omitempty"`
	// PrevPromptUsed is the preceding storyboard's English image-generation prompt.
	// Injected at dispatch time for single-frame re-generation continuity.
	PrevPromptUsed string `json:"prev_prompt_used,omitempty"`
	// PrevImageURL is the preceding storyboard's already-generated image URL.
	// Appended to referenceImageURLs so multi-ref models (Gemini, gpt-image-1, Baidu)
	// use it as a visual continuity anchor for color grading and style consistency.
	PrevImageURL string  `json:"prev_image_url,omitempty"`
	AssetIDs     []int64 `json:"asset_ids,omitempty"`
}

// StoryboardGenerateResult is published after generation completes or fails.
type StoryboardGenerateResult struct {
	StoryboardID uint64 `json:"storyboard_id"`
	VersionID    uint64 `json:"version_id"`
	Status       string `json:"status"`
	ImageURL     string `json:"image_url,omitempty"`
	ErrorMsg     string `json:"error_msg,omitempty"`
}

type storyboardAssetReference struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	ImageURL   string `json:"image_url"`
	PromptUsed string `json:"prompt_used"`
}

type assetReferenceMaps struct {
	CharacterImages  map[string]string
	CharacterPrompts map[string]string
	SceneImages      map[string]string
	PropImages       map[string]string
}

func isTransientStoryboardGenerationError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"timeout",
		"timed out",
		"context deadline exceeded",
		"temporary",
		"temporarily unavailable",
		"connection reset",
		"connection refused",
		"broken pipe",
		"unexpected eof",
		"eof",
		"lookup ",
		"no such host",
		"i/o timeout",
		"tls handshake timeout",
		"503",
		"502",
		"429",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// KafkaProducer publishes storyboard generation requests.
type KafkaProducer struct {
	writer *kafka.Writer
	logger *zap.Logger
}

// NewKafkaProducer —— 创建 Kafka 生产者实例，用于发送分镜生成请求
func NewKafkaProducer(brokers []string, topic string, logger *zap.Logger) *KafkaProducer {
	w := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	return &KafkaProducer{writer: w, logger: logger}
}

// Publish —— 将分镜生成请求序列化并发布到 Kafka 主题
func (p *KafkaProducer) Publish(ctx context.Context, req StoryboardGenerateRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal kafka message: %w", err)
	}
	msg := kafka.Message{
		Key:   []byte(fmt.Sprintf("sb-%d", req.StoryboardID)),
		Value: data,
	}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.logger.Error("failed to publish kafka message", zap.Error(err), zap.Uint64("storyboard_id", req.StoryboardID))
		return err
	}
	p.logger.Info("published storyboard generate request", zap.Uint64("storyboard_id", req.StoryboardID))
	return nil
}

// Close —— 关闭 Kafka 生产者连接
func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}

// KafkaConsumer reads generation requests, calls the image-service, and updates storyboards.
type KafkaConsumer struct {
	reader           *kafka.Reader
	resultWriter     *kafka.Writer
	storyboardSvc    *StoryboardService
	projectRepo      *repository.ProjectRepo
	imageBaseURL     string
	jwtSecret        string
	logger           *zap.Logger
	sem              chan struct{} // concurrency limiter
	llmBaseURL       string
	llmAPIKey        string
	llmModel         string
	characterBaseURL string // optional, for fetching character appearance descriptions

	// skillHintsCache memoizes fetchProjectSkillHints per-project with a short TTL
	// so a burst of storyboards for the same project doesn't fan out into N HTTP calls
	// to character-service. The mutex is coarse-grained but the cache map access is cheap.
	skillHintsMu    sync.Mutex
	skillHintsCache map[uint64]skillHintsEntry
}

type skillHintsEntry struct {
	hints     string
	fetchedAt time.Time
}

const skillHintsCacheTTL = 90 * time.Second

// NewKafkaConsumer —— 创建 Kafka 消费者实例，用于处理分镜生成请求
func NewKafkaConsumer(
	brokers []string,
	consumerTopic string,
	consumerGroup string,
	resultTopic string,
	storyboardSvc *StoryboardService,
	projectRepo *repository.ProjectRepo,
	imageBaseURL string,
	jwtSecret string,
	logger *zap.Logger,
	maxConcurrent int,
	llmBaseURL, llmAPIKey, llmModel string,
) *KafkaConsumer {
	if maxConcurrent <= 0 {
		maxConcurrent = 8
	}
	if llmBaseURL == "" {
		llmBaseURL = "https://api.easyart.cc/v1"
	}
	if llmModel == "" {
		llmModel = "gpt-5.4-mini"
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     consumerGroup,
		Topic:       consumerTopic,
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafka.LastOffset,
	})
	resultWriter := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    resultTopic,
		Balancer: &kafka.LeastBytes{},
	}
	return &KafkaConsumer{
		reader:          reader,
		resultWriter:    resultWriter,
		storyboardSvc:   storyboardSvc,
		projectRepo:     projectRepo,
		imageBaseURL:    imageBaseURL,
		jwtSecret:       jwtSecret,
		logger:          logger,
		sem:             make(chan struct{}, maxConcurrent),
		llmBaseURL:      strings.TrimRight(llmBaseURL, "/"),
		llmAPIKey:       llmAPIKey,
		llmModel:        llmModel,
		skillHintsCache: make(map[uint64]skillHintsEntry),
	}
}

// SetCharacterBaseURL configures the optional character-service URL so that
// the consumer can fetch per-character appearance descriptions and inject
// them into storyboard image prompts for visual consistency.
func (c *KafkaConsumer) SetCharacterBaseURL(url string) {
	c.characterBaseURL = url
}

// Run —— 启动消费者循环，阻塞直到 ctx 被取消
// Run starts the consumer loop. It blocks until ctx is cancelled.
func (c *KafkaConsumer) Run(ctx context.Context) {
	c.logger.Info("kafka storyboard consumer started", zap.Int("max_concurrent", cap(c.sem)))
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.logger.Info("kafka consumer stopped")
				return
			}
			c.logger.Error("kafka fetch error", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}

		// Commit immediately so Kafka won't redeliver; status tracking is in DB
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error("kafka commit error", zap.Error(err))
		}

		// Acquire semaphore slot and process concurrently
		c.sem <- struct{}{}
		go func(m kafka.Message) {
			defer func() { <-c.sem }()
			if ctx.Err() != nil {
				return
			}
			c.handle(ctx, m)
		}(msg)
	}
}

// handle —— 处理单条 Kafka 消息，调用图片服务生成图片并更新分镜状态
func (c *KafkaConsumer) handle(ctx context.Context, msg kafka.Message) {
	var req StoryboardGenerateRequest
	if err := json.Unmarshal(msg.Value, &req); err != nil {
		c.logger.Error("unmarshal kafka message failed", zap.Error(err))
		return
	}
	currentStoryboard, err := c.storyboardSvc.GetByID(req.StoryboardID)
	if err != nil {
		c.logger.Warn("storyboard generation skipped: storyboard missing", zap.Uint64("storyboard_id", req.StoryboardID), zap.Error(err))
		return
	}
	if c.storyboardSvc.isProjectGenerationPaused(req.ProjectID) || currentStoryboard.Status == "paused" {
		if currentStoryboard.Status != "paused" {
			_ = c.storyboardSvc.markStoryboardPaused(currentStoryboard)
		}
		c.logger.Info("storyboard generation paused before processing", zap.Uint64("storyboard_id", req.StoryboardID), zap.Uint64("project_id", req.ProjectID))
		return
	}

	c.logger.Info("processing storyboard generation",
		zap.Uint64("storyboard_id", req.StoryboardID),
		zap.Uint64("version_id", req.VersionID),
	)

	projectVisualPrompt := ""
	imageStylePrefix := ""
	stylePreset := stylepreset.Default
	negativePrompt := buildStoryboardNegativePrompt(nil)
	var locationDescs map[string]string // from project keyword library profiles
	projectUserID := uint64(1)          // default; overridden below when project is loaded
	if project, projectErr := c.projectRepo.FindByIDNoAuth(req.ProjectID); projectErr == nil {
		projectVisualPrompt = buildProjectVisualPrompt(project)
		stylePreset = storyboardStylePreset(project)
		negativePrompt = buildStoryboardNegativePrompt(project)
		imageStylePrefix = project.ImageStylePrefix
		// Build location description map from stored visual profiles for consistency.
		locationDescs = buildLocationDescMap(project.KeywordLibrary)
		projectUserID = project.UserID
	} else {
		c.logger.Warn("load project visual config for storyboard generation", zap.Uint64("project_id", req.ProjectID), zap.Error(projectErr))
	}
	// Translate any Chinese text to English so SDXL/DALL-E understands it.
	// Only translate SceneDescription if PromptUsed is empty or Chinese — if PromptUsed is
	// already an English prompt (e.g. from LLM refinement at creation time), SceneDescription
	// is redundant and translating it wastes an LLM call.
	originalLocation := req.Location
	req.PromptUsed = c.translateIfNeeded(ctx, req.PromptUsed)
	if strings.TrimSpace(req.PromptUsed) == "" || containsChinese(req.PromptUsed) || isFullGeneratedPrompt(req.PromptUsed) {
		req.SceneDescription = c.translateIfNeeded(ctx, req.SceneDescription)
	}
	req.Location = c.translateIfNeeded(ctx, req.Location)
	// Preserve the original (possibly Chinese) character names for DB lookups BEFORE
	// translating them. The character/asset tables are keyed by the original DB name,
	// so translating first would cause every map lookup below to silently fail for
	// Chinese projects (no CHARACTER LOCK text, no style_reference_url picked up).
	originalCharacters := make([]string, len(req.Characters))
	copy(originalCharacters, req.Characters)
	for i, ch := range req.Characters {
		req.Characters[i] = c.translateIfNeeded(ctx, ch)
	}

	// Fetch character appearance descriptions and inject into prompt (char-c2).
	charAppearances := c.fetchCharacterAppearances(ctx, req.ProjectID, originalCharacters)
	// Fetch project skill hints for image generation enrichment.
	skillHints := c.fetchProjectSkillHints(ctx, req.ProjectID)

	// Fetch generated asset reference images for visual consistency (char-c10).
	// Use the primary character's generated image as style_reference_url so models
	// that support reference input (Tongyi/wanx i2i, Gemini, Baidu) produce visually
	// consistent characters instead of text-only descriptions.
	// We also collect prompt_used because it carries the era/region/ethnicity/gender
	// wardrobe constraints that were baked into the asset image — preferring that
	// over the plain Character.appearance_desc keeps the storyboard text aligned
	// with what the asset image actually depicts.
	directAssetRefs := c.fetchAssetReferencesByIDs(ctx, req.ProjectID, req.AssetIDs)
	directAssetMaps := buildAssetReferenceMaps(directAssetRefs)

	// BUG-3 FIX: replace two separate fetchAssetReferenceData calls (one for "character",
	// one for "scene") with a single batch fetch that retrieves ALL completed assets in one
	// HTTP round-trip and splits them by type in memory.
	charRefImages, charAssetPrompts, sceneRefImages := c.fetchAllAssetRefsOnce(ctx, req.ProjectID)
	charRefImages = mergeAssetReferenceMap(charRefImages, directAssetMaps.CharacterImages)
	charAssetPrompts = mergeAssetReferenceMap(charAssetPrompts, directAssetMaps.CharacterPrompts)
	sceneRefImages = mergeAssetReferenceMap(sceneRefImages, directAssetMaps.SceneImages)

	// Pick the first listed character that has a generated image as the primary reference.
	var styleReferenceURL string
	// Also collect every available character asset image so multi-image-aware
	// generators (Gemini parts[], gpt-image-1 image[], Baidu messages.content[])
	// can see every subject that appears in the panel. Order is preserved to
	// match the text order of characters in the prompt.
	var referenceImageURLs []string
	seenRef := make(map[string]struct{})
	for _, name := range originalCharacters {
		imgURL, ok := charRefImages[strings.ToLower(strings.TrimSpace(name))]
		if !ok || imgURL == "" {
			continue
		}
		if styleReferenceURL == "" {
			styleReferenceURL = imgURL
		}
		if _, dup := seenRef[imgURL]; !dup {
			seenRef[imgURL] = struct{}{}
			referenceImageURLs = append(referenceImageURLs, imgURL)
		}
	}
	if styleReferenceURL == "" {
		lookupLocation := strings.ToLower(strings.TrimSpace(originalLocation))
		if lookupLocation != "" {
			styleReferenceURL = sceneRefImages[lookupLocation]
		}
		if styleReferenceURL == "" && req.Location != "" {
			styleReferenceURL = sceneRefImages[strings.ToLower(strings.TrimSpace(req.Location))]
		}
	}
	if styleReferenceURL == "" {
		for _, imageURL := range directAssetMaps.SceneImages {
			if strings.TrimSpace(imageURL) != "" {
				styleReferenceURL = imageURL
				break
			}
		}
	}
	if styleReferenceURL == "" {
		for _, imageURL := range directAssetMaps.PropImages {
			if strings.TrimSpace(imageURL) != "" {
				styleReferenceURL = imageURL
				break
			}
		}
	}
	lookupLocation := strings.ToLower(strings.TrimSpace(originalLocation))
	if lookupLocation == "" {
		lookupLocation = strings.ToLower(strings.TrimSpace(req.Location))
	}
	if lookupLocation != "" {
		if sceneURL := sceneRefImages[lookupLocation]; sceneURL != "" {
			if _, dup := seenRef[sceneURL]; !dup {
				seenRef[sceneURL] = struct{}{}
				referenceImageURLs = append(referenceImageURLs, sceneURL)
			}
		}
	}
	for _, sceneURL := range directAssetMaps.SceneImages {
		if sceneURL == "" {
			continue
		}
		if _, dup := seenRef[sceneURL]; !dup {
			seenRef[sceneURL] = struct{}{}
			referenceImageURLs = append(referenceImageURLs, sceneURL)
		}
	}
	for _, propURL := range directAssetMaps.PropImages {
		if propURL == "" {
			continue
		}
		if _, dup := seenRef[propURL]; !dup {
			seenRef[propURL] = struct{}{}
			referenceImageURLs = append(referenceImageURLs, propURL)
		}
	}
	// Append the previous storyboard's generated image as a visual continuity anchor.
	// This lets multi-ref models (Gemini, gpt-image-1, Baidu) use it for color grading
	// and overall style consistency across consecutive panels. Single-ref models
	// (styleReferenceURL) are unaffected because they only read the first ref.
	if prevURL := strings.TrimSpace(req.PrevImageURL); prevURL != "" {
		if _, dup := seenRef[prevURL]; !dup {
			referenceImageURLs = append(referenceImageURLs, prevURL)
		}
	}

	modelName := req.ModelName
	if modelName == "" {
		modelName = "dalle"
	}
	// Append model-specific negative prompt tokens for diffusion models.
	negativePrompt = appendModelNegativeTokens(negativePrompt, modelName)
	// D: environment/prop-only frames (no characters listed) must actively exclude people,
	// otherwise diffusion models hallucinate incidental figures in landscape/prop shots.
	if len(req.Characters) == 0 {
		negativePrompt = appendNoPeopleNegativeTokens(negativePrompt)
	}
	referenceImageURLs = prioritizeStoryboardReferenceImages(referenceImageURLs)
	c.logger.Info("storyboard references prepared",
		zap.Uint64("storyboard_id", req.StoryboardID),
		zap.String("model", modelName),
		zap.Int("character_count", len(originalCharacters)),
		zap.Int("direct_asset_count", len(req.AssetIDs)),
		zap.String("style_reference_url", styleReferenceURL),
		zap.Int("reference_image_count", len(referenceImageURLs)),
		zap.Bool("has_prev_image", strings.TrimSpace(req.PrevImageURL) != ""),
	)
	prompt := buildImagePromptWithAppearances(req, stylePreset, projectVisualPrompt, imageStylePrefix, charAppearances, locationDescs, skillHints, modelName, originalCharacters, charAssetPrompts)
	imageURL, err := c.callImageServiceWithRetry(ctx, req.ProjectID, projectUserID, prompt, modelName, req.StoryboardID, stylePreset, negativePrompt, req.AspectRatio, styleReferenceURL, referenceImageURLs)

	result := StoryboardGenerateResult{
		StoryboardID: req.StoryboardID,
		VersionID:    req.VersionID,
	}

	if err != nil {
		c.logger.Error("image generation failed",
			zap.Uint64("storyboard_id", req.StoryboardID),
			zap.Error(err),
		)
		result.Status = "failed"
		result.ErrorMsg = err.Error()
		_ = c.storyboardSvc.UpdateGenerationResult(req.StoryboardID, req.VersionID, "", "failed", err.Error(), "")
	} else {
		c.logger.Info("image generation succeeded",
			zap.Uint64("storyboard_id", req.StoryboardID),
			zap.String("image_url", imageURL),
		)
		result.Status = "completed"
		result.ImageURL = imageURL
		// Save the full built English prompt so the UI EN toggle shows the actual generation prompt.
		_ = c.storyboardSvc.UpdateGenerationResult(req.StoryboardID, req.VersionID, imageURL, "completed", "", prompt)
	}

	c.publishResult(ctx, result)
}

func (c *KafkaConsumer) callImageServiceWithRetry(ctx context.Context, projectID uint64, userID uint64, prompt string, modelName string, storyboardID uint64, stylePreset string, negativePrompt string, aspectRatio string, styleReferenceURL string, referenceImageURLs []string) (string, error) {
	backoffs := []time.Duration{0, 5 * time.Second, 15 * time.Second}
	var lastErr error
	for attempt, backoff := range backoffs {
		if backoff > 0 {
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}

		imageURL, err := c.callImageService(ctx, projectID, userID, prompt, modelName, stylePreset, negativePrompt, aspectRatio, styleReferenceURL, referenceImageURLs)
		if err == nil {
			return imageURL, nil
		}
		lastErr = err
		if !isTransientStoryboardGenerationError(err) || attempt == len(backoffs)-1 {
			break
		}
		c.logger.Warn("transient storyboard image generation failure, retrying",
			zap.Uint64("storyboard_id", storyboardID),
			zap.Int("attempt", attempt+1),
			zap.Error(err),
		)
	}
	return "", lastErr
}

// buildImagePrompt —— 根据分镜请求信息拼接图片生成的提示词
func buildImagePrompt(req StoryboardGenerateRequest, stylePreset, projectVisualPrompt, imageStylePrefix string) string {
	return buildImagePromptWithAppearances(req, stylePreset, projectVisualPrompt, imageStylePrefix, nil, nil, "", "", nil, nil)
}

// buildImagePromptWithAppearances —— 同 buildImagePrompt，但注入角色外貌描述和地点环境描述以提升一致性
// charAppearances maps original (pre-translation) character name (lowercase) → Character.appearance_desc.
// charAssetPrompts maps original character name (lowercase) → Asset.prompt_used (richer: carries the
// era / region / ethnicity / gender wardrobe constraints injected when the reference image was generated).
// When both are available, charAssetPrompts wins because it matches what the reference image actually depicts.
// locationDescs maps location name (lowercase) → visual environment description.
// originalCharacters is the pre-translation character name slice, used ONLY for map lookups;
// req.Characters (translated) is still used for the surface "Featured subjects" text so
// downstream diffusion models receive English tokens.
// modelName is used to append model-specific quality suffix tokens (T4).
func buildImagePromptWithAppearances(req StoryboardGenerateRequest, stylePreset, projectVisualPrompt, imageStylePrefix string, charAppearances map[string]string, locationDescs map[string]string, skillHints string, modelName string, originalCharacters []string, charAssetPrompts map[string]string) string {
	opening := storyboardOpeningSentence(stylePreset)
	parts := []string{opening}
	// Prepend project-level global style prefix if set (e.g. "anime style, clean linework")
	if prefix := strings.TrimSpace(imageStylePrefix); prefix != "" {
		parts = append(parts, "Visual style: "+prefix+".")
	}
	// Inject skill-based modifiers (combat/special/etc.) after style prefix.
	if sh := strings.TrimSpace(skillHints); sh != "" {
		parts = append(parts, sh)
	}
	// Primary dramatic beat: prefer the stored English PromptUsed over translating SceneDescription.
	// LLM-refined English prompts (set at storyboard creation) are higher quality than on-the-fly
	// translations. Full generated prompts saved back after a previous generation are detected by
	// isFullGeneratedPrompt and fall back to SceneDescription to avoid redundant nesting.
	pu := strings.TrimSpace(req.PromptUsed)
	scene := strings.TrimSpace(req.SceneDescription)
	usedPromptUsedAsPrimary := pu != "" && !containsChinese(pu) && !isFullGeneratedPrompt(pu)
	if usedPromptUsedAsPrimary {
		parts = append(parts, "Primary dramatic beat: "+pu+".")
	} else if scene != "" && !containsChinese(scene) {
		parts = append(parts, "Primary dramatic beat: "+scene+".")
	} else {
		// Fallback B: translation failed — preserve raw source text so the prompt still
		// carries the dramatic intent. Many modern multilingual models accept Chinese input;
		// without this the prompt would collapse to a generic template.
		if raw := pickBeatFallback(pu, scene); raw != "" {
			parts = append(parts, "Primary dramatic beat (source language, best-effort): "+raw+".")
		}
	}
	// Inject previous storyboard frame as visual continuity anchor (for single-frame re-generation).
	if prevCtx := strings.TrimSpace(req.PrevPromptUsed); prevCtx != "" && !containsChinese(prevCtx) {
		// Cap length to avoid bloating the prompt with a very long prior frame description.
		if len(prevCtx) > 400 {
			prevCtx = prevCtx[:400] + "…"
		}
		parts = append(parts, "VISUAL CONTINUITY — previous scene (maintain identical character identity, hairstyle, makeup, wardrobe colors and garment layers, accessories, skin tone, and overall color grading; only pose, expression, and camera framing may change): "+prevCtx)
	}
	hasCharacters := len(req.Characters) > 0
	if hasCharacters {
		parts = append(parts, "Featured subjects: "+strings.Join(req.Characters, ", ")+".")
	} else {
		// D: environment/prop-only frame — prevent models from inventing incidental people.
		parts = append(parts, "Environment / prop-only frame: no visible human figures; focus on setting, atmosphere, and object detail.")
	}
	// Inject per-character appearance descriptions for visual consistency.
	// Prefer Asset.prompt_used (carries the era/region/ethnicity/gender wardrobe
	// constraints baked into the reference image) over Character.appearance_desc
	// (plain user/LLM summary). Look up by ORIGINAL (pre-translation) name so
	// Chinese-named characters aren't silently missed.
	if len(charAppearances) > 0 || len(charAssetPrompts) > 0 {
		var descParts []string
		for i, name := range req.Characters {
			lookupKey := strings.ToLower(strings.TrimSpace(name))
			if i < len(originalCharacters) && strings.TrimSpace(originalCharacters[i]) != "" {
				lookupKey = strings.ToLower(strings.TrimSpace(originalCharacters[i]))
			}
			desc := charAssetPrompts[lookupKey]
			if strings.TrimSpace(desc) == "" {
				desc = charAppearances[lookupKey]
			}
			if strings.TrimSpace(desc) != "" {
				descParts = append(descParts, strings.TrimSpace(name)+": "+strings.TrimSpace(desc))
			}
		}
		if len(descParts) > 0 {
			parts = append(parts, "CHARACTER LOCK — every listed subject must match these canonical descriptions exactly across all storyboard frames; DO NOT alter face shape, skin tone, hairstyle, hair color, makeup, garment layers, fabric colors, accessories, or body build between frames unless the scene description explicitly changes outfit: "+strings.Join(descParts, " | ")+".")
		}
	}
	if trimmedLocation := strings.TrimSpace(req.Location); trimmedLocation != "" {
		// If we have a stored visual description for this location, use it for consistency.
		locDesc := ""
		if len(locationDescs) > 0 {
			locDesc = locationDescs[strings.ToLower(trimmedLocation)]
		}
		if locDesc != "" {
			parts = append(parts, "Environment and setting: "+trimmedLocation+" — "+locDesc+".")
		} else {
			parts = append(parts, "Environment and setting: "+trimmedLocation+".")
		}
	}
	if moodCue := storyboardMoodCue(req.Mood); moodCue != "" {
		parts = append(parts, moodCue)
	}
	if cameraCue := storyboardCameraCue(req.CameraMovement); cameraCue != "" {
		parts = append(parts, cameraCue)
	}
	if aspectCue := storyboardAspectRatioCue(req.AspectRatio); aspectCue != "" {
		parts = append(parts, aspectCue)
	}
	// Only add as "additional requirements" when PromptUsed wasn't already used as the primary beat,
	// and only if it's not a full saved-back generated prompt, and not Chinese (translation may have failed).
	if !usedPromptUsedAsPrimary {
		if trimmedPrompt := strings.TrimSpace(req.PromptUsed); trimmedPrompt != "" && !isFullGeneratedPrompt(trimmedPrompt) && !containsChinese(trimmedPrompt) {
			parts = append(parts, "Additional visual requirements: "+trimmedPrompt+".")
		}
	}
	parts = append(parts,
		"Frame one decisive story moment with clear subject hierarchy, readable blocking, believable foreground-midground-background separation, and lighting that supports the narrative beat.",
		"Keep anatomy, costumes, props, and environment consistent and production-ready. Clothing must strictly match each character's gender — never render female garments (chest-high ruqun, pibo sashes, hairpin buns) on male characters, or male garments on female characters.",
		"Facial expressions and body language should read as natural, micro-expression-level human performance (subtle brow, eye, and mouth movement) — avoid stiff mannequin poses, frozen smiles, or exaggerated anime reactions in live-action frames. Ensure limb articulation and weight distribution feel physically plausible.",
		"Output a single cinematic image with no text, no subtitle, no watermark, no split screen, and no collage layout.",
	)
	// Append model-specific quality suffix tokens (T4: per-model keyword optimization).
	if qualitySuffix := modelQualitySuffix(modelName, stylePreset); qualitySuffix != "" {
		parts = append(parts, qualitySuffix)
	}
	return appendProjectVisualPrompt(strings.Join(parts, " "), projectVisualPrompt)
}

// modelQualitySuffix returns model-specific quality/fidelity suffix tokens.
// These improve output quality for diffusion models that benefit from quality keywords (T4).
func modelQualitySuffix(modelName, stylePreset string) string {
	lm := strings.ToLower(strings.TrimSpace(modelName))
	switch {
	case strings.Contains(lm, "sdxl") || strings.Contains(lm, "comfyui") || strings.Contains(lm, "sd"):
		// Stable Diffusion / SDXL / ComfyUI respond well to quality descriptor tokens.
		base := "masterpiece, best quality, ultra-detailed, sharp focus, 8k uhd"
		if strings.Contains(stylePreset, "anime") {
			return base + ", vibrant colors, clean cel shading"
		}
		return base + ", photorealistic, cinematic color grading, professional lighting"
	case strings.Contains(lm, "flux"):
		return "high quality, detailed, professional photography, cinematic"
	case strings.Contains(lm, "dalle") || strings.Contains(lm, "dall-e") || lm == "":
		// DALL-E works better with natural language — no tag-style quality tokens needed.
		return ""
	default:
		return "high quality, detailed, cinematic"
	}
}

// storyboardOpeningSentence returns a style-aware opening instruction for the storyboard prompt.
func storyboardOpeningSentence(stylePreset string) string {
	switch stylePreset {
	case "live-action-film":
		return "Create a single photorealistic cinematic film keyframe for professional video production. No anime, no cartoon, no illustration style."
	case "live-action-short":
		return "Create a single photorealistic short drama keyframe for professional video production. No anime, no cartoon, no illustration style."
	case "anime-2d":
		return "Create a single 2D anime-style storyboard keyframe with clean line art and cel-shaded colors."
	case "anime-3d":
		return "Create a single 3D anime CG-style storyboard keyframe with toon-shaded volumetric depth."
	default:
		return "Create a single polished storyboard keyframe for a film/video production."
	}
}

func buildAssetReferenceMaps(assets []storyboardAssetReference) assetReferenceMaps {
	maps := assetReferenceMaps{
		CharacterImages:  make(map[string]string),
		CharacterPrompts: make(map[string]string),
		SceneImages:      make(map[string]string),
		PropImages:       make(map[string]string),
	}
	for _, asset := range assets {
		nameKey := strings.ToLower(strings.TrimSpace(asset.Name))
		if nameKey == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(asset.Type)) {
		case "character":
			if asset.ImageURL != "" {
				maps.CharacterImages[nameKey] = asset.ImageURL
			}
			if asset.PromptUsed != "" {
				maps.CharacterPrompts[nameKey] = asset.PromptUsed
			}
		case "scene", "location":
			if asset.ImageURL != "" {
				maps.SceneImages[nameKey] = asset.ImageURL
			}
		case "prop", "item":
			if asset.ImageURL != "" {
				maps.PropImages[nameKey] = asset.ImageURL
			}
		default:
			// Unknown asset types (e.g. "clothing", "vehicle", "weapon") are treated as
			// prop references so they still contribute to the visual-consistency maps
			// instead of being silently dropped.
			if asset.ImageURL != "" {
				maps.PropImages[nameKey] = asset.ImageURL
			}
		}
	}
	return maps
}

func mergeAssetReferenceMap(dst map[string]string, src map[string]string) map[string]string {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]string, len(src))
	}
	for key, value := range src {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		dst[key] = value
	}
	return dst
}

func prioritizeStoryboardReferenceImages(referenceImageURLs []string) []string {
	const maxStoryboardRefs = 4
	seen := make(map[string]struct{}, len(referenceImageURLs))
	var out []string
	for _, url := range referenceImageURLs {
		trimmed := strings.TrimSpace(url)
		if trimmed == "" {
			continue
		}
		if _, dup := seen[trimmed]; dup {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
		if len(out) >= maxStoryboardRefs {
			break
		}
	}
	return out
}

// fetchAssetReferencesByIDs fetches a set of specific assets in one batch HTTP call.
// It calls /api/v1/projects/:pid/assets?status=completed&page_size=500 (no per-ID round
// trips) and then filters the result down to the requested IDs in memory.  This replaces
// the prior N-serial-GET implementation that caused O(N) latency per storyboard frame.
func (c *KafkaConsumer) fetchAssetReferencesByIDs(ctx context.Context, projectID uint64, assetIDs []int64) []storyboardAssetReference {
	if c.characterBaseURL == "" || len(assetIDs) == 0 {
		return nil
	}

	// Build a set of the requested IDs for fast in-memory lookup.
	wanted := make(map[int64]struct{}, len(assetIDs))
	for _, id := range assetIDs {
		if id > 0 {
			wanted[id] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return nil
	}

	batchURL := fmt.Sprintf("%s/api/v1/projects/%d/assets?status=completed&page_size=500", c.characterBaseURL, projectID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, batchURL, nil)
	if err != nil {
		return nil
	}
	c.applyServiceAuthHeader(httpReq, projectID)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.logger.Warn("fetchAssetReferencesByIDs batch fetch failed", zap.Uint64("project_id", projectID), zap.Error(err))
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var paginated struct {
		Data struct {
			Items []storyboardAssetReference `json:"items"`
		} `json:"data"`
	}
	out := make([]storyboardAssetReference, 0, len(wanted))
	if err := json.Unmarshal(body, &paginated); err == nil && len(paginated.Data.Items) > 0 {
		for _, item := range paginated.Data.Items {
			if _, ok := wanted[item.ID]; ok {
				out = append(out, item)
			}
		}
		return out
	}
	// Fallback: legacy flat array response.
	var legacy []storyboardAssetReference
	if err2 := json.Unmarshal(body, &legacy); err2 == nil {
		for _, item := range legacy {
			if _, ok := wanted[item.ID]; ok {
				out = append(out, item)
			}
		}
	}
	return out
}

func (c *KafkaConsumer) applyServiceAuthHeader(req *http.Request, projectID uint64) {
	if req == nil {
		return
	}
	token, err := c.buildServiceToken(projectID)
	if err != nil || strings.TrimSpace(token) == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
}

// fetchCharacterAppearances calls character-service to get appearance descriptions
// for all characters in the project. Returns a map of lowercase name → appearance desc.
// Returns nil on any error so callers gracefully continue without descriptions.
func (c *KafkaConsumer) fetchCharacterAppearances(ctx context.Context, projectID uint64, charNames []string) map[string]string {
	if c.characterBaseURL == "" || len(charNames) == 0 {
		return nil
	}
	url := fmt.Sprintf("%s/api/v1/characters?project_id=%d&page=1&page_size=100", c.characterBaseURL, projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	c.applyServiceAuthHeader(req, projectID)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.logger.Warn("fetch character appearances failed", zap.Uint64("project_id", projectID), zap.Error(err))
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var result struct {
		Data struct {
			Items []struct {
				Name           string `json:"name"`
				AppearanceDesc string `json:"appearance_desc"`
			} `json:"items"`
		} `json:"data"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return nil
	}
	appearances := make(map[string]string)
	for _, item := range result.Data.Items {
		if item.Name != "" && item.AppearanceDesc != "" {
			appearances[strings.ToLower(strings.TrimSpace(item.Name))] = item.AppearanceDesc
		}
	}
	if len(appearances) == 0 {
		return nil
	}
	return appearances
}

// fetchAllAssetRefsOnce fetches ALL completed assets for the project in ONE HTTP call
// (no type filter) and returns three maps split by asset type in memory.
// This replaces the prior two-call pattern (separate calls for "character" and "scene")
// and reduces per-frame HTTP overhead from 2+ round-trips to 1.
func (c *KafkaConsumer) fetchAllAssetRefsOnce(ctx context.Context, projectID uint64) (charImages, charPrompts, sceneImages map[string]string) {
	if c.characterBaseURL == "" {
		return nil, nil, nil
	}
	batchURL := fmt.Sprintf("%s/api/v1/projects/%d/assets?status=completed&page_size=500", c.characterBaseURL, projectID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, batchURL, nil)
	if err != nil {
		return nil, nil, nil
	}
	c.applyServiceAuthHeader(httpReq, projectID)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.logger.Warn("fetchAllAssetRefsOnce failed", zap.Uint64("project_id", projectID), zap.Error(err))
		return nil, nil, nil
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil
	}

	type assetItem struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		ImageURL   string `json:"image_url"`
		PromptUsed string `json:"prompt_used"`
	}
	parseItems := func(items []assetItem) {
		charImages = make(map[string]string)
		charPrompts = make(map[string]string)
		sceneImages = make(map[string]string)
		for _, item := range items {
			key := strings.ToLower(strings.TrimSpace(item.Name))
			if key == "" {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(item.Type)) {
			case "character":
				if item.ImageURL != "" {
					charImages[key] = item.ImageURL
				}
				if item.PromptUsed != "" {
					charPrompts[key] = item.PromptUsed
				}
			case "scene", "location":
				if item.ImageURL != "" {
					sceneImages[key] = item.ImageURL
				}
			}
			// prop/item/clothing/vehicle etc. are not needed for scene/char ref maps
		}
	}

	var paginated struct {
		Data struct {
			Items []assetItem `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &paginated); err == nil && len(paginated.Data.Items) > 0 {
		parseItems(paginated.Data.Items)
		return charImages, charPrompts, sceneImages
	}
	var legacy struct {
		Data []assetItem `json:"data"`
	}
	if err2 := json.Unmarshal(body, &legacy); err2 == nil && len(legacy.Data) > 0 {
		parseItems(legacy.Data)
	}
	return charImages, charPrompts, sceneImages
}

// fetchAssetReferenceImages —— 获取项目已生成完成的资产图片，用于分镜生成时的视觉一致性参考 (char-c10)
// assetType should be "character", "scene", or "prop".
// Returns a map of lowercase asset name → image_url for all completed assets of that type.
func (c *KafkaConsumer) fetchAssetReferenceImages(ctx context.Context, projectID uint64, assetType string) map[string]string {
	images, _ := c.fetchAssetReferenceData(ctx, projectID, assetType)
	return images
}

// fetchAssetReferenceData —— 同 fetchAssetReferenceImages，但同时返回 asset.prompt_used 映射。
// prompt_used 由 character-service 的 buildCharacterVisualHint 生成，带有时代/区域/民族/性别/
// 阶层等约束，用于替代 characters.appearance_desc 做 CHARACTER LOCK，以保证分镜文本描述与
// 资产参考图所描绘的人物细节一致。Map key 使用数据库中原始 asset.Name（通常为中文）。
func (c *KafkaConsumer) fetchAssetReferenceData(ctx context.Context, projectID uint64, assetType string) (map[string]string, map[string]string) {
	if c.characterBaseURL == "" {
		return nil, nil
	}
	url := fmt.Sprintf("%s/api/v1/projects/%d/assets?type=%s&status=completed&page_size=100", c.characterBaseURL, projectID, assetType)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil
	}
	c.applyServiceAuthHeader(req, projectID)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.logger.Warn("fetch asset reference images failed", zap.Uint64("project_id", projectID), zap.String("asset_type", assetType), zap.Error(err))
		return nil, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	body, _ := io.ReadAll(resp.Body)
	var paginated struct {
		Data struct {
			Items []struct {
				Name       string `json:"name"`
				ImageURL   string `json:"image_url"`
				PromptUsed string `json:"prompt_used"`
			} `json:"items"`
		} `json:"data"`
	}
	images := make(map[string]string)
	prompts := make(map[string]string)
	if err := json.Unmarshal(body, &paginated); err == nil && len(paginated.Data.Items) > 0 {
		for _, item := range paginated.Data.Items {
			key := strings.ToLower(strings.TrimSpace(item.Name))
			if key == "" {
				continue
			}
			if item.ImageURL != "" {
				images[key] = item.ImageURL
			}
			if item.PromptUsed != "" {
				prompts[key] = item.PromptUsed
			}
		}
	} else {
		var legacy struct {
			Data []struct {
				Name       string `json:"name"`
				ImageURL   string `json:"image_url"`
				PromptUsed string `json:"prompt_used"`
			} `json:"data"`
		}
		if err2 := json.Unmarshal(body, &legacy); err2 == nil {
			for _, item := range legacy.Data {
				key := strings.ToLower(strings.TrimSpace(item.Name))
				if key == "" {
					continue
				}
				if item.ImageURL != "" {
					images[key] = item.ImageURL
				}
				if item.PromptUsed != "" {
					prompts[key] = item.PromptUsed
				}
			}
		}
	}
	if len(images) == 0 && len(prompts) == 0 {
		return nil, nil
	}
	return images, prompts
}

// keyword library JSON. Returns a map of lowercase location name → description.
func buildLocationDescMap(kwLibJSON []byte) map[string]string {
	if len(kwLibJSON) == 0 {
		return nil
	}
	var lib struct {
		LocationProfiles []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"location_profiles"`
	}
	if err := json.Unmarshal(kwLibJSON, &lib); err != nil || len(lib.LocationProfiles) == 0 {
		return nil
	}
	m := make(map[string]string, len(lib.LocationProfiles))
	for _, p := range lib.LocationProfiles {
		if p.Name != "" && p.Description != "" {
			m[strings.ToLower(strings.TrimSpace(p.Name))] = p.Description
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// fetchProjectSkillHints calls character-service to get active storyboard skills for a project.
// Returns a formatted hint string to inject into image generation prompts.
//
// Results are cached in memory per project for skillHintsCacheTTL so a burst of storyboard
// generations for the same project does not fan out into N HTTP calls. Skills rarely change
// between adjacent scene renders, so a 90s TTL is a safe trade-off.
func (c *KafkaConsumer) fetchProjectSkillHints(ctx context.Context, projectID uint64) string {
	if c.characterBaseURL == "" {
		return ""
	}
	// Fast path: cached & fresh.
	c.skillHintsMu.Lock()
	if entry, ok := c.skillHintsCache[projectID]; ok && time.Since(entry.fetchedAt) < skillHintsCacheTTL {
		c.skillHintsMu.Unlock()
		return entry.hints
	}
	c.skillHintsMu.Unlock()

	hints := c.fetchProjectSkillHintsUncached(ctx, projectID)

	c.skillHintsMu.Lock()
	c.skillHintsCache[projectID] = skillHintsEntry{hints: hints, fetchedAt: time.Now()}
	c.skillHintsMu.Unlock()
	return hints
}

func (c *KafkaConsumer) fetchProjectSkillHintsUncached(ctx context.Context, projectID uint64) string {
	token, err := c.buildServiceToken(projectID)
	if err != nil {
		return ""
	}
	url := fmt.Sprintf("%s/api/v1/skills?project_id=%d&use_case=storyboard&is_active=true&page_size=50", c.characterBaseURL, projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.logger.Warn("fetch project skill hints failed", zap.Uint64("project_id", projectID), zap.Error(err))
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
				UseCase     string `json:"use_case"`
				IsActive    bool   `json:"is_active"`
			} `json:"items"`
		} `json:"data"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}
	var hints []string
	for _, sk := range result.Data.Items {
		if sk.IsActive && strings.TrimSpace(sk.Description) != "" {
			hints = append(hints, fmt.Sprintf("[%s] %s", sk.Name, sk.Description))
		}
	}
	if len(hints) == 0 {
		return ""
	}
	return "Project skill modifiers — apply to all subjects and environment: " + strings.Join(hints, "; ") + "."
}

func storyboardCameraCue(cameraMovement string) string {
	switch strings.TrimSpace(cameraMovement) {
	case "auto", "":
		return "Camera language: choose the most cinematic framing for the beat while keeping the action immediately readable."
	case "static":
		return "Camera language: static locked-off framing, emphasis on composition, blocking, and stillness."
	case "push-in":
		return "Camera language: gentle push-in feeling, stronger subject emphasis, intimate cinematic focus."
	case "pull-out":
		return "Camera language: slow pull-out feeling, reveal more environment and spatial relationship."
	case "pan-left":
		return "Camera language: leftward pan energy, lateral composition, directional movement across frame."
	case "pan-right":
		return "Camera language: rightward pan energy, lateral composition, directional movement across frame."
	case "tracking":
		return "Camera language: tracking shot feel, camera moving with the subject, strong forward scene continuity."
	case "handheld":
		return "Camera language: restrained handheld realism, observational texture, grounded human presence."
	default:
		return "Camera language: " + strings.TrimSpace(cameraMovement) + "."
	}
}

// storyboardMoodCue maps a mood tag (Chinese or English) to an English emotional-tone
// directive for diffusion models. Extends the prompt with facial-expression, color-grade
// and lighting intent that match the scripted mood.
func storyboardMoodCue(mood string) string {
	m := strings.ToLower(strings.TrimSpace(mood))
	if m == "" {
		return ""
	}
	switch m {
	case "tense", "suspense", "紧张", "悬疑":
		return "Emotional tone: tense, suspenseful, low-key chiaroscuro lighting, cool desaturated grade, tight facial tension."
	case "tragic", "sad", "melancholy", "悲伤", "忧郁", "哀伤":
		return "Emotional tone: melancholic and sorrowful, soft overcast light, muted cool grade, restrained downward gaze and quiet body language."
	case "joyful", "happy", "warm", "温馨", "欢乐", "喜悦":
		return "Emotional tone: warm and uplifting, soft golden-hour light, warm color grade, natural relaxed expressions."
	case "angry", "rage", "confrontation", "愤怒", "对峙":
		return "Emotional tone: confrontational intensity, harder key light, higher contrast, sharp jaw set and taut posture."
	case "romantic", "intimate", "浪漫", "柔情":
		return "Emotional tone: intimate romance, soft diffused glow, warm pastel grade, tender eye contact and close framing."
	case "mysterious", "eerie", "神秘", "诡异":
		return "Emotional tone: mysterious and uncanny, fog or haze, cool teal grade, selective rim light, ambiguous expressions."
	case "epic", "heroic", "grand", "史诗", "壮阔", "英雄":
		return "Emotional tone: epic and heroic, strong backlight, high-contrast cinematic grade, determined stance, wide monumental framing."
	case "action", "combat", "fight", "动作", "战斗":
		return "Emotional tone: kinetic action energy, motion tension in pose, dramatic directional light, adrenaline-charged expression."
	case "peaceful", "serene", "calm", "宁静", "平和":
		return "Emotional tone: peaceful and serene, balanced natural light, soft pastel grade, relaxed posture."
	default:
		// Pass the raw descriptor through — trust multilingual models to interpret.
		return "Emotional tone: " + strings.TrimSpace(mood) + " — reflect this in facial expression, lighting temperature, and color grade."
	}
}

// pickBeatFallback chooses the best raw source-language text to keep as the primary
// dramatic beat when no English-translated version is available. Returns "" when both
// inputs are empty.
func pickBeatFallback(pu, scene string) string {
	pu = strings.TrimSpace(pu)
	scene = strings.TrimSpace(scene)
	// Prefer SceneDescription when PromptUsed is a full-generated prompt (to avoid nesting).
	if pu != "" && !isFullGeneratedPrompt(pu) {
		return pu
	}
	if scene != "" {
		return scene
	}
	return pu
}

func storyboardAspectRatioCue(aspectRatio string) string {
	switch strings.TrimSpace(aspectRatio) {
	case "16:9":
		return "Framing target: 16:9 widescreen composition with strong cinematic left-right balance."
	case "9:16":
		return "Framing target: 9:16 vertical composition optimized for mobile viewing and stacked depth."
	case "4:3":
		return "Framing target: 4:3 composition with classic centered balance and controlled background space."
	case "3:4":
		return "Framing target: 3:4 portrait composition with strong subject emphasis and vertical layering."
	case "1:1":
		return "Framing target: 1:1 square composition with clean central balance and readable silhouettes."
	default:
		if strings.TrimSpace(aspectRatio) == "" {
			return ""
		}
		return "Framing target: " + strings.TrimSpace(aspectRatio) + "."
	}
}

// containsChinese returns true when s has at least one CJK character.
func containsChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// isFullGeneratedPrompt returns true if s looks like a complete storyboard image-generation
// prompt that was built by buildImagePromptWithAppearances and saved back to PromptUsed after
// a previous generation run. Such prompts start with the storyboardOpeningSentence prefix
// ("Create a single …") and should not be re-used as "Primary dramatic beat" or "Additional
// visual requirements" — they already contain all structured prompt parts and would cause
// redundant nesting if included again.
func isFullGeneratedPrompt(s string) bool {
	return strings.HasPrefix(s, "Create a single")
}

// translateIfNeeded translates text that contains Chinese characters to English
// using the LLM. Returns the original text if translation fails or is unnecessary.
func (c *KafkaConsumer) translateIfNeeded(ctx context.Context, text string) string {
	if strings.TrimSpace(text) == "" || !containsChinese(text) {
		return text
	}
	tCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	reqBody := map[string]interface{}{
		"model": c.llmModel,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a professional translator. Translate the user's Chinese text into vivid, descriptive English suitable for an image-generation prompt. IMPORTANT: if the input contains structural labels such as '视觉基调：', '场景：', '时代：', '人物：', '地域造型：', '项目视觉基调：' or their variants, those sections carry authoritative period, regional, ethnicity and visual-tone constraints — INTEGRATE their content naturally into the translated scene description (e.g. weave era-specific architecture, wardrobe, cultural details into the prose). Drop only the label syntax itself ('时代：'/'视觉基调：' etc.), never the content. Output only the translated text, no explanation."},
			{"role": "user", "content": text},
		},
		"temperature": 0.3,
		"max_tokens":  512,
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(tCtx, http.MethodPost, c.llmBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		c.logger.Warn("translate request create failed", zap.Error(err))
		return text
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.llmAPIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.logger.Warn("translate LLM call failed", zap.Error(err))
		return text
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &llmResp); err != nil || len(llmResp.Choices) == 0 {
		c.logger.Warn("translate LLM response parse failed", zap.Error(err))
		return text
	}
	translated := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	if translated == "" {
		return text
	}
	c.logger.Info("translated Chinese to English for image prompt",
		zap.String("original", text), zap.String("translated", translated))
	return translated
}

// containsCompositeCharacterSheet reports whether any of the given reference
// image URLs points to a 4-panel character turnaround sheet produced by
// character-service (filename pattern "asset_*_composite.jpg"). When true,
// image-service will inject explicit "SAME person, 4 views" guidance into the
// prompt so generators don't paint the sheet as four separate characters.
func containsCompositeCharacterSheet(primary string, extras []string) bool {
	check := func(u string) bool {
		return strings.Contains(strings.ToLower(u), "_composite.")
	}
	if check(primary) {
		return true
	}
	for _, u := range extras {
		if check(u) {
			return true
		}
	}
	return false
}

// aspectRatioDimensions converts an aspect-ratio string into a width/height pair.
// The returned dimensions are intentionally model-agnostic; image-service's
// NormalizeGenerateSize will clamp/round them to the per-model allowed size set.
func aspectRatioDimensions(aspectRatio string) (int, int) {
	switch strings.TrimSpace(aspectRatio) {
	case "16:9":
		return 1280, 720
	case "9:16":
		return 720, 1280
	case "4:3":
		return 1024, 768
	case "3:4":
		return 768, 1024
	case "21:9":
		return 1344, 576
	case "1:1", "":
		return 1024, 1024
	default:
		return 1024, 1024
	}
}

// callImageService —— 调用图片服务提交生成任务并轮询结果，返回图片 URL
func (c *KafkaConsumer) callImageService(ctx context.Context, projectID uint64, userID uint64, prompt string, modelName string, stylePreset string, negativePrompt string, aspectRatio string, styleReferenceURL string, referenceImageURLs []string) (string, error) {
	width, height := aspectRatioDimensions(aspectRatio)
	if strings.TrimSpace(stylePreset) == "" {
		stylePreset = stylepreset.Default
	}
	body := map[string]interface{}{
		"project_id":      projectID,
		"user_id":         userID,
		"prompt":          prompt,
		"negative_prompt": negativePrompt,
		"style_preset":    stylePreset,
		"model_name":      modelName,
		"task_type":       "storyboard",
		"width":           width,
		"height":          height,
	}
	if styleReferenceURL != "" {
		body["style_reference_url"] = styleReferenceURL
	}
	if len(referenceImageURLs) > 0 {
		body["reference_image_urls"] = referenceImageURLs
	}
	// Tell image-service when any attached reference is a 4-panel character
	// turnaround sheet (filename "asset_*_composite.jpg") so Gemini / Baidu /
	// SDXL can inject the "SAME person, 4 views" interpretation guide. Without
	// this, models tend to paint four separate characters into the output.
	if containsCompositeCharacterSheet(styleReferenceURL, referenceImageURLs) {
		body["is_character_sheet"] = true
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal image request: %w", err)
	}

	token, err := c.buildServiceToken(projectID)
	if err != nil {
		return "", fmt.Errorf("build service token: %w", err)
	}

	baseURL := strings.TrimRight(c.imageBaseURL, "/")
	client := &http.Client{Timeout: 30 * time.Second}

	// Step 1: Submit the generation task.
	genURL := baseURL + "/api/v1/images/generate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, genURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create image request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("image-service unreachable: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("image-service error %d: %s", resp.StatusCode, string(respBody))
	}

	var createResp struct {
		Code int `json:"code"`
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &createResp); err != nil {
		return "", fmt.Errorf("parse create response: %w", err)
	}
	taskID := createResp.Data.ID
	if taskID == 0 {
		return "", fmt.Errorf("image-service returned no task id")
	}
	c.logger.Info("image task created", zap.Int64("task_id", taskID))

	// Step 2: Poll task status until completed or failed.
	taskURL := fmt.Sprintf("%s/api/v1/images/tasks/%d", baseURL, taskID)
	pollInterval := 2 * time.Second
	deadline := time.After(30 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline:
			return "", fmt.Errorf("image generation timed out for task %d", taskID)
		case <-time.After(pollInterval):
		}

		pollReq, err := http.NewRequestWithContext(ctx, http.MethodGet, taskURL, nil)
		if err != nil {
			return "", fmt.Errorf("create poll request: %w", err)
		}
		pollReq.Header.Set("Authorization", "Bearer "+token)

		pollResp, err := client.Do(pollReq)
		if err != nil {
			c.logger.Warn("poll image task failed", zap.Error(err))
			continue
		}
		pollBody, _ := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()

		var taskResp struct {
			Data struct {
				Status    string `json:"status"`
				ResultURL string `json:"result_url"`
				ErrorMsg  string `json:"error_msg"`
			} `json:"data"`
		}
		if err := json.Unmarshal(pollBody, &taskResp); err != nil {
			c.logger.Warn("parse poll response failed", zap.Error(err))
			continue
		}

		switch taskResp.Data.Status {
		case "succeeded":
			if taskResp.Data.ResultURL == "" {
				return "", fmt.Errorf("image task %d succeeded but has no URL", taskID)
			}
			return taskResp.Data.ResultURL, nil
		case "failed":
			return "", fmt.Errorf("image generation failed: %s", taskResp.Data.ErrorMsg)
		default:
			c.logger.Debug("image task still running", zap.Int64("task_id", taskID), zap.String("status", taskResp.Data.Status))
		}
	}
}

// publishResult —— 将分镜生成结果发布到 Kafka 结果主题
func (c *KafkaConsumer) publishResult(ctx context.Context, result StoryboardGenerateResult) {
	data, err := json.Marshal(result)
	if err != nil {
		c.logger.Error("marshal result failed", zap.Error(err))
		return
	}
	msg := kafka.Message{
		Key:   []byte(fmt.Sprintf("sb-%d", result.StoryboardID)),
		Value: data,
	}
	if err := c.resultWriter.WriteMessages(ctx, msg); err != nil {
		c.logger.Error("publish result failed", zap.Error(err))
	}
}

// Close —— 关闭 Kafka 消费者和结果写入器的连接
func (c *KafkaConsumer) Close() error {
	_ = c.reader.Close()
	return c.resultWriter.Close()
}

// buildServiceToken —— 生成用于服务间调用的 HS256 JWT 令牌
// buildServiceToken creates a minimal HS256 JWT for internal service-to-service calls.
func (c *KafkaConsumer) buildServiceToken(projectID uint64) (string, error) {
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
	mac := hmac.New(sha256.New, []byte(c.jwtSecret))
	mac.Write([]byte(unsigned))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + sig, nil
}
