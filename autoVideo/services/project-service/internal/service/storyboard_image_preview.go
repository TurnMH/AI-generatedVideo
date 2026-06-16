package service

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/autovideo/project-service/internal/stylepreset"
)

// StoryboardImageGenerationParams is the exact payload sent to image-service.
type StoryboardImageGenerationParams struct {
	Prompt                  string   `json:"prompt"`
	PromptDisplayZh         string   `json:"prompt_display_zh"`
	PromptAutoSupplementsZh string   `json:"prompt_auto_supplements_zh"`
	NegativePrompt          string   `json:"negative_prompt"`
	ModelName          string   `json:"model_name"`
	StylePreset        string   `json:"style_preset"`
	AspectRatio        string   `json:"aspect_ratio"`
	Width              int      `json:"width"`
	Height             int      `json:"height"`
	StyleReferenceURL  string   `json:"style_reference_url,omitempty"`
	ReferenceImageURLs []string `json:"reference_image_urls"`
	IsCharacterSheet   bool     `json:"is_character_sheet"`
	RawPromptMode      bool     `json:"raw_prompt_mode"`
	TaskType           string   `json:"task_type"`
}

// StoryboardImageGenerationPreparer builds the image-service request for a storyboard.
type StoryboardImageGenerationPreparer interface {
	PrepareStoryboardImageGeneration(ctx context.Context, req StoryboardGenerateRequest) (*StoryboardImageGenerationParams, error)
}

// PrepareStoryboardImageGeneration resolves prompt, references, and image-service body fields.
func (c *KafkaConsumer) PrepareStoryboardImageGeneration(ctx context.Context, req StoryboardGenerateRequest) (*StoryboardImageGenerationParams, error) {
	bundle, err := c.prepareStoryboardImageBundle(ctx, req)
	if err != nil {
		return nil, err
	}
	width, height := aspectRatioDimensions(bundle.AspectRatio)
	return &StoryboardImageGenerationParams{
		Prompt:                  bundle.Prompt,
		PromptDisplayZh:         bundle.PromptDisplayZh,
		PromptAutoSupplementsZh: bundle.PromptAutoSupplementsZh,
		NegativePrompt:          bundle.NegativePrompt,
		ModelName:          bundle.ModelName,
		StylePreset:        bundle.StylePreset,
		AspectRatio:        bundle.AspectRatio,
		Width:              width,
		Height:             height,
		StyleReferenceURL:  bundle.StyleReferenceURL,
		ReferenceImageURLs: bundle.ReferenceImageURLs,
		IsCharacterSheet:   false,
		RawPromptMode:      bundle.RawPromptMode,
		TaskType:           "storyboard",
	}, nil
}

type storyboardImageBundle struct {
	Prompt                  string
	PromptDisplayZh         string
	PromptAutoSupplementsZh string
	NegativePrompt          string
	ModelName          string
	StylePreset        string
	AspectRatio        string
	StyleReferenceURL  string
	ReferenceImageURLs []string
	RawPromptMode      bool
	ProjectUserID      uint64
}

func (c *KafkaConsumer) prepareStoryboardImageBundle(ctx context.Context, req StoryboardGenerateRequest) (storyboardImageBundle, error) {
	projectVisualPrompt := ""
	imageStylePrefix := ""
	stylePreset := stylepreset.Default
	negativePrompt := buildStoryboardNegativePrompt(nil)
	var locationDescs map[string]string
	var keywordLibraryJSON []byte
	projectUserID := uint64(1)
	if project, projectErr := c.projectRepo.FindByIDNoAuth(req.ProjectID); projectErr == nil {
		projectVisualPrompt = buildProjectVisualPrompt(project)
		stylePreset = storyboardStylePreset(project)
		negativePrompt = buildStoryboardNegativePrompt(project)
		imageStylePrefix = project.ImageStylePrefix
		keywordLibraryJSON = project.KeywordLibrary
		locationDescs = buildLocationDescMap(project.KeywordLibrary)
		projectUserID = project.UserID
	} else if c.logger != nil {
		c.logger.Warn("load project visual config for storyboard generation", zap.Uint64("project_id", req.ProjectID), zap.Error(projectErr))
	}

	rawMode := req.RawPrompt && strings.TrimSpace(req.PromptUsed) != ""
	displayReq := req
	originalLocation := req.Location
	originalCharacters := make([]string, len(req.Characters))
	copy(originalCharacters, req.Characters)
	if !rawMode {
		req.PromptUsed = c.translateIfNeeded(ctx, req.PromptUsed)
		if strings.TrimSpace(req.PromptUsed) == "" || containsChinese(req.PromptUsed) || isFullGeneratedPrompt(req.PromptUsed) {
			req.SceneDescription = c.translateIfNeeded(ctx, req.SceneDescription)
		}
		req.Location = c.translateIfNeeded(ctx, req.Location)
	}

	for i, ch := range req.Characters {
		req.Characters[i] = c.translateIfNeeded(ctx, ch)
	}

	charAppearances := c.fetchCharacterAppearances(ctx, req.ProjectID, originalCharacters)
	skillHints := c.fetchProjectSkillHints(ctx, req.ProjectID)

	directAssetRefs := c.fetchAssetReferencesByIDs(ctx, req.ProjectID, req.AssetIDs)
	directAssetMaps := buildAssetReferenceMaps(directAssetRefs)

	charRefImages, charAssetPrompts, sceneRefImages, sceneAssetEntries := c.fetchAllAssetRefsOnce(ctx, req.ProjectID)
	charRefImages = mergeAssetReferenceMap(charRefImages, directAssetMaps.CharacterImages)
	charAssetPrompts = mergeAssetReferenceMap(charAssetPrompts, directAssetMaps.CharacterPrompts)
	sanitizeCharacterPromptMapForStoryboard(charAssetPrompts)
	sceneRefImages = mergeAssetReferenceMap(sceneRefImages, directAssetMaps.SceneImages)

	requestedView := InferLocationViewType(displayReq.SceneDescription, displayReq.Location, "", req.LocationZone)
	locProfileIdx := buildLocationProfileIndex(keywordLibraryJSON)
	sceneRefPick := pickStoryboardSceneReference(requestedView, displayReq.Location, sceneRefImages, append(sceneAssetEntries, sceneEntriesFromDirectAssets(directAssetRefs)...))

	promptDisplayZh := buildStoryboardPromptDisplayZh(
		displayReq,
		originalCharacters,
		charAppearances,
		charAssetPrompts,
		locationDescs,
		directAssetRefs,
		rawMode,
	)

	var styleReferenceURL string
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
	if styleReferenceURL == "" && sceneRefPick.Matched && sceneRefPick.URL != "" {
		styleReferenceURL = sceneRefPick.URL
	} else if styleReferenceURL == "" && !sceneRefPick.Skipped {
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
	if sceneRefPick.Matched && sceneRefPick.URL != "" {
		if _, dup := seenRef[sceneRefPick.URL]; !dup {
			seenRef[sceneRefPick.URL] = struct{}{}
			referenceImageURLs = append(referenceImageURLs, sceneRefPick.URL)
		}
	} else if !sceneRefPick.Skipped {
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
	if prevURL := strings.TrimSpace(req.PrevImageURL); prevURL != "" {
		if _, dup := seenRef[prevURL]; !dup {
			referenceImageURLs = append(referenceImageURLs, prevURL)
		}
	}

	modelName := req.ModelName
	if modelName == "" {
		modelName = "dalle"
	}
	negativePrompt = appendModelNegativeTokens(negativePrompt, modelName)
	negativePrompt = appendStoryboardAntiSheetNegativeTokens(negativePrompt)
	if len(req.Characters) == 0 {
		negativePrompt = appendNoPeopleNegativeTokens(negativePrompt)
	}
	referenceImageURLs = filterStoryboardReferenceURLs(referenceImageURLs)
	referenceImageURLs = prioritizeStoryboardReferenceImages(referenceImageURLs)
	styleReferenceURL = resolveStoryboardStyleReferenceURL(styleReferenceURL, referenceImageURLs)

	var prompt string
	if rawMode {
		prompt = strings.TrimSpace(req.PromptUsed)
	} else {
		prompt = buildImagePromptWithAppearances(req, stylePreset, projectVisualPrompt, imageStylePrefix, charAppearances, locationDescs, skillHints, modelName, originalCharacters, charAssetPrompts)
	}
	if shared := resolveLocationSharedVisual(displayReq.Location, locProfileIdx); shared != "" && !rawMode {
		prompt = strings.TrimSpace(prompt + " Location continuity anchor: " + truncateForImagePrompt(shared, 180) + ".")
	}
	zoneDesc := resolveLocationDescriptionForPrompt(displayReq.Location, req.LocationZone, requestedView, locProfileIdx)
	if zoneDesc != "" && !rawMode && !isMostlyASCII(zoneDesc) {
		zoneDesc = c.translateIfNeeded(ctx, zoneDesc)
	}
	if zoneDesc != "" && !rawMode {
		prompt = strings.TrimSpace(prompt + " Zone-specific setting: " + truncateForImagePrompt(zoneDesc, 180) + ".")
	}

	aspectRatio := strings.TrimSpace(req.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}

	promptAutoSupplementsZh := buildStoryboardPromptAutoSupplementsZh(storyboardAutoSupplementInput{
		RawMode:             rawMode,
		StylePreset:         stylePreset,
		ImageStylePrefix:    imageStylePrefix,
		SkillHints:          skillHints,
		ProjectVisualPrompt: projectVisualPrompt,
		ModelName:           modelName,
		NegativePrompt:      negativePrompt,
		ReferenceImageURLs:  referenceImageURLs,
		PrevImageURL:        strings.TrimSpace(req.PrevImageURL),
		HasCharacters:       len(originalCharacters) > 0,
		LocationViewType:    requestedView,
		SceneReferenceNote:  sceneRefPick.NoteZh,
	})

	return storyboardImageBundle{
		Prompt:                  prompt,
		PromptDisplayZh:         promptDisplayZh,
		PromptAutoSupplementsZh: promptAutoSupplementsZh,
		NegativePrompt:          negativePrompt,
		ModelName:          modelName,
		StylePreset:        stylePreset,
		AspectRatio:        aspectRatio,
		StyleReferenceURL:  styleReferenceURL,
		ReferenceImageURLs: referenceImageURLs,
		RawPromptMode:      rawMode,
		ProjectUserID:      projectUserID,
	}, nil
}

func storyboardAssetTypeLabelZh(assetType string) string {
	switch strings.ToLower(strings.TrimSpace(assetType)) {
	case "character":
		return "人物"
	case "scene":
		return "场景"
	case "prop":
		return "物品"
	default:
		return "资源"
	}
}

// buildStoryboardPromptDisplayZh assembles a Chinese-readable prompt for UI preview.
// The English prompt sent to image-service is built separately at generation time.
func buildStoryboardPromptDisplayZh(
	req StoryboardGenerateRequest,
	originalCharacters []string,
	charAppearances map[string]string,
	charAssetPrompts map[string]string,
	locationDescs map[string]string,
	directAssetRefs []storyboardAssetReference,
	rawMode bool,
) string {
	if rawMode {
		if pu := strings.TrimSpace(req.PromptUsed); pu != "" && containsChinese(pu) {
			return pu
		}
	}

	var lines []string
	if scene := strings.TrimSpace(req.SceneDescription); scene != "" {
		lines = append(lines, "【主要画面】"+scene)
	} else if beat := pickBeatFallback(req.PromptUsed, ""); beat != "" && containsChinese(beat) {
		lines = append(lines, "【主要画面】"+beat)
	}

	if len(originalCharacters) > 0 {
		lines = append(lines, "【出场人物】"+strings.Join(originalCharacters, "、"))
	}

	for _, name := range originalCharacters {
		key := strings.ToLower(strings.TrimSpace(name))
		desc := strings.TrimSpace(charAssetPrompts[key])
		if desc == "" {
			desc = strings.TrimSpace(charAppearances[key])
		}
		if desc == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("【人物锁定·%s】%s", strings.TrimSpace(name), truncateForImagePrompt(desc, 200)))
	}

	for _, asset := range directAssetRefs {
		if strings.EqualFold(strings.TrimSpace(asset.Type), "character") {
			continue
		}
		desc := strings.TrimSpace(asset.PromptUsed)
		if desc == "" {
			desc = strings.TrimSpace(asset.Name)
		}
		if desc == "" {
			continue
		}
		typeLabel := storyboardAssetTypeLabelZh(asset.Type)
		name := strings.TrimSpace(asset.Name)
		if name == "" {
			name = typeLabel
		}
		lines = append(lines, fmt.Sprintf("【%s锁定·%s】%s", typeLabel, name, truncateForImagePrompt(desc, 200)))
	}

	if loc := strings.TrimSpace(req.Location); loc != "" {
		locDesc := ""
		if len(locationDescs) > 0 {
			locDesc = strings.TrimSpace(locationDescs[strings.ToLower(loc)])
		}
		if locDesc != "" {
			lines = append(lines, fmt.Sprintf("【场景地点】%s — %s", loc, truncateForImagePrompt(locDesc, 120)))
		} else {
			lines = append(lines, "【场景地点】"+loc)
		}
	}

	if mood := strings.TrimSpace(req.Mood); mood != "" {
		lines = append(lines, "【情绪氛围】"+mood)
	}

	if len(lines) == 0 {
		return "暂无描述"
	}

	return strings.Join(lines, "\n")
}

type storyboardAutoSupplementInput struct {
	RawMode             bool
	StylePreset         string
	ImageStylePrefix    string
	SkillHints          string
	ProjectVisualPrompt string
	ModelName           string
	NegativePrompt      string
	ReferenceImageURLs  []string
	PrevImageURL        string
	HasCharacters       bool
	LocationViewType    string
	SceneReferenceNote  string
}

func storyboardOpeningSentenceZh(stylePreset string) string {
	switch stylepreset.Canonical(stylePreset) {
	case "live-action-film":
		return "写实电影感单帧静帧（非动漫/插画）"
	case "live-action-short":
		return "写实短剧感单帧静帧（非动漫/插画）"
	case "anime-2d":
		return "二维动漫风单帧分镜（线稿清晰、赛璐璐着色）"
	case "anime-3d":
		return "三维动漫 CG 单帧分镜（卡通渲染）"
	default:
		return "精修分镜单帧静帧"
	}
}

func storyboardStylePresetLabelZh(stylePreset string) string {
	switch stylepreset.Canonical(stylePreset) {
	case "live-action-film":
		return "写实电影"
	case "live-action-short":
		return "写实短剧"
	case "anime-2d":
		return "二维动漫"
	case "anime-3d":
		return "三维动漫"
	default:
		return stylePreset
	}
}

func countNegativePromptItems(negativePrompt string) int {
	parts := strings.Split(negativePrompt, ",")
	count := 0
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			count++
		}
	}
	return count
}

// buildStoryboardPromptAutoSupplementsZh lists what the backend actually appends beyond user-editable scene text.
func buildStoryboardPromptAutoSupplementsZh(in storyboardAutoSupplementInput) string {
	if in.RawMode {
		return "【高级锁定】已启用：原样使用最终提示词，不自动拼接风格前缀、Skill、质量词等模板。"
	}

	var lines []string
	if opening := storyboardOpeningSentenceZh(in.StylePreset); opening != "" {
		lines = append(lines, "【风格开头】"+opening)
	}
	if preset := strings.TrimSpace(in.StylePreset); preset != "" {
		lines = append(lines, "【风格 preset】"+storyboardStylePresetLabelZh(preset))
	}
	if prefix := strings.TrimSpace(in.ImageStylePrefix); prefix != "" {
		lines = append(lines, "【项目风格前缀】"+truncateForImagePrompt(prefix, 240))
	}
	if hints := strings.TrimSpace(in.SkillHints); hints != "" {
		lines = append(lines, "【项目 Skill 指引】"+truncateForImagePrompt(hints, 240))
	}
	if visual := strings.TrimSpace(in.ProjectVisualPrompt); visual != "" {
		lines = append(lines, "【项目视觉配置】"+truncateForImagePrompt(visual, 240))
	}
	lines = append(lines, "【固定约束】单帧分镜静帧；禁止文字、水印、分格、人物设定图拼贴。")
	if quality := strings.TrimSpace(modelQualitySuffix(in.ModelName, in.StylePreset)); quality != "" {
		lines = append(lines, "【模型质量词】"+truncateForImagePrompt(quality, 200))
	} else {
		lines = append(lines, "【模型质量词】当前模型不使用额外质量 tag（如 DALL·E）")
	}
	if neg := strings.TrimSpace(in.NegativePrompt); neg != "" {
		lines = append(lines, fmt.Sprintf("【负面提示词】共 %d 项（见下方 negative_prompt）", countNegativePromptItems(neg)))
	}
	refCount := len(in.ReferenceImageURLs)
	if refCount > 0 {
		refLine := fmt.Sprintf("【参考图】%d 张（人物/场景/道具资源图", refCount)
		if in.PrevImageURL != "" {
			refLine += "，含上一镜图片作视觉连贯"
		}
		refLine += "）"
		lines = append(lines, refLine)
	}
	if !in.HasCharacters {
		lines = append(lines, "【无人场景】自动补充环境/道具聚焦约束")
	}
	if vt := viewTypeLabelZh(in.LocationViewType); vt != "" && in.LocationViewType != "" && in.LocationViewType != LocationViewUnknown {
		lines = append(lines, "【空间视角】"+vt)
	}
	if note := strings.TrimSpace(in.SceneReferenceNote); note != "" {
		lines = append(lines, "【场景参考图】"+note)
	}
	if len(lines) == 0 {
		return "暂无自动补充项"
	}
	return strings.Join(lines, "\n")
}
