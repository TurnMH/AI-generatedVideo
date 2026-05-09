package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/autovideo/character-service/internal/stylepreset"
)

type projectVisualProfile struct {
	ScriptText        string
	StyleTags         []string
	StoryboardConfig  map[string]interface{}
	ProjectType       string
	ProjectTitle      string
	ProjectDesciption string
}

func fetchProjectVisualProfile(ctx context.Context, projectServiceURL string, projectID uint64, authToken string) (*projectVisualProfile, error) {
	url := fmt.Sprintf("%s/api/v1/projects/%d", strings.TrimRight(projectServiceURL, "/"), projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call project-service: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("project-service %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			Title            string                 `json:"title"`
			Description      string                 `json:"description"`
			ProjectType      string                 `json:"project_type"`
			ScriptText       string                 `json:"script_text"`
			StyleTags        []string               `json:"style_tags"`
			StoryboardConfig map[string]interface{} `json:"storyboard_config"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse project response: %w", err)
	}

	return &projectVisualProfile{
		ScriptText:        result.Data.ScriptText,
		StyleTags:         result.Data.StyleTags,
		StoryboardConfig:  result.Data.StoryboardConfig,
		ProjectType:       result.Data.ProjectType,
		ProjectTitle:      result.Data.Title,
		ProjectDesciption: result.Data.Description,
	}, nil
}

func buildProjectVisualHint(profile *projectVisualProfile) string {
	if profile == nil {
		return ""
	}
	return buildProjectVisualHintFromFields(profile.StyleTags, profile.StoryboardConfig)
}

// buildProjectVisualHintForAsset is like buildProjectVisualHint but filters and reorders
// live-action context cues based on asset type so image generators receive the most
// important context tokens first:
//   - character: era+ethnicity placed FIRST (highest weight), no region/场景
//   - prop/item:  era placed FIRST (period style), no region, no ethnicity
//   - scene/location: region placed FIRST (dominant context), then era, no ethnicity
func buildProjectVisualHintForAsset(profile *projectVisualProfile, assetType string) string {
	if profile == nil {
		return ""
	}
	switch {
	case isCharacterAssetType(assetType):
		// Era+ethnicity come first so image generators weight them highest.
		// Region/场景 is intentionally excluded — character sheets use neutral backdrops.
		return buildCharacterVisualHint(profile.StyleTags, profile.StoryboardConfig)
	case isPropAssetType(assetType):
		// Era comes first for props — period/cultural style is the dominant visual signal.
		// Region is omitted (objects are context-agnostic); ethnicity is irrelevant.
		return buildPropVisualHint(profile.StyleTags, profile.StoryboardConfig)
	case isSceneAssetType(assetType):
		// Region comes first for scenes — the environment is the dominant visual signal.
		// Era follows; ethnicity is omitted.
		return buildSceneVisualHint(profile.StyleTags, profile.StoryboardConfig)
	default:
		return buildProjectVisualHintFromFields(profile.StyleTags, profile.StoryboardConfig)
	}
}

// buildCharacterVisualHint builds the visual hint for character reference sheets with
// era, ethnicity, and region placed FIRST — before visual style cues — so that image
// generators assign them maximum token weight.
//
// Era/ethnicity/region wardrobe cues are applied to ALL style presets (live-action
// AND anime): a Tang-dynasty anime character must still wear Tang-era garments, so
// period accuracy has to flow through regardless of render style. The scene/architecture
// facet of region is intentionally omitted here (handled by buildSceneVisualHint) —
// only the character-facing cultural styling cue is injected.
func buildCharacterVisualHint(styleTags []string, storyboardConfig map[string]interface{}) string {
	stylePreset := strings.TrimSpace(stringValue(storyboardConfig["style_preset"]))
	parts := make([]string, 0, 6)

	// 1. Era + ethnicity + region cultural styling FIRST — applies to every render
	//    style so period/geographical accuracy is preserved in anime and live-action.
	era := stringValue(storyboardConfig["era"])
	ethnicity := stringValue(storyboardConfig["ethnicity"])
	region := stringValue(storyboardConfig["region"])
	if cue := visualCueForCharacterContext(era, ethnicity, region); cue != "" {
		parts = append(parts, cue)
	}

	// 2. Visual style tone.
	if cue := visualCueForStylePreset(stylePreset); cue != "" {
		parts = append(parts, cue)
	}

	// 3. Additional style tags.
	for _, tag := range styleTags {
		if cue := visualCueForStyleTag(tag); cue != "" {
			parts = append(parts, cue)
		}
	}

	// 4. Motion mode.
	if cue := visualCueForMotionMode(strings.TrimSpace(stringValue(storyboardConfig["motion_mode"]))); cue != "" {
		parts = append(parts, cue)
	}

	parts = dedupeVisualCues(parts)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "；")
}

// isPropAssetType reports whether assetType is a prop or item type.
func isPropAssetType(assetType string) bool {
	switch strings.TrimSpace(strings.ToLower(assetType)) {
	case "prop", "道具", "item", "物品":
		return true
	}
	return false
}

// isSceneAssetType reports whether assetType is a scene or location type.
func isSceneAssetType(assetType string) bool {
	switch strings.TrimSpace(strings.ToLower(assetType)) {
	case "scene", "场景", "location", "地点", "环境":
		return true
	}
	return false
}

// isPropOrSceneAssetType reports whether assetType is a non-character type
// (prop / item / scene / location / image) that should not receive person-appearance descriptors.
func isPropOrSceneAssetType(assetType string) bool {
	return isPropAssetType(assetType) || isSceneAssetType(assetType) || strings.TrimSpace(strings.ToLower(assetType)) == "image"
}

// buildPropVisualHint builds the visual hint for prop/item reference images.
// Era is placed FIRST — the object's cultural period is the dominant visual signal.
// Region and ethnicity are omitted; objects are context-agnostic.
func buildPropVisualHint(styleTags []string, storyboardConfig map[string]interface{}) string {
	stylePreset := strings.TrimSpace(stringValue(storyboardConfig["style_preset"]))
	canonical := stylepreset.Canonical(stylePreset)
	parts := make([]string, 0, 5)

	// 1. Era FIRST: period/cultural style is the primary signal for prop design.
	if canonical == "live-action-film" || canonical == "live-action-short" {
		era := stringValue(storyboardConfig["era"])
		if cue := visualCueForPropContext(era); cue != "" {
			parts = append(parts, cue)
		}
	}

	// 2. Visual style tone.
	if cue := visualCueForStylePreset(stylePreset); cue != "" {
		parts = append(parts, cue)
	}

	// 3. Additional style tags.
	for _, tag := range styleTags {
		if cue := visualCueForStyleTag(tag); cue != "" {
			parts = append(parts, cue)
		}
	}

	// 4. Motion mode.
	if cue := visualCueForMotionMode(strings.TrimSpace(stringValue(storyboardConfig["motion_mode"]))); cue != "" {
		parts = append(parts, cue)
	}

	parts = dedupeVisualCues(parts)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "；")
}

// buildSceneVisualHint builds the visual hint for scene/location reference images.
// Region is placed FIRST — the environment type is the dominant visual signal for scenes.
// Era follows; ethnicity is omitted.
func buildSceneVisualHint(styleTags []string, storyboardConfig map[string]interface{}) string {
	stylePreset := strings.TrimSpace(stringValue(storyboardConfig["style_preset"]))
	canonical := stylepreset.Canonical(stylePreset)
	parts := make([]string, 0, 5)

	// 1. Region + era FIRST: environment and period architecture define the scene's look.
	if canonical == "live-action-film" || canonical == "live-action-short" {
		region := stringValue(storyboardConfig["region"])
		era := stringValue(storyboardConfig["era"])
		if cue := visualCueForSceneContext(region, era); cue != "" {
			parts = append(parts, cue)
		}
	}

	// 2. Visual style tone.
	if cue := visualCueForStylePreset(stylePreset); cue != "" {
		parts = append(parts, cue)
	}

	// 3. Additional style tags.
	for _, tag := range styleTags {
		if cue := visualCueForStyleTag(tag); cue != "" {
			parts = append(parts, cue)
		}
	}

	// 4. Motion mode.
	if cue := visualCueForMotionMode(strings.TrimSpace(stringValue(storyboardConfig["motion_mode"]))); cue != "" {
		parts = append(parts, cue)
	}

	parts = dedupeVisualCues(parts)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "；")
}

func buildProjectVisualHintFromFields(styleTags []string, storyboardConfig map[string]interface{}) string {
	return buildProjectVisualHintFromFieldsFiltered(styleTags, storyboardConfig, false, false)
}

// buildProjectVisualHintFromFieldsFiltered builds the visual hint with optional suppression
// of individual live-action context sections:
//
//	skipRegion    — omit the region/场景 descriptor (used for character reference sheets)
//	skipEthnicity — omit the ethnicity/人物 descriptor (used for prop/item/scene assets)
func buildProjectVisualHintFromFieldsFiltered(styleTags []string, storyboardConfig map[string]interface{}, skipRegion, skipEthnicity bool) string {
	stylePreset := strings.TrimSpace(stringValue(storyboardConfig["style_preset"]))
	parts := make([]string, 0, 5)
	if cue := visualCueForStylePreset(stylePreset); cue != "" {
		parts = append(parts, cue)
	}
	for _, tag := range styleTags {
		if cue := visualCueForStyleTag(tag); cue != "" {
			parts = append(parts, cue)
		}
	}
	if cue := visualCueForMotionMode(strings.TrimSpace(stringValue(storyboardConfig["motion_mode"]))); cue != "" {
		parts = append(parts, cue)
	}
	// Append live-action context (region/era/ethnicity) only for live-action styles.
	canonical := stylepreset.Canonical(stylePreset)
	if canonical == "live-action-film" || canonical == "live-action-short" {
		region := ""
		if !skipRegion {
			region = stringValue(storyboardConfig["region"])
		}
		era := stringValue(storyboardConfig["era"])
		ethnicity := ""
		if !skipEthnicity {
			ethnicity = stringValue(storyboardConfig["ethnicity"])
		}
		if cue := visualCueForLiveActionContext(region, era, ethnicity); cue != "" {
			parts = append(parts, cue)
		}
	}
	parts = dedupeVisualCues(parts)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "；")
}

func appendProjectVisualHint(text, visualHint string) string {
	trimmedText := strings.TrimSpace(text)
	trimmedHint := strings.TrimSpace(visualHint)
	if trimmedHint == "" || strings.Contains(trimmedText, trimmedHint) {
		return trimmedText
	}
	if trimmedText == "" {
		return "项目视觉基调：" + trimmedHint
	}
	return trimmedText + "\n视觉基调：" + trimmedHint
}

func appendProjectVisualHintInline(text, visualHint string) string {
	trimmedText := strings.TrimSpace(text)
	trimmedHint := strings.TrimSpace(visualHint)
	if trimmedHint == "" || strings.Contains(trimmedText, trimmedHint) {
		return trimmedText
	}
	if trimmedText == "" {
		return "项目视觉基调：" + trimmedHint
	}
	return trimmedText + " 项目视觉基调：" + trimmedHint
}

func projectImageStylePreset(profile *projectVisualProfile) string {
	if profile == nil {
		return "anime-2d"
	}
	return stylepreset.Canonical(strings.TrimSpace(stringValue(profile.StoryboardConfig["style_preset"])))
}

func buildProjectImageNegativePrompt(profile *projectVisualProfile, assetType string) string {
	parts := []string{
		// Universal quality failures
		"low quality", "blurry", "out of focus", "low detail", "low resolution",
		"jpeg artifacts", "compression artifacts", "pixelated", "noisy image",
		"overexposed", "underexposed", "washed out",
		// Universal overlays / branding
		"text", "watermark", "logo", "subtitle", "caption",
		"frame border", "collage", "split screen",
		// Universal anatomy errors
		"bad anatomy", "deformed hands", "extra fingers", "extra limbs", "cropped body",
		"duplicate subject",
	}

	switch stylepreset.Canonical(projectImageStylePreset(profile)) {
	case "anime-2d", "anime-3d":
		parts = append(parts,
			"photorealistic skin", "live-action photography", "raw camera snapshot",
			"realistic photo", "hyperrealistic",
			// Anime-specific artifacts to suppress
			"motion blur", "low frame rate blur", "oversaturated colors", "color bleeding",
			"3D CGI render", "video game render", "photo filter effect",
			"low resolution anime", "poorly drawn", "bad proportions",
		)
	case "live-action-film", "live-action-short":
		parts = append(parts,
			// Anti-stylisation
			"anime", "cartoon", "illustration", "cel shading", "chibi",
			"comic book", "manga", "2D drawing", "painted artwork",
			"digital painting", "concept art illustration",
			// Anti-CGI / plastic
			"plastic CGI skin", "airbrushed skin", "skin smoothing filter",
			"glossy plastic look", "video game render", "3D CGI render",
			"toon shading", "stylized proportions",
			// Anti-text/overlay
			"text overlay", "subtitle", "watermark", "logo",
			// Chromatic / exposure artifacts
			"chromatic aberration", "lens flare", "vignette", "overprocessed",
		)
	}

	normalizedType := strings.TrimSpace(strings.ToLower(assetType))
	canonical := stylepreset.Canonical(projectImageStylePreset(profile))
	isLiveAction := canonical == stylepreset.LiveActionFilm || canonical == stylepreset.LiveActionShort

	switch normalizedType {
	case "character", "角色", "人物":
		parts = append(parts,
			"crowd", "multiple people", "occluded face", "tiny distant figure",
			// 4-panel layout failure modes (for turnaround reference sheets)
			"(2x2 grid:1.5)", "(4x1 vertical stack:1.4)", "(single pose only:1.5)",
			"(missing close-up panel:1.4)", "(missing back view:1.4)",
			"(different characters in different panels:1.5)",
			"(inconsistent costume across panels:1.4)",
			"(inconsistent body proportions:1.4)",
			"panel number labels", "caption under panel",
		)
		if isLiveAction {
			parts = append(parts,
				// Face / expression defects
				"warped face", "asymmetrical eyes", "bad teeth", "unnatural expression",
				"lazy eye", "melting face",
				// Hand / finger defects
				"bad hands", "extra fingers", "fused fingers", "missing fingers",
				"extra limbs", "floating limbs",
				// Body completeness — reference sheets must show full body
				"(floating torso:1.5)", "(cut off at waist:1.5)", "(missing legs:1.5)",
				"(headless:1.5)", "(not full body:1.5)", "(cropped at knees:1.5)",
			)
		}
	case "scene", "场景":
		parts = append(parts,
			"person", "people", "character", "human figure", "human body",
			"face", "portrait", "crowd", "silhouette of person",
			"dominant foreground portrait", "close-up face",
		)
		if isLiveAction {
			parts = append(parts,
				"oversaturated", "HDR tone mapping", "artificially vivid colors",
				"flat lighting", "studio backdrop", "white background",
			)
		}
	case "prop", "道具", "item", "物品":
		parts = append(parts,
			"person", "people", "character", "human figure", "human body",
			"face", "portrait", "hands holding object", "human hands",
			"multiple props", "human portrait", "cluttered background",
		)
		if isLiveAction {
			parts = append(parts,
				"(object levitating:1.5)", "(floating object:1.5)",
				"extreme close-up distortion", "fish-eye distortion",
				"inconsistent scale", "prop in use by character",
			)
		}
	}

	return strings.Join(dedupeVisualCues(parts), ", ")
}

func dedupeVisualCues(parts []string) []string {
	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func visualCueForStylePreset(stylePreset string) string {
	switch stylepreset.Canonical(stylePreset) {
	case "anime-2d":
		return "保持二维动漫叙事感、平面线条、插画化角色设计与番剧镜头表达（2D anime storytelling, clean line art, illustrated character design, TV-anime framing）"
	case "anime-3d":
		return "保持三维动漫角色体积感、材质层次和三渲二镜头表现（3D anime look, dimensional characters, stylized materials, toon-shaded cinematic depth）"
	case "live-action-film":
		return "真人电影感、真实大场景、强层次光影与电影构图，避免动漫卡通化（live-action film look, realistic environments, dramatic cinematic lighting, avoid anime/cartoon stylization）"
	case "live-action-short":
		return "真人短剧感、真实人物表演、近景对白戏、自然肤质与真实空间，避免动漫卡通化（live-action short drama, close-up performance, natural skin texture, grounded real-world sets）"
	default:
		return ""
	}
}

func visualCueForStyleTag(tag string) string {
	switch strings.TrimSpace(tag) {
	case "写实":
		return "整体保持真实材质、自然肤感和非卡通化表达（realistic textures, natural skin detail）"
	case "纪录片":
		return "优先真实场景、自然光与纪实视角（documentary tone, natural light, real locations）"
	case "广告感":
		return "突出商业级质感、干净布光与高端成片感（commercial polish, premium lighting）"
	case "欧美电影感", "美剧感":
		return "参考真人影视构图、真实场景与电影级光影层次（live-action cinematic framing, real-world sets）"
	case "都市夜景":
		return "保留真实都市夜景、街道霓虹与人物环境关系（real city night scenes, practical lighting）"
	case "复古胶片":
		return "加入真实胶片颗粒、复古色温与年代空间质感（film grain, vintage palette）"
	default:
		return ""
	}
}

// buildCharacterStylePrefix returns leading quality/style tags for a character-sheet prompt.
// These are prepended to the tag-based prompt produced by composeAssetImagePrompt so that
// style-specific tokens appear first, where image models weight them most heavily.
func buildCharacterStylePrefix(stylePreset string) string {
	switch stylepreset.Canonical(stylePreset) {
	case "live-action-film":
		return strings.Join([]string{
			"(RAW photo:1.4)", "(photorealistic:1.5)", "(hyperrealistic:1.3)",
			"professional DSLR full-frame photography, prime lens",
			"full-body character reference sheet photography",
			"cinematic three-point studio lighting", "cinematic color grade",
			"no cartoon", "no anime", "no illustration",
		}, ", ")
	case "live-action-short":
		return strings.Join([]string{
			"(RAW photo:1.3)", "(photorealistic:1.4)",
			"professional DSLR photography, natural window light with studio fill",
			"full-body character reference sheet", "warm natural tones",
			"no cartoon", "no anime", "no illustration",
		}, ", ")
	case "anime-2d":
		return "(masterpiece:1.5), 高质量动漫插画, 角色设定图, clean line art, flat color"
	case "anime-3d":
		return "(masterpiece:1.5), 3D动漫角色, 角色设定图, toon render, volumetric shading"
	default:
		return "高质量, 极致细节"
	}
}

// isCharacterAssetType reports whether the asset type string represents a character.
func isCharacterAssetType(assetType string) bool {
	switch strings.TrimSpace(strings.ToLower(assetType)) {
	case "character", "角色", "人物":
		return true
	}
	return false
}

func visualCueForMotionMode(motionMode string) string {
	switch motionMode {
	case "gentle":
		return "镜头气质偏柔和克制、生活流与自然观察（gentle pacing, restrained framing）"
	case "dynamic":
		return "镜头气质偏动感张力、动作线更明确（dynamic energy, stronger motion cues）"
	case "cinematic":
		return "镜头气质偏电影化、强调景别变化与银幕感（cinematic pacing, theatrical framing）"
	default:
		return ""
	}
}

// visualCueForLiveActionContext builds a rich visual prompt cue from the optional
// live-action reference fields (region, era, ethnicity) stored in storyboard_config.
// Each field is expanded into detailed descriptors so that image generators produce
// culturally accurate environments, costumes, and character appearances.
// Returns "" when all fields are empty.
func visualCueForLiveActionContext(region, era, ethnicity string) string {
	region = strings.TrimSpace(region)
	era = strings.TrimSpace(era)
	ethnicity = strings.TrimSpace(ethnicity)
	if region == "" && era == "" && ethnicity == "" {
		return ""
	}
	parts := make([]string, 0, 3)
	if region != "" {
		parts = append(parts, expandRegionVisualCueZH(region))
	}
	if era != "" {
		parts = append(parts, expandEraVisualCueZH(era))
	}
	if ethnicity != "" {
		parts = append(parts, expandEthnicityVisualCueZH(ethnicity))
	}
	return strings.Join(parts, "；")
}

// visualCueForCharacterContext builds a character-focused live-action cue that includes
// era-wardrobe (no architecture, no props) plus ethnicity, plus a region cultural styling
// cue that is character-specific (no architecture). Region is intentionally narrowed to
// character styling so it doesn't leak scene-architecture into the character description.
func visualCueForCharacterContext(era, ethnicity, region string) string {
	era = strings.TrimSpace(era)
	ethnicity = strings.TrimSpace(ethnicity)
	region = strings.TrimSpace(region)
	if era == "" && ethnicity == "" && region == "" {
		return ""
	}
	parts := make([]string, 0, 3)
	if era != "" {
		parts = append(parts, expandEraWardrobeCueZH(era))
	}
	if ethnicity != "" {
		parts = append(parts, expandEthnicityVisualCueZH(ethnicity))
	}
	if region != "" {
		if cue := expandRegionCharacterCueZH(region); cue != "" {
			parts = append(parts, cue)
		}
	}
	return strings.Join(parts, "；")
}

// visualCueForSceneContext builds a scene-focused live-action cue: region plus era
// architecture only (no wardrobe, no props, no ethnicity).
func visualCueForSceneContext(region, era string) string {
	region = strings.TrimSpace(region)
	era = strings.TrimSpace(era)
	if region == "" && era == "" {
		return ""
	}
	parts := make([]string, 0, 2)
	if region != "" {
		parts = append(parts, expandRegionVisualCueZH(region))
	}
	if era != "" {
		parts = append(parts, expandEraArchCueZH(era))
	}
	return strings.Join(parts, "；")
}

// visualCueForPropContext builds a prop-focused live-action cue: era props/crafts only
// (no wardrobe, no architecture, no ethnicity, no region).
func visualCueForPropContext(era string) string {
	era = strings.TrimSpace(era)
	if era == "" {
		return ""
	}
	return expandEraPropsCueZH(era)
}

// liveActionVisualKeyword reports whether s (already lower-cased) contains any of subs.
func liveActionVisualKeyword(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// expandRegionVisualCueZH converts a free-text region/country value into a rich
// Chinese + English visual descriptor for image generation models.
func expandRegionVisualCueZH(region string) string {
	lower := strings.ToLower(region)
	switch {
	case liveActionVisualKeyword(lower, "古代中国", "ancient china"):
		return "场景：古代中国宫廷环境，高大木构宫殿、红漆圆柱、石雕牌坊、青石铺地庭院、垂帘红绸旗幡、铜铸宫灯（ancient Chinese imperial setting: timber palace halls, vermilion columns, stone archways, stone-paved courtyards, silk banners）"
	case liveActionVisualKeyword(lower, "中国", "china", "chinese"):
		return "场景：真实中国环境，传统建筑飞檐翘角、月亮门、红漆木柱、石庭院、灯笼；或现代中国城市高密度楼群、霓虹汉字招牌、街市集市（authentic Chinese setting: upturned eave rooftops, moon gates, silk lanterns; or modern Chinese cityscape with high-rises, Chinese-character neon signage, street markets）"
	case liveActionVisualKeyword(lower, "日本", "japan", "japanese"):
		return "场景：真实日本环境，木质鸟居、石砾庭园、和纸障子屏风、榻榻米内室、樱花树冠；或现代日本都市便利店招牌、自动贩卖机、多层密集霓虹店面（authentic Japanese setting: torii gates, stone gardens, shoji screens, tatami interiors, cherry blossoms; or modern neon urban district）"
	case liveActionVisualKeyword(lower, "韩国", "korea", "korean"):
		return "场景：真实韩国环境，韩屋曲线瓦屋顶村落、青瓷器皿、韩纸灯笼；或现代韩国都市韩文霓虹招牌、密集公寓楼群、路边炸鸡街食（authentic Korean setting: hanok curved-tile rooftop village, celadon ceramics; or modern Hangul neon signage, dense apartments）"
	case liveActionVisualKeyword(lower, "英国", "england", "britain", "british", "uk"):
		return "场景：英国环境，维多利亚红砖排屋、铸铁煤气灯柱、乔治亚石砌立面、雨中鹅卵石街道、传统酒吧招牌、双层红色公共汽车（British setting: Victorian red-brick terraces, gas lampposts, rainy cobblestone streets, double-decker buses）"
	case liveActionVisualKeyword(lower, "法国", "france", "paris", "french"):
		return "场景：法式巴黎环境，奥斯曼米色石砌公寓、锻铁阳台、马赛克地砖咖啡馆、梧桐大道、温暖金色光线、鲜花集市（French Parisian setting: Haussmann stone apartments, wrought-iron balconies, mosaic cafe floors, tree-lined boulevards）"
	case liveActionVisualKeyword(lower, "欧洲", "europe", "european"):
		return "场景：欧洲环境，哥特或巴洛克石砌建筑、教堂尖塔、鹅卵石广场、锻铁阳台、暖色石灰石立面、露天咖啡大道（European setting: Gothic or Baroque stone architecture, cobblestone plazas, ornate spires, cafe-lined boulevards）"
	case liveActionVisualKeyword(lower, "美国", "america", "american", "usa"):
		return "场景：美国环境，宽阔郊区街道、木板条或红砖住宅、复古镀铬餐车、摩天楼峡谷、霓虹路边汽车旅馆招牌（American setting: suburban streets with clapboard houses, chrome diners, skyscraper canyons, neon roadside signs）"
	case liveActionVisualKeyword(lower, "印度", "india", "indian"):
		return "场景：印度环境，莫卧儿或达罗毗荼雕花石砌神庙、香料纺织集市色彩饱和、嘟嘟车、热带金色阳光、焚香薄雾（Indian setting: Mughal carved stone architecture, vibrant bazaars, autorickshaws, tropical golden light）"
	case liveActionVisualKeyword(lower, "中东", "middle east", "arab", "阿拉伯"):
		return "场景：中东环境，伊斯兰几何彩砖建筑、马蹄形拱门、沙色土砖墙、椰枣树与沙漠景观、铜灯挂顶集市（Middle Eastern setting: Islamic tile mosaic architecture, horseshoe arches, mudbrick walls, desert palms, bazaar brass lanterns）"
	case liveActionVisualKeyword(lower, "非洲", "africa", "african"):
		return "场景：非洲环境，赭红色土砌建筑、热带稀树草原宽冠金合欢树、尘金暖光；或多彩西非市集、鲜艳花布商贩摊位（African setting: terracotta earthen architecture, savanna acacia trees, dust-gold light; or vibrant market with patterned fabric stalls）"
	case liveActionVisualKeyword(lower, "古罗马", "roman", "rome"):
		return "场景：古罗马环境，白大理石圆柱与凯旋门、方格马赛克铺地庭院、列柱广场、赭红屋瓦别墅、油灯壁烛（ancient Roman setting: marble columns, triumphal arches, mosaic floors, colonnaded forum, oil-lamp sconces）"
	case liveActionVisualKeyword(lower, "东南亚", "southeast asia", "thailand", "泰国", "越南", "vietnam"):
		return "场景：东南亚热带环境，鎏金佛塔尖顶、木质高脚屋、茂密丛林绿荫、水上集市船只、莲花与兰花装饰（Southeast Asian setting: gilded temple spires, wooden stilt houses, jungle canopy, floating market boats）"
	case liveActionVisualKeyword(lower, "俄罗斯", "russia", "russian"):
		return "场景：俄罗斯环境，洋葱顶大教堂剪影、苏式建构主义混凝土楼群、白桦林灰色冬日天空、木制达恰小屋（Russian setting: onion-dome cathedrals, Soviet concrete blocks, birch forest under winter skies, wooden dacha cottages）"
	default:
		return fmt.Sprintf("场景：%s地域环境，具有该地区文化特色的建筑风格、本土材料与施工工艺、地方特有道具与环境细节，画面一眼能辨识为%s地区（Setting: %s environment with culturally authentic architecture, indigenous materials, and region-specific environmental details）", region, region, region)
	}
}

// expandRegionCharacterCueZH returns a character-focused cultural styling cue for a
// region. Unlike expandRegionVisualCueZH which describes architecture/scene, this cue
// only injects region-specific hairstyle, accessory, and costume signals relevant to
// character appearance. It is appended as a LOW-weight supplement to character hints
// (era wardrobe remains dominant). Returns "" when the region has no distinct character-
// level styling worth highlighting beyond ethnicity.
func expandRegionCharacterCueZH(region string) string {
	lower := strings.ToLower(region)
	switch {
	case liveActionVisualKeyword(lower, "古代中国", "ancient china"):
		return "地域造型：古代中国发饰丝绸纹样与玉石配件，汉族传统妆容（ancient Chinese regional styling: silk-patterned garments, jade ornaments, traditional Han cosmetics）"
	case liveActionVisualKeyword(lower, "中国", "china", "chinese"):
		return "地域造型：中式发饰与绸缎纹样，或现代中国都市潮流造型（Chinese regional styling: traditional silk patterns and hair ornaments, or modern Chinese urban fashion）"
	case liveActionVisualKeyword(lower, "日本", "japan", "japanese"):
		return "地域造型：日式和服腰带与木屐、簪花盘发，或现代日系街头潮流（Japanese regional styling: kimono obi and geta sandals, kanzashi hairpins, or modern Japanese street fashion cues）"
	case liveActionVisualKeyword(lower, "韩国", "korea", "korean"):
		return "地域造型：韩服赤古里与裙摆结带、韩式淡妆与自然妆容，或现代韩系时尚剪裁（Korean regional styling: hanbok jeogori with ribbon ties, soft K-beauty makeup, or modern K-fashion tailoring）"
	case liveActionVisualKeyword(lower, "英国", "england", "britain", "british", "uk"):
		return "地域造型：英伦绅士/淑女风格剪裁，羊毛呢料、格纹围巾与皮手套（British regional styling: tweeds and tartans, wool coats, leather gloves）"
	case liveActionVisualKeyword(lower, "法国", "france", "paris", "french"):
		return "地域造型：法式优雅剪裁、丝质围巾与精致小配饰，淡妆裸唇（French regional styling: refined silhouettes, silk scarves, understated nude makeup）"
	case liveActionVisualKeyword(lower, "欧洲", "europe", "european"):
		return "地域造型：欧式量身剪裁大衣、皮革配饰、欧洲发型审美（European regional styling: tailored coats, leather accents, European hairstyling）"
	case liveActionVisualKeyword(lower, "美国", "america", "american", "usa"):
		return "地域造型：美式休闲或工装风格、牛仔棉质面料、实用配饰（American regional styling: casual/workwear aesthetic, denim and cotton, practical accessories）"
	case liveActionVisualKeyword(lower, "印度", "india", "indian"):
		return "地域造型：纱丽/库尔塔刺绣纹样、鼻环与手镯、印度传统妆容（Indian regional styling: sari/kurta embroidery, nose ring and bangles, traditional Indian cosmetics）"
	case liveActionVisualKeyword(lower, "中东", "middle east", "arab", "阿拉伯"):
		return "地域造型：头巾/长袍剪裁、金线刺绣与铜饰、中东传统妆饰（Middle Eastern regional styling: keffiyeh or abaya, gold embroidery, brass jewellery）"
	case liveActionVisualKeyword(lower, "非洲", "africa", "african"):
		return "地域造型：非洲编发造型、蜡染花布与珠串配饰（African regional styling: braided or natural hair, wax-print fabrics, beaded accessories）"
	case liveActionVisualKeyword(lower, "东南亚", "southeast asia", "thailand", "泰国", "越南", "vietnam"):
		return "地域造型：东南亚丝绸纱笼、传统发簪与花饰、温润湿热的自然妆感（Southeast Asian regional styling: sarongs/silk wraps, floral hairpieces, tropical natural makeup）"
	case liveActionVisualKeyword(lower, "俄罗斯", "russia", "russian"):
		return "地域造型：俄式毛皮帽与厚呢大衣，刺绣民族花纹（Russian regional styling: ushanka fur hats, heavy wool coats, embroidered folk motifs）"
	default:
		return fmt.Sprintf("地域造型：%s地区人物的典型服饰纹样、发饰与妆容习惯（%s regional character styling: distinctive garment patterns, hair ornaments, and grooming conventions）", region, region)
	}
}

// eraFacetsZH returns era-specific visual facets split by asset usage:
//   wardrobe — period clothing/hairstyle for characters
//   arch     — period architecture/environment for scenes
//   props    — period objects/materials/crafts for props
// Each facet is a bilingual ZH+EN descriptor; facets are focused so that a
// character description never inherits palace-architecture text, a scene never
// inherits wardrobe text, and a prop never inherits character/scene text. This
// prevents bleed-through like "letter description" containing "palace halls".
func eraFacetsZH(era string) (wardrobe, arch, props string) {
	lower := strings.ToLower(era)
	switch {
	case liveActionVisualKeyword(lower, "唐朝", "唐代", "tang dynasty", "tang"):
		wardrobe = "时代（唐朝 618—907年）服饰：男子圆领窄/宽袖袍衫，腰系蹀躞带，头戴幞头软脚或硬脚，足蹬六合靴；贵族男子可加锦绣半臂与玉带銙，文官佩鱼袋。女子齐胸襦裙或袒领襦裙，帔帛披肩飘垂，足履翘头锦履；贵族女子高髻（堕马髻、双环望仙髻、惊鹄髻），金步摇、花钿、斜红、面靥、朱唇小巧，珥珰耳饰；平民女子低髻素裙、麻布交领短襦。严格按性别和阶层选用，避免明清服饰混入。（Tang 618-907 garments: men in round-collar narrow/wide-sleeve pao robes, die-xie belt, puku/futou headdress, liuhe boots; nobles add brocade banbi half-sleeves and jade belt plaques. Women in chest-high ruqun or open-collar ruqun with pibo silk sash, upturned embroidered shoes; noble women wear elaborate high buns—duoma/shuanghuan/jinghu styles—with golden buyao hairpins, huadian forehead stamps, xiehong red strokes, mianye cheek dots, small rouged lips; commoners wear simple low buns and hemp garments）"
		arch = "时代（唐朝 618—907年）建筑：彩绘飞檐宫殿、木结构寺塔、青砖瓦院、丝绸之路市集氛围（Tang architecture: eave-rooftop painted palatial halls, timber pagodas, cosmopolitan market atmosphere）"
		props = "时代（唐朝 618—907年）器物：唐三彩陶器、金银錾花器、丝绸织锦、毛笔与黄麻纸卷轴、铜镜（Tang crafts: sancai pottery, silver repoussé, silk brocade, brush-and-hemp paper scrolls, bronze mirrors）"
	case liveActionVisualKeyword(lower, "宋朝", "宋代", "song dynasty"):
		wardrobe = "时代（宋朝 960—1279年）服饰：男子文人着直领对襟褙子或圆领襕衫，头戴东坡巾或簪花幞头，官员乌纱帽展脚长至两尺；脚着白绫袜与乌皮靴。女子上衣多为交领或直领对襟褙子，内着抹胸，下着百迭裙或宋裤，外披旋裙；发式流行同心髻、朝天髻或包髻，发间簪鲜花或绢花，淡雅薄妆、柳叶眉、樱唇；金银耳坠与玉佩贴身。整体清雅素淡，色调以月白、藕荷、秋香、茶绿为主，严禁大红大紫与明清厚重元素。（Song garments: scholar-gentlemen in straight-collar beizi or round-collar lanshan robes, dongpo-scarf or flower-pinned futou headdress, long-winged wushamao for officials, white silk socks and black leather boots; women in cross-collar or open-front beizi over mo-xiong bodice, baidie pleated skirt or Song trousers, xuanqun overskirt; hairstyles favour tongxin/chaotian/baoji buns with fresh or silk flowers, pale refined makeup, willow-leaf brows, cherry lips, jade pendants and subtle earrings; palette stays ink-wash pale with moon-white, lotus-mauve, autumn-honey, tea-green）"
		arch = "时代（宋朝 960—1279年）建筑：歇山飞檐木构、庭园回廊、汝窑青瓷陈设、水墨卷轴悬挂（Song architecture: timber halls with flying eaves, garden corridors, celadon and ink-wash ambience）"
		props = "时代（宋朝 960—1279年）器物：汝窑青瓷、斗笠茶盏、折枝花卉水墨、宣纸卷轴、漆器托盘（Song crafts: Ru-kiln celadon, tea bowls, ink-wash paintings, xuan paper scrolls, lacquered trays）"
	case liveActionVisualKeyword(lower, "明朝", "明代", "ming dynasty", "ming"):
		wardrobe = "时代（明朝 1368—1644年）服饰：男子交领右衽汉服与曳撒、道袍、贴里，腰系玉带或素丝绦；头戴四方平定巾、网巾、乌纱帽，官员补服前胸背后补子按品级（文禽武兽），脚蹬皂靴。女子上袄下裙（袄裙）或披风/比甲，交领或立领扣襻，里着抹胸马面裙配襕边，挂玉禁步；发式以牡丹头、桃心髻为主，簪花钿珠翠满头，淡妆柳眉朱唇；贵族女子缀明珠珊瑚头面，庶民仅素巾布钗。严格按性别选用，避免清代旗袍马褂与唐宋样式。（Ming garments: men in cross-collar right-lapel hanfu, yesa, daopao, tieli robes with jade or silk belts, square-flat-top headscarf or network cap, wushamao with rank-badges (birds for civil, beasts for military), black leather boots; women in short jacket + pleated skirt (aoqun) or cloak/bijia vest over mo-xiong and mamian pleated skirt with embroidered borders, jade chime-stone pendants; peony-head and peach-heart buns bristling with gilded kingfisher-feather hair ornaments, pale makeup, willow brows, vermilion lips; noble women add pearl-and-coral tousse, commoners wear plain scarves）"
		arch = "时代（明朝 1368—1644年）建筑：朱漆大型木构宫殿、雕花宫门、抄手游廊、琉璃瓦屋脊（Ming architecture: red-lacquered palatial halls, carved gates, glazed-tile rooftops）"
		props = "时代（明朝 1368—1644年）器物：青花瓷器、绸缎织锦、宣纸折叠书信与红印封缄、铜镜漆盒（Ming crafts: blue-and-white porcelain, silk brocade, folded xuan-paper letters with red wax/seal, lacquered boxes）"
	case liveActionVisualKeyword(lower, "清朝", "清代", "qing dynasty", "qing"):
		wardrobe = "时代（清朝 1644—1912年）服饰：男子长袍马褂、箭袖（马蹄袖），头顶薙发留辫垂至腰际，腰佩荷包与玉扳指；官员着补服与朝珠，戴花翎顶戴（按品级镶红蓝宝石），蹬朝靴；平民短打小褂、布鞋。满族女子旗装（长及脚踝的圆领大襟袍，下摆不开衩），外罩马甲，足穿花盆底鞋或木底鞋；头梳两把头（旗头板），高顶装饰绒花、绢花、珠翠、翡翠扁方；妆容重唇点、粉面、柳眉；汉族女子仍着袄裙或旗袍改良样式，缠足三寸金莲穿绣花弓鞋；皆饰长耳坠、项圈与手镯。严格按性别与满汉区别选用。（Qing garments: men in changshan + magua short riding jacket, horsehoof cuffs, shaved forehead with long queue to waist, silk purse and archer's jade thumb ring at belt; officials wear dragon-robe court vest, court beads, hat-finial with feather plume in graded ruby/sapphire, court boots. Manchu women wear floor-length round-collar qipao robes without side slits, over vest, on elevated flower-pot or wooden platform shoes; the liangbatou head-board is decorated with velvet flowers, silk blossoms, pearl and jade bianfang pins; makeup emphasises a small rouge lip dot, white face powder, willow brows. Han women keep aoqun or adapted qipao silhouettes with bound feet in lotus shoes）"
		arch = "时代（清朝 1644—1912年）建筑：金黄琉璃瓦大型宫殿、彩画梁枋、紫檀木家具厅堂、漆屏风屏风（Qing architecture: imperial yellow tile rooftops, painted beams, rosewood interiors, lacquer screens）"
		props = "时代（清朝 1644—1912年）器物：粉彩瓷、珐琅彩鼻烟壶、紫檀案几、朱砂红印奏折（Qing crafts: famille-rose porcelain, enamel snuff bottles, rosewood desks, vermilion-seal memorials）"
	case liveActionVisualKeyword(lower, "民国", "republic of china"):
		wardrobe = "时代（民国 1912—1949年）服饰：男子长衫马褂或中山装，文人戴圆框眼镜与礼帽，军人着北洋/国民革命军军装配皮带皮靴；西化青年穿修身西装、背心、皮鞋。女子改良立领旗袍（侧开衩至膝，月白、墨绿、胭脂红绸缎为主），外搭毛呢短外套或斗篷，足穿玛丽珍皮鞋；发式多学生头齐耳短发、指推波纹烫发或盘髻，妆容烟熏眉、红唇妆；名媛佩珍珠项链与手套，配绣花手帕与小羊皮手包；平民多粗布衣衫与布鞋。严格区分西式学生装与传统长衫的身份差。（Republican garments: men in changshan + magua or Zhongshan suit; scholars with round spectacles and fedora; soldiers in Beiyang/Nationalist uniform with belts and leather boots; westernised youth in slim three-piece suits. Women in updated stand-collar qipao with knee-high slit in moon-white/ink-green/rouge-red silk, paired with short wool jacket or cape and Mary-Jane shoes; hair in ear-length bob, finger-waves or low chignon; smoky brows and red lips; socialites add pearl ropes, gloves, embroidered handkerchief and kid-leather clutch; commoners wear coarse cotton）"
		arch = "时代（民国 1912—1949年）建筑：中西合璧骑楼、石库门里弄、青石板街巷与黄包车（Republican architecture: qilou arcades, shikumen lanes, cobblestone streets with rickshaws）"
		props = "时代（民国 1912—1949年）器物：手写钢笔信笺、机织邮票、老式留声机、黄铜煤油灯、搪瓷杯（Republican props: fountain-pen letters, postage stamps, gramophones, brass kerosene lamps, enamel mugs）"
	case liveActionVisualKeyword(lower, "元朝", "元代", "yuan dynasty"):
		wardrobe = "时代（元朝 1271—1368年）服饰：蒙古质孙服、毛皮镶边袍服、珍珠朝冠（Yuan garments: Mongol jisun robes, fur-trimmed court gowns）"
		arch = "时代（元朝 1271—1368年）建筑：中式木构与毡包并置、帐宫行营、青石方砖庭院（Yuan architecture: Chinese timber halls blended with Mongol yurts and ceremonial tent-palaces）"
		props = "时代（元朝 1271—1368年）器物：釉里红瓷、錾花铜马具、皮制壶囊、藏经木匣（Yuan crafts: underglaze-red porcelain, embossed bronze horse tack, leather flasks）"
	case liveActionVisualKeyword(lower, "战国", "warring states"):
		wardrobe = "时代（战国 475—221 BC）服饰：深衣交领袍服、铜甲漆皮革甲胄（Warring States garments: cross-collar deep robes, bronze-fitted lacquered armor）"
		arch = "时代（战国 475—221 BC）建筑：夯土城墙、木制望楼、列国都邑市肆（Warring States architecture: rammed-earth walls, wooden watchtowers）"
		props = "时代（战国 475—221 BC）器物：青铜礼器、漆木简牍、战车与戟戈（Warring States props: bronze ritual vessels, lacquered bamboo slips, war chariots）"
	case liveActionVisualKeyword(lower, "秦朝", "秦代", "qin dynasty", "qin"):
		wardrobe = "时代（秦朝 221—206 BC）服饰：兵马俑式青铜札甲、黑色深衣（Qin garments: terracotta-style bronze armor, black deep robes）"
		arch = "时代（秦朝 221—206 BC）建筑：夯土高台、石砌礼道、阿房宫式巍峨宫阙（Qin architecture: earthen platforms, stone ceremonial roads, monumental palace silhouettes）"
		props = "时代（秦朝 221—206 BC）器物：青铜印玺、帝国旌旗、木简诏书、竹制量器（Qin props: bronze seals, imperial banners, edict wooden slips）"
	case liveActionVisualKeyword(lower, "汉朝", "汉代", "han dynasty", "han"):
		wardrobe = "时代（汉朝 206 BC—220 AD）服饰：宽袖曳地汉服、交领右衽、男子幅巾女子堕马髻（Han garments: long-sleeve hanfu with cross-collar right lapel）"
		arch = "时代（汉朝 206 BC—220 AD）建筑：瓦顶土木宅院、宫阙阙楼、驿道邮亭（Han architecture: tile-roofed compounds, watchtower gatehouses, post-road pavilions）"
		props = "时代（汉朝 206 BC—220 AD）器物：漆木器皿、铜熏炉、竹简木牍、丝绸之路异域货品（Han props: lacquered wooden vessels, bronze incense burners, bamboo slips, Silk Road trade goods）"
	case liveActionVisualKeyword(lower, "中世纪", "medieval", "middle ages"):
		wardrobe = "时代（中世纪 5—15世纪）服饰：锁甲与板甲、农民麻布外袍、贵族丝绒披风（Medieval garments: chain mail and plate armor, peasant linen tunics, velvet noble cloaks）"
		arch = "时代（中世纪 5—15世纪）建筑：石砌城堡雉堞、哥特教堂尖拱、茅草屋村落（Medieval architecture: crenellated castles, Gothic arches, thatched villages）"
		props = "时代（中世纪 5—15世纪）器物：羊皮卷书信封蜡、纹章旗帜、铁制烛台、木匣（Medieval props: parchment letters with wax seals, heraldic banners, iron candlesticks）"
	case liveActionVisualKeyword(lower, "维多利亚", "victorian"):
		wardrobe = "时代（维多利亚 1837—1901）服饰：束腰裙撑长裙、礼服外套与高礼帽、蕾丝手套（Victorian garments: corseted bustle gowns, frock coats, top hats, lace gloves）"
		arch = "时代（维多利亚 1837—1901）建筑：铁玻璃工业建筑、煤气灯鹅卵石街道、繁复花卉壁纸室内（Victorian architecture: iron-and-glass industrial halls, gas-lit cobblestones, wallpapered parlours）"
		props = "时代（维多利亚 1837—1901）器物：手写羽毛笔信件、怀表、铜制油灯、瓷茶具（Victorian props: quill-pen letters, pocket watches, brass oil lamps, porcelain tea sets）"
	case liveActionVisualKeyword(lower, "二战", "world war", "ww2", "1940s", "四十年代"):
		wardrobe = "时代（二战 1940年代）服饰：军装制服、平民呢大衣与配给款连衣裙（WWII 1940s garments: military uniforms, utility dresses, double-breasted suits）"
		arch = "时代（二战 1940年代）建筑：弹坑废墟街道、工业砖厂、战时宣传海报墙（WWII 1940s architecture: rubble streets, brick factories, propaganda-poster walls）"
		props = "时代（二战 1940年代）器物：老式打字机、军邮信件、镀铬保险杠汽车、黑白照片（WWII 1940s props: typewriters, field-post letters, chrome-bumper cars, black-and-white photographs）"
	case liveActionVisualKeyword(lower, "1990", "九十年代", "90s"):
		wardrobe = "时代（1990年代）服饰：高腰牛仔裤、宽松风衣、运动休闲套装（1990s garments: high-waist denim, oversized windbreakers, athleisure）"
		arch = "时代（1990年代）建筑：荧光灯管室内、马赛克外墙办公楼、VHS 录像店招牌（1990s architecture: fluorescent-lit offices, mosaic facades, VHS rental storefronts）"
		props = "时代（1990年代）器物：阴极射线管电视、磁带随身听、大哥大手机、软盘（1990s props: CRT televisions, cassette Walkmans, brick cell phones, floppy disks）"
	case liveActionVisualKeyword(lower, "1980", "八十年代", "80s"):
		wardrobe = "时代（1980年代）服饰：霓虹色运动服、几何图案毛衣、夸张垫肩（1980s garments: neon athletic wear, geometric knitwear, shoulder-padded suits）"
		arch = "时代（1980年代）建筑：街机厅霓虹招牌、波普印花室内、早期商场大厅（1980s architecture: arcade halls, pop-print interiors, early mall atria）"
		props = "时代（1980年代）器物：录音机、显像管电视、街机与胶卷相机（1980s props: boombox radios, tube TVs, arcade cabinets, film cameras）"
	case liveActionVisualKeyword(lower, "未来", "future", "futuristic", "sci-fi", "科幻"):
		wardrobe = "时代（未来）服饰：贴身高科技合成纤维装、可穿戴发光模块（Futuristic garments: techwear with luminescent modules, sleek synthetic fibers）"
		arch = "时代（未来）建筑：哑光金属与碳纤维结构、高对比霓虹夜景（Futuristic architecture: matte metals and carbon fiber, neon night cityscape）"
		props = "时代（未来）器物：悬浮全息界面、无人机、精密几何金属器（Futuristic props: holographic displays, drones, geometric machined artifacts）"
	case liveActionVisualKeyword(lower, "现代", "modern", "contemporary", "当代", "现在"):
		wardrobe = "时代（当代）服饰：当季流行服装、常见休闲装与正装（Contemporary garments: current-season fashion, casual and business attire）"
		arch = "时代（当代）建筑：玻璃幕墙钢混建筑、电动汽车、数字屏幕广告（Contemporary architecture: glass-steel-concrete buildings, EVs, digital signage）"
		props = "时代（当代）器物：智能手机、笔记本电脑、数字屏幕、LED 灯具（Contemporary props: smartphones, laptops, digital screens, LED lighting）"
	case liveActionVisualKeyword(lower, "古代", "ancient", "古典", "classical"):
		wardrobe = "时代（古代）服饰：手缝纤维织物、天然染色、传统配饰（Ancient garments: hand-sewn fibers, natural dyes, period accessories）"
		arch = "时代（古代）建筑：手工建造、夯土或木构、油灯蜡烛照明（Ancient architecture: hand-built, earthen or timber, oil-lamp lit）"
		props = "时代（古代）器物：陶器、铜器、骨木器具、卷轴与简牍（Ancient props: pottery, bronze vessels, bone/wood utensils, scrolls and bamboo slips）"
	default:
		base := fmt.Sprintf("%s时期", era)
		wardrobe = fmt.Sprintf("时代（%s）服饰：服装廓形、发型、配饰均与%s高度吻合，无时代错乱元素（Era-accurate garments for %s）", era, base, era)
		arch = fmt.Sprintf("时代（%s）建筑：建筑风格、街道环境均与%s高度吻合（Era-accurate architecture for %s）", era, base, era)
		props = fmt.Sprintf("时代（%s）器物：道具、材质、科技水平均与%s高度吻合（Era-accurate props for %s）", era, base, era)
	}
	return
}

// expandEraVisualCueZH converts a free-text era/dynasty value into a detailed Chinese + English
// descriptor covering period-accurate clothing, architecture, props, and atmosphere.
// Used for storyboards where a single frame naturally covers all three facets.
func expandEraVisualCueZH(era string) string {
	wardrobe, arch, props := eraFacetsZH(era)
	return strings.Join([]string{wardrobe, arch, props}, "；")
}

// expandEraWardrobeCueZH returns only the wardrobe/hairstyle facet for character asset hints.
func expandEraWardrobeCueZH(era string) string {
	wardrobe, _, _ := eraFacetsZH(era)
	return wardrobe
}

// expandEraArchCueZH returns only the architecture/environment facet for scene asset hints.
func expandEraArchCueZH(era string) string {
	_, arch, _ := eraFacetsZH(era)
	return arch
}

// expandEraPropsCueZH returns only the props/crafts facet for prop asset hints.
func expandEraPropsCueZH(era string) string {
	_, _, props := eraFacetsZH(era)
	return props
}

// expandEthnicityVisualCueZH converts a free-text ethnicity value into a focused
// Chinese + English descriptor covering appearance, skin tone, and styling.
func expandEthnicityVisualCueZH(ethnicity string) string {
	lower := strings.ToLower(ethnicity)
	switch {
	case liveActionVisualKeyword(lower, "汉族", "汉人", "中国人", "东亚人", "east asian", "chinese"):
		return "人物：东亚/汉族外貌，直黑发，暖白至浅棕肤色，杏形眼，精致东亚五官比例，符合中国文化的发型服饰（East Asian/Chinese: straight black hair, warm ivory complexion, almond eyes, refined East Asian proportions）"
	case liveActionVisualKeyword(lower, "日本人", "japanese"):
		return "人物：日本东亚外貌，直黑或深棕发，白皙至浅肤色，精致东亚五官，日式发型服饰细节（Japanese: straight black hair, fair complexion, refined East Asian features, Japanese styling cues）"
	case liveActionVisualKeyword(lower, "韩国人", "korean"):
		return "人物：韩国东亚外貌，直黑发，白皙至浅肤色，光滑肌肤，当代韩式发型与造型（Korean: straight black hair, fair skin, smooth complexion, Korean fashion and grooming）"
	case liveActionVisualKeyword(lower, "白人", "欧洲人", "caucasian", "european", "western"):
		return "人物：高加索/欧洲外貌，发色从金发到深棕不等，白皙肤色冷暖色调各异，欧洲面部骨骼结构（Caucasian/European: varied hair from fair blond to dark brown, light complexion, European facial structure）"
	case liveActionVisualKeyword(lower, "非洲人", "黑人", "african", "black"):
		return "人物：非洲/黑人外貌，深色黑皮肤，自然卷发从紧卷到松卷，强健骨骼结构，非洲面部比例（African/Black: rich dark complexion, natural coiled-to-curly hair, strong bone structure, African facial proportions）"
	case liveActionVisualKeyword(lower, "印度人", "南亚", "indian", "south asian"):
		return "人物：南亚/印度外貌，暖中棕至深棕肤色，黑发，深色眼睛，南亚面部结构（South Asian/Indian: warm medium-to-deep brown skin, black hair, dark expressive eyes, South Asian facial structure）"
	case liveActionVisualKeyword(lower, "中东", "阿拉伯", "middle eastern", "arab"):
		return "人物：中东/阿拉伯外貌，橄榄至暖棕肤色，深色眼睛，深发，棱角分明的中东面部特征（Middle Eastern/Arab: olive to warm tan skin, dark eyes and hair, strong angular facial features）"
	case liveActionVisualKeyword(lower, "拉丁", "latino", "hispanic", "latin"):
		return "人物：拉丁裔外貌，暖橄榄至中棕肤色，深发，暖色皮肤底色，拉美面部比例（Latino/Hispanic: warm olive to medium-brown skin, dark hair, warm undertone, Latin American facial proportions）"
	case liveActionVisualKeyword(lower, "东南亚", "southeast asian"):
		return "人物：东南亚外貌，暖棕至中棕肤色，黑发，深色眼睛，东南亚面部结构（Southeast Asian: warm tan to medium-brown skin, black hair, dark eyes, Southeast Asian facial structure）"
	case liveActionVisualKeyword(lower, "少数民族", "民族", "ethnic minority"):
		return "人物：少数民族外貌，具有该民族特色的面部特征，传统民族服饰与饰品配件（ethnic minority: distinctive facial features with traditional ethnic costume and accessories）"
	default:
		return fmt.Sprintf("人物：%s外貌特征，面部特征、肤色、发型均具有民族真实性，服饰造型与%s文化背景一致（Characters: %s appearance with ethnically authentic facial features, skin tone, hair type, and culturally appropriate styling）", ethnicity, ethnicity, ethnicity)
	}
}
